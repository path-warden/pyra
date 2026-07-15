package agents

import (
	"strings"
	"testing"
)

func TestParseIDsStableAndDeduplicated(t *testing.T) {
	got, err := ParseIDs([]string{"pi", "claude", "pi", "codex"})
	if err != nil {
		t.Fatal(err)
	}
	want := []ID{Claude, Codex, Pi}
	if len(got) != len(want) {
		t.Fatalf("ParseIDs=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ParseIDs=%v want %v", got, want)
		}
	}
}

func TestParseIDsRejectsUnknownWithSupportedList(t *testing.T) {
	_, err := ParseIDs([]string{"cursor"})
	if err == nil {
		t.Fatal("expected unsupported agent error")
	}
	for _, s := range []string{"cursor", "claude", "codex", "opencode", "pi", "kiro"} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("error %q missing %q", err, s)
		}
	}
}

func TestRenderAgentsCreatesAndPreservesContent(t *testing.T) {
	got, err := renderAgents("# Existing\n\nKeep this.\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "# Existing\n\nKeep this.\n") {
		t.Fatalf("existing content changed:\n%s", got)
	}
	for _, required := range []string{
		agentsBegin, agentsEnd, "pyra project", "pyra gate .", "find_decisions",
		"get_artifact", "get_context", "pyra rebuild .", "pyra relationships . --summary --validate",
		"regardless of skill, command, prompt, or workflow name", "Accepted Canon", "blocking",
		"Before design", "approved requirements", "applicable Canon", "explicit human approval",
		"Do not implement unapproved or failing authority", "stop and report", "test evidence",
	} {
		if !strings.Contains(got, required) {
			t.Errorf("managed instructions missing %q", required)
		}
	}
	if words := len(strings.Fields(managedInstructions)); words > 210 {
		t.Errorf("managed instructions are too verbose: %d words (limit 210)", words)
	}

	again, err := renderAgents(got)
	if err != nil {
		t.Fatal(err)
	}
	if again != got {
		t.Errorf("render is not idempotent\nfirst:\n%s\nsecond:\n%s", got, again)
	}
	if strings.Count(again, agentsBegin) != 1 || strings.Count(again, agentsEnd) != 1 {
		t.Errorf("managed block duplicated:\n%s", again)
	}
}

func TestRenderAgentsUpdatesOwnedBlock(t *testing.T) {
	existing := "before\n" + agentsBegin + "\nold\n" + agentsEnd + "\nafter\n"
	got, err := renderAgents(existing)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "\nold\n") || !strings.HasPrefix(got, "before\n") || !strings.HasSuffix(got, "after\n") {
		t.Fatalf("owned replacement damaged surrounding content:\n%s", got)
	}
}

func TestRenderAgentsRejectsMalformedMarkers(t *testing.T) {
	for _, existing := range []string{
		agentsBegin + "\nmissing end\n",
		agentsEnd + "\nmissing begin\n",
		agentsBegin + "\na\n" + agentsEnd + "\n" + agentsBegin + "\nb\n" + agentsEnd,
	} {
		if _, err := renderAgents(existing); err == nil {
			t.Errorf("expected marker error for %q", existing)
		}
	}
}
