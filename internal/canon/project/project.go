// Package project turns an approved spec document (requirements.md / design.md,
// from the local specs/ layout or Kiro's .kiro/specs/ layout) into a typed Canon
// artifact. It is thin orchestration over the existing authority pipeline
// (identity → parse → classify → validate → relate): it mints or reuses a stable
// ID, fills the resolved type's required sections from the source prose, infers
// typed relationships from literal ID references already in the text, validates
// the result, and writes only after a ratify-or-correct decision.
//
// The spec-document ↔ Canon-artifact link is a deterministic path convention
// (see mapTargetPath), never hidden state, so re-projection is idempotent.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/chasedputnam/pyra/internal/canon"
	"github.com/chasedputnam/pyra/internal/canon/artifacts"
	"github.com/chasedputnam/pyra/internal/canon/classify"
	"github.com/chasedputnam/pyra/internal/canon/frontmatter"
	"github.com/chasedputnam/pyra/internal/canon/identity"
	"github.com/chasedputnam/pyra/internal/canon/model"
	"github.com/chasedputnam/pyra/internal/canon/parse"
	"github.com/chasedputnam/pyra/internal/canon/relate"
	"github.com/chasedputnam/pyra/internal/canon/validate"
	"github.com/chasedputnam/pyra/internal/config"
)

// canonIDRef matches a literal canonical artifact ID anywhere in text (the
// unanchored, word-bounded form of identity's ID grammar). Inference is
// literal-only: a near-miss that is not a well-formed ID never matches.
var canonIDRef = regexp.MustCompile(`\b[A-Z0-9]+-[0-9A-HJKMNP-TV-Z]{12}\b`)

// structuredAliasRe matches deliberate cross-reference aliases like "adr-002".
// Bare filename stems (no digit suffix) are excluded so common words in prose
// never produce spurious edges.
var structuredAliasRe = regexp.MustCompile(`^[A-Za-z]+-\d+$`)

// Edge is a relationship inferred from a literal reference in the source.
type Edge struct {
	Section string // normalized relationship section, e.g. "related decisions"
	Target  string // resolved artifact ID
	Ref     string // the literal reference string as found in the source
}

// Options controls a projection.
type Options struct {
	Store     string // store root
	Type      string // optional type override; else inferred from the basename
	DryRun    bool   // compute everything, write nothing
	Write     bool   // permit overwriting an existing artifact
	KiroAgent string // reserved (parity with hooks); unused by projection
}

// Result is the outcome of projecting one document. When DryRun is set, or an
// existing artifact would change without Write, no file is written.
type Result struct {
	SourcePath         string
	ArtifactPath       string
	Type               string
	ID                 string
	Created            bool // true → newly created; false → updated/would-update existing
	IncompleteSections []string
	InferredEdges      []Edge
	UnresolvedRefs     []string
	BlockingIssues     []model.Issue
	Diff               string
	Changed            bool
}

// basenameType maps a spec document's basename (sans extension, lower-cased) to
// the Canon type it projects into.
var basenameType = map[string]string{
	"requirements": artifacts.TypeRequirement,
	"design":       artifacts.TypeDesign,
}

// resolveType determines the Canon type for a source document: an explicit
// override (any registered type) wins; otherwise the basename is mapped. An
// unmappable document is an error naming the supported documents and types.
func resolveType(sourcePath, override string, reg artifacts.Registry) (string, error) {
	if strings.TrimSpace(override) != "" {
		t := strings.ToLower(strings.TrimSpace(override))
		if _, ok := reg[t]; !ok {
			return "", fmt.Errorf("unknown --type %q; valid types: %s", override, strings.Join(reg.Types(), ", "))
		}
		return t, nil
	}
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath)))
	if t, ok := basenameType[base]; ok {
		return t, nil
	}
	return "", fmt.Errorf(
		"cannot infer Canon type from %q: project a requirements.md or design.md, or pass --type (valid: %s)",
		filepath.Base(sourcePath), strings.Join(reg.Types(), ", "))
}

