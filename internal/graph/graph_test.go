package graph

import (
	"testing"

	"github.com/okfy/okf-mcp/internal/types"
)

func TestExtractInternalLinks(t *testing.T) {
	tests := []struct {
		name    string
		concept *types.Concept
		want    []string
	}{
		{
			name: "simple relative link",
			concept: &types.Concept{
				Path: "docs/intro.md",
				Body: "See [other](./other.md) for more.",
			},
			want: []string{"docs/other"},
		},
		{
			name: "absolute link",
			concept: &types.Concept{
				Path: "docs/intro.md",
				Body: "See [guide](/guides/start.md) for more.",
			},
			want: []string{"guides/start"},
		},
		{
			name: "parent directory link",
			concept: &types.Concept{
				Path: "docs/nested/intro.md",
				Body: "See [parent](../other.md) for more.",
			},
			want: []string{"docs/other"},
		},
		{
			name: "skip external URLs",
			concept: &types.Concept{
				Path: "intro.md",
				Body: "See [Google](https://google.com) and [local](./local.md).",
			},
			want: []string{"local"},
		},
		{
			name: "skip anchor only",
			concept: &types.Concept{
				Path: "intro.md",
				Body: "See [section](#section) and [local](./local.md).",
			},
			want: []string{"local"},
		},
		{
			name: "strip hash from link",
			concept: &types.Concept{
				Path: "intro.md",
				Body: "See [section](./other.md#section).",
			},
			want: []string{"other"},
		},
		{
			name: "multiple links",
			concept: &types.Concept{
				Path: "intro.md",
				Body: "See [a](./a.md) and [b](./b.md) and [c](./c.md).",
			},
			want: []string{"a", "b", "c"},
		},
		{
			name: "skip mailto",
			concept: &types.Concept{
				Path: "intro.md",
				Body: "Contact [us](mailto:test@example.com) or see [docs](./docs.md).",
			},
			want: []string{"docs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractInternalLinks(tt.concept)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractInternalLinks() = %v, want %v", got, tt.want)
				return
			}
			for i, link := range got {
				if link != tt.want[i] {
					t.Errorf("ExtractInternalLinks()[%d] = %q, want %q", i, link, tt.want[i])
				}
			}
		})
	}
}

func TestBuildGraph(t *testing.T) {
	concepts := map[string]*types.Concept{
		"intro": {
			ID:   "intro",
			Path: "intro.md",
			Body: "See [guide](./guide.md) and [api](./api.md).",
		},
		"guide": {
			ID:   "guide",
			Path: "guide.md",
			Body: "Back to [intro](./intro.md).",
		},
		"api": {
			ID:   "api",
			Path: "api.md",
			Body: "No links here.",
		},
	}

	graph := BuildGraph(concepts)

	// Check outbound links
	if len(graph.Outbound["intro"]) != 2 {
		t.Errorf("Outbound[intro] = %v, want 2 links", graph.Outbound["intro"])
	}
	if len(graph.Outbound["guide"]) != 1 {
		t.Errorf("Outbound[guide] = %v, want 1 link", graph.Outbound["guide"])
	}
	if len(graph.Outbound["api"]) != 0 {
		t.Errorf("Outbound[api] = %v, want 0 links", graph.Outbound["api"])
	}

	// Check backlinks
	if len(graph.Backlinks["intro"]) != 1 {
		t.Errorf("Backlinks[intro] = %v, want 1 link", graph.Backlinks["intro"])
	}
	if len(graph.Backlinks["guide"]) != 1 {
		t.Errorf("Backlinks[guide] = %v, want 1 link", graph.Backlinks["guide"])
	}
	if len(graph.Backlinks["api"]) != 1 {
		t.Errorf("Backlinks[api] = %v, want 1 link", graph.Backlinks["api"])
	}
}

func TestBuildGraphFiltersNonExistent(t *testing.T) {
	concepts := map[string]*types.Concept{
		"intro": {
			ID:   "intro",
			Path: "intro.md",
			Body: "See [missing](./missing.md) and [guide](./guide.md).",
		},
		"guide": {
			ID:   "guide",
			Path: "guide.md",
			Body: "Content.",
		},
	}

	graph := BuildGraph(concepts)

	// Should only have link to guide, not missing
	if len(graph.Outbound["intro"]) != 1 {
		t.Errorf("Outbound[intro] = %v, want 1 link (missing should be filtered)", graph.Outbound["intro"])
	}
	if graph.Outbound["intro"][0] != "guide" {
		t.Errorf("Outbound[intro][0] = %q, want %q", graph.Outbound["intro"][0], "guide")
	}
}
