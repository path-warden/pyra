package classify

import (
	"testing"

	"github.com/chasedputnam/pyra/internal/canon/artifacts"
	"github.com/chasedputnam/pyra/internal/canon/model"
)

func product(sections ...string) *model.Product {
	m := map[string]string{}
	for _, s := range sections {
		m[s] = "body"
	}
	return &model.Product{Sections: m}
}

func TestClassify_Decision(t *testing.T) {
	p := product("status", "context", "decision", "consequences")
	c := Classify(p, artifacts.Default())
	if c.Type != artifacts.TypeDecision {
		t.Fatalf("got type %q (conf %.2f, scores %v)", c.Type, c.Confidence, c.Scores)
	}
}

func TestClassify_Requirement(t *testing.T) {
	p := product("problem", "requirements", "success metrics")
	c := Classify(p, artifacts.Default())
	if c.Type != artifacts.TypeRequirement {
		t.Fatalf("got type %q scores %v", c.Type, c.Scores)
	}
}

func TestClassify_Design(t *testing.T) {
	p := product("context", "user need", "design", "constraints")
	c := Classify(p, artifacts.Default())
	if c.Type != artifacts.TypeDesign {
		t.Fatalf("got type %q scores %v", c.Type, c.Scores)
	}
}

func TestClassify_Roadmap(t *testing.T) {
	p := product("status", "outcomes", "initiatives", "success measures")
	c := Classify(p, artifacts.Default())
	if c.Type != artifacts.TypeRoadmap {
		t.Fatalf("got type %q scores %v", c.Type, c.Scores)
	}
}

func TestClassify_Prompt(t *testing.T) {
	p := product("objective", "input", "instructions", "output")
	c := Classify(p, artifacts.Default())
	if c.Type != artifacts.TypePrompt {
		t.Fatalf("got type %q scores %v", c.Type, c.Scores)
	}
}

func TestClassify_SynonymSatisfiesRecommended(t *testing.T) {
	// "success criteria" is a synonym for the requirement's "success metrics".
	p := product("problem", "requirements", "success criteria")
	c := Classify(p, artifacts.Default())
	if c.Type != artifacts.TypeRequirement {
		t.Fatalf("synonym match failed: got %q scores %v", c.Type, c.Scores)
	}
}

func TestClassify_UnknownBelowThreshold(t *testing.T) {
	p := product("acceptance criteria") // not a scored section of any type
	c := Classify(p, artifacts.Default())
	if c.Type != artifacts.TypeUnknown {
		t.Fatalf("expected unknown, got %q (conf %.3f scores %v)", c.Type, c.Confidence, c.Scores)
	}
}

func TestClassify_PartialRequiredClearsThreshold(t *testing.T) {
	// Prompt: 4 required, 3 recommended -> ceiling 5.5. Three required matched ->
	// points 3.0 -> fit 0.545 >= 0.5, and matched_required > 0.
	p := product("objective", "input", "instructions")
	c := Classify(p, artifacts.Default())
	if c.Type != artifacts.TypePrompt {
		t.Fatalf("expected prompt, got %q (conf %.3f)", c.Type, c.Confidence)
	}
}

func TestClassify_NoRequiredMatchedIsUnknown(t *testing.T) {
	// Only recommended sections of the requirement type, no required -> unknown
	// even if some fit accrues (rac-core: not best.matched_required).
	p := product("success metrics", "risks", "assumptions")
	c := Classify(p, artifacts.Default())
	if c.Type != artifacts.TypeUnknown {
		t.Errorf("expected unknown when no required section matched, got %q (conf %.3f)", c.Type, c.Confidence)
	}
}

func TestClassify_EmptyProductUnknown(t *testing.T) {
	c := Classify(product(), artifacts.Default())
	if c.Type != artifacts.TypeUnknown || c.Confidence != 0 {
		t.Errorf("expected unknown/0, got %q/%.3f", c.Type, c.Confidence)
	}
}

func TestClassify_ScoresPopulatedForAllTypes(t *testing.T) {
	c := Classify(product("context", "decision", "consequences"), artifacts.Default())
	if len(c.Scores) != len(artifacts.Default()) {
		t.Errorf("expected scores for all %d types, got %d", len(artifacts.Default()), len(c.Scores))
	}
}
