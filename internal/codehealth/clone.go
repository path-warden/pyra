package codehealth

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Clone-detection constants (documented; not learned).
const (
	cloneWindow = 30            // token window size
	cloneBase   = 1099511628211 // FNV-style polynomial base
	cloneMod    = (1 << 61) - 1 // large prime modulus
)

// detectClones runs a deterministic Rabin-Karp pass over normalized token
// windows across all files and returns per-file clone findings. A window that
// hashes equal AND is byte-equal (after normalization) to a window in another
// file is a clone. Test-file clones map to duplicated_assertion_block; others to
// dry_violation.
func detectClones(in *Inputs, contexts []*FileContext) map[string][]Finding {
	if in.Root == "" {
		return nil
	}
	type site struct {
		file string
		toks []string
	}
	var sites []site
	for _, fc := range contexts {
		data, err := os.ReadFile(filepath.Join(in.Root, fc.Path))
		if err != nil {
			continue
		}
		sites = append(sites, site{file: fc.Path, toks: normalizeTokens(string(data))})
	}

	// hash of a window → list of (file, joined tokens) for verification.
	type occ struct {
		file string
		key  string
	}
	seen := map[uint64][]occ{}
	cloned := map[string]bool{} // files involved in a verified clone
	for _, s := range sites {
		if len(s.toks) < cloneWindow {
			continue
		}
		for i := 0; i+cloneWindow <= len(s.toks); i++ {
			win := s.toks[i : i+cloneWindow]
			key := strings.Join(win, " ")
			h := windowHash(win)
			for _, o := range seen[h] {
				if o.file != s.file && o.key == key {
					cloned[s.file] = true
					cloned[o.file] = true
				}
			}
			seen[h] = append(seen[h], occ{file: s.file, key: key})
		}
	}

	out := map[string][]Finding{}
	for _, f := range sortedStrings(cloned) {
		bm := "dry_violation"
		if isTestFile(f) {
			bm = "duplicated_assertion_block"
		}
		out[f] = append(out[f], Finding{Biomarker: bm, Severity: "medium", File: f,
			Details: "duplicated code block detected"})
	}
	return out
}

// normalizeTokens splits source into identifier/operator tokens, folding all
// identifiers/numbers to a single placeholder so structural clones (renamed
// variables) still match, and dropping whitespace.
func normalizeTokens(src string) []string {
	var toks []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			toks = append(toks, "id")
			cur.Reset()
		}
	}
	for _, r := range src {
		switch {
		case r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			cur.WriteRune(r)
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			flush()
			toks = append(toks, string(r))
		}
	}
	flush()
	return toks
}

func windowHash(win []string) uint64 {
	var h uint64
	for _, t := range win {
		for _, b := range []byte(t) {
			h = (h*cloneBase + uint64(b)) % cloneMod
		}
		h = (h*cloneBase + 0x1f) % cloneMod
	}
	return h
}

func isTestFile(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasPrefix(base, "test_") ||
		strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") ||
		strings.HasSuffix(base, "Test.java")
}

func sortedStrings(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
