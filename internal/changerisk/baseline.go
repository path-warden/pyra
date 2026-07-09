package changerisk

import "strings"

// BaselineScores scores the repo's recent commits to build a local risk
// distribution for repo-relative ranking. It uses one `git log --numstat` call
// (no per-commit author lookup) so it stays cheap enough for a pre-merge gate.
//
// Experience is left unknown for every baseline commit; the target change is
// ranked with experience likewise unknown (see Report.Assess), so the comparison
// is like-with-like — a diff-shape percentile within this repo. excludeSHA drops
// the target from its own baseline (short or full sha).
func BaselineScores(root, anchor string, limit int, exts []string, excludeSHA string) []float64 {
	out := git(root, "log", "-n"+itoa(limit), "--no-merges", "--format=%x1e%H", "--numstat", anchor)
	var scores []float64
	for _, block := range strings.Split(out, "\x1e") {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if len(lines) == 0 || lines[0] == "" {
			continue
		}
		sha := strings.TrimSpace(lines[0])
		if excludeSHA != "" && (strings.HasPrefix(sha, excludeSHA) || strings.HasPrefix(excludeSHA, sha)) {
			continue
		}
		changes := parseNumstat(strings.Join(lines[1:], "\n"), exts)
		if len(changes) == 0 {
			continue
		}
		f := featuresFromChanges(changes, nil, "", "", sha) // exp unknown, like-with-like
		scores = append(scores, ScoreChange(f).Score)
	}
	return scores
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