// mapTargetPath applies the deterministic spec→canon path convention:
//
//	<specRoot>/<feature>/<base>.md  ->  <canonRoot>/<feature>/<base>.md
//
// where <feature> is the spec document's parent directory name and <canonRoot>
// is the store's primary canon root. The mapping depends only on paths, so the
// spec-doc ↔ artifact link is recoverable across runs without stored state.
func mapTargetPath(cfg config.Config, storeRoot, sourcePath string) string {
	canonRoot := "canon"
	if len(cfg.CanonRoots) > 0 {
		canonRoot = cfg.CanonRoots[0]
	}
	feature := filepath.Base(filepath.Dir(sourcePath))
	base := filepath.Base(sourcePath)
	return filepath.Join(storeRoot, canonRoot, feature, base)
}

// Project computes the typed Canon artifact for one spec document and writes it
// unless opts.DryRun is set. The artifact's ID is reused from an existing target
// (the deterministic path) or freshly minted. Required sections unfillable from
// the source are emitted with a placeholder and reported in IncompleteSections.
func Project(cfg config.Config, sourcePath string, opts Options) (Result, error) {
	reg := artifacts.Default()
	typ, err := resolveType(sourcePath, opts.Type, reg)
	if err != nil {
		return Result{}, err
	}
	spec := reg[typ]

	storeRoot := opts.Store
	if storeRoot == "" {
		storeRoot = "."
	}
	if err := detectCollision(cfg, storeRoot, sourcePath); err != nil {
		return Result{}, err
	}
	target := mapTargetPath(cfg, storeRoot, sourcePath)

	srcRaw, err := os.ReadFile(sourcePath)
	if err != nil {
		return Result{}, err
	}
	src := parse.Parse(srcRaw)

	// Load the Canon corpus once and reuse it for both relationship inference and
	// candidate validation.
	arts, err := canon.LoadCorpus(storeRoot, cfg)
	if err != nil {
		return Result{}, err
	}

	id, created, err := reuseOrMintID(cfg, target)
	if err != nil {
		return Result{}, err
	}

	edges, unresolved := inferEdges(arts, typ, srcRaw, reg)

	title := strings.TrimSpace(src.Title)
	if title == "" {
		title = humanize(filepath.Base(filepath.Dir(sourcePath)))
	}

	content, incomplete := buildArtifact(id, typ, title, spec, src, edges)

	existing, _ := os.ReadFile(target) // empty when the target does not exist
	changed := created || string(existing) != content
	diff := ""
	if !created && string(existing) != content {
		diff = unifiedDiff(string(existing), content, target)
	}

	blocking := validateCandidate(cfg, arts, storeRoot, target, content)

	res := Result{
		SourcePath:         sourcePath,
		ArtifactPath:       target,
		Type:               typ,
		ID:                 id,
		Created:            created,
		IncompleteSections: incomplete,
		InferredEdges:      edges,
		UnresolvedRefs:     unresolved,
		BlockingIssues:     blocking,
		Diff:               diff,
		Changed:            changed,
	}

	// Ratify-or-correct: write a new artifact freely, but only overwrite an
	// existing one with explicit permission, and never under --dry-run.
	if !opts.DryRun && changed && (created || opts.Write) {
		if err := writeArtifact(target, content); err != nil {
			return res, err
		}
	}
	return res, nil
}

