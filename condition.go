// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import "strings"

// condition is a boolean expression built from comparisons joined by and/or.
// Liquid evaluates and/or strictly right-to-left with equal precedence, which
// this right-leaning tree reproduces.
type condition struct {
	left  comparison
	op    string // "and" / "or" / "" (leaf)
	right *condition
}

func (c *condition) eval(ctx *context) (bool, *Error) {
	lv, err := c.left.eval(ctx)
	if err != nil {
		return false, err
	}
	if c.op == "" {
		return lv, nil
	}
	rv, err := c.right.eval(ctx)
	if err != nil {
		return false, err
	}
	if c.op == "and" {
		return lv && rv, nil
	}
	return lv || rv, nil
}

// comparison.eval may return an *Error (an ordering type mismatch is an
// ArgumentError), so condition.eval keeps its error return.

// comparison is a single relational test, or a bare expression tested for
// truthiness when op == "".
type comparison struct {
	lhs expr
	op  string
	rhs expr
}

func (cm comparison) eval(ctx *context) (bool, *Error) {
	l := cm.lhs.eval(ctx)
	if cm.op == "" {
		return truthy(l), nil
	}
	r := cm.rhs.eval(ctx)
	return compare(l, cm.op, r)
}

// parseCondition parses a full boolean expression (if/unless body, when is just
// a value). It tokenises on the boolean keywords and the relational operators.
func parseCondition(s string) (*condition, error) {
	return buildCondition(splitConditionTokens(s))
}

// splitConditionTokens breaks a condition string into operand/operator tokens,
// respecting quoted strings and parenthesised ranges.
func splitConditionTokens(s string) []string {
	var toks []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			toks = append(toks, cur.String())
			cur.Reset()
		}
	}
	i := 0
	n := len(s)
	for i < n {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			flush()
			i++
		case c == '\'' || c == '"':
			// Consume a quoted string as a single token.
			q := c
			start := i
			i++
			for i < n && s[i] != q {
				i++
			}
			if i < n {
				i++ // closing quote
			}
			cur.WriteString(s[start:i])
			flush()
		case c == '(':
			// Consume a parenthesised range literal — e.g. (1..5) — as one token.
			// Liquid ranges do not nest, so a scan to the closing ')' suffices.
			start := i
			i++
			for i < n && s[i] != ')' {
				i++
			}
			if i < n {
				i++ // include the ')'
			}
			cur.WriteString(s[start:i])
			flush()
		case strings.HasPrefix(s[i:], "=="), strings.HasPrefix(s[i:], "!="),
			strings.HasPrefix(s[i:], "<="), strings.HasPrefix(s[i:], ">="):
			flush()
			toks = append(toks, s[i:i+2])
			i += 2
		case c == '<' || c == '>':
			flush()
			toks = append(toks, string(c))
			i++
		default:
			cur.WriteByte(c)
			i++
		}
	}
	flush()
	return toks
}

// isBoolKw reports the boolean-join keywords.
func isBoolKw(t string) bool { return t == "and" || t == "or" }

// isRelOp reports the relational operators.
func isRelOp(t string) bool {
	switch t {
	case "==", "!=", "<", ">", "<=", ">=", "contains":
		return true
	}
	return false
}

// buildCondition assembles tokens into a right-leaning and/or tree of
// comparisons, matching Liquid's right-to-left evaluation.
func buildCondition(toks []string) (*condition, error) {
	// Find the FIRST boolean keyword; everything to its right is the nested
	// condition, so the tree leans right (right-to-left evaluation).
	for i, t := range toks {
		if isBoolKw(t) {
			left, err := parseComparison(toks[:i])
			if err != nil {
				return nil, err
			}
			right, err := buildCondition(toks[i+1:])
			if err != nil {
				return nil, err
			}
			return &condition{left: left, op: t, right: right}, nil
		}
	}
	cm, err := parseComparison(toks)
	if err != nil {
		return nil, err
	}
	return &condition{left: cm}, nil
}

