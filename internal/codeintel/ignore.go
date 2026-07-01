package codeintel

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ignorer matches paths against .gitignore files. Following git's model, a
// .gitignore in directory D applies to everything under D, so a path is checked
// against every .gitignore from the working root down to its parent directory
// (root, ancestors, and nested per-directory files). The .git directory is
// always excluded by the caller. This is not a byte-for-byte reimplementation
// of git's spec (no negation, no ** beyond a leading segment), but it honors the
// common cases REQ-802 needs: root-level ignores when searching a subdirectory,
// and nested ignores during traversal.
type ignorer struct {
	root  string                     // absolute working root; matching stops here
	cache map[string][]ignorePattern // abs dir -> patterns declared in dir/.gitignore
}

type ignorePattern struct {
	glob    string
	dirOnly bool
}

func newIgnorer(root string) *ignorer {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	return &ignorer{root: abs, cache: map[string][]ignorePattern{}}
}

// patternsFor returns (and caches) the patterns declared in dir/.gitignore.
func (ig *ignorer) patternsFor(dir string) []ignorePattern {
	if p, ok := ig.cache[dir]; ok {
		return p
	}
	p := loadIgnorePatterns(filepath.Join(dir, ".gitignore"))
	ig.cache[dir] = p
	return p
}

func loadIgnorePatterns(file string) []ignorePattern {
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()
	var patterns []ignorePattern
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		dirOnly := strings.HasSuffix(line, "/")
		line = strings.TrimSuffix(line, "/")
		line = strings.TrimPrefix(line, "/")
		if line == "" {
			continue
		}
		patterns = append(patterns, ignorePattern{glob: line, dirOnly: dirOnly})
	}
	return patterns
}

// match reports whether path is ignored by any applicable .gitignore, checking
// every directory from the working root down to path's parent.
func (ig *ignorer) match(pathStr string, isDir bool) bool {
	abs, err := filepath.Abs(pathStr)
	if err != nil {
		return false
	}
	for dir := filepath.Dir(abs); ; dir = filepath.Dir(dir) {
		for _, p := range ig.patternsFor(dir) {
			if p.dirOnly && !isDir {
				continue
			}
			rel, err := filepath.Rel(dir, abs)
			if err != nil {
				continue
			}
			if p.matches(filepath.ToSlash(rel)) {
				return true
			}
		}
		if dir == ig.root || dir == filepath.Dir(dir) {
			return false
		}
	}
}

// matches tests a single pattern against a path relative to the .gitignore's
// directory. A pattern without a slash matches any path segment (git semantics);
// a pattern with a slash matches the relative path from the pattern's directory.
func (p ignorePattern) matches(rel string) bool {
	if strings.Contains(p.glob, "/") {
		ok, _ := filepath.Match(p.glob, rel)
		return ok
	}
	if ok, _ := filepath.Match(p.glob, path.Base(rel)); ok {
		return true
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == p.glob {
			return true
		}
	}
	return false
}
