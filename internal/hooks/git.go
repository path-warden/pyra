package hooks

import (
	"os"
	"path/filepath"
	"strings"
)

// gitInstaller manages repo-local git hooks under <store>/.git/hooks. The
// pre-commit hook runs the blocking gate; the post-merge hook runs the store
// integrity guard and never aborts the merge.
type gitInstaller struct{}

func (gitInstaller) Target() Target { return TargetGit }

type gitHook struct {
	name string
	body string
}

// gitHooks defines the managed-block body for each git hook. Each body checks
// that pyra is on PATH before invoking it (Requirement 6.4).
func gitHooks() []gitHook {
	return []gitHook{
		{
			name: "pre-commit",
			body: "# " + ManagedMarker + ": block the commit on Canon gate failure.\n" +
				"if ! command -v pyra >/dev/null 2>&1; then\n" +
				"  echo \"pyra: not found on PATH; install pyra or remove this hook\" >&2\n" +
				"  exit 1\n" +
				"fi\n" +
				"pyra gate || exit 1",
		},
		{
			name: "post-merge",
			body: "# " + ManagedMarker + ": store integrity guard (report-only; never aborts a merge).\n" +
				"if ! command -v pyra >/dev/null 2>&1; then\n" +
				"  echo \"pyra: not found on PATH; skipping store integrity check\" >&2\n" +
				"else\n" +
				"  pyra rebuild || echo \"pyra: store integrity check reported issues (merge already applied)\" >&2\n" +
				"fi",
		},
	}
}

func (gitInstaller) hooksDir(ctx Context) string {
	return filepath.Join(ctx.StoreRoot, ".git", "hooks")
}

func (g gitInstaller) Detect(ctx Context) bool {
	fi, err := os.Stat(filepath.Join(ctx.StoreRoot, ".git"))
	return err == nil && fi.IsDir()
}

func (g gitInstaller) Install(ctx Context) (Result, error) {
	dir := g.hooksDir(ctx)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{}, err
	}
	res := Result{Target: TargetGit, Action: ActionUnchanged}
	createdAny, updatedAny := false, false

	for _, h := range gitHooks() {
		path := filepath.Join(dir, h.name)
		existing, existed := readFileOK(path)
		seed := existing
		if strings.TrimSpace(seed) == "" {
			seed = "#!/bin/sh\n"
		}
		next := upsertBlock(seed, h.body)
		if next == existing {
			res.Paths = append(res.Paths, path)
			continue
		}
		if err := os.WriteFile(path, []byte(next), 0o755); err != nil {
			return Result{}, err
		}
		if err := os.Chmod(path, 0o755); err != nil {
			return Result{}, err
		}
		res.Paths = append(res.Paths, path)
		if existed {
			updatedAny = true
		} else {
			createdAny = true
		}
	}

	switch {
	case createdAny:
		res.Action = ActionCreated
	case updatedAny:
		res.Action = ActionUpdated
	}
	return res, nil
}

func (g gitInstaller) Uninstall(ctx Context) (Result, error) {
	dir := g.hooksDir(ctx)
	res := Result{Target: TargetGit, Action: ActionUnchanged}
	for _, h := range gitHooks() {
		path := filepath.Join(dir, h.name)
		existing, existed := readFileOK(path)
		if !existed || !hasBlock(existing) {
			continue
		}
		stripped := removeBlock(existing)
		res.Paths = append(res.Paths, path)
		// If only the shebang we seeded remains, remove the file we created.
		if strings.TrimSpace(stripped) == "#!/bin/sh" {
			if err := os.Remove(path); err != nil {
				return Result{}, err
			}
		} else if err := os.WriteFile(path, []byte(stripped), 0o755); err != nil {
			return Result{}, err
		}
		res.Action = ActionRemoved
	}
	return res, nil
}

func (g gitInstaller) Status(ctx Context) (Result, error) {
	dir := g.hooksDir(ctx)
	res := Result{Target: TargetGit, Action: ActionAbsent}
	for _, h := range gitHooks() {
		path := filepath.Join(dir, h.name)
		if existing, ok := readFileOK(path); ok && hasBlock(existing) {
			res.Paths = append(res.Paths, path)
			res.Action = ActionPresent
		}
	}
	return res, nil
}

func readFileOK(path string) (string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(raw), true
}
