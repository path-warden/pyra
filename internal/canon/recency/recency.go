// Package recency derives artifact modification times from Git history rather
// than from timestamps stored in frontmatter. When Git is unavailable (no
// binary, or the store is not a repository) it degrades to filesystem mtime and
// reports a single advisory so callers can surface the degradation.
package recency

import (
	"os/exec"
	"strings"
	"time"

	"github.com/chasedputnam/pyra/internal/canon/model"
)

// Provider returns last-modified times for artifact paths.
type Provider interface {
	LastModified(path string) (time.Time, bool)
}

// GitProvider derives recency from `git log`. Use NewGitProvider to construct.
type GitProvider struct {
	root   string
	gitOK  bool
	statFn func(string) (time.Time, bool)
}

// NewGitProvider probes for a usable Git repository at root.
func NewGitProvider(root string) *GitProvider {
	return &GitProvider{root: root, gitOK: gitRepoAvailable(root), statFn: statMtime}
}

// Degraded reports whether the provider fell back to filesystem mtime.
func (g *GitProvider) Degraded() bool { return !g.gitOK }

// Advisory returns a single advisory issue when the provider is degraded.
func (g *GitProvider) Advisory() (model.Issue, bool) {
	if g.gitOK {
		return model.Issue{}, false
	}
	return model.Issue{
		Severity: model.SeverityWarning,
		Code:     "recency_degraded",
		Message:  "git history unavailable; using filesystem modification times for recency",
	}, true
}

// LastModified returns the git commit time of path, or its mtime as a fallback.
func (g *GitProvider) LastModified(path string) (time.Time, bool) {
	if g.gitOK {
		out, err := exec.Command("git", "-C", g.root, "log", "-1", "--format=%cI", "--", path).Output()
		if err == nil {
			s := strings.TrimSpace(string(out))
			if s != "" {
				if ts, e := time.Parse(time.RFC3339, s); e == nil {
					return ts, true
				}
			}
		}
	}
	return g.statFn(path)
}

func gitRepoAvailable(root string) bool {
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}
	out, err := exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree").Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}
