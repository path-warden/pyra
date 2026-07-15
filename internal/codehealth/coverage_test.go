package codehealth

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestParseCoverage_LCOV(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cov.info")
	if err := os.WriteFile(p, []byte("SF:a.go\nDA:1,3\nDA:2,0\nDA:3,1\nend_of_record\nSF:b.go\nDA:1,0\nend_of_record\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cov, err := ParseCoverage(p)
	if err != nil {
		t.Fatal(err)
	}
	if a := cov["a.go"]; a.Covered != 2 || a.Total != 3 || math.Abs(a.Rate-2.0/3) > 1e-9 {
		t.Errorf("a.go = %+v, want 2/3", a)
	}
	if b := cov["b.go"]; b.Rate != 0 {
		t.Errorf("b.go rate = %v, want 0", b.Rate)
	}
}

func TestParseCoverage_Cobertura(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cobertura.xml")
	if err := os.WriteFile(p, []byte(`<?xml version="1.0"?><coverage><classes><class filename="x.go" line-rate="0.8"></class></classes></coverage>`), 0o644); err != nil {
		t.Fatal(err)
	}
	cov, err := ParseCoverage(p)
	if err != nil {
		t.Fatal(err)
	}
	if x := cov["x.go"]; math.Abs(x.Rate-0.8) > 1e-9 {
		t.Errorf("x.go rate = %v, want 0.8", x.Rate)
	}
}

func TestCoverageDetectors_Gates(t *testing.T) {
	// Poorly covered hotspot → gap + gradient + untested_hotspot.
	fc := &FileContext{Path: "x.go", IsHotspot: true, Coverage: &FileCoverage{Rate: 0.2}}
	if len(coverageGap(fc, nil)) == 0 {
		t.Error("coverage_gap should fire below 50%")
	}
	grad := coverageGradient(fc, nil)
	if len(grad) == 0 || grad[0].Deduction == nil {
		t.Fatal("coverage_gradient should carry a continuous deduction")
	}
	if math.Abs(*grad[0].Deduction-4.0*0.8) > 1e-9 {
		t.Errorf("gradient deduction = %v, want 3.2", *grad[0].Deduction)
	}
	if len(untestedHotspot(fc, nil)) == 0 {
		t.Error("untested_hotspot should fire on a poorly-covered hotspot")
	}
	// Well-covered file → nothing.
	good := &FileContext{Path: "y.go", IsHotspot: true, Coverage: &FileCoverage{Rate: 0.95}}
	if len(coverageGap(good, nil)) != 0 || len(untestedHotspot(good, nil)) != 0 {
		t.Error("well-covered file should have no coverage findings")
	}
}

func TestCoverageDetectors_OmittedWithoutReport(t *testing.T) {
	fc := &FileContext{Path: "x.go", IsHotspot: true, Coverage: nil}
	for _, d := range coverageDetectors() {
		if got := d(fc, nil); len(got) != 0 {
			t.Errorf("no coverage report → coverage biomarkers omitted, got %v", got)
		}
	}
}
