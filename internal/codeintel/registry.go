package codeintel

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

//go:embed registry
var registryFS embed.FS

// ErrUnsupportedLanguage is returned when a file's language has no provisioned
// grammar/query set. Callers doing directory walks skip such files; single-file
// operations surface it.
var ErrUnsupportedLanguage = errors.New("unsupported language")

// Profile parameterizes extraction per language (ported from grove manifests).
type Profile struct {
	FunctionKinds    []string    `json:"function_kinds"`
	Containers       []NameField `json:"-"`
	IdentifierKinds  []string    `json:"identifier_kinds"`
	ImportResolution string      `json:"import_resolution"`
}

// NameField is a (node kind, name field) pair used to find container nodes.
type NameField struct {
	Kind  string
	Field string
}

// rawProfile matches grove's on-disk profile JSON where containers are
// [ [kind, field], ... ] pairs.
type rawProfile struct {
	FunctionKinds    []string   `json:"function_kinds"`
	Containers       [][]string `json:"containers"`
	IdentifierKinds  []string   `json:"identifier_kinds"`
	ImportResolution string     `json:"import_resolution"`
}

// Language bundles everything needed to operate on one language.
type Language struct {
	Name    string
	Grammar *gts.Language
	Tags    string
	Locals  string // may be empty
	Imports string // may be empty
	Profile Profile
}

// langDef holds a language's embedded assets (queries are text; grammar is
// resolved lazily from gotreesitter to keep versions consistent).
type langDef struct {
	name    string
	tags    string
	locals  string
	imports string
	profile Profile
	grammar func() *gts.Language
}

// Registry resolves files to their language bundle. It is built once from the
// embedded assets and is safe for concurrent use.
type Registry struct {
	langs map[string]*langDef // keyed by canonical language name
}

var (
	defaultRegistry *Registry
	defaultOnce     sync.Once
)

// DefaultRegistry returns the process-wide registry built from embedded assets.
func DefaultRegistry() *Registry {
	defaultOnce.Do(func() {
		reg, err := loadRegistry()
		if err != nil {
			// Embedded assets are compiled in; a failure is a programming error.
			panic(fmt.Sprintf("codeintel: loading embedded registry: %v", err))
		}
		defaultRegistry = reg
	})
	return defaultRegistry
}

// grammarLoaders maps canonical language names to gotreesitter grammar loaders.
// Only languages with both an embedded query set and a grammar loader here are
// supported. Adding a language means vendoring its assets and adding a loader.
var grammarLoaders = map[string]func() *gts.Language{
	"go":         grammars.GoLanguage,
	"python":     grammars.PythonLanguage,
	"javascript": grammars.JavascriptLanguage,
	"typescript": grammars.TypescriptLanguage,
	"tsx":        grammars.TsxLanguage,
	"java":       grammars.JavaLanguage,
	"rust":       grammars.RustLanguage,
}

func loadRegistry() (*Registry, error) {
	r := &Registry{langs: map[string]*langDef{}}
	entries, err := registryFS.ReadDir("registry")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		loader, ok := grammarLoaders[name]
		if !ok {
			// Assets present but no grammar loader wired: skip rather than fail.
			continue
		}
		def, err := loadLangDef(name)
		if err != nil {
			return nil, fmt.Errorf("language %s: %w", name, err)
		}
		def.grammar = loader
		r.langs[name] = def
	}
	if len(r.langs) == 0 {
		return nil, errors.New("no languages loaded")
	}
	return r, nil
}

func loadLangDef(name string) (*langDef, error) {
	base := "registry/" + name
	tags, err := readAsset(base + "/tags.scm")
	if err != nil {
		return nil, fmt.Errorf("tags.scm required: %w", err)
	}
	locals, _ := readOptional(base + "/locals.scm")
	imports, _ := readOptional(base + "/imports.scm")
	profBytes, err := readAsset(base + "/profile.json")
	if err != nil {
		return nil, fmt.Errorf("profile.json required: %w", err)
	}
	var raw rawProfile
	if err := json.Unmarshal([]byte(profBytes), &raw); err != nil {
		return nil, fmt.Errorf("profile.json: %w", err)
	}
	prof := Profile{
		FunctionKinds:    raw.FunctionKinds,
		IdentifierKinds:  raw.IdentifierKinds,
		ImportResolution: raw.ImportResolution,
	}
	for _, pair := range raw.Containers {
		if len(pair) == 2 {
			prof.Containers = append(prof.Containers, NameField{Kind: pair[0], Field: pair[1]})
		}
	}
	return &langDef{name: name, tags: tags, locals: locals, imports: imports, profile: prof}, nil
}

func readAsset(path string) (string, error) {
	b, err := registryFS.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func readOptional(path string) (string, error) {
	b, err := registryFS.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

// ForFile resolves a file path to its language bundle. It uses gotreesitter's
// filename detection to pick the grammar (keeping grammar and query versions
// consistent), then looks up the embedded queries/profile by language name.
// Returns ErrUnsupportedLanguage when the language is not provisioned.
func (r *Registry) ForFile(path string) (*Language, error) {
	entry := grammars.DetectLanguage(filepath.Base(path))
	if entry == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLanguage, filepath.Ext(path))
	}
	def, ok := r.langs[entry.Name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLanguage, entry.Name)
	}
	return def.resolve(), nil
}

func (d *langDef) resolve() *Language {
	return &Language{
		Name:    d.name,
		Grammar: d.grammar(),
		Tags:    d.tags,
		Locals:  d.locals,
		Imports: d.imports,
		Profile: d.profile,
	}
}

// SupportedLanguages returns the sorted names of provisioned languages.
func (r *Registry) SupportedLanguages() []string {
	names := make([]string, 0, len(r.langs))
	for n := range r.langs {
		names = append(names, n)
	}
	// simple insertion sort to avoid importing sort for a tiny slice
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j-1] > names[j]; j-- {
			names[j-1], names[j] = names[j], names[j-1]
		}
	}
	return names
}

// isGeneratedDecl reports TypeScript declaration files excluded from directory
// walks (matching grove's is_generated_decl).
func isGeneratedDecl(path string) bool {
	for _, suf := range []string{".d.ts", ".d.cts", ".d.mts"} {
		if strings.HasSuffix(path, suf) {
			return true
		}
	}
	return false
}
