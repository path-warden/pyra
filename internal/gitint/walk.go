package gitint

import (
	"os/exec"
	"strings"
)

// fileDelta is one file's line change within a commit. Binary rows count 0.
type fileDelta struct {
	Path    string
	Added   int
	Deleted int
}

// commitRec is one parsed commit from the history walk.
type commitRec struct {
	SHA    string
	Author string
	TS     int64 // committer/author unix timestamp (%ct)
	Files  []fileDelta
}

// Record/field separators embedded in the git log format (unlikely to appear in
// author names or paths).
const (
	recSep   = "\x1e"
	fieldSep = "\x1f"
)

// gitLogFormat requests one record-separated header per commit carrying
// sha / author name / committer timestamp, followed by --numstat rows.
const gitLogFormat = "--format=" + recSep + "%H" + fieldSep + "%an" + fieldSep + "%ct"

// walk runs one bounded `git log --numstat` over root and returns the parsed
// records, the anchor timestamp asOf (the max commit ts = HEAD's commit time),
// whether the walk hit the window cap, and ok=false when root is not a git repo.
func walk(root string, window int) (recs []commitRec, asOf int64, capped bool, ok bool) {
	out, err := exec.Command("git", "-C", root,
		"log", "-n"+itoa(window), "--no-merges", "--date-order", gitLogFormat, "--numstat").Output()
	if err != nil {
		return nil, 0, false, false
	}
	recs = parseLog(string(out))
	for _, r := range recs {
		if r.TS > asOf {
			asOf = r.TS
		}
	}
	return recs, asOf, len(recs) >= window, true
}

// parseLog parses `git log <gitLogFormat> --numstat` output into records. Pure
// (no subprocess) so it is unit-testable without a repository.
func parseLog(raw string) []commitRec {
	var recs []commitRec
	for _, block := range strings.Split(raw, recSep) {
		block = strings.TrimLeft(block, "\n")
		if block == "" {
			continue
		}
		lines := strings.Split(block, "\n")
		header := strings.SplitN(lines[0], fieldSep, 3)
		if len(header) != 3 {
			continue
		}
		rec := commitRec{
			SHA:    header[0],
			Author: header[1],
			TS:     int64(atoiOrZero(strings.TrimSpace(header[2]))),
		}
		for _, line := range lines[1:] {
			if line = strings.TrimSpace(line); line == "" {
				continue
			}
			parts := strings.Split(line, "\t")
			if len(parts) != 3 {
				continue
			}
			rec.Files = append(rec.Files, fileDelta{
				Path:    parts[2],
				Added:   atoiOrZero(parts[0]), // "-" (binary) → 0
				Deleted: atoiOrZero(parts[1]),
			})
		}
		recs = append(recs, rec)
	}
	return recs
}