// validateCandidate runs the same per-artifact and relationship validation the
// gate uses against the in-memory candidate (with it substituted for any existing
// artifact at the same path), returning the blocking (error-severity) issues.
//
// Relationship issues are attributed by a baseline diff — the issues present with
// the candidate in the corpus minus those present without it — so any issue the
// candidate introduces is reported regardless of which file relate.Build blames
// (e.g. a duplicate-identifier or cycle whose path is another artifact), while
// pre-existing corpus issues unrelated to this projection are not surfaced.
func validateCandidate(cfg config.Config, arts []canon.Artifact, storeRoot, targetPath, content string) []model.Issue {
	reg := artifacts.Default()
	p := parse.Parse([]byte(content))
	c := classify.Classify(p, reg)

	var issues []model.Issue
	issues = append(issues, validate.Validate(p, c, validate.Options{TicketProvider: cfg.Ticketing.Provider})...)

	relTarget, rerr := filepath.Rel(storeRoot, targetPath)
	if rerr != nil {
		relTarget = targetPath
	}

	baseline := make([]relate.Entry, 0, len(arts))
	for _, a := range arts {
		if a.Path == relTarget {
			continue // replaced by the candidate below
		}
		baseline = append(baseline, relate.Entry{
			ID: a.ID, Type: a.Type, Status: a.Status, Retired: a.Retired,
			Path: a.Path, Aliases: a.Aliases, Product: a.Product,
		})
	}
	cid, _ := identity.Resolve(p, targetPath)
	candidate := relate.Entry{
		ID: cid, Type: c.Type, Path: relTarget,
		Aliases: identity.Aliases(targetPath), Product: p,
	}

	_, baseIssues := relate.Build(baseline, relate.DefaultSpecs())
	_, fullIssues := relate.Build(append(baseline, candidate), relate.DefaultSpecs())
	issues = append(issues, issuesIntroduced(baseIssues, fullIssues)...)

	var blocking []model.Issue
	for _, iss := range issues {
		if iss.Severity == model.SeverityError {
			blocking = append(blocking, iss)
		}
	}
	return blocking
}

// issuesIntroduced returns the issues present in after but not before, keyed by
// code+path+message, so only the findings a candidate adds are reported.
func issuesIntroduced(before, after []model.Issue) []model.Issue {
	key := func(i model.Issue) string { return i.Code + "\x00" + i.Path + "\x00" + i.Message }
	seen := make(map[string]int, len(before))
	for _, i := range before {
		seen[key(i)]++
	}
	var out []model.Issue
	for _, i := range after {
		k := key(i)
		if seen[k] > 0 {
			seen[k]--
			continue
		}
		out = append(out, i)
	}
	return out
}

// unifiedDiff renders a unified diff of the current vs projected artifact bytes.
func unifiedDiff(current, projected, path string) string {
	out, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(current),
		B:        difflib.SplitLines(projected),
		FromFile: path + " (current)",
		ToFile:   path + " (projected)",
		Context:  3,
	})
	if err != nil {
		return ""
	}
	return out
}

// ProjectDir projects every recognized spec document in a spec directory,
// skipping tasks.md and any file without a Canon type. Documents are processed in
// sorted order for determinism; the first error aborts and is returned with the
// results produced so far.
func ProjectDir(cfg config.Config, specDir string, opts Options) ([]Result, error) {
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var results []Result
	for _, name := range names {
		base := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
		if _, ok := basenameType[base]; !ok {
			continue // tasks.md and other non-Canon documents are skipped
		}
		r, perr := Project(cfg, filepath.Join(specDir, name), opts)
		if perr != nil {
			return results, perr
		}
		results = append(results, r)
	}
	return results, nil
}

