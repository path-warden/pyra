package util

import "testing"

func TestToPosixPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"foo\\bar\\baz", "foo/bar/baz"},
		{"foo/bar/baz", "foo/bar/baz"},
		{"C:\\Users\\docs", "C:/Users/docs"},
		{"", ""},
	}
	for _, tt := range tests {
		got := ToPosixPath(tt.input)
		if got != tt.want {
			t.Errorf("ToPosixPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStripMdExtension(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"file.md", "file"},
		{"file.MD", "file"},
		{"file.Md", "file"},
		{"file.txt", "file.txt"},
		{"path/to/file.md", "path/to/file"},
		{"noext", "noext"},
	}
	for _, tt := range tests {
		got := StripMdExtension(tt.input)
		if got != tt.want {
			t.Errorf("StripMdExtension(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSafeSegment(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"foo_bar_baz", "foo-bar-baz"},
		{"API Reference", "api-reference"},
		{"file.md", "file.md"},
		{"--leading--trailing--", "leading-trailing"},
		{"", "index"},
		{"   ", "index"},
		{"123-test", "123-test"},
	}
	for _, tt := range tests {
		got := SafeSegment(tt.input)
		if got != tt.want {
			t.Errorf("SafeSegment(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEnsureMarkdownPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "index.md"},
		{"/", "index.md"},
		{"foo", "foo.md"},
		{"foo/bar", "foo/bar.md"},
		{"/foo/bar", "foo/bar.md"},
		{"foo.md", "foo.md"},
		{"foo.html", "foo.md"},
		{"foo.txt", "foo.md"},
		{"docs/guide.mdx", "docs/guide.md"},
	}
	for _, tt := range tests {
		got := EnsureMarkdownPath(tt.input)
		if got != tt.want {
			t.Errorf("EnsureMarkdownPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestURLToOutputPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com/", "index.md"},
		{"https://example.com", "index.md"},
		{"https://example.com/docs", "docs.md"},
		{"https://example.com/docs/", "docs/index.md"},
		{"https://example.com/docs/guide", "docs/guide.md"},
		{"https://example.com/docs/guide/", "docs/guide/index.md"},
		{"/docs/guide", "docs/guide.md"},
	}
	for _, tt := range tests {
		got := URLToOutputPath(tt.input)
		if got != tt.want {
			t.Errorf("URLToOutputPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRelativeMarkdownLink(t *testing.T) {
	tests := []struct {
		from string
		to   string
		want string
	}{
		{"docs/intro.md", "docs/guide.md", "./guide.md"},
		{"docs/intro.md", "api/ref.md", "../api/ref.md"},
		{"intro.md", "guide.md", "./guide.md"},
		{"a/b/c.md", "a/d.md", "../d.md"},
		{"a/b.md", "a/b.md", "./b.md"},
	}
	for _, tt := range tests {
		got := RelativeMarkdownLink(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("RelativeMarkdownLink(%q, %q) = %q, want %q", tt.from, tt.to, got, tt.want)
		}
	}
}
