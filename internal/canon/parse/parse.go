// Package parse turns a Canon Markdown artifact into a model.Product AST.
//
// Headings are detected structurally with goldmark (a CommonMark parser) so that
// "#" characters inside fenced code blocks are never mistaken for section
// boundaries and setext/ATX forms are handled the same way a real Markdown
// renderer would. Section bodies are then sliced from the raw source as text,
// because the authority path stores and compares raw section content.
package parse

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"

	"github.com/chasedputnam/pyra/internal/canon/model"
)

var (
	mdParser     = goldmark.New()
	trailingHash = regexp.MustCompile(`\s+#+\s*$`)
	// listMarker matches a single leading Markdown list marker.
	listMarker = regexp.MustCompile(`^(?:[-*+]|\d+\.)\s+`)
	// bracketRe captures a leading [..] id token and the trailing description,
	// matching rac-core's _BRACKET_RE so a malformed ID is distinguishable from a
	// missing one.
	bracketRe = regexp.MustCompile(`^\[([^\]]*)\]\s*(.*)$`)
	// canonicalID is rac-core's _CANONICAL_ID_RE: REQ-<digits>.
	canonicalID = regexp.MustCompile(`^REQ-\d+$`)
)

// Normalize canonicalizes a heading for use as a section key: lower-cased with
// runs of whitespace collapsed to single spaces.
func Normalize(heading string) string {
	return strings.ToLower(strings.Join(strings.Fields(heading), " "))
}

// Parse builds a Product from a raw Canon artifact's bytes.
func Parse(raw []byte) *model.Product {
	p := &model.Product{Sections: map[string]string{}}

	fm, body, bodyStartLine, present := splitFrontmatter(raw)
	p.Metadata.Present = present
	if present {
		// Lenient best-effort decode; canon/frontmatter performs strict hardening.
		_ = yaml.Unmarshal(fm, &p.Metadata)
	}

	heads := headings(body)

	// Title is the first level-1 heading; count them all so validation can
	// enforce "exactly one title".
	for _, h := range heads {
		if h.level == 1 {
			if p.TitleCount == 0 {
				p.Title = h.text
			}
			p.TitleCount++
		}
	}

	// Level-2 headings delimit sections; their body runs until the next heading
	// of level <= 2 (a new section or a new title).
	for i, h := range heads {
		if h.level != 2 {
			continue
		}
		end := len(body)
		for j := i + 1; j < len(heads); j++ {
			if heads[j].level <= 2 {
				end = heads[j].lineStart
				break
			}
		}
		sectionBody := strings.TrimSpace(string(body[h.contentStart:end]))
		key := Normalize(h.text)
		p.Sections[key] = sectionBody
		p.Order = append(p.Order, key)

		if key == "requirements" {
			parseRequirements(p, body, h.contentStart, end, bodyStartLine)
		}
	}

	return p
}

type headingInfo struct {
	level        int
	text         string
	lineStart    int // byte offset of the start of the heading's line
	contentStart int // byte offset of the first byte after the heading line
}

func headings(body []byte) []headingInfo {
	reader := text.NewReader(body)
	doc := mdParser.Parser().Parse(reader)

	var heads []headingInfo
	// Headings are always top-level blocks in goldmark.
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		h, ok := c.(*ast.Heading)
		if !ok {
			continue
		}
		start, ok := firstTextOffset(h)
		if !ok {
			continue // empty heading; not a usable section boundary
		}
		ls := lineStart(body, start)
		le := lineEnd(body, start)
		raw := strings.TrimSpace(string(body[ls:le]))
		cs := le + 1
		if cs > len(body) {
			cs = len(body)
		}
		// Skip a setext underline line if present so it is not treated as body.
		if h.Level <= 2 {
			if nextLs, nextLe, ok := lineRange(body, cs); ok {
				underline := strings.TrimSpace(string(body[nextLs:nextLe]))
				if isSetextUnderline(underline) {
					cs = nextLe + 1
					if cs > len(body) {
						cs = len(body)
					}
				}
			}
		}
		heads = append(heads, headingInfo{
			level:        h.Level,
			text:         stripATX(raw),
			lineStart:    ls,
			contentStart: cs,
		})
	}
	return heads
}

func firstTextOffset(n ast.Node) (int, bool) {
	min := -1
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if t, ok := c.(*ast.Text); ok {
			if min == -1 || t.Segment.Start < min {
				min = t.Segment.Start
			}
		}
		return ast.WalkContinue, nil
	})
	if min == -1 {
		return 0, false
	}
	return min, true
}