// parseComparison parses up to three tokens into a comparison.
func parseComparison(toks []string) (comparison, error) {
	switch len(toks) {
	case 0:
		return comparison{}, syntaxErr("blank condition")
	case 1:
		e, err := parseExpr(toks[0])
		if err != nil {
			return comparison{}, err
		}
		return comparison{lhs: e}, nil
	case 3:
		if !isRelOp(toks[1]) {
			return comparison{}, syntaxErr("invalid operator " + toks[1])
		}
		l, err := parseExpr(toks[0])
		if err != nil {
			return comparison{}, err
		}
		r, err := parseExpr(toks[2])
		if err != nil {
			return comparison{}, err
		}
		return comparison{lhs: l, op: toks[1], rhs: r}, nil
	default:
		return comparison{}, syntaxErr("invalid condition: " + strings.Join(toks, " "))
	}
}

// compare applies a relational operator with Liquid semantics, including the
// blank/empty sentinels and `contains`.
func compare(l any, op string, r any) (bool, *Error) {
	switch r.(type) {
	case blankSentinel:
		return cmpBlankEmpty(l, op, isBlank(l))
	case emptySentinel:
		return cmpBlankEmpty(l, op, isEmpty(l))
	}
	switch l.(type) {
	case blankSentinel:
		return cmpBlankEmpty(r, op, isBlank(r))
	case emptySentinel:
		return cmpBlankEmpty(r, op, isEmpty(r))
	}
	switch op {
	case "==":
		return valueEqual(l, r), nil
	case "!=":
		return !valueEqual(l, r), nil
	case "contains":
		return contains(l, r), nil
	default: // one of < > <= >= (parseComparison admits no other operator)
		return orderCompare(l, op, r)
	}
}

func cmpBlankEmpty(_ any, op string, match bool) (bool, *Error) {
	switch op {
	case "==":
		return match, nil
	case "!=":
		return !match, nil
	}
	return false, nil
}

// valueEqual implements Liquid == for scalars, arrays, and maps.
func valueEqual(l, r any) bool {
	if isNumber(l) && isNumber(r) {
		return toFloat(l) == toFloat(r)
	}
	switch a := l.(type) {
	case nil:
		return r == nil
	case string:
		b, ok := r.(string)
		return ok && a == b
	case bool:
		b, ok := r.(bool)
		return ok && a == b
	case []any:
		b, ok := r.([]any)
		if !ok || len(a) != len(b) {
			return false
		}
		for i := range a {
			if !valueEqual(a[i], b[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// orderCompare implements < > <= >= for numbers and strings.
func orderCompare(l any, op string, r any) (bool, *Error) {
	var c int
	switch {
	case isNumber(l) && isNumber(r):
		lf, rf := toFloat(l), toFloat(r)
		switch {
		case lf < rf:
			c = -1
		case lf > rf:
			c = 1
		}
	default:
		ls, lok := l.(string)
		rs, rok := r.(string)
		if !lok || !rok {
			return false, argErr("comparison of " + className(l) + " with " + className(r) + " failed")
		}
		c = strings.Compare(ls, rs)
	}
	switch op {
	case "<":
		return c < 0, nil
	case ">":
		return c > 0, nil
	case "<=":
		return c <= 0, nil
	default: // ">="
		return c >= 0, nil
	}
}

// contains implements `a contains b`: substring for strings, membership for
// arrays, key presence for maps.
func contains(l, r any) bool {
	switch a := l.(type) {
	case string:
		return strings.Contains(a, toStr(r))
	case []any:
		for _, e := range a {
			if valueEqual(e, r) {
				return true
			}
		}
		return false
	case map[string]any:
		_, ok := a[toStr(r)]
		return ok
	}
	return false
}

func isNumber(v any) bool {
	switch v.(type) {
	case int, int64, float64:
		return true
	}
	return false
}

func className(v any) string {
	switch v.(type) {
	case nil:
		return "NilClass"
	case string:
		return "String"
	case bool:
		return "Boolean"
	case int, int64:
		return "Integer"
	case float64:
		return "Float"
	case []any:
		return "Array"
	case map[string]any:
		return "Hash"
	}
	return "Object"
}
