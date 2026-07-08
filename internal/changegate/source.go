package changegate

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// SourceKind selects how the change set is obtained.
type SourceKind int

const (
	// SourceStaged reads the git staged index (what a commit would include).
	SourceStaged SourceKind = iota
	// SourceSince reads the diff between a git ref and the working state.
	SourceSince
	// SourceExplicit uses a caller-provided file list and never invokes git.
	SourceExplicit
)

// Source describes how to obtain the set of changed files.
type Source struct {
	Kind  SourceKind
	Ref   string   // for SourceSince
	Files []string // for SourceExplicit
}

// ErrNoChangeSource is returned when a git-backed source is requested but the
// store is not inside a git repository (and no explicit file list was given), so
// the caller reports the change source is unavailable rather than falsely
// reporting zero governed changes.
var ErrNoChangeSource = errors.New("change source unavailable: not a git repository and no explicit file list provided")

// ChangedFiles resolves a Source to store-root-relative, slash-form file paths,
// sorted and de-duplicated. Paths that resolve outside the store root are
// dropped. It is deterministic given repository state.
func ChangedFiles(storeRoot string, src Source) ([]string, error) {
	switch src.Kind {
	case SourceExplicit:
		absStore, err := realAbs(storeRoot)
		if err != nil {
			return nil, err
		}
		// Explicit paths are interpreted relative to the store root.
		return normalize(absStore, absStore, src.Files), nil
	case SourceStaged:
		return gitDiff(storeRoot, []string{"diff", "--cached", "--name-only"})
	case SourceSince:
		if strings.TrimSpace(src.Ref) == "" {
			return nil, fmt.Errorf("--since requires a git ref")
		}
		return gitDiff(storeRoot, []string{"diff", "--name-only", src.Ref})
	default:
		return nil, ErrNoChangeSource
	}
}

// gitDiff runs a git diff variant rooted at storeRoot and normalizes its
// repo-root-relative output to store-root-relative paths.
func gitDiff(storeRoot string, args []string) ([]string, error) {
	gitRoot, err := gitTopLevel(storeRoot)
	if err != nil {
		return nil, ErrNoChangeSource
	}
	full := append([]string{"-C", storeRoot}, args...)
	out, err := exec.Command("git", full...).Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	absStore, err := realAbs(storeRoot)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	// git prints paths relative to the repository root.
	return normalize(absStore, gitRoot, paths), nil
}

func gitTopLevel(storeRoot string) (string, error) {
	out, err := exec.Command("git", "-C", storeRoot, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// realAbs returns the absolute, symlink-resolved form of path. git's
// --show-toplevel reports a symlink-resolved path (e.g. /private/var/... on
// macOS), so the store root must be resolved the same way or filepath.Rel would
// treat every changed file as outside the store.
func realAbs(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return abs, nil
}

// normalize converts paths (each relative to base, or absolute) into
// store-root-relative slash paths, dropping any that escape the store root, then
// sorts and de-duplicates.
func normalize(absStore, base string, paths []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(base, p)
		}
		rel, err := filepath.Rel(absStore, abs)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if rel == ".." || strings.HasPrefix(rel, "../") {
			continue // outside the store root
		}
		if seen[rel] {
			continue
		}
		seen[rel] = true
		out = append(out, rel)
	}
	sort.Strings(out)
	return out
}
