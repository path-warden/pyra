package mcp

import (
	"context"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chasedputnam/memphis/internal/codeintel"
	"github.com/chasedputnam/memphis/internal/store"
)

// registerCodeIntelTools adds the read-only structural code-intelligence tools
// (a native Go port of grove) plus the Canon<->code grounding tools. These
// function even when no Canon store is loaded; grounding degrades when it is.
func (s *Server) registerCodeIntelTools() {
	s.mcpServer.AddTool(mcp.NewTool("outline",
		mcp.WithDescription("List the definitions in one source file as a compact skeleton (kind, name, parent container, signature, position, and a stable symbol-id). Use this instead of reading a whole file."),
		mcp.WithString("file", mcp.Required(), mcp.Description("Path to a source file")),
		mcp.WithString("kind", mcp.Description("Only this kind, e.g. class, function, method")),
		mcp.WithNumber("detail", mcp.Description("0 terse · 1 default · 2 full. Default 1.")),
	), s.handleOutline)

	s.mcpServer.AddTool(mcp.NewTool("symbols",
		mcp.WithDescription("Find symbols across a directory (gitignore-aware). name matches exactly (case-insensitive); set nameContains for substring. Returns stable symbol-ids."),
		mcp.WithString("dir", mcp.Required(), mcp.Description("Directory to search")),
		mcp.WithString("kind", mcp.Description("Only this kind")),
		mcp.WithString("name", mcp.Description("Only definitions whose name equals this (case-insensitive)")),
		mcp.WithBoolean("nameContains", mcp.Description("Substring matching for name")),
		mcp.WithBoolean("refs", mcp.Description("Include references, not just definitions")),
	), s.handleSymbols)

	s.mcpServer.AddTool(mcp.NewTool("source",
		mcp.WithDescription("Return the exact full source of one symbol — by its symbol-id, or by file + name. Returns { id, source, other_candidates? }."),
		mcp.WithString("id", mcp.Description("A symbol-id like <lang>:<relpath>#<name>@<line> (line 1-based)")),
		mcp.WithString("file", mcp.Description("Alternatively, the source file (with name)")),
		mcp.WithString("name", mcp.Description("...and the symbol name to find in it")),
	), s.handleSource)

	s.mcpServer.AddTool(mcp.NewTool("check",
		mcp.WithDescription("Parse a source file and report syntax errors (ERROR/MISSING nodes) with positions. Empty array means syntactically valid."),
		mcp.WithString("file", mcp.Required(), mcp.Description("Path to a source file")),
	), s.handleCheck)

	s.mcpServer.AddTool(mcp.NewTool("callers",
		mcp.WithDescription("Find every reference to a symbol by name across a directory, with enclosing function. Results are 'structural' (tree-sitter) or 'textual' (whole-word grep)."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Function/method name to find references to")),
		mcp.WithString("dir", mcp.Description("Directory to search (default: current)")),
	), s.handleCallers)

	s.mcpServer.AddTool(mcp.NewTool("map",
		mcp.WithDescription("Compact structural map of a directory: every definition grouped by file, with each definition's outgoing references. No source bodies."),
		mcp.WithString("dir", mcp.Required(), mcp.Description("Directory to map")),
		mcp.WithString("kind", mcp.Description("Only definitions of this kind")),
		mcp.WithString("name", mcp.Description("Only definitions whose name equals this")),
		mcp.WithBoolean("nameContains", mcp.Description("Substring matching for name")),
	), s.handleMap)

	s.mcpServer.AddTool(mcp.NewTool("definition",
		mcp.WithDescription("Find where a symbol is defined. name: exact-name lookup. at (file:line:col, 1-based): scope-aware, cross-file resolution of the identifier under a usage site, falling back to name lookup."),
		mcp.WithString("name", mcp.Description("Exact symbol name to resolve (provide this or at)")),
		mcp.WithString("at", mcp.Description("Usage site to resolve: file:line:col (1-based)")),
		mcp.WithString("dir", mcp.Description("Directory to search (default: current)")),
	), s.handleDefinition)

	// Grounding: bridge authoritative Canon and real code. Read-only.
	s.mcpServer.AddTool(mcp.NewTool("code_for_artifact",
		mcp.WithDescription("Resolve the code symbols a Canon artifact references. Scans the artifact body for symbol-ids and returns each symbol's source; unresolvable references are listed separately."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Canon artifact ID")),
	), s.handleCodeForArtifact)

	s.mcpServer.AddTool(mcp.NewTool("artifacts_for_symbol",
		mcp.WithDescription("Find Canon artifacts (decisions/requirements/designs) that reference a code symbol-id or file path."),
		mcp.WithString("id", mcp.Description("A symbol-id or file path to search Canon for")),
	), s.handleArtifactsForSymbol)
}

func (s *Server) ops() *codeintel.Ops {
	return s.code
}

func (s *Server) handleOutline(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := r.Params.Arguments.(map[string]any)
	file := getArgString(args, "file")
	kind := getArgString(args, "kind")
	detail := int(getArgFloat(args, "detail"))
	if _, ok := args["detail"]; !ok {
		detail = 1
	}
	rows, err := s.ops().Outline(file, kind, detail)
	if err != nil {
		return errorResult(err), nil
	}
	return s.jsonResult(rows)
}

func (s *Server) handleSymbols(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := r.Params.Arguments.(map[string]any)
	syms, err := s.ops().Symbols(
		getArgString(args, "dir"), getArgString(args, "kind"), getArgString(args, "name"),
		getArgBool(args, "refs", false), getArgBool(args, "nameContains", false))
	if err != nil {
		return errorResult(err), nil
	}
	return s.jsonResult(syms)
}

func (s *Server) handleSource(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := r.Params.Arguments.(map[string]any)
	id := getArgString(args, "id")
	file := getArgString(args, "file")
	name := getArgString(args, "name")
	idOrFile := id
	if idOrFile == "" {
		idOrFile = file
	}
	res, err := s.ops().Source(idOrFile, name)
	if err != nil {
		return errorResult(err), nil
	}
	return s.jsonResult(res)
}

func (s *Server) handleCheck(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := r.Params.Arguments.(map[string]any)
	defects, err := s.ops().Check(getArgString(args, "file"))
	if err != nil {
		return errorResult(err), nil
	}
	return s.jsonResult(defects)
}

func (s *Server) handleCallers(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := r.Params.Arguments.(map[string]any)
	dir := getArgString(args, "dir")
	if dir == "" {
		dir = "."
	}
	sites, err := s.ops().Callers(dir, getArgString(args, "name"))
	if err != nil {
		return errorResult(err), nil
	}
	return s.jsonResult(sites)
}

func (s *Server) handleMap(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := r.Params.Arguments.(map[string]any)
	maps, err := s.ops().Map(
		getArgString(args, "dir"), getArgString(args, "kind"),
		getArgString(args, "name"), getArgBool(args, "nameContains", false))
	if err != nil {
		return errorResult(err), nil
	}
	return s.jsonResult(maps)
}

func (s *Server) handleDefinition(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := r.Params.Arguments.(map[string]any)
	dir := getArgString(args, "dir")
	if dir == "" {
		dir = "."
	}
	res, err := s.ops().Definition(getArgString(args, "name"), getArgString(args, "at"), dir)
	if err != nil {
		return errorResult(err), nil
	}
	return s.jsonResult(res)
}

// symbolIDRe matches a grove-style symbol-id embedded in prose.
var symbolIDRe = regexp.MustCompile(`[\w.-]+:[^#\s]+#[^@\s]+@\d+`)

func (s *Server) handleCodeForArtifact(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := r.Params.Arguments.(map[string]any)
	id := getArgString(args, "id")
	if s.store == nil {
		return mcp.NewToolResultText(`{"error":"no canon store loaded"}`), nil
	}
	item := s.store.ByID(id)
	if item == nil || item.Tier != store.TierCanon {
		return mcp.NewToolResultText(`{"error":"canon artifact not found: ` + jsonEscape(id) + `"}`), nil
	}
	ops := s.ops()
	ids := symbolIDRe.FindAllString(item.Body, -1)
	seen := map[string]bool{}
	var resolved []map[string]any
	var unresolved []string
	for _, sid := range ids {
		if seen[sid] {
			continue
		}
		seen[sid] = true
		res, err := ops.Source(sid, "")
		if err != nil {
			unresolved = append(unresolved, sid)
			continue
		}
		resolved = append(resolved, map[string]any{"id": res.ID, "source": res.Source})
	}
	return s.jsonResult(map[string]any{
		"artifact":   map[string]string{"id": item.ID, "title": item.Title, "type": item.Type},
		"resolved":   resolved,
		"unresolved": unresolved,
	})
}

func (s *Server) handleArtifactsForSymbol(_ context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := r.Params.Arguments.(map[string]any)
	q := getArgString(args, "id")
	if s.store == nil {
		return mcp.NewToolResultText(`{"artifacts":[]}`), nil
	}
	// Search for the symbol name and path pieces; Discover ranks over the index.
	query := q
	if path, name, _, ok := codeintel.ParseID(q); ok {
		query = name + " " + path
	}
	hits := s.store.Discover(query, 20)
	var out []map[string]any
	for _, h := range hits {
		if h.Item == nil || h.Item.Tier != store.TierCanon {
			continue
		}
		// Only surface artifacts that actually mention the query token.
		if !strings.Contains(h.Item.Body, q) && q != query {
			continue
		}
		out = append(out, map[string]any{
			"id": h.Item.ID, "title": h.Item.Title, "type": h.Item.Type, "path": h.Item.Path,
		})
	}
	return s.jsonResult(map[string]any{"symbol": q, "artifacts": out})
}

// errorResult encodes a domain error as a tool result with isError, without
// killing the server (the Go error is nil).
func errorResult(err error) *mcp.CallToolResult {
	return mcp.NewToolResultText(`{"error":"` + jsonEscape(err.Error()) + `"}`)
}