// detectCollision reports an error when the same feature document is provided by
// more than one configured spec root, since both would project to the same Canon
// path. The collision is surfaced rather than letting one source silently win.
func detectCollision(cfg config.Config, storeRoot, sourcePath string) error {
	// The cross-root collision only applies to sources that actually live under a
	// configured spec root; a path elsewhere (e.g. an ad-hoc doc passed directly)
	// is never a spec-root collision even if a same-named feature exists.
	rel, err := filepath.Rel(storeRoot, sourcePath)
	if err != nil {
		return nil
	}
	rel = filepath.ToSlash(rel)
	underSpecRoot := false
	for _, sr := range cfg.SpecRoots {
		if strings.HasPrefix(rel, filepath.ToSlash(sr)+"/") {
			underSpecRoot = true
			break
		}
	}
	if !underSpecRoot {
		return nil
	}

	feature := filepath.Base(filepath.Dir(sourcePath))
	base := filepath.Base(sourcePath)
	var found []string
	for _, sr := range cfg.SpecRoots {
		cand := filepath.Join(storeRoot, sr, feature, base)
		if _, err := os.Stat(cand); err == nil {
			found = append(found, cand)
		}
	}
	if len(found) > 1 {
		sort.Strings(found)
		return fmt.Errorf(
			"spec collision: %s is provided by multiple spec roots (%s) that map to the same Canon path; remove or rename one before projecting",
			filepath.Join(feature, base), strings.Join(found, ", "))
	}
	return nil
}

// reuseOrMintID returns the artifact ID to use for the target path and whether
// the artifact is newly created. An existing artifact's explicit frontmatter ID
// is reused so re-projection is identity-stable; otherwise an ID is minted.
func reuseOrMintID(cfg config.Config, targetPath string) (id string, created bool, err error) {
	raw, rerr := os.ReadFile(targetPath)
	if rerr != nil {
		if errors.Is(rerr, fs.ErrNotExist) {
			return identity.Mint(cfg.RepositoryKey, identity.NewEntropy()), true, nil
		}
		return "", false, rerr
	}
	p := parse.Parse(raw)
	if existing := strings.TrimSpace(p.Metadata.ID); existing != "" {
		return existing, false, nil
	}
	// An existing target without a usable ID has no identity to preserve. Treat it
	// as a create so the minted ID is written once and then reused on subsequent
	// runs; otherwise each projection would mint a fresh ID and never converge.
	return identity.Mint(cfg.RepositoryKey, identity.NewEntropy()), true, nil
}

// buildArtifact renders the typed Canon artifact: the frontmatter envelope plus
// the type's required and recommended sections, each filled from the matching
// source section or a placeholder. Required sections that could not be filled
// are returned in incomplete.
func buildArtifact(id, typ, title string, spec artifacts.ArtifactSpec, src *model.Product, edges []Edge) (content string, incomplete []string) {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "schema_version: %d\n", frontmatter.CurrentSchemaVersion)
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "type: %s\n", typ)
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", title)

	emit := func(s artifacts.Section, required bool) {
		body, ok := matchSourceSection(src, s)
		body = strings.TrimSpace(body)
		if !ok || body == "" {
			body = placeholder(spec, s.Name)
			if required {
				incomplete = append(incomplete, s.Name)
			}
		}
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", titleize(s.Name), body)
	}
	for _, s := range spec.Required {
		emit(s, true)
	}
	for _, s := range spec.Recommended {
		emit(s, false)
	}

	// Inferred relationship sections, grouped by section in canonical order.
	for _, section := range relationshipSectionOrder {
		var refs []string
		for _, e := range edges {
			if e.Section == section {
				refs = append(refs, e.Target)
			}
		}
		if len(refs) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", titleize(section))
		for _, r := range refs {
			fmt.Fprintf(&b, "- %s\n", r)
		}
		b.WriteString("\n")
	}
	return b.String(), incomplete
}

// relationshipSectionOrder is the canonical relationship-section order (mirrors
// relate.relationshipSections) so emitted edges are deterministic.
var relationshipSectionOrder = []string{
	"related requirements", "related decisions", "related roadmaps",
	"related prompts", "related designs", "related tickets",
}

