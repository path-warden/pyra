package codeintel

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// metricsProfile names the per-language AST node kinds the metrics walker counts.
// control kinds are both a cyclomatic branch and a nesting level; caseKind kinds
// are branches only. The func hooks handle language-specific boolean operators,
// field access (for LCOM), and error-handling patterns.
type metricsProfile struct {
	control      map[string]bool // a cyclomatic branch AND a nesting level
	nestOnly     map[string]bool // a nesting level but not a branch (switch/try/with)
	caseKind     map[string]bool // a branch only (case/except clauses)
	countBoolOp  func(n *gts.Node, p *parsed) bool
	paramsField  string
	paramKind    map[string]bool
	primitives   map[string]bool
	detectFields func(n *gts.Node, p *parsed, out map[string]bool)
	detectErrors func(n *gts.Node, p *parsed, out map[string]bool)
}

// binaryBoolOp reports a C-style && / || binary expression (Go, JS, TS).
func binaryBoolOp(kind string) func(*gts.Node, *parsed) bool {
	return func(n *gts.Node, p *parsed) bool {
		if n.Type(p.lang.Grammar) != kind {
			return false
		}
		if op := n.ChildByFieldName("operator", p.lang.Grammar); op != nil {
			s := op.Text(p.src)
			return s == "&&" || s == "||"
		}
		return false
	}
}

// selfFieldAccess collects `<self>.<field>` member accesses whose receiver is one
// of the given identifiers (self / this), for LCOM cohesion.
func selfFieldAccess(memberKind, propField string, selves map[string]bool) func(*gts.Node, *parsed, map[string]bool) {
	return func(n *gts.Node, p *parsed, out map[string]bool) {
		if n.Type(p.lang.Grammar) != memberKind {
			return
		}
		obj := n.ChildByFieldName("object", p.lang.Grammar)
		prop := n.ChildByFieldName(propField, p.lang.Grammar)
		if obj != nil && prop != nil && selves[obj.Text(p.src)] {
			out[prop.Text(p.src)] = true
		}
	}
}

var metricsProfiles = map[string]metricsProfile{
	"go": {
		control:     toSet([]string{"if_statement", "for_statement"}),
		nestOnly:    toSet([]string{"expression_switch_statement", "type_switch_statement", "select_statement"}),
		caseKind:    toSet([]string{"expression_case", "type_case", "communication_case"}),
		countBoolOp: binaryBoolOp("binary_expression"),
		paramsField: "parameters",
		paramKind:   toSet([]string{"parameter_declaration"}),
		primitives:  toSet([]string{"int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "byte", "rune", "float32", "float64", "string", "bool", "error"}),
		detectErrors: func(n *gts.Node, p *parsed, out map[string]bool) {
			// `_ = f()` / `_, _ = f()` discards a (possibly error) return.
			if n.Type(p.lang.Grammar) == "assignment_statement" {
				if left := n.ChildByFieldName("left", p.lang.Grammar); left != nil && strings.Contains(left.Text(p.src), "_") {
					out["ignored_error"] = true
				}
			}
		},
	},
	"python": {
		control:  toSet([]string{"if_statement", "for_statement", "while_statement", "elif_clause"}),
		nestOnly: toSet([]string{"with_statement", "try_statement"}),
		caseKind: toSet([]string{"except_clause", "case_clause", "conditional_expression"}),
		countBoolOp: func(n *gts.Node, p *parsed) bool {
			return n.Type(p.lang.Grammar) == "boolean_operator"
		},
		paramsField:  "parameters",
		paramKind:    toSet([]string{"identifier", "typed_parameter", "default_parameter", "typed_default_parameter"}),
		primitives:   toSet([]string{"int", "float", "str", "bool", "bytes"}),
		detectFields: selfFieldAccess("attribute", "attribute", map[string]bool{"self": true}),
		detectErrors: func(n *gts.Node, p *parsed, out map[string]bool) {
			// A bare `except:` with no exception type.
			if n.Type(p.lang.Grammar) == "except_clause" && n.NamedChildCount() > 0 {
				if first := n.NamedChild(0); first != nil && first.Type(p.lang.Grammar) == "block" {
					out["bare_except"] = true
				}
			}
		},
	},
	"javascript": jsMetricsProfile(),
	"typescript": jsMetricsProfile(),
	"tsx":        jsMetricsProfile(),
}

func jsMetricsProfile() metricsProfile {
	return metricsProfile{
		control:      toSet([]string{"if_statement", "for_statement", "for_in_statement", "while_statement", "do_statement", "catch_clause"}),
		nestOnly:     toSet([]string{"switch_statement", "try_statement"}),
		caseKind:     toSet([]string{"switch_case", "ternary_expression"}),
		countBoolOp:  binaryBoolOp("binary_expression"),
		paramsField:  "parameters",
		paramKind:    toSet([]string{"required_parameter", "optional_parameter", "identifier"}),
		primitives:   toSet([]string{"number", "string", "boolean", "bigint", "symbol"}),
		detectFields: selfFieldAccess("member_expression", "property", map[string]bool{"this": true}),
		detectErrors: func(n *gts.Node, p *parsed, out map[string]bool) {
			// An empty catch block: `catch (e) {}`.
			if n.Type(p.lang.Grammar) == "catch_clause" {
				if body := n.ChildByFieldName("body", p.lang.Grammar); body != nil && body.NamedChildCount() == 0 {
					out["empty_catch"] = true
				}
			}
		},
	}
}
