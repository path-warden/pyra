// Package sarif renders []model.Issue as a SARIF 2.1.0 document so the gate can
// emit a single machine-readable report for CI systems.
package sarif

import (
	"sort"

	"github.com/chasedputnam/pyra/internal/canon/model"
)

const schemaURL = "https://json.schemastore.org/sarif-2.1.0.json"

// Document is a minimal SARIF 2.1.0 log.
type Document struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`
	Runs    []Run  `json:"runs"`
}

type Run struct {
	Tool    Tool     `json:"tool"`
	Results []Result `json:"results"`
}

type Tool struct {
	Driver Driver `json:"driver"`
}

type Driver struct {
	Name           string `json:"name"`
	Version        string `json:"version,omitempty"`
	InformationURI string `json:"informationUri,omitempty"`
	Rules          []Rule `json:"rules"`
}

type Rule struct {
	ID string `json:"id"`
}

type Result struct {
	RuleID    string     `json:"ruleId"`
	Level     string     `json:"level"`
	Message   Message    `json:"message"`
	Locations []Location `json:"locations,omitempty"`
}

type Message struct {
	Text string `json:"text"`
}

type Location struct {
	PhysicalLocation PhysicalLocation `json:"physicalLocation"`
}

type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           *Region          `json:"region,omitempty"`
}

type ArtifactLocation struct {
	URI string `json:"uri"`
}

type Region struct {
	StartLine int `json:"startLine"`
}

func level(severity string) string {
	if severity == model.SeverityError {
		return "error"
	}
	return "warning"
}

// FromIssues builds a SARIF document from issues.
func FromIssues(toolName, version string, issues []model.Issue) Document {
	ruleSet := map[string]bool{}
	results := make([]Result, 0, len(issues))

	for _, iss := range issues {
		ruleSet[iss.Code] = true
		res := Result{
			RuleID:  iss.Code,
			Level:   level(iss.Severity),
			Message: Message{Text: iss.Message},
		}
		if iss.Path != "" {
			loc := Location{PhysicalLocation: PhysicalLocation{
				ArtifactLocation: ArtifactLocation{URI: iss.Path},
			}}
			if iss.Line > 0 {
				loc.PhysicalLocation.Region = &Region{StartLine: iss.Line}
			}
			res.Locations = []Location{loc}
		}
		results = append(results, res)
	}

	rules := make([]Rule, 0, len(ruleSet))
	for id := range ruleSet {
		rules = append(rules, Rule{ID: id})
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })

	return Document{
		Schema:  schemaURL,
		Version: "2.1.0",
		Runs: []Run{{
			Tool: Tool{Driver: Driver{
				Name:           toolName,
				Version:        version,
				InformationURI: "https://github.com/chasedputnam/pyra",
				Rules:          rules,
			}},
			Results: results,
		}},
	}
}
