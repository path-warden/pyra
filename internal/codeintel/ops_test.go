package codeintel

import (
	"os"
	"path/filepath"
	"testing"
)

const goFixture = `package sample

import "fmt"

type Greeter struct{ name string }

func (g Greeter) Greet() string {
	return fmt.Sprintf("hi %s", g.name)
}

func New(name string) Greeter {
	return Greeter{name: name}
}

func main() {
	g := New("x")
	println(g.Greet())
}
`

func writeFixture(t *testing.T, name, content string) (dir, file string) {
	t.Helper()
	dir = t.TempDir()
	file = filepath.Join(dir, name)
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, file
}

func TestOutline_Go(t *testing.T) {
	dir, file := writeFixture(t, "sample.go", goFixture)
	o := NewOps(nil, dir)
	got, err := o.Outline(file, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{} // name -> kind
	for _, m := range got {
		names[m["name"].(string)] = m["kind"].(string)
	}
	for name, kind := range map[string]string{"Greet": "method", "New": "function", "main": "function", "Greeter": "type"} {
		if names[name] != kind {
			t.Errorf("outline: %s want kind %q, got %q (all=%v)", name, kind, names[name], names)
		}
	}
	// detail 1 includes an id and signature.
	for _, m := range got {
		if _, ok := m["id"]; !ok {
			t.Errorf("detail 1 missing id: %v", m)
		}
	}
}

func TestOutline_DetailTiers(t *testing.T) {
	_, file := writeFixture(t, "sample.go", goFixture)
	o := NewOps(nil, filepath.Dir(file))
	terse, _ := o.Outline(file, "", 0)
	if len(terse) == 0 {
		t.Fatal("no symbols")
	}
	if _, ok := terse[0]["id"]; ok {
		t.Error("detail 0 must not include id")
	}
	full, _ := o.Outline(file, "", 2)
	if _, ok := full[0]["start_byte"]; !ok {
		t.Error("detail 2 must include start_byte")
	}
}

func TestSymbols_KindAndNameFilter(t *testing.T) {
	dir, _ := writeFixture(t, "sample.go", goFixture)
	o := NewOps(nil, dir)
	fns, err := o.Symbols(dir, "function", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range fns {
		if s.Kind != "function" {
			t.Errorf("kind filter leaked: %+v", s)
		}
	}
	// exact name match is case-insensitive.
	greet, _ := o.Symbols(dir, "", "greet", false, false)
	found := false
	for _, s := range greet {
		if s.Name == "Greet" {
			found = true
		}
	}
	if !found {
		t.Error("expected case-insensitive exact match on Greet")
	}
	// substring match.
	sub, _ := o.Symbols(dir, "", "gree", false, true)
	if len(sub) == 0 {
		t.Error("expected substring match for 'gree'")
	}
}

func TestSource_BySymbolID(t *testing.T) {
	dir, _ := writeFixture(t, "sample.go", goFixture)
	o := NewOps(nil, dir)
	syms, _ := o.Symbols(dir, "function", "New", false, false)
	if len(syms) == 0 {
		t.Fatal("New not found")
	}
	res, err := o.Source(syms[0].ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Source == "" || !containsStr(res.Source, "return Greeter{name: name}") {
		t.Errorf("source body wrong: %q", res.Source)
	}
}

func TestCheck_ReportsSyntaxError(t *testing.T) {
	dir, file := writeFixture(t, "broken.go", "package x\nfunc f( {\n")
	o := NewOps(nil, dir)
	defects, err := o.Check(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(defects) == 0 {
		t.Fatal("expected defects for broken syntax")
	}
}

func TestCheck_CleanFile(t *testing.T) {
	dir, file := writeFixture(t, "ok.go", goFixture)
	o := NewOps(nil, dir)
	defects, _ := o.Check(file)
	if len(defects) != 0 {
		t.Errorf("expected no defects, got %v", defects)
	}
}

func TestCallers_StructuralAndTextual(t *testing.T) {
	dir, _ := writeFixture(t, "sample.go", goFixture)
	o := NewOps(nil, dir)
	sites, err := o.Callers(dir, "Greet")
	if err != nil {
		t.Fatal(err)
	}
	var structural, textual int
	for _, s := range sites {
		switch s.Source {
		case "structural":
			structural++
		case "textual":
			textual++
		}
	}
	if structural == 0 {
		t.Errorf("expected a structural call site for Greet, got %+v", sites)
	}
}

func TestMap_ReferencesAttributed(t *testing.T) {
	dir, _ := writeFixture(t, "sample.go", goFixture)
	o := NewOps(nil, dir)
	maps, err := o.Map(dir, "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	// New references Greeter (its return construction is not a call, but main
	// references New and Greet). Assert main's entry lists outgoing refs.
	var mainRefs []string
	for _, fm := range maps {
		for _, e := range fm.Entries {
			if e.Name == "main" {
				mainRefs = e.References
			}
		}
	}
	if !containsInSlice(mainRefs, "New") {
		t.Errorf("expected main to reference New, got %v", mainRefs)
	}
}

func TestDefinition_NameMode(t *testing.T) {
	dir, _ := writeFixture(t, "sample.go", goFixture)
	o := NewOps(nil, dir)
	res, err := o.Definition("New", "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Resolved != "New" || len(res.Definitions) == 0 {
		t.Fatalf("definition name-mode: %+v", res)
	}
	if res.Definitions[0].Kind != "function" {
		t.Errorf("expected function, got %q", res.Definitions[0].Kind)
	}
}

func TestDefinition_AtFallsBackToName(t *testing.T) {
	dir, file := writeFixture(t, "sample.go", goFixture)
	o := NewOps(nil, dir)
	// Position of "New" usage inside main: line 17, col ~7 (1-based).
	// Find it robustly by scanning.
	pos := findUsage(t, file, "New")
	res, err := o.Definition("", pos, dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Resolved != "New" {
		t.Fatalf("resolved=%q want New (pos=%s)", res.Resolved, pos)
	}
	if len(res.Definitions) == 0 {
		t.Errorf("expected New definition via --at, got none")
	}
}

func TestDeterminism_IdenticalOutput(t *testing.T) {
	dir, _ := writeFixture(t, "sample.go", goFixture)
	o := NewOps(nil, dir)
	a, _ := o.Symbols(dir, "", "", true, false)
	b, _ := o.Symbols(dir, "", "", true, false)
	if len(a) != len(b) {
		t.Fatalf("len differ %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Errorf("nondeterministic at %d: %q vs %q", i, a[i].ID, b[i].ID)
		}
	}
}

// --- helpers ---

func containsStr(hay, needle string) bool {
	return len(hay) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(hay); i++ {
			if hay[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}

func containsInSlice(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

// findUsage returns "file:line:col" (1-based) of the first whole-word usage of
// name that is not the definition line.
func findUsage(t *testing.T, file, name string) string {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	lines := splitLines(string(src))
	for i, line := range lines {
		if col, ok := wholeWordCol(line, name); ok {
			// skip the "func New" definition line
			if containsStr(line, "func "+name) {
				continue
			}
			return file + ":" + itoa(i+1) + ":" + itoa(col+1)
		}
	}
	t.Fatalf("usage of %q not found", name)
	return ""
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
