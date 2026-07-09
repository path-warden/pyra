package deadcode

import (
	"encoding/json"
	"testing"
)

func TestAnalyze_ByteIdenticalAcrossRuns(t *testing.T) {
	g, ops, root := deadRepo(t)
	run := func() string {
		b, _ := json.Marshal(Analyze(g, ops, root, nil))
		return string(b)
	}
	first := run()
	for i := 0; i < 3; i++ {
		if again := run(); again != first {
			t.Fatalf("dead-code report not deterministic:\n%s\nvs\n%s", first, again)
		}
	}
}
