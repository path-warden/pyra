package codehealth

import (
	"testing"

	"github.com/chasedputnam/memphis/internal/codegraph"
	"github.com/chasedputnam/memphis/internal/gitint"
)

func TestOrganizational_Gates(t *testing.T) {
	// Below every threshold → no findings.
	low := &FileContext{Path: "x.go", Git: &gitint.FileHistory{CommitsTotal: 1, ContributorCount: 1}}
	for _, d := range organizationalDetectors() {
		if got := d(low, &Inputs{}); len(got) != 0 {
			t.Errorf("quiet file should yield no org findings, got %v", got)
		}
	}

	// Churn + entropy + prior-defect gates fire.
	hot := &FileContext{Path: "x.go", Git: &gitint.FileHistory{
		CommitsTotal: 60, ChangeEntropy: 3.0, PriorDefectCount: 4, Commits90d: 5, AgeDays: 400,
	}}
	if len(churnRisk(hot, nil)) == 0 {
		t.Error("churn_risk should fire at 60 commits")
	}
	if len(changeEntropy(hot, nil)) == 0 {
		t.Error("change_entropy should fire at 3.0 bits")
	}
	if len(priorDefect(hot, nil)) == 0 {
		t.Error("prior_defect should fire at 4 fixes")
	}
	if len(codeAgeVolatility(hot, nil)) == 0 {
		t.Error("code_age_volatility should fire on an old, recently-churned file")
	}
}

func TestOrganizational_OwnershipAndKnowledge(t *testing.T) {
	// Dispersed ownership (no majority owner) → ownership_risk.
	disp := &FileContext{Path: "x.go", Git: &gitint.FileHistory{ContributorCount: 4, PrimaryOwnerPct: 0.3}}
	if len(ownershipRisk(disp, nil)) == 0 {
		t.Error("ownership_risk should fire when no author owns a majority")
	}
	// Recent owner differs from historical primary → knowledge_loss.
	kl := &FileContext{Path: "x.go", Git: &gitint.FileHistory{PrimaryOwner: "Ann", RecentOwner: "Bob"}}
	if len(knowledgeLoss(kl, nil)) == 0 {
		t.Error("knowledge_loss should fire when recent owner differs from primary")
	}
}

func TestOrganizational_HiddenCouplingMinusGraph(t *testing.T) {
	git := &gitint.FileHistory{Path: "a.go", CoChange: []gitint.Partner{{Path: "hidden.go", Count: 3}}}
	fc := &FileContext{Path: "a.go", Git: git}
	// No graph edge a.go→hidden.go → hidden coupling.
	g := &codegraph.Graph{FileEdges: map[string][]string{"a.go": {"linked.go"}}}
	if len(hiddenCoupling(fc, &Inputs{Graph: g})) == 0 {
		t.Error("hidden_coupling should fire for a co-change partner with no graph edge")
	}
	// When the partner IS graph-linked → no hidden coupling.
	g2 := &codegraph.Graph{FileEdges: map[string][]string{"a.go": {"hidden.go"}}}
	if len(hiddenCoupling(fc, &Inputs{Graph: g2})) != 0 {
		t.Error("graph-linked co-change is not hidden coupling")
	}
}

func TestOrganizational_NoGitDegrades(t *testing.T) {
	fc := &FileContext{Path: "x.go", Git: nil}
	for _, d := range organizationalDetectors() {
		if got := d(fc, &Inputs{}); len(got) != 0 {
			t.Errorf("no-git file should yield no org findings, got %v", got)
		}
	}
}
