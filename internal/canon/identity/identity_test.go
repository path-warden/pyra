package identity

import (
	"testing"

	"github.com/chasedputnam/pyra/internal/canon/model"
)

func TestMint_FormatAndDeterminism(t *testing.T) {
	e := [8]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0}
	got1 := Mint("okf", e)
	got2 := Mint("okf", e)
	if got1 != got2 {
		t.Fatalf("mint not deterministic: %q vs %q", got1, got2)
	}
	if !ValidID(got1) {
		t.Errorf("minted id %q failed ValidID", got1)
	}
	if got1[:4] != "OKF-" {
		t.Errorf("repo key not upper-cased/prefixed: %q", got1)
	}
	if len(got1) != len("OKF-")+12 {
		t.Errorf("unexpected length: %q", got1)
	}
}

func TestMint_DifferentEntropyDiffers(t *testing.T) {
	a := Mint("OKF", [8]byte{1})
	b := Mint("OKF", [8]byte{2})
	if a == b {
		t.Error("different entropy produced identical ids")
	}
}

func TestValidID(t *testing.T) {
	valid := []string{"OKF-0123456789AB", "RAC-KTQ63DPSMF19"}
	for _, v := range valid {
		if !ValidID(v) {
			t.Errorf("expected %q valid", v)
		}
	}
	invalid := []string{
		"OKF-SHORT",          // too short
		"okf-0123456789AB",   // lowercase key
		"OKF-0123456789AI",   // I not in Crockford
		"OKF-0123456789AL",   // L not in Crockford
		"OKF-0123456789ABCD", // too long
		"NODASH0123456789AB", // missing dash
	}
	for _, v := range invalid {
		if ValidID(v) {
			t.Errorf("expected %q invalid", v)
		}
	}
}

func TestResolve_ExplicitFrontmatterWins(t *testing.T) {
	p := &model.Product{Metadata: model.Frontmatter{ID: "OKF-0123456789AB"}}
	id, structured := Resolve(p, "adr-001-something.md")
	if id != "OKF-0123456789AB" || !structured {
		t.Errorf("got (%q,%v)", id, structured)
	}
}

func TestResolve_FilenamePrefix(t *testing.T) {
	id, structured := Resolve(&model.Product{}, "/x/y/adr-001-markdown-first.md")
	if id != "adr-001" || !structured {
		t.Errorf("got (%q,%v) want (adr-001,true)", id, structured)
	}
}

func TestResolve_StemFallback(t *testing.T) {
	id, structured := Resolve(&model.Product{}, "/x/notes.md")
	if id != "notes" || structured {
		t.Errorf("got (%q,%v) want (notes,false)", id, structured)
	}
}
