package codegraph

import (
	"sort"

	"github.com/chasedputnam/pyra/internal/codeintel"
)

// Build constructs the code graph from one codeintel Map walk per root (or the
// scope subdirectory). It is a pure function of repository state. A root whose
// walk errors contributes nothing; the remaining roots still build.
func Build(ops *codeintel.Ops, roots []string, opts Options) (*Graph, error) {
	dirs := roots
	if opts.Scope != "" {
		dirs = []string{opts.Scope}
	}

	// Gather file maps deterministically (sorted by file, deduped across roots).
	var fileMaps []codeintel.FileMap
	seenFile := map[string]bool{}
	for _, d := range dirs {
		fms, err := ops.Map(d, "", "", false)
		if err != nil {
			continue // other roots proceed (REQ-704 / REQ-106)
		}
		for _, fm := range fms {
			if seenFile[fm.File] {
				continue
			}
			seenFile[fm.File] = true
			fileMaps = append(fileMaps, fm)
		}
	}
	sort.SliceStable(fileMaps, func(i, j int) bool { return fileMaps[i].File < fileMaps[j].File })

	g := &Graph{
		Symbols:   map[string]*SymbolNode{},
		In:        map[string][]string{},
		Files:     map[string][]string{},
		FileEdges: map[string][]string{},
	}

	// Pass 1: nodes, containment, and the name→definition index.
	nameIndex := map[string][]string{}
	for _, fm := range fileMaps {
		for _, e := range fm.Entries {
			if _, ok := g.Symbols[e.ID]; ok {
				continue
			}
			parent := ""
			if e.Parent != nil {
				parent = *e.Parent
			}
			lang := splitLang(e.ID)
			g.Symbols[e.ID] = &SymbolNode{
				ID: e.ID, Name: e.Name, Kind: e.Kind, File: fm.File,
				Lang: lang, Parent: parent, Exported: isExported(lang, e.Name, parent),
			}
			g.Files[fm.File] = append(g.Files[fm.File], e.ID)
			nameIndex[e.Name] = append(nameIndex[e.Name], e.ID)
		}
	}

	// Node cap: keep the first N symbol-ids in sorted order; prune the rest.
	g.Order = sortedKeys(g.Symbols)
	if opts.NodeCap > 0 && len(g.Order) > opts.NodeCap {
		keep := map[string]bool{}
		for _, id := range g.Order[:opts.NodeCap] {
			keep[id] = true
		}
		for id := range g.Symbols {
			if !keep[id] {
				delete(g.Symbols, id)
			}
		}
		g.Order = g.Order[:opts.NodeCap]
		g.Truncated = true
		// Prune containment to kept symbols.
		for f, ids := range g.Files {
			var kept []string
			for _, id := range ids {
				if keep[id] {
					kept = append(kept, id)
				}
			}
			if len(kept) == 0 {
				delete(g.Files, f)
			} else {
				g.Files[f] = kept
			}
		}
	}

	// Pass 2: reference edges (to every name match, self and pruned excluded).
	fileEdgeSet := map[string]map[string]bool{}
	for _, fm := range fileMaps {
		for _, e := range fm.Entries {
			src := g.Symbols[e.ID]
			if src == nil {
				continue // pruned by the cap
			}
			outSet := map[string]bool{}
			for _, refName := range e.References {
				for _, target := range nameIndex[refName] {
					if target == e.ID {
						continue
					}
					if _, ok := g.Symbols[target]; !ok {
						continue // pruned
					}
					outSet[target] = true
				}
			}
			src.Out = sortedSet(outSet)
			for _, t := range src.Out {
				g.In[t] = append(g.In[t], e.ID)
				// File edge (exclude self-loops).
				sf, tf := src.File, g.Symbols[t].File
				if sf != tf {
					if fileEdgeSet[sf] == nil {
						fileEdgeSet[sf] = map[string]bool{}
					}
					fileEdgeSet[sf][tf] = true
				}
			}
		}
	}

	// Finalize reverse edges and file edges deterministically.
	for t := range g.In {
		sort.Strings(g.In[t])
	}
	for f, set := range fileEdgeSet {
		g.FileEdges[f] = sortedSet(set)
	}
	for f := range g.Files {
		sort.Strings(g.Files[f])
	}

	return g, nil
}

func sortedKeys(m map[string]*SymbolNode) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedSet(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