// inferEdges scans the source for literal canonical IDs and known aliases of
// artifacts in the store, returning the resolved relationship edges permitted for
// the projected type and the id-shaped references that resolved to nothing.
// Inference is literal-only and high-precision: ambiguous or not-permitted
// references are dropped rather than written as questionable edges.
func inferEdges(arts []canon.Artifact, typ string, srcRaw []byte, reg artifacts.Registry) (edges []Edge, unresolved []string) {
	type target struct{ id, typ string }
	idx := map[string][]target{}
	add := func(ident, id, ttyp string) {
		if ident == "" {
			return
		}
		k := strings.ToLower(ident)
		for _, e := range idx[k] {
			if e.id == id {
				return
			}
		}
		idx[k] = append(idx[k], target{id: id, typ: ttyp})
	}
	for _, a := range arts {
		add(a.ID, a.ID, a.Type)
		for _, al := range a.Aliases {
			// Only structured aliases (e.g. "adr-002") participate in word-scan
			// matching. A bare filename stem like "requirements" is a common word
			// and would spuriously match prose, so it is never indexed.
			if structuredAliasRe.MatchString(al) {
				add(al, a.ID, a.Type)
			}
		}
	}

	permitted := reg[typ].Optional
	sectionForType := func(targetType string) (string, bool) {
		want := "related " + targetType + "s"
		for _, sec := range permitted {
			if sec == want {
				return want, true
			}
		}
		return "", false
	}

	text := string(srcRaw)
	seen := map[string]bool{} // dedupe resolved edges by target id
	unresolvedSeen := map[string]bool{}

	resolveOne := func(ref string) {
		matches := idx[strings.ToLower(ref)]
		switch len(matches) {
		case 0:
			if identity.ValidID(ref) && !unresolvedSeen[ref] {
				unresolvedSeen[ref] = true
				unresolved = append(unresolved, ref)
			}
		case 1:
			tgt := matches[0]
			if seen[tgt.id] {
				return
			}
			sec, ok := sectionForType(tgt.typ)
			if !ok {
				return // not permitted for this type → drop
			}
			seen[tgt.id] = true
			edges = append(edges, Edge{Section: sec, Target: tgt.id, Ref: ref})
		default:
			// ambiguous → drop
		}
	}

	// Canonical IDs first (deterministic document order).
	for _, m := range canonIDRef.FindAllString(text, -1) {
		resolveOne(m)
	}
	// Then aliases: any non-ID index identifier present as a whole word.
	aliasKeys := make([]string, 0, len(idx))
	for k := range idx {
		if identity.ValidID(strings.ToUpper(k)) {
			continue
		}
		aliasKeys = append(aliasKeys, k)
	}
	sort.Strings(aliasKeys)
	for _, k := range aliasKeys {
		if wordPresent(text, k) {
			resolveOne(k)
		}
	}

	sort.SliceStable(edges, func(i, j int) bool {
		if edges[i].Section != edges[j].Section {
			return edges[i].Section < edges[j].Section
		}
		return edges[i].Target < edges[j].Target
	})
	return edges, unresolved
}

// wordPresent reports whether ident appears in text as a whole token.
func wordPresent(text, ident string) bool {
	re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(ident) + `\b`)
	if err != nil {
		return false
	}
	return re.MatchString(text)
}

// matchSourceSection returns the source body whose normalized heading satisfies
// section s, scanning in document order for determinism.
func matchSourceSection(src *model.Product, s artifacts.Section) (string, bool) {
	for _, key := range src.Order {
		if s.Matches(key) {
			return src.Sections[key], true
		}
	}
	return "", false
}

// placeholder mirrors the scaffold placeholder logic: the first allowed value for
// a constrained metadata section, a sample requirement line, else TODO.
func placeholder(spec artifacts.ArtifactSpec, name string) string {
	if allowed, ok := spec.Metadata[name]; ok && len(allowed) > 0 {
		return allowed[0]
	}
	if name == "requirements" {
		return "[REQ-001] The system SHALL do a specific, testable thing."
	}
	return "TODO"
}

func writeArtifact(target, content string) error {
	if dir := filepath.Dir(target); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(target, []byte(content), 0o644)
}

func titleize(normalized string) string {
	words := strings.Fields(normalized)
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func humanize(name string) string {
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	return titleize(name)
}
