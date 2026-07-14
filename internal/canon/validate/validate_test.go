package validate

import (
	"testing"

	"github.com/chasedputnam/pyra/internal/canon/artifacts"
	"github.com/chasedputnam/pyra/internal/canon/classify"
	"github.com/chasedputnam/pyra/internal/canon/model"
	"github.com/chasedputnam/pyra/internal/canon/parse"
)

func check(src string) []model.Issue {
	p := parse.Parse([]byte(src))
	c := classify.Classify(p, artifacts.Default())
	return Validate(p, c, Options{})
}

func codes(issues []model.Issue) map[string]string {
	m := map[string]string{}
	for _, i := range issues {
		m[i.Code] = i.Severity
	}
	return m
}

func has(issues []model.Issue, code string) bool {
	_, ok := codes(issues)[code]
	return ok
}

func TestValidate_CleanDecision(t *testing.T) {
	issues := check(`# A Decision

## Status

Accepted

## Context

Background.

## Decision

We SHALL do X.

## Consequences

Trade-offs.
`)
	for _, i := range issues {
		if i.Severity == model.SeverityError {
			t.Errorf("unexpected error %s: %s", i.Code, i.Message)
		}
	}
}

func TestValidate_DecisionMissingRequiredSection(t *testing.T) {
	// Missing ## Decision.
	issues := check("# D\n\n## Status\n\nAccepted\n\n## Context\n\nx\n\n## Consequences\n\ny\n")
	if codes(issues)["missing-decision"] != model.SeverityError {
		t.Errorf("expected missing-decision error, got %v", codes(issues))
	}
}

func TestValidate_DecisionStatusAndCategoryEnums(t *testing.T) {
	issues := check("# D\n\n## Context\n\nx\n\n## Decision\n\ny\n\n## Consequences\n\nz\n\n## Status\n\nBananas\n\n## Category\n\nNonsense\n")
	c := codes(issues)
	if c["invalid-decision-status"] != model.SeverityError {
		t.Errorf("expected invalid-decision-status, got %v", c)
	}
	if c["invalid-decision-category"] != model.SeverityError {
		t.Errorf("expected invalid-decision-category, got %v", c)
	}
}

func TestValidate_TitleCounts(t *testing.T) {
	if codes(check("## Context\n\nx\n\n## Decision\n\ny\n\n## Consequences\n\nz\n"))[CodeMissingTitle] != model.SeverityError {
		t.Error("expected missing-title")
	}
	if codes(check("# One\n\n# Two\n\n## Context\n\nx\n\n## Decision\n\ny\n\n## Consequences\n\nz\n"))[CodeMultipleTitles] != model.SeverityError {
		t.Error("expected multiple-titles")
	}
}

func TestValidate_RequirementRequiresProblemAndRequirements(t *testing.T) {
	issues := check("# Spec\n\n## Requirements\n\n- [REQ-001] The system SHALL do a thing.\n")
	if !has(issues, CodeMissingProblem) {
		t.Errorf("expected missing-problem, got %v", codes(issues))
	}
}

func TestValidate_MalformedRequirementCodes(t *testing.T) {
	issues := check(`# Spec

## Problem

p

## Requirements

- [REQ-001] valid one.
- [REQ-] missing number.
- [REQ-002]
- a plain line with no bracket
`)
	c := codes(issues)
	for _, want := range []string{CodeMalformedReqID, CodeEmptyReqText, CodeReqMissingID} {
		if c[want] != model.SeverityError {
			t.Errorf("expected %s error, got %v", want, c)
		}
	}
}

func TestValidate_DuplicateRequirementID(t *testing.T) {
	issues := check("# Spec\n\n## Problem\n\np\n\n## Requirements\n\n- [REQ-001] The system SHALL do X.\n- [REQ-001] The system SHALL do Y.\n")
	if codes(issues)[CodeDuplicateReqID] != model.SeverityError {
		t.Errorf("expected duplicate-req-id, got %v", codes(issues))
	}
}

func TestValidate_BCP14FlagsMixedCaseEvenWithUppercase(t *testing.T) {
	// rac-core: a non-uppercase keyword is flagged even alongside an uppercase one.
	issues := check("# Spec\n\n## Problem\n\np\n\n## Requirements\n\n- [REQ-001] The system SHALL do X and should do Y.\n")
	if codes(issues)[CodeReqNormativeKeyword] != model.SeverityError {
		t.Errorf("expected requirement-normative-keyword for lowercase 'should', got %v", codes(issues))
	}
}

