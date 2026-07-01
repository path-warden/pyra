package codeintel

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Ops is the shared operation surface both the CLI and MCP faces call. All
// operations are read-only and deterministic.
type Ops struct {
	eng  *Engine
	root string // confinement root for directory walks (see rel/confined)
}

// NewOps builds an Ops over an engine (nil = a fresh engine on the default
// registry). root is the working root used to compute relative paths and to
// confine directory traversal; "" means the current directory.
func NewOps(eng *Engine, root string) *Ops {
	if eng == nil {
		eng = NewEngine(nil)
	}
	if root == "" {
		root = "."
	}
	return &Ops{eng: eng, root: root}
}

// rel returns a repo-relative, slash-cleaned path for use in symbol-ids.
func (o *Ops) rel(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	base, err := filepath.Abs(o.root)
	if err != nil {
		return filepath.ToSlash(path)
	}
	if r, err := filepath.Rel(base, abs); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return filepath.ToSlash(path)
}

// Outline returns a file's definitions, filtered by kind, projected by detail
// tier (0 terse, 1 default, 2 full).
func (o *Ops) Outline(file, kind string, detail int) ([]map[string]any, error) {
	openPath, err := o.confinedPath(file)
	if err != nil {
		return nil, err
	}
	syms, _, err := o.eng.Extract(openPath, o.rel(openPath))
	if err != nil {
		return nil, err
	}
	var defs []Symbol
	for _, s := range syms {
		if s.IsDefinition && kindMatches(kind, s.Kind) {
			defs = append(defs, s)
		}
	}
	return projectOutline(defs, detail), nil
}

