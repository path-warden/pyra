#!/usr/bin/env bash
#
# install_skills.sh — install the bundled pyra skills and gate hooks into
# whichever agent toolchains are present on this machine.
#
# It auto-detects each supported tool by the presence of its folder, and only
# installs into the ones it finds:
#
#   Claude Code  (~/.claude or <store>/.claude)  -> skills + PostToolUse gate hook
#   Kiro         (~/.kiro   or <store>/.kiro)    -> skills + IDE + CLI gate hooks
#   git          (<store>/.git)                  -> pre-commit / post-merge gate
#
# Claude Code and Kiro share the same SKILL.md format, so the same skill folders
# install into both (~/.claude/skills and ~/.kiro/skills). Hooks are installed
# via the `pyra` binary (which must be on PATH) and require the target store
# to be a pyra store (a .okf/config.yaml exists).
#
# Usage:
#   ./install_skills.sh [STORE_DIR]
#
#   STORE_DIR   pyra store to wire hooks into (default: current directory).
#               Skills install to your personal ~/.claude/skills and ~/.kiro/skills.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_SRC="$SCRIPT_DIR/.claude/skills"
SKILLS=(spec dev code-review)
STORE_DIR="${1:-.}"

# --- output helpers ---------------------------------------------------------
if [ -t 1 ]; then
	BOLD=$'\033[1m'; GREEN=$'\033[32m'; YELLOW=$'\033[33m'
	RED=$'\033[31m'; DIM=$'\033[2m'; RESET=$'\033[0m'
else
	BOLD=""; GREEN=""; YELLOW=""; RED=""; DIM=""; RESET=""
fi
head() { printf "\n%s%s%s\n" "$BOLD" "$*" "$RESET"; }
ok()   { printf "  %s✓%s %s\n" "$GREEN" "$RESET" "$*"; }
warn() { printf "  %s!%s %s\n" "$YELLOW" "$RESET" "$*"; }
skip() { printf "  %s–%s %s\n" "$DIM" "$RESET" "$*"; }
die()  { printf "%s✗%s %s\n" "$RED" "$RESET" "$*" >&2; exit 1; }

# --- preflight --------------------------------------------------------------
[ -d "$SKILLS_SRC" ] || die "skills not found at $SKILLS_SRC — run this from a clone of the pyra repo"

HAVE_PYRA=0
command -v pyra >/dev/null 2>&1 && HAVE_PYRA=1

IS_STORE=0
[ -f "$STORE_DIR/.okf/config.yaml" ] && IS_STORE=1

# Detect each tool by its folder, globally (~) or in the target store.
CLAUDE_PRESENT=0
{ [ -d "$HOME/.claude" ] || [ -d "$STORE_DIR/.claude" ]; } && CLAUDE_PRESENT=1

KIRO_PRESENT=0
{ [ -d "$HOME/.kiro" ] || [ -d "$STORE_DIR/.kiro" ]; } && KIRO_PRESENT=1

# install_hooks <flag> <label> — install gate hooks for one target, with guards.
install_hooks() {
	local flag="$1" label="$2"
	if [ "$HAVE_PYRA" -ne 1 ]; then
		warn "pyra not on PATH — skipping $label hooks (install pyra, then re-run)"
		return
	fi
	if [ "$IS_STORE" -ne 1 ]; then
		warn "$STORE_DIR is not a pyra store (no .okf/config.yaml) — skipping $label hooks"
		return
	fi
	pyra hooks install "$flag" --store "$STORE_DIR" 2>&1 | sed 's/^/  /'
}

# install_skills_into <dest-dir> — copy every bundled skill into a skills dir.
# Skills use the same SKILL.md format for Claude Code and Kiro, so the same
# folders install into both ~/.claude/skills and ~/.kiro/skills.
install_skills_into() {
	local dest="$1"
	mkdir -p "$dest"
	for s in "${SKILLS[@]}"; do
		if [ -d "$SKILLS_SRC/$s" ]; then
			rm -rf "${dest:?}/$s"
			cp -R "$SKILLS_SRC/$s" "$dest/$s"
			ok "skill /$s -> $dest/$s"
		else
			warn "skill source missing: $SKILLS_SRC/$s"
		fi
	done
}

# --- run --------------------------------------------------------------------
printf "%spyra skills + hooks installer%s\n" "$BOLD" "$RESET"
printf "store: %s\n" "$STORE_DIR"

# Claude Code: skills + hooks
if [ "$CLAUDE_PRESENT" -eq 1 ]; then
	head "Claude Code detected"
	install_skills_into "$HOME/.claude/skills"
	install_hooks --claude "Claude Code"
else
	head "Claude Code"
	skip "not detected (no ~/.claude or $STORE_DIR/.claude) — skipping skills + hooks"
fi

# Kiro: skills + hooks
if [ "$KIRO_PRESENT" -eq 1 ]; then
	head "Kiro detected"
	install_skills_into "$HOME/.kiro/skills"
	install_hooks --kiro "Kiro"
else
	head "Kiro"
	skip "not detected (no ~/.kiro or $STORE_DIR/.kiro) — skipping skills + hooks"
fi

# git: gate hooks (foundational, installed whenever the store is a git repo)
if [ -d "$STORE_DIR/.git" ]; then
	head "git repository detected"
	install_hooks --git "git"
fi

# Summary
if [ "$CLAUDE_PRESENT" -eq 0 ] && [ "$KIRO_PRESENT" -eq 0 ]; then
	head "Nothing to install"
	warn "No supported agent toolchains found (Claude Code, Kiro)."
else
	head "Done"
	{ [ "$CLAUDE_PRESENT" -eq 1 ] || [ "$KIRO_PRESENT" -eq 1 ]; } && ok "skills available as /spec, /dev, /code-review"
	if [ "$HAVE_PYRA" -ne 1 ]; then
		warn "Install the pyra binary and re-run to finish wiring the gate hooks."
	elif [ "$IS_STORE" -ne 1 ]; then
		warn "Run 'pyra init $STORE_DIR' then re-run to wire the gate hooks."
	fi
fi
