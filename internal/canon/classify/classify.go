// Package classify infers a Canon artifact's type from the sections it contains,
// ported from rac-core's classification.py.
//
// Scoring: points = matched_required + 0.5*matched_recommended, divided by the
// spec's ceiling (|required| + 0.5*|recommended|). Best fit wins; ties are broken
// by more matched-required sections, then rac-core declaration order. A document
// is "unknown" if the best fit is below CONFIDENCE_THRESHOLD or matches no
// required section.
package classify

import (
	"math"

	"github.com/chasedputnam/pyra/internal/canon/artifacts"
	"github.com/chasedputnam/pyra/internal/canon/model"
)

// ConfidenceThreshold mirrors rac-core CONFIDENCE_THRESHOLD.
const ConfidenceThreshold = 0.5

// Classification is the result of classifying a Product.
type Classification struct {
	Type       string             `json:"type"`
	Confidence float64            `json:"confidence"`
	Scores     map[string]float64 `json:"scores"`
}

type typeScore struct {
	name            string
	matchedRequired int
	fit             float64
}

// Classify scores the product against every spec and returns the best match, or
// "unknown".
func Classify(p *model.Product, reg artifacts.Registry) Classification {
	scores := make(map[string]float64, len(reg))
	var best typeScore
	haveBest := false

	for _, t := range reg.Ordered() { // declaration order → stable tie-break
		spec := reg[t]
		ts := score(p, spec)
		scores[t] = ts.fit
		// Best by fit, then matched-required count; first in declaration order wins
		// remaining ties (strictly-greater comparison preserves earlier entries).
		if !haveBest || ts.fit > best.fit || (ts.fit == best.fit && ts.matchedRequired > best.matchedRequired) {
			best = ts
			haveBest = true
		}
	}

	result := Classification{Type: artifacts.TypeUnknown, Confidence: round2(best.fit), Scores: scores}
	if haveBest && best.fit >= ConfidenceThreshold && best.matchedRequired > 0 {
		result.Type = best.name
	}
	return result
}

func score(p *model.Product, spec artifacts.ArtifactSpec) typeScore {
	ceiling := float64(len(spec.Required)) + 0.5*float64(len(spec.Recommended))
	matchedReq := 0
	for _, s := range spec.Required {
		if s.Present(p.Sections) {
			matchedReq++
		}
	}
	matchedRec := 0
	for _, s := range spec.Recommended {
		if s.Present(p.Sections) {
			matchedRec++
		}
	}
	points := float64(matchedReq) + 0.5*float64(matchedRec)
	fit := 0.0
	if ceiling > 0 {
		fit = points / ceiling
	}
	return typeScore{name: spec.Type, matchedRequired: matchedReq, fit: fit}
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
