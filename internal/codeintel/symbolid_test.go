package codeintel

import "testing"

func TestFormatAndParseID_RoundTrip(t *testing.T) {
	id := FormatID("go", "internal/x/y.go", "Method", 42)
	if id != "go:internal/x/y.go#Method@42" {
		t.Fatalf("format: got %q", id)
	}
	path, name, line, ok := ParseID(id)
	if !ok {
		t.Fatal("ParseID returned ok=false")
	}
	if path != "internal/x/y.go" || name != "Method" || line != 42 {
		t.Fatalf("parse: path=%q name=%q line=%d", path, name, line)
	}
}

func TestParseID_ColonInPath(t *testing.T) {
	// The relpath may itself contain a colon; only the first colon is the lang sep.
	path, name, line, ok := ParseID("py:weird:dir/main.py#run@7")
	if !ok {
		t.Fatal("ok=false")
	}
	if path != "weird:dir/main.py" || name != "run" || line != 7 {
		t.Fatalf("path=%q name=%q line=%d", path, name, line)
	}
}

func TestParseID_MissingLine(t *testing.T) {
	path, name, line, ok := ParseID("go:a.go#Foo")
	if !ok || path != "a.go" || name != "Foo" || line != 0 {
		t.Fatalf("ok=%v path=%q name=%q line=%d", ok, path, name, line)
	}
}

func TestParseID_Malformed(t *testing.T) {
	for _, bad := range []string{"nohash", "go:a.go", "", "go:#@1"} {
		if _, _, _, ok := ParseID(bad); ok {
			t.Errorf("expected malformed for %q", bad)
		}
	}
}

func TestParsePos(t *testing.T) {
	path, row, col, ok := ParsePos("src/lib.rs:12:4")
	if !ok {
		t.Fatal("ok=false")
	}
	// 1-based input -> 0-based output.
	if path != "src/lib.rs" || row != 11 || col != 3 {
		t.Fatalf("path=%q row=%d col=%d", path, row, col)
	}
}

func TestParsePos_ColonInPath(t *testing.T) {
	path, row, col, ok := ParsePos("C:/proj/main.go:1:1")
	if !ok || path != "C:/proj/main.go" || row != 0 || col != 0 {
		t.Fatalf("ok=%v path=%q row=%d col=%d", ok, path, row, col)
	}
}

func TestParsePos_Invalid(t *testing.T) {
	for _, bad := range []string{"a.go", "a.go:x:y", "a.go:1"} {
		if _, _, _, ok := ParsePos(bad); ok {
			t.Errorf("expected invalid for %q", bad)
		}
	}
}

func TestRegistry_SupportedLanguages(t *testing.T) {
	langs := DefaultRegistry().SupportedLanguages()
	want := map[string]bool{"go": false, "python": false, "javascript": false, "rust": false}
	for _, l := range langs {
		if _, ok := want[l]; ok {
			want[l] = true
		}
	}
	for l, seen := range want {
		if !seen {
			t.Errorf("language %q not provisioned (got %v)", l, langs)
		}
	}
}

func TestRegistry_UnsupportedLanguage(t *testing.T) {
	if _, err := DefaultRegistry().ForFile("photo.png"); err == nil {
		t.Fatal("expected ErrUnsupportedLanguage for .png")
	}
}
