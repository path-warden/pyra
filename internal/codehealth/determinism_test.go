package codehealth

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chasedputnam/memphis/internal/codeintel"
)

func TestAnalyze_ByteIdenticalAcrossRuns(t *testing.T) {
	root := t.TempDir()
	var b strings.Builder
	b.WriteString("package p\n")
	for i := 0; i < 22; i++ {
		b.WriteString("func F")
		b.WriteByte(byte('a' + i%26))
		b.WriteString("() {}\n")
	}
	b.WriteString("\nfunc huge(a int) int {\n")
	for i := 0; i < 70; i++ {
		b.WriteString("\tif a>0 { if a>1 { if a>2 { if a>3 { if a>4 { a++ } } } } }\n")
	}
	b.WriteString("\treturn a\n}\n")
	writeFile(t, root, "bad.go", b.String())
	writeFile(t, root, "good.go", "package p\n\nfunc Ok() int { return 1 }\n")
	ops := codeintel.NewOps(nil, root)

	run := func() string {
		rep, err := Analyze(Inputs{Ops: ops, Roots: []string{root}, Root: root})
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(rep)
		return string(data)
	}
	first := run()
	for i := 0; i < 3; i++ {
		if again := run(); again != first {
			t.Fatalf("health report not deterministic:\n%s\nvs\n%s", first, again)
		}
	}
}
