package codeintel

import (
	"path"
	"path/filepath"
	"strings"
)

// importBinding is a resolved import: Name as referenced in this file, Source
// the original name in the target module, Module the module-path text.
type importBinding struct {
	Name   string
	Source string
	Module string
}

// resolveImportAt attempts import-edge resolution for name using the file's
// imports query and the profile's import_resolution strategy. Returns the
// definitions found in candidate target files, or ok=false to fall through.
func (o *Ops) resolveImportAt(p *parsed, name, dir string) ([]Symbol, bool) {
	if p.cq.imports == nil || p.lang.Profile.ImportResolution == "" {
		return nil, false
	}
	var binding *importBinding
	for _, b := range extractImports(p) {
		if b.Name == name {
			bb := b
			binding = &bb
			break
		}
	}
	if binding == nil {
		return nil, false
	}
	candidates := o.importCandidatePaths(p, binding.Module)
	for _, cand := range candidates {
		syms, _, err := o.eng.Extract(cand, o.rel(cand))
		if err != nil {
			continue
		}
		var defs []Symbol
		for _, s := range syms {
			if s.IsDefinition && s.Name == binding.Source {
				defs = append(defs, s)
			}
		}
		if len(defs) > 0 {
			return defs, true
		}
	}
	return nil, false
}

// extractImports runs the imports query, mapping import.name/import.source/
// import.module captures into bindings (needs at least name + module).
func extractImports(p *parsed) []importBinding {
	var out []importBinding
	for _, m := range p.cq.imports.Execute(p.tree) {
		var name, source, module string
		for _, c := range m.Captures {
			switch c.Name {
			case "import.name":
				name = c.Node.Text(p.src)
			case "import.source":
				source = c.Node.Text(p.src)
			case "import.module":
				module = c.Node.Text(p.src)
			}
		}
		if name == "" || module == "" {
			continue
		}
		if source == "" {
			source = name
		}
		out = append(out, importBinding{Name: name, Source: source, Module: strings.Trim(module, `"'`)})
	}
	return out
}

// importCandidatePaths turns a module path into candidate target files per the
// language's import_resolution strategy.
func (o *Ops) importCandidatePaths(p *parsed, module string) []string {
	fileDir := filepath.Dir(o.resolvePath(p.rel))
	switch p.lang.Profile.ImportResolution {
	case "dotted_package":
		return dottedPackageCandidates(fileDir, module)
	case "relative_path":
		return relativePathCandidates(fileDir, module)
	default:
		return nil
	}
}

// dottedPackageCandidates: "foo.bar" -> dir/foo/bar.py | dir/foo/bar/__init__.py.
// Leading dots are relative to the current package (1 dot = own dir).
func dottedPackageCandidates(fileDir, module string) []string {
	base := fileDir
	rest := module
	for strings.HasPrefix(rest, ".") {
		rest = rest[1:]
		if len(rest) > 0 && rest[0] != '.' {
			break
		}
		base = filepath.Dir(base)
	}
	parts := strings.Split(rest, ".")
	joined := filepath.Join(append([]string{base}, parts...)...)
	return []string{joined + ".py", filepath.Join(joined, "__init__.py")}
}

// relativePathCandidates: only "./"-prefixed specifiers resolve. "./util" ->
// ./util.js | ./util.jsx | ./util/index.js. Bare specifiers return nil.
func relativePathCandidates(fileDir, module string) []string {
	if !strings.HasPrefix(module, "./") && !strings.HasPrefix(module, "../") {
		return nil
	}
	joined := normalizeLexical(filepath.Join(fileDir, module))
	return []string{
		joined + ".js",
		joined + ".jsx",
		joined + ".ts",
		joined + ".tsx",
		filepath.Join(joined, "index.js"),
	}
}

// normalizeLexical resolves . and .. without touching disk.
func normalizeLexical(p string) string {
	return filepath.FromSlash(path.Clean(filepath.ToSlash(p)))
}