// Symbols searches a directory (gitignore-aware) for symbols, filtered by kind
// and name. name matches exactly (case-insensitive) unless nameContains is set.
func (o *Ops) Symbols(dir, kind, name string, refs, nameContains bool) ([]Symbol, error) {
	var out []Symbol
	err := o.walk(dir, func(path, rel string) error {
		syms, _, err := o.eng.Extract(path, rel)
		if err != nil {
			return nil // skip unparseable/unsupported files
		}
		for _, s := range syms {
			if !refs && !s.IsDefinition {
				continue
			}
			if !kindMatches(kind, s.Kind) {
				continue
			}
			if !nameMatches(name, s.Name, nameContains) {
				continue
			}
			out = append(out, s)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortSymbols(out)
	return out, nil
}

// Source returns the exact source of a symbol, by symbol-id or by file+name.
func (o *Ops) Source(idOrFile, name string) (SourceResult, error) {
	var file, want string
	var wantLine int
	if name != "" {
		file, want = idOrFile, name
	} else if p, n, line, ok := ParseID(idOrFile); ok {
		file, want, wantLine = p, n, line
	} else {
		return SourceResult{}, fmt.Errorf("symbol id must look like <lang>:<path>#<name>@<line>")
	}
	// Resolve file relative to root when it isn't directly openable, and confine
	// it to the working root.
	openPath, err := o.confinedPath(file)
	if err != nil {
		return SourceResult{}, err
	}
	syms, p, err := o.eng.Extract(openPath, o.rel(openPath))
	if err != nil {
		return SourceResult{}, err
	}
	var matches []Symbol
	for _, s := range syms {
		if s.IsDefinition && s.Name == want {
			matches = append(matches, s)
		}
	}
	if len(matches) == 0 {
		return SourceResult{}, fmt.Errorf("no definition named %q in %s", want, file)
	}
	chosen := matches[0]
	if wantLine > 0 {
		for _, m := range matches {
			if m.Line == wantLine {
				chosen = m
				break
			}
		}
	}
	res := SourceResult{ID: chosen.ID, Source: Slice(p.src, chosen)}
	for _, m := range matches {
		if m.ID != chosen.ID {
			res.OtherCandidates = append(res.OtherCandidates, m.ID)
		}
	}
	return res, nil
}

// Check returns syntax defects for a file.
func (o *Ops) Check(file string) ([]Defect, error) {
	openPath, err := o.confinedPath(file)
	if err != nil {
		return nil, err
	}
	return o.eng.Check(openPath)
}

// Callers finds references to name across a directory: a structural pass (tag
// references) plus a textual whole-word pass, deduped by line.
func (o *Ops) Callers(dir, name string) ([]CallSite, error) {
	var sites []CallSite
	err := o.walk(dir, func(path, rel string) error {
		syms, p, err := o.eng.Extract(path, rel)
		if err != nil {
			return nil
		}
		skip := map[int]bool{}
		// Structural pass.
		for _, s := range syms {
			if s.Name != name {
				continue
			}
			skip[s.Line] = true // both defs and refs suppress textual on that line
			if s.IsDefinition {
				continue
			}
			inFn := enclosingFunction(p, s.StartByte)
			sites = append(sites, CallSite{
				File: rel, Line: s.Line, Col: s.Col, InFunction: inFn,
				Text: lineText(p.src, s.StartByte), Source: "structural",
			})
		}
		// Textual pass over lines not already covered structurally.
		for i, line := range strings.Split(string(p.src), "\n") {
			lineNo := i + 1
			if skip[lineNo] {
				continue
			}
			if col, ok := wholeWordCol(line, name); ok {
				byteOff := lineStartByte(p.src, i) + col
				inFn := enclosingFunction(p, byteOff)
				sites = append(sites, CallSite{
					File: rel, Line: lineNo, Col: col + 1, InFunction: inFn,
					Text: strings.TrimSpace(line), Source: "textual",
				})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(sites, func(i, j int) bool {
		if sites[i].File != sites[j].File {
			return sites[i].File < sites[j].File
		}
		if sites[i].Line != sites[j].Line {
			return sites[i].Line < sites[j].Line
		}
		return sites[i].Col < sites[j].Col
	})
	return sites, nil
}

// Map returns per-file definitions with their outgoing references.
func (o *Ops) Map(dir, kind, name string, nameContains bool) ([]FileMap, error) {
	var maps []FileMap
	err := o.walk(dir, func(path, rel string) error {
		syms, _, err := o.eng.Extract(path, rel)
		if err != nil {
			return nil
		}
		// Definitions sorted by span ascending (innermost first).
		var defs []Symbol
		var refsList []Symbol
		for _, s := range syms {
			if s.IsDefinition {
				defs = append(defs, s)
			} else {
				refsList = append(refsList, s)
			}
		}
		sort.SliceStable(defs, func(i, j int) bool {
			return (defs[i].EndByte - defs[i].StartByte) < (defs[j].EndByte - defs[j].StartByte)
		})
		refsByDef := map[int]map[string]bool{}
		for _, r := range refsList {
			for di, d := range defs {
				if r.StartByte >= d.StartByte && r.EndByte <= d.EndByte {
					if r.Name == d.Name {
						break // drop self-reference
					}
					if refsByDef[di] == nil {
						refsByDef[di] = map[string]bool{}
					}
					refsByDef[di][r.Name] = true
					break // innermost enclosing def wins
				}
			}
		}
		var entries []MapEntry
		for di, d := range defs {
			if !d.IsDefinition || !kindMatches(kind, d.Kind) || !nameMatches(name, d.Name, nameContains) {
				continue
			}
			var refs []string
			for r := range refsByDef[di] {
				refs = append(refs, r)
			}
			sort.Strings(refs)
			entries = append(entries, MapEntry{
				ID: d.ID, Kind: d.Kind, Name: d.Name, Parent: d.Parent,
				Row: d.Line, Signature: d.Signature, References: refs,
			})
		}
		if len(entries) == 0 {
			return nil
		}
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].Row < entries[j].Row })
		maps = append(maps, FileMap{File: rel, Entries: entries})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(maps, func(i, j int) bool { return maps[i].File < maps[j].File })
	return maps, nil
}

// Definition resolves a symbol: name-mode (exact name across dir) or position
// mode (--at "file:line:col", scope-aware + import-edge, falling back to name).
func (o *Ops) Definition(name, at, dir string) (DefinitionResult, error) {
	if at != "" {
		return o.definitionAt(at, dir)
	}
	if name == "" {
		return DefinitionResult{}, fmt.Errorf("definition requires name or at")
	}
	defs, err := o.definitionByName(dir, name)
	if err != nil {
		return DefinitionResult{}, err
	}
	return DefinitionResult{Resolved: name, Definitions: defs}, nil
}

func (o *Ops) definitionByName(dir, name string) ([]Symbol, error) {
	syms, err := o.Symbols(dir, "", name, false, false)
	if err != nil {
		return nil, err
	}
	var out []Symbol
	for _, s := range syms {
		if s.IsDefinition && s.Name == name {
			out = append(out, s)
		}
	}
	return out, nil
}

func (o *Ops) definitionAt(at, dir string) (DefinitionResult, error) {
	file, row, col, ok := ParsePos(at)
	if !ok {
		return DefinitionResult{}, fmt.Errorf("position must look like file:line:col (1-based)")
	}
	openPath := o.resolvePath(file)
	_, p, err := o.eng.Extract(openPath, o.rel(openPath))
	if err != nil {
		return DefinitionResult{}, err
	}
	name, refNode, ok := identifierAt(p, row, col)
	if !ok {
		return DefinitionResult{}, fmt.Errorf("no identifier at %s", at)
	}
	// 1) scope-aware local.
	if local, ok := o.eng.resolveLocalAt(p, name, refNode); ok {
		return DefinitionResult{Resolved: name, Definitions: []Symbol{local}}, nil
	}
	// 2) import edge.
	if defs, ok := o.resolveImportAt(p, name, dir); ok && len(defs) > 0 {
		return DefinitionResult{Resolved: name, Definitions: defs}, nil
	}
	// 3) fallback: name lookup across the directory.
	if dir == "" {
		dir = filepath.Dir(openPath)
	}
	defs, err := o.definitionByName(dir, name)
	if err != nil {
		return DefinitionResult{}, err
	}
	return DefinitionResult{Resolved: name, Definitions: defs}, nil
}

// --- helpers ---

// resolvePath returns an openable path: the given path if it exists, else joined
// under the working root.
func (o *Ops) resolvePath(path string) string {
	if _, err := os.Stat(path); err == nil {
		return path
	}
	joined := filepath.Join(o.root, path)
	if _, err := os.Stat(joined); err == nil {
		return joined
	}
	return path
}

// confinedPath resolves a single-file path and confines it to the working root,
// mirroring the traversal confinement in walk (REQ-803). It returns the openable
// path or an error if the path resolves outside the root.
func (o *Ops) confinedPath(path string) (string, error) {
	resolved := o.resolvePath(path)
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(o.root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, absResolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %s escapes working root %s", path, o.root)
	}
	return resolved, nil
}

// walk visits supported source files under dir, confined to the working root,
// honoring .gitignore and skipping generated declaration files.
func (o *Ops) walk(dir string, fn func(path, rel string) error) error {
	base := dir
	if base == "" {
		base = o.root
	} else if !filepath.IsAbs(base) {
		// A relative dir is interpreted against the working root, not the CWD,
		// so an MCP server rooted at the store resolves "pkg" -> <root>/pkg.
		base = filepath.Join(o.root, base)
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return err
	}
	absRoot, err := filepath.Abs(o.root)
	if err != nil {
		return err
	}
	// Confine traversal: dir must be within the working root.
	if rel, err := filepath.Rel(absRoot, absBase); err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path %s escapes working root %s", dir, o.root)
	}
	ig := newIgnorer(o.root)
	return filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || ig.match(path, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if isGeneratedDecl(path) || ig.match(path, false) {
			return nil
		}
		return fn(path, o.rel(path))
	})
}

func kindMatches(want, got string) bool {
	if want == "" {
		return true
	}
	if strings.EqualFold(want, got) {
		return true
	}
	// struct/union/record are synonyms for class (no grammar emits them directly).
	if (want == "struct" || want == "union" || want == "record") && got == "class" {
		return true
	}
	return false
}

func nameMatches(want, got string, substr bool) bool {
	if want == "" {
		return true
	}
	if substr {
		return strings.Contains(strings.ToLower(got), strings.ToLower(want))
	}
	return strings.EqualFold(want, got)
}

func projectOutline(syms []Symbol, detail int) []map[string]any {
	out := make([]map[string]any, 0, len(syms))
	for _, s := range syms {
		m := map[string]any{"kind": s.Kind, "name": s.Name, "line": s.Line}
		if s.Parent != nil {
			m["parent"] = *s.Parent
		}
		switch {
		case detail <= 0:
			// terse: kind/name/parent/line only
		case detail == 1:
			m["id"] = s.ID
			m["col"] = s.Col
			m["signature"] = s.Signature
		default: // >= 2 full
			m["id"] = s.ID
			m["col"] = s.Col
			m["signature"] = s.Signature
			m["start_byte"] = s.StartByte
			m["end_byte"] = s.EndByte
			m["is_definition"] = s.IsDefinition
			m["file"] = s.File
		}
		out = append(out, m)
	}
	return out
}

func sortSymbols(syms []Symbol) {
	sort.SliceStable(syms, func(i, j int) bool {
		if syms[i].File != syms[j].File {
			return syms[i].File < syms[j].File
		}
		if syms[i].Line != syms[j].Line {
			return syms[i].Line < syms[j].Line
		}
		return syms[i].Col < syms[j].Col
	})
}
