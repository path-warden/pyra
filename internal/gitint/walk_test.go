package gitint

import (
	"os"
	"os/exec"
	"strconv"
	"testing"
)

func TestParseLog_RecordsAndNumstat(t *testing.T) {
	// Two commits; second touches two files, one binary row.
	raw := "\x1eabc123\x1fAnn\x1f1700000000\n" +
		"10\t2\ta.go\n" +
		"\x1edef456\x1fBob\x1f1700000100\n" +
		"5\t0\tb.go\n" +
		"-\t-\tlogo.png\n"
	recs := parseLog(raw)
	if len(recs) != 2 {
		t.Fatalf("records = %d, want 2", len(recs))
	}
	if recs[0].SHA != "abc123" || recs[0].Author != "Ann" || recs[0].TS != 1700000000 {
		t.Errorf("rec0 header wrong: %+v", recs[0])
	}
	if len(recs[0].Files) != 1 || recs[0].Files[0] != (fileDelta{"a.go", 10, 2}) {
		t.Errorf("rec0 files wrong: %+v", recs[0].Files)
	}
	if len(recs[1].Files) != 2 {
		t.Fatalf("rec1 files = %d, want 2", len(recs[1].Files))
	}
	// Binary row counts 0/0.
	if recs[1].Files[1] != (fileDelta{"logo.png", 0, 0}) {
		t.Errorf("binary row = %+v, want logo.png 0/0", recs[1].Files[1])
	}
}

func TestParseLog_Empty(t *testing.T) {
	if r := parseLog(""); len(r) != 0 {
		t.Errorf("empty log → %d records, want 0", len(r))
	}
}

// commitAt commits staged files with a fixed author/committer date (unix ts).
func commitAt(t *testing.T, root, msg string, ts int64) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	date := strconv.FormatInt(ts, 10) + " +0000"
	cmd := exec.Command("git", "-C", root, "commit", "-m", msg)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Ann", "GIT_AUTHOR_EMAIL=ann@e",
		"GIT_COMMITTER_NAME=Ann", "GIT_COMMITTER_EMAIL=ann@e",
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}
}

func TestWalk_AsOfIsHeadCommitTime(t *testing.T) {
	root := t.TempDir()
	gitT(t, root, "init")
	writeF(t, root, "a.go", "package a\n")
	gitT(t, root, "add", ".")
	commitAt(t, root, "c1", 1700000000)
	writeF(t, root, "a.go", "package a\nvar X = 1\n")
	gitT(t, root, "add", ".")
	commitAt(t, root, "c2", 1700000500) // HEAD

	recs, asOf, capped, ok := walk(root, 100)
	if !ok {
		t.Fatal("walk should succeed in a git repo")
	}
	if len(recs) != 2 {
		t.Fatalf("records = %d, want 2", len(recs))
	}
	if asOf != 1700000500 {
		t.Errorf("asOf = %d, want HEAD ts 1700000500", asOf)
	}
	if capped {
		t.Error("2 commits under window 100 should not be capped")
	}
}

func TestWalk_CapFlag(t *testing.T) {
	root := t.TempDir()
	gitT(t, root, "init")
	for i := 0; i < 3; i++ {
		writeF(t, root, "a.go", "package a\n// "+strconv.Itoa(i)+"\n")
		gitT(t, root, "add", ".")
		commitAt(t, root, "c", int64(1700000000+i))
	}
	_, _, capped, ok := walk(root, 2) // window smaller than history
	if !ok {
		t.Fatal("walk ok")
	}
	if !capped {
		t.Error("window 2 over 3 commits should be capped")
	}
}

func TestWalk_NonGit(t *testing.T) {
	if _, _, _, ok := walk(t.TempDir(), 100); ok {
		t.Error("walk in a non-git dir should return ok=false")
	}
}
