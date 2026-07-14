// Package validate checks a parsed, classified Canon artifact against RAC's
// format rules, ported from rac-core's validation.py. It returns a flat list of
// findings (errors and warnings) and never stops at the first problem.
//
// Validation dispatches on artifact type. Required-section presence is checked
// against raw canonical headings (synonyms are a classification aid only).
// Constrained metadata enums, the requirement-quality standards (BCP-14/RFC 8174,
// ISO/IEC/IEEE 29148 singular, EARS), and the roadmap horizon/advancement rules
// mirror rac-core's codes and severities exactly.
package validate

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/chasedputnam/pyra/internal/canon/artifacts"
	"github.com/chasedputnam/pyra/internal/canon/classify"
	"github.com/chasedputnam/pyra/internal/canon/model"
)

// MaxRequirements mirrors rac-core MAX_REQUIREMENTS.
const MaxRequirements = 50

// Issue codes (ported verbatim from rac-core for SARIF rule-id parity).
const (
	CodeMissingTitle        = "missing-title"
	CodeMultipleTitles      = "multiple-titles"
	CodeMissingProblem      = "missing-problem"
	CodeMissingRequirements = "missing-requirements"
	CodeReqMissingID        = "req-missing-id"
	CodeEmptyReqText        = "empty-req-text"
	CodeMalformedReqID      = "malformed-req-id"
	CodeDuplicateReqID      = "duplicate-req-id"
	CodeDuplicateReqText    = "duplicate-req-text"
	CodeMissingMetrics      = "missing-success-metrics"
	CodeMissingRisks        = "missing-risks"
	CodeEmptyProblem        = "empty-problem"
	CodeTooManyReqs         = "too-many-requirements"
	CodeAmbiguousVerb       = "ambiguous-verb"
	CodeReqNormativeKeyword = "requirement-normative-keyword"
	CodeReqNotSingular      = "requirement-not-singular"
	CodeReqNonEARS          = "requirement-non-ears"
	CodeReqEARSClause       = "requirement-ears-clause"
	CodeInvalidHorizon      = "invalid-roadmap-horizon"
	CodeRoadmapNoLink       = "roadmap-no-advancement-link"
	CodeMalformedTicket     = "malformed-ticket-reference"
)

// Options configures validation.
type Options struct {
	Registry       artifacts.Registry
	TicketProvider string
}

func (o Options) registry() artifacts.Registry {
	if o.Registry == nil {
		return artifacts.Default()
	}
	return o.Registry
}

var (
	normativeRe = regexp.MustCompile(`(?i)\b(shall|must|should)\b`)
	earsIfRe    = regexp.MustCompile(`(?i)^\s*if\b`)
	thenRe      = regexp.MustCompile(`(?i)\bthen\b`)
	ambiguousRe = regexp.MustCompile(`(?i)\b(support|handle|allow|enable)\b`)
	quarterRe   = regexp.MustCompile(`^Q[1-4]\s+\d{4}$`)
	listMarker  = regexp.MustCompile(`^(?:[-*+]|\d+\.)\s+`)
)

var horizonValues = map[string]bool{"now": true, "next": true, "later": true}

// Validate runs all applicable checks for a parsed, classified artifact.
func Validate(p *model.Product, c classify.Classification, opts Options) []model.Issue {
	reg := opts.registry()
	issues := validateTicketing(p, c, reg, opts.TicketProvider)

	switch c.Type {
	case artifacts.TypeDecision:
		issues = append(issues, validateTitle(p)...)
		issues = append(issues, validateRequiredSections(p, reg[c.Type])...)
		issues = append(issues, validateMetadata(p, reg[c.Type])...)
	case artifacts.TypeRoadmap:
		issues = append(issues, validateTitle(p)...)
		issues = append(issues, validateRequiredSections(p, reg[c.Type])...)
		issues = append(issues, validateRoadmapExtras(p)...)
		issues = append(issues, validateMetadata(p, reg[c.Type])...)
	case artifacts.TypePrompt, artifacts.TypeDesign:
		issues = append(issues, validateTitle(p)...)
		issues = append(issues, validateRequiredSections(p, reg[c.Type])...)
		issues = append(issues, validateMetadata(p, reg[c.Type])...)
	case artifacts.TypeRequirement:
		issues = append(issues, validateRequirement(p)...)
		issues = append(issues, validateMetadata(p, reg[c.Type])...)
		issues = append(issues, requirementStandards(p)...)
	default: // unknown / legacy fallback: requirement rules only
		issues = append(issues, validateRequirement(p)...)
	}
	return issues
}

// ValidateStatus checks only the lifecycle status enum (kept for callers that
// want the status check in isolation).
func ValidateStatus(p *model.Product, c classify.Classification, reg artifacts.Registry) []model.Issue {
	spec, ok := reg[c.Type]
	if !ok {
		return nil
	}
	return metadataField(p, spec, "status")
}

func validateTitle(p *model.Product) []model.Issue {
	var issues []model.Issue
	if p.TitleCount == 0 || p.Title == "" {
		issues = append(issues, errIssue(CodeMissingTitle, "File has no top-level # title."))
	}
	if p.TitleCount > 1 {
		issues = append(issues, errIssue(CodeMultipleTitles,
			"File has more than one top-level # title; expected exactly one."))
	}
	return issues
}

