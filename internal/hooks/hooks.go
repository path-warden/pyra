// Package hooks installs pyra's deterministic checks onto the event surfaces
// of the surrounding toolchains — git, Claude Code, the Kiro IDE, and the Kiro
// CLI — so that `pyra gate` (and a store-integrity guard) run automatically
// when artifacts change. Every installer writes only its toolchain's documented
// integration point, marks its content with a stable marker for idempotent
// install/uninstall, and never disturbs unrelated content in those files.
package hooks

import "github.com/chasedputnam/pyra/internal/config"

// Target identifies a hook surface.
type Target string

const (
	TargetGit     Target = "git"
	TargetClaude  Target = "claude"
	TargetCodex   Target = "codex"
	TargetKiroIDE Target = "kiro-ide"
	TargetKiroCLI Target = "kiro-cli"
)

// ManagedMarker is embedded in every pyra-authored hook command/entry so that
// install/uninstall can find and replace exactly pyra's content.
const ManagedMarker = "pyra-managed"

// Delimiters bracket the pyra-managed block in line-oriented hook files (git).
const (
	BlockBegin = "# >>> pyra (managed) >>>"
	BlockEnd   = "# <<< pyra (managed) <<<"
)

// Action is what an installer did (or found) for a target.
type Action int

const (
	ActionUnchanged Action = iota
	ActionCreated
	ActionUpdated
	ActionRemoved
	ActionSkipped
	ActionAmbiguous
	ActionPresent
	ActionAbsent
)

func (a Action) String() string {
	switch a {
	case ActionCreated:
		return "created"
	case ActionUpdated:
		return "updated"
	case ActionRemoved:
		return "removed"
	case ActionSkipped:
		return "skipped"
	case ActionAmbiguous:
		return "ambiguous"
	case ActionPresent:
		return "present"
	case ActionAbsent:
		return "absent"
	default:
		return "unchanged"
	}
}

// Context carries everything an installer needs.
type Context struct {
	StoreRoot string
	Config    config.Config
	KiroAgent string // explicit Kiro CLI agent selection (kiro-cli only)
}

// Result reports an installer's outcome for one target.
type Result struct {
	Target Target
	Action Action
	Paths  []string // files created, modified, or inspected
	Detail string   // human explanation (e.g. why skipped/ambiguous)
}

// Installer manages pyra hooks for one toolchain surface.
type Installer interface {
	Target() Target
	Detect(ctx Context) bool
	Install(ctx Context) (Result, error)
	Uninstall(ctx Context) (Result, error)
	Status(ctx Context) (Result, error)
}

// Installers returns one installer per supported hook surface, in stable order
// (git first, as the universal backstop).
func Installers() []Installer {
	return []Installer{
		gitInstaller{},
		claudeInstaller{},
		codexInstaller{},
		kiroIDEInstaller{},
		kiroCLIInstaller{},
	}
}
