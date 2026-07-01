package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Render returns the YAML text for cfg as a self-documenting .okf/config.yaml:
// each key is preceded by a short comment, and the enforcement policy is shown
// as commented examples that load as an empty policy.
//
// The output is guaranteed to parse back through Load to a Config equal to cfg
// (modulo the zero-value normalization Load already performs). Render and the
// Config struct must evolve together: a new Config field that Render omits would
// be silently absent from scaffolded stores.
func Render(cfg Config) string {
	var b strings.Builder

	b.WriteString("# Repository key: prefix for minted Canon artifact IDs (e.g. OKF-3F8A...).\n")
	fmt.Fprintf(&b, "repository_key: %s\n\n", yamlScalar(cfg.RepositoryKey))

	b.WriteString("# Canon roots: directories that hold the authoritative tier. Everything else\n")
	b.WriteString("# under the store is treated as Reference. Files here are validated by `memphis gate`.\n")
	b.WriteString("canon_roots:\n")
	for _, r := range cfg.CanonRoots {
		// Block sequence: commas in a value are not delimiters here, so a root
		// like "a,b" survives. yamlScalar still quotes ":"/"#"/leading-"[" etc.
		fmt.Fprintf(&b, "  - %s\n", yamlScalar(r))
	}
	b.WriteString("\n")

	b.WriteString("# Spec roots: directories scanned for spec documents (requirements.md,\n")
	b.WriteString("# design.md) that `memphis project` turns into typed Canon. Covers the local\n")
	b.WriteString("# specs/ layout and Kiro's .kiro/specs/ layout by default.\n")
	b.WriteString("spec_roots:\n")
	for _, r := range cfg.SpecRoots {
		fmt.Fprintf(&b, "  - %s\n", yamlScalar(r))
	}
	b.WriteString("\n")

	b.WriteString("# Code roots: directories that structural code-intelligence operations\n")
	b.WriteString("# (outline, symbols, map, ...) search by default when no path is given.\n")
	b.WriteString("code_roots:\n")
	for _, r := range cfg.CodeRoots {
		fmt.Fprintf(&b, "  - %s\n", yamlScalar(r))
	}
	b.WriteString("\n")

	b.WriteString("# Ticketing provider: format-lints external \"## Related Tickets\" links.\n")
	b.WriteString("# One of: github, jira, linear, azure-devops, servicenow, none.\n")
	b.WriteString("ticketing:\n")
	fmt.Fprintf(&b, "  provider: %s\n\n", yamlScalar(cfg.Ticketing.Provider))

	b.WriteString("# Enforcement: reclassify gate findings by rule code. Empty = each rule keeps\n")
	b.WriteString("# its default severity. Uncomment and list rule codes to override.\n")
	if enforcementIsEmpty(cfg.Enforcement) {
		b.WriteString("enforcement: {}\n")
		b.WriteString("  # blocking: [missing_required_section]\n")
		b.WriteString("  # advisory: [ears_conformance]\n")
		b.WriteString("  # disabled: [iso29148_singular]\n")
	} else {
		b.WriteString("enforcement:\n")
		writeStringList(&b, "blocking", cfg.Enforcement.Blocking)
		writeStringList(&b, "advisory", cfg.Enforcement.Advisory)
		writeStringList(&b, "disabled", cfg.Enforcement.Disabled)
	}

	return b.String()
}

func enforcementIsEmpty(e Enforcement) bool {
	return len(e.Blocking) == 0 && len(e.Advisory) == 0 && len(e.Disabled) == 0
}

// writeStringList renders `  key: [a, b]` when non-empty; omits the key otherwise.
func writeStringList(b *strings.Builder, key string, vals []string) {
	if len(vals) == 0 {
		return
	}
	fmt.Fprintf(b, "  %s: [%s]\n", key, strings.Join(yamlScalars(vals), ", "))
}

// yamlScalar renders s as a YAML scalar, quoting when the value contains
// characters (`:`, `#`, leading `[`, etc.) that would otherwise break parsing.
// This keeps Render's output loadable by Load for any resolved value. Plain
// values (e.g. "OKF", "canon", "github") marshal to themselves unchanged.
func yamlScalar(s string) string {
	out, err := yaml.Marshal(s)
	if err != nil {
		return s // yaml.Marshal never fails for a plain string
	}
	return strings.TrimRight(string(out), "\n")
}

func yamlScalars(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = yamlScalar(s)
	}
	return out
}
