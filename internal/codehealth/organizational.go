package codehealth

// Documented organizational thresholds (pyra-owned, tunable).
const (
	churnRiskCommits     = 20
	changeEntropyBits    = 2.5
	coChangeScatterN     = 5
	coChangeSupport      = 2
	ownershipMinShare    = 0.5
	ageVolatilityDays    = 365
	ageVolatilityRecent  = 3
	devCongestionAuthors = 5
	priorDefectN         = 2
	complexForHotspot    = complexMethodCCN
)

// organizationalDetectors returns the git-derived detectors. Each returns nil
// when the file has no git history (fc.Git == nil).
func organizationalDetectors() []Detector {
	return []Detector{
		churnRisk, changeEntropy, coChangeScatter, ownershipRisk, codeAgeVolatility,
		functionHotspot, developerCongestion, knowledgeLoss, hiddenCoupling, priorDefect,
	}
}

func churnRisk(fc *FileContext, _ *Inputs) []Finding {
	if fc.Git == nil || fc.Git.CommitsTotal < churnRiskCommits {
		return nil
	}
	return one("churn_risk", fc.Path, stepSeverity(fc.Git.CommitsTotal, churnRiskCommits, 50, 100))
}

func changeEntropy(fc *FileContext, _ *Inputs) []Finding {
	if fc.Git == nil || fc.Git.ChangeEntropy < changeEntropyBits {
		return nil
	}
	sev := "medium"
	if fc.Git.ChangeEntropy >= 4.0 {
		sev = "high"
	}
	return one("change_entropy", fc.Path, sev)
}

func coChangeScatter(fc *FileContext, _ *Inputs) []Finding {
	if fc.Git == nil {
		return nil
	}
	strong := 0
	for _, p := range fc.Git.CoChange {
		if p.Count >= coChangeSupport {
			strong++
		}
	}
	if strong < coChangeScatterN {
		return nil
	}
	return one("co_change_scatter", fc.Path, stepSeverity(strong, coChangeScatterN, 10, 20))
}

func ownershipRisk(fc *FileContext, _ *Inputs) []Finding {
	// Dispersed ownership: no single author owns a majority.
	if fc.Git == nil || fc.Git.ContributorCount < 2 || fc.Git.PrimaryOwnerPct >= ownershipMinShare {
		return nil
	}
	return one("ownership_risk", fc.Path, "medium")
}

func codeAgeVolatility(fc *FileContext, _ *Inputs) []Finding {
	if fc.Git == nil || fc.Git.AgeDays < ageVolatilityDays || fc.Git.Commits90d < ageVolatilityRecent {
		return nil
	}
	return one("code_age_volatility", fc.Path, "medium")
}

func functionHotspot(fc *FileContext, _ *Inputs) []Finding {
	if fc.Git == nil || !fc.IsHotspot || !fc.Metrics.Supported {
		return nil
	}
	for _, f := range fc.Metrics.Funcs {
		if f.Cyclomatic >= complexForHotspot {
			return one("function_hotspot", fc.Path, "high")
		}
	}
	return nil
}

func developerCongestion(fc *FileContext, _ *Inputs) []Finding {
	if fc.Git == nil || fc.Git.ContributorCount < devCongestionAuthors {
		return nil
	}
	return one("developer_congestion", fc.Path, "low")
}

func knowledgeLoss(fc *FileContext, _ *Inputs) []Finding {
	// The historical primary owner is no longer the recent contributor.
	g := fc.Git
	if g == nil || g.PrimaryOwner == "" || g.RecentOwner == "" || g.PrimaryOwner == g.RecentOwner {
		return nil
	}
	return one("knowledge_loss", fc.Path, "low")
}

func hiddenCoupling(fc *FileContext, in *Inputs) []Finding {
	if fc.Git == nil || in.Graph == nil {
		return nil
	}
	linked := map[string]bool{}
	for _, dep := range in.Graph.FileEdges[fc.Path] {
		linked[dep] = true
	}
	for _, p := range fc.Git.CoChange {
		if p.Count >= coChangeSupport && !linked[p.Path] {
			return one("hidden_coupling", fc.Path, "medium")
		}
	}
	return nil
}

func priorDefect(fc *FileContext, _ *Inputs) []Finding {
	if fc.Git == nil || fc.Git.PriorDefectCount < priorDefectN {
		return nil
	}
	return one("prior_defect", fc.Path, stepSeverity(fc.Git.PriorDefectCount, priorDefectN, 5, 10))
}

func one(biomarker, file, severity string) []Finding {
	return []Finding{{Biomarker: biomarker, Severity: severity, File: file}}
}