func validateRequiredSections(p *model.Product, spec artifacts.ArtifactSpec) []model.Issue {
	var issues []model.Issue
	for _, s := range spec.Required {
		// Required-section presence uses the raw canonical heading (no synonyms).
		if _, ok := p.Sections[s.Name]; !ok {
			issues = append(issues, errIssue("missing-"+hyphenate(s.Name),
				spec.Display+" is missing a ## "+titleCase(s.Name)+" section."))
		}
	}
	return issues
}

func validateMetadata(p *model.Product, spec artifacts.ArtifactSpec) []model.Issue {
	var issues []model.Issue
	// Deterministic field order.
	fields := make([]string, 0, len(spec.Metadata))
	for f := range spec.Metadata {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	for _, f := range fields {
		issues = append(issues, metadataField(p, spec, f)...)
	}
	return issues
}

func metadataField(p *model.Product, spec artifacts.ArtifactSpec, field string) []model.Issue {
	allowed, ok := spec.Metadata[field]
	if !ok {
		return nil
	}
	value := firstValue(p.Sections[field])
	if value == "" {
		return nil
	}
	for _, a := range allowed {
		if strings.EqualFold(value, a) {
			return nil
		}
	}
	return []model.Issue{errIssue("invalid-"+spec.Type+"-"+field,
		"## "+titleCase(field)+" value '"+value+"' is not one of: "+strings.Join(allowed, ", ")+".")}
}

func validateRoadmapExtras(p *model.Product) []model.Issue {
	var issues []model.Issue
	if h := firstValue(p.Sections["horizon"]); h != "" {
		if !horizonValues[strings.ToLower(h)] && !quarterRe.MatchString(h) {
			issues = append(issues, errIssue(CodeInvalidHorizon,
				"## Horizon value '"+h+"' is not one of: now, next, later, or a quarter (e.g. Q3 2026)."))
		}
	}
	if !sectionPresent(p, "related requirements") && !sectionPresent(p, "related decisions") {
		issues = append(issues, warnIssue(CodeRoadmapNoLink,
			"Roadmap links no ## Related Requirements or ## Related Decisions it advances."))
	}
	return issues
}

func validateRequirement(p *model.Product) []model.Issue {
	issues := validateTitle(p)

	if !sectionPresent(p, "problem") {
		issues = append(issues, errIssue(CodeMissingProblem, "File is missing a ## Problem section."))
	}
	if !sectionPresent(p, "requirements") {
		issues = append(issues, errIssue(CodeMissingRequirements, "File is missing a ## Requirements section."))
	}

	issues = append(issues, malformedRequirementIssues(p)...)
	issues = append(issues, reportDuplicates(p.Requirements,
		func(r model.Requirement) string { return r.ID }, model.SeverityError, CodeDuplicateReqID,
		func(r model.Requirement, n int) string { return "Duplicate requirement ID " + r.ID + "." })...)
	issues = append(issues, requirementWarnings(p)...)
	return issues
}

func malformedRequirementIssues(p *model.Product) []model.Issue {
	var issues []model.Issue
	for _, m := range p.Malformed {
		switch m.Reason {
		case "missing-id":
			issues = append(issues, lineIssue(model.SeverityError, CodeReqMissingID, m.Line,
				"Requirement line has no [REQ-NNN] ID: "+m.Raw))
		case "empty-text":
			issues = append(issues, lineIssue(model.SeverityError, CodeEmptyReqText, m.Line,
				"Requirement ["+m.BadID+"] has no description text."))
		default: // bad-id
			issues = append(issues, lineIssue(model.SeverityError, CodeMalformedReqID, m.Line,
				"Malformed requirement ID ["+m.BadID+"]; expected form [REQ-NNN]."))
		}
	}
	return issues
}

func requirementWarnings(p *model.Product) []model.Issue {
	var issues []model.Issue
	if !sectionPresent(p, "success metrics") {
		issues = append(issues, warnIssue(CodeMissingMetrics, "No ## Success Metrics section (optional, but recommended)."))
	}
	if !sectionPresent(p, "risks") {
		issues = append(issues, warnIssue(CodeMissingRisks, "No ## Risks section (optional, but recommended)."))
	}
	if body, ok := p.Sections["problem"]; ok && strings.TrimSpace(body) == "" {
		issues = append(issues, warnIssue(CodeEmptyProblem, "## Problem section is empty."))
	}
	if len(p.Requirements) > MaxRequirements {
		issues = append(issues, warnIssue(CodeTooManyReqs,
			strconv.Itoa(len(p.Requirements))+" requirements (more than "+strconv.Itoa(MaxRequirements)+"); consider splitting the feature."))
	}
	issues = append(issues, reportDuplicates(p.Requirements,
		func(r model.Requirement) string { return strings.ToLower(strings.TrimSpace(r.Text)) },
		model.SeverityWarning, CodeDuplicateReqText,
		func(r model.Requirement, n int) string { return "Duplicate requirement text: " + r.Text })...)
	issues = append(issues, ambiguousVerbIssues(p)...)
	return issues
}

func ambiguousVerbIssues(p *model.Product) []model.Issue {
	var issues []model.Issue
	for _, r := range p.Requirements {
		if found := ambiguousRe.FindAllString(r.Text, -1); len(found) > 0 {
			issues = append(issues, lineIssue(model.SeverityWarning, CodeAmbiguousVerb, r.Line,
				r.ID+" uses ambiguous verb(s) ("+joinLowerUnique(found)+"); be more specific."))
		}
	}
	return issues
}

func requirementStandards(p *model.Product) []model.Issue {
	var issues []model.Issue
	for _, r := range p.Requirements {
		keywords := normativeRe.FindAllString(r.Text, -1)

		// BCP-14/RFC 8174: only ALL-CAPS normative keywords carry weight; a
		// lower/mixed-case shall/must/should is ambiguous normative language.
		var ambiguous []string
		seen := map[string]bool{}
		for _, k := range keywords {
			if k != strings.ToUpper(k) && !seen[k] {
				seen[k] = true
				ambiguous = append(ambiguous, k)
			}
		}
		sort.Strings(ambiguous)
		if len(ambiguous) > 0 {
			issues = append(issues, lineIssue(model.SeverityError, CodeReqNormativeKeyword, r.Line,
				r.ID+" uses non-normative '"+strings.Join(ambiguous, ", ")+
					"'; only uppercase MUST/SHALL/SHOULD/MAY carry normative weight (BCP 14)."))
		}

		// ISO/IEC/IEEE 29148: a requirement should be singular.
		if len(keywords) > 1 {
			issues = append(issues, lineIssue(model.SeverityWarning, CodeReqNotSingular, r.Line,
				r.ID+" has "+strconv.Itoa(len(keywords))+" normative keywords; a requirement should be singular (ISO/IEC/IEEE 29148)."))
		}

		// EARS: a requirement must state a normative response; a sentence-initial
		// "If" needs a "then" response clause.
		if len(keywords) == 0 {
			issues = append(issues, lineIssue(model.SeverityWarning, CodeReqNonEARS, r.Line,
				r.ID+" has no normative keyword (SHALL/SHOULD/MAY); it does not state a testable requirement (EARS)."))
		} else if earsIfRe.MatchString(r.Text) && !thenRe.MatchString(r.Text) {
			issues = append(issues, lineIssue(model.SeverityWarning, CodeReqEARSClause, r.Line,
				r.ID+" opens with 'If' but has no 'then' response clause (EARS unwanted-behaviour pattern)."))
		}
	}
	return issues
}

func validateTicketing(p *model.Product, c classify.Classification, reg artifacts.Registry, provider string) []model.Issue {
	if provider == "" || provider == "none" {
		return nil
	}
	validator, ok := ticketValidators[provider]
	if !ok {
		return nil
	}
	spec, ok := reg[c.Type]
	if !ok || !contains(spec.Optional, "related tickets") {
		return nil
	}
	var issues []model.Issue
	for _, line := range strings.Split(p.Sections["related tickets"], "\n") {
		entry := strings.TrimSpace(listMarker.ReplaceAllString(strings.TrimSpace(line), ""))
		if entry != "" && !validator.valid(entry) {
			issues = append(issues, errIssue(CodeMalformedTicket,
				"## Related Tickets entry '"+entry+"' is not a valid "+validator.label+"."))
		}
	}
	return issues
}

// --- helpers ---------------------------------------------------------------

func reportDuplicates(reqs []model.Requirement, key func(model.Requirement) string,
	sev, code string, msg func(model.Requirement, int) string) []model.Issue {
	counts := map[string]int{}
	for _, r := range reqs {
		counts[key(r)]++
	}
	var issues []model.Issue
	reported := map[string]bool{}
	for _, r := range reqs {
		k := key(r)
		if counts[k] > 1 && !reported[k] {
			reported[k] = true
			issues = append(issues, lineIssue(sev, code, r.Line, msg(r, counts[k])))
		}
	}
	return issues
}

func sectionPresent(p *model.Product, name string) bool {
	_, ok := p.Sections[name]
	return ok
}

func firstValue(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			return s
		}
	}
	return ""
}

func hyphenate(name string) string { return strings.ReplaceAll(name, " ", "-") }

func titleCase(name string) string {
	words := strings.Fields(name)
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func joinLowerUnique(items []string) string {
	seen := map[string]bool{}
	var out []string
	for _, i := range items {
		l := strings.ToLower(i)
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

func contains(items []string, want string) bool {
	for _, i := range items {
		if i == want {
			return true
		}
	}
	return false
}

func errIssue(code, msg string) model.Issue {
	return model.Issue{Severity: model.SeverityError, Code: code, Message: msg}
}
func warnIssue(code, msg string) model.Issue {
	return model.Issue{Severity: model.SeverityWarning, Code: code, Message: msg}
}
func lineIssue(sev, code string, line int, msg string) model.Issue {
	return model.Issue{Severity: sev, Code: code, Message: msg, Line: line}
}
