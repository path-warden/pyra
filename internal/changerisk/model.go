package changerisk

import "math"

// RiskDriver is one feature's signed contribution to the change-risk logit.
type RiskDriver struct {
	Feature      string
	Value        float64 // raw feature value (NaN when unknown)
	Contribution float64 // signed push on the logit (coef * standardized value)
	Label        string  // human-readable, relative to the model's baseline commit
	Known        bool    // false when the feature was unavailable (e.g. exp)
}

// ChangeRisk is the raw scoring result for one change.
type ChangeRisk struct {
	Score       float64 // 0–10 (round(10*prob, 1))
	Probability float64 // logistic probability
	Drivers     []RiskDriver
}

// TopDrivers returns the drivers sorted by absolute contribution (strongest
// first), stable on ties by feature order.
func (r ChangeRisk) TopDrivers() []RiskDriver {
	out := append([]RiskDriver(nil), r.Drivers...)
	// Insertion sort keeps it stable and dependency-free.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && math.Abs(out[j].Contribution) > math.Abs(out[j-1].Contribution); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// ScoreChange scores a ChangeFeatures vector. The model is a plain L2-logistic
// over standardized, optionally log1p'd features:
//
//	logit = intercept + Σ coef_i · z_i,  z_i = (x_i − mean_i) / std_i
//
// Each coef_i·z_i is reported as an attributable RiskDriver. An unknown feature
// (e.g. author experience on a staged diff) contributes nothing rather than
// imputing a value. Deterministic: identical features → identical result.
func ScoreChange(f ChangeFeatures) ChangeRisk {
	c := portedConstants
	logit := c.Intercept
	drivers := make([]RiskDriver, 0, len(c.Features))
	for i, name := range c.Features {
		raw, known := rawFeature(f, name)
		if !known {
			drivers = append(drivers, RiskDriver{
				Feature: name, Value: math.NaN(), Contribution: 0, Label: name + " (unknown)", Known: false,
			})
			continue
		}
		x := raw
		if c.Log1p[i] {
			x = math.Log1p(raw)
		}
		std := c.Std[i]
		if std == 0 {
			std = 1
		}
		z := (x - c.Mean[i]) / std
		contribution := c.Coef[i] * z
		logit += contribution
		drivers = append(drivers, RiskDriver{
			Feature: name, Value: raw, Contribution: contribution,
			Label: driverLabel(name, x >= c.Mean[i]), Known: true,
		})
	}
	prob := sigmoid(logit)
	return ChangeRisk{
		Score:       math.Round(10*prob*10) / 10, // round(10*prob, 1)
		Probability: prob,
		Drivers:     drivers,
	}
}

// rawFeature returns the raw value of a named feature and whether it is known.
func rawFeature(f ChangeFeatures, name string) (float64, bool) {
	switch name {
	case "la":
		return float64(f.LA), true
	case "ld":
		return float64(f.LD), true
	case "nf":
		return float64(f.NF), true
	case "nd":
		return float64(f.ND), true
	case "ns":
		return float64(f.NS), true
	case "entropy":
		return f.Entropy, true
	case "exp":
		if f.Exp == nil {
			return 0, false
		}
		return float64(*f.Exp), true
	}
	return 0, false
}

func sigmoid(z float64) float64 {
	if z >= 0 {
		return 1.0 / (1.0 + math.Exp(-z))
	}
	e := math.Exp(z)
	return e / (1.0 + e)
}

// driverLabel describes a feature's standing relative to the model's baseline
// commit. The signed contribution (not this label) carries the risk direction.
func driverLabel(feature string, aboveBaseline bool) string {
	pair, ok := featureLabels[feature]
	if !ok {
		return feature
	}
	if aboveBaseline {
		return pair[0]
	}
	return pair[1]
}

// featureLabels holds (above-baseline, below-baseline) phrases per feature.
var featureLabels = map[string][2]string{
	"la":      {"more lines added than baseline", "fewer lines added than baseline"},
	"ld":      {"more lines deleted than baseline", "fewer lines deleted than baseline"},
	"nf":      {"more files than baseline", "fewer files than baseline"},
	"nd":      {"more directories than baseline", "fewer directories than baseline"},
	"ns":      {"more subsystems than baseline", "fewer subsystems than baseline"},
	"entropy": {"more scattered than baseline", "more focused than baseline"},
	"exp":     {"more experienced author than baseline", "less familiar author than baseline"},
}