func TestValidate_UppercaseOnlyNoBCP14(t *testing.T) {
	issues := check("# Spec\n\n## Problem\n\np\n\n## Requirements\n\n- [REQ-001] The system SHALL do exactly one thing.\n")
	if has(issues, CodeReqNormativeKeyword) {
		t.Errorf("did not expect BCP-14 issue, got %v", codes(issues))
	}
}

func TestValidate_NotSingular(t *testing.T) {
	issues := check("# Spec\n\n## Problem\n\np\n\n## Requirements\n\n- [REQ-001] The system SHALL do X and SHALL do Y.\n")
	if codes(issues)[CodeReqNotSingular] != model.SeverityWarning {
		t.Errorf("expected requirement-not-singular warning, got %v", codes(issues))
	}
}

func TestValidate_NonEARS(t *testing.T) {
	issues := check("# Spec\n\n## Problem\n\np\n\n## Requirements\n\n- [REQ-001] The system does indexing of records.\n")
	if codes(issues)[CodeReqNonEARS] != model.SeverityWarning {
		t.Errorf("expected requirement-non-ears warning, got %v", codes(issues))
	}
}

func TestValidate_EARSIfWithoutThen(t *testing.T) {
	issues := check("# Spec\n\n## Problem\n\np\n\n## Requirements\n\n- [REQ-001] If the cache is full the system SHALL evict entries.\n")
	if codes(issues)[CodeReqEARSClause] != model.SeverityWarning {
		t.Errorf("expected requirement-ears-clause warning, got %v", codes(issues))
	}
}

func TestValidate_EARSIfWithThenOK(t *testing.T) {
	issues := check("# Spec\n\n## Problem\n\np\n\n## Requirements\n\n- [REQ-001] If the cache is full then the system SHALL evict entries.\n")
	if has(issues, CodeReqEARSClause) {
		t.Errorf("did not expect ears-clause when 'then' present, got %v", codes(issues))
	}
}

func TestValidate_AmbiguousVerb(t *testing.T) {
	issues := check("# Spec\n\n## Problem\n\np\n\n## Requirements\n\n- [REQ-001] The system SHALL support documents.\n")
	if codes(issues)[CodeAmbiguousVerb] != model.SeverityWarning {
		t.Errorf("expected ambiguous-verb warning, got %v", codes(issues))
	}
}

func TestValidate_RoadmapNoAdvancementLink(t *testing.T) {
	issues := check("# R\n\n## Outcomes\n\no\n\n## Initiatives\n\ni\n")
	if codes(issues)[CodeRoadmapNoLink] != model.SeverityWarning {
		t.Errorf("expected roadmap-no-advancement-link, got %v", codes(issues))
	}
}

func TestValidate_RoadmapHorizon(t *testing.T) {
	issues := check("# R\n\n## Outcomes\n\no\n\n## Initiatives\n\ni\n\n## Horizon\n\nyesterday\n\n## Related Decisions\n\n- adr-001\n")
	if codes(issues)[CodeInvalidHorizon] != model.SeverityError {
		t.Errorf("expected invalid-roadmap-horizon, got %v", codes(issues))
	}
	// A valid quarter passes.
	ok := check("# R\n\n## Outcomes\n\no\n\n## Initiatives\n\ni\n\n## Horizon\n\nQ3 2026\n\n## Related Decisions\n\n- adr-001\n")
	if has(ok, CodeInvalidHorizon) {
		t.Errorf("valid quarter horizon flagged, got %v", codes(ok))
	}
}

func TestValidate_TicketingFormatLint(t *testing.T) {
	src := "# D\n\n## Context\n\nx\n\n## Decision\n\ny\n\n## Consequences\n\nz\n\n## Related Tickets\n\n- not-a-ticket\n"
	p := parse.Parse([]byte(src))
	c := classify.Classify(p, artifacts.Default())
	issues := Validate(p, c, Options{TicketProvider: "github"})
	if codes(issues)[CodeMalformedTicket] != model.SeverityError {
		t.Errorf("expected malformed-ticket-reference for github, got %v", codes(issues))
	}
	// Valid github ref passes.
	src2 := "# D\n\n## Context\n\nx\n\n## Decision\n\ny\n\n## Consequences\n\nz\n\n## Related Tickets\n\n- owner/repo#12\n"
	p2 := parse.Parse([]byte(src2))
	c2 := classify.Classify(p2, artifacts.Default())
	if has(Validate(p2, c2, Options{TicketProvider: "github"}), CodeMalformedTicket) {
		t.Error("valid github ref flagged")
	}
	// No provider -> no lint.
	if has(Validate(p, c, Options{}), CodeMalformedTicket) {
		t.Error("ticket linted without a provider")
	}
}