func stripATX(line string) string {
	s := strings.TrimSpace(line)
	s = strings.TrimLeft(s, "#")
	s = strings.TrimSpace(s)
	s = trailingHash.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func isSetextUnderline(s string) bool {
	if s == "" {
		return false
	}
	all := func(r byte) bool {
		for i := 0; i < len(s); i++ {
			if s[i] != r {
				return false
			}
		}
		return true
	}
	return all('=') || all('-')
}

func lineStart(b []byte, offset int) int {
	if offset > len(b) {
		offset = len(b)
	}
	if i := bytes.LastIndexByte(b[:offset], '\n'); i >= 0 {
		return i + 1
	}
	return 0
}

func lineEnd(b []byte, offset int) int {
	if offset >= len(b) {
		return len(b)
	}
	if i := bytes.IndexByte(b[offset:], '\n'); i >= 0 {
		return offset + i
	}
	return len(b)
}

func lineRange(b []byte, offset int) (start, end int, ok bool) {
	if offset >= len(b) {
		return 0, 0, false
	}
	return offset, lineEnd(b, offset), true
}

// parseRequirements extracts [REQ-NNN] lines within [start,end) of body.
func parseRequirements(p *model.Product, body []byte, start, end, bodyStartLine int) {
	if start > len(body) {
		return
	}
	if end > len(body) {
		end = len(body)
	}
	section := body[start:end]
	baseLine := bodyStartLine + bytes.Count(body[:start], []byte{'\n'})

	// Every non-blank line in the Requirements section is a requirement candidate
	// (rac-core semantics): a leading [...] id token is required, else the line is
	// malformed (missing id). Sub-headings and fenced code blocks are skipped —
	// markdown-it excludes them from requirement candidates, so we do too.
	// Duplicates are kept; validation reports duplicate-req-id by counting.
	lines := strings.Split(string(section), "\n")
	inFence := false
	var fenceMarker string
	for i, line := range lines {
		fileLine := baseLine + i
		s := strings.TrimSpace(line)
		// Track fenced code blocks (``` or ~~~) and skip their contents.
		if marker := fenceOpener(s); marker != "" {
			if !inFence {
				inFence = true
				fenceMarker = marker
				continue
			}
			if strings.HasPrefix(s, fenceMarker) {
				inFence = false
				fenceMarker = ""
			}
			continue
		}
		if inFence {
			continue
		}
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		s = strings.TrimSpace(listMarker.ReplaceAllString(s, ""))
		if s == "" {
			continue
		}
		m := bracketRe.FindStringSubmatch(s)
		if m == nil {
			p.Malformed = append(p.Malformed, model.MalformedRequirement{
				Raw: s, Line: fileLine, Reason: "missing-id",
			})
			continue
		}
		id := strings.TrimSpace(m[1])
		desc := strings.TrimSpace(m[2])
		switch {
		case !canonicalID.MatchString(id):
			p.Malformed = append(p.Malformed, model.MalformedRequirement{
				Raw: s, Line: fileLine, Reason: "bad-id", BadID: id,
			})
		case desc == "":
			p.Malformed = append(p.Malformed, model.MalformedRequirement{
				Raw: s, Line: fileLine, Reason: "empty-text", BadID: id,
			})
		default:
			p.Requirements = append(p.Requirements, model.Requirement{ID: id, Text: desc, Line: fileLine})
		}
	}
}

// fenceOpener returns the fence marker ("```" or "~~~") if the line opens or
// closes a fenced code block, else "".
func fenceOpener(s string) string {
	switch {
	case strings.HasPrefix(s, "```"):
		return "```"
	case strings.HasPrefix(s, "~~~"):
		return "~~~"
	}
	return ""
}

// splitFrontmatter separates a leading "---" YAML block from the body. It
// returns the frontmatter bytes, the body bytes, the 1-based line number on
// which the body begins, and whether a frontmatter block was present.
func splitFrontmatter(raw []byte) (fm, body []byte, bodyStartLine int, present bool) {
	lines := strings.SplitAfter(string(raw), "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r\n") != "---" {
		return nil, raw, 1, false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r\n") == "---" {
			fmStr := strings.Join(lines[1:i], "")
			bodyStr := strings.Join(lines[i+1:], "")
			return []byte(fmStr), []byte(bodyStr), i + 2, true
		}
	}
	return nil, raw, 1, false
}
