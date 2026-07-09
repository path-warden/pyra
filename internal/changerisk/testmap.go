package changerisk

import (
	"path"
	"strings"
)

// isSourceNeedingTest reports whether a changed path is a source file for which a
// companion test file is a reasonable expectation (a recognized language, and not
// itself a test file). Languages whose tests live in-file (Rust) return false.
func isSourceNeedingTest(p string) bool {
	if isTestFile(p) {
		return false
	}
	switch ext(p) {
	case ".go", ".py", ".ts", ".tsx", ".js", ".jsx", ".java":
		return true
	}
	return false // .rs (in-file tests), unknown languages, config, etc.
}

// isTestFile reports whether a path is itself a test file by convention.
func isTestFile(p string) bool {
	base := path.Base(p)
	switch {
	case strings.HasSuffix(base, "_test.go"):
		return true
	case strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py"):
		return true
	case strings.HasSuffix(base, "_test.py"):
		return true
	case containsAny(base, ".test.", ".spec."):
		return true
	case strings.HasSuffix(base, "Test.java"):
		return true
	}
	// Anything under a top-level tests/ or test/ directory.
	segs := strings.Split(p, "/")
	for _, s := range segs[:max0(len(segs)-1)] {
		if s == "tests" || s == "test" || s == "__tests__" {
			return true
		}
	}
	return false
}

// testCandidates returns the conventional test-file path(s) for a source file.
// An empty result means the language demands no separate test file.
func testCandidates(p string) []string {
	dir := path.Dir(p)
	base := path.Base(p)
	stem := strings.TrimSuffix(base, ext(p))
	join := func(name string) string {
		if dir == "." || dir == "" {
			return name
		}
		return dir + "/" + name
	}
	switch ext(p) {
	case ".go":
		return []string{join(stem + "_test.go")}
	case ".py":
		return []string{join("test_" + base), join(stem + "_test.py"), "tests/test_" + base}
	case ".ts", ".tsx", ".js", ".jsx":
		e := ext(p)
		return []string{join(stem + ".test" + e), join(stem + ".spec" + e)}
	case ".java":
		return []string{join(stem + "Test.java")}
	}
	return nil
}

func ext(p string) string { return strings.ToLower(path.Ext(p)) }

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
