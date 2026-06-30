// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import (
	"strconv"
	"strings"
)

// expr is an evaluable Liquid expression: a literal, a variable lookup, or a
// range. Expression evaluation is total (it never fails — an undefined lookup
// resolves to nil); recoverable errors arise only from filters, handled by the
// output/assign nodes. Filters are layered on top by the output node, not here.
type expr interface {
	eval(ctx *context) any
}

// literalExpr is a constant value (string/number/bool/nil) or one of the
// special keywords (blank/empty) carried as sentinel types.
type literalExpr struct{ v any }

func (e literalExpr) eval(*context) any { return e.v }

// blankSentinel and emptySentinel back the `blank` / `empty` keywords used in
// comparisons (x == blank, x == empty).
type blankSentinel struct{}
type emptySentinel struct{}

// lookupExpr is a variable reference: a base name followed by dotted/[indexed]
// accessors, e.g. a.b[0]['c'].size.
type lookupExpr struct {
	name string
	keys []lookupKey
}

// lookupKey is one accessor step: either a constant key/index or a dynamic
// expression (a[var]).
type lookupKey struct {
	static  any  // string key or int index, when dyn == nil
	dyn     expr // dynamic key expression (a[x])
	dynamic bool
}

func (e lookupExpr) eval(ctx *context) any {
	cur, ok := ctx.get(e.name)
	if !ok {
		// A base name that is not a variable resolves to nil (the gem treats an
		// undefined variable as nil).
		cur = nil
	}
	for _, k := range e.keys {
		var key any
		if k.dynamic {
			key = k.dyn.eval(ctx)
		} else {
			key = k.static
		}
		cur = index(cur, key)
	}
	return cur
}

// rangeExpr is a literal range (begin..end); bounds may be expressions.
type rangeExpr struct{ lo, hi expr }

func (e rangeExpr) eval(ctx *context) any {
	a := toInt(e.lo.eval(ctx))
	b := toInt(e.hi.eval(ctx))
	if b < a {
		return []any{}
	}
	out := make([]any, 0, b-a+1)
	for i := a; i <= b; i++ {
		out = append(out, i)
	}
	return out
}

// parseExpr parses a single value expression (no filters): a literal, a range,
// or a variable lookup.
func parseExpr(s string) (expr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return literalExpr{nil}, nil
	}
	// Range literal: (a..b)
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		inner := s[1 : len(s)-1]
		if i := strings.Index(inner, ".."); i >= 0 {
			lo, err := parseExpr(inner[:i])
			if err != nil {
				return nil, err
			}
			hi, err := parseExpr(inner[i+2:])
			if err != nil {
				return nil, err
			}
			return rangeExpr{lo, hi}, nil
		}
	}
	if lit, ok := parseLiteral(s); ok {
		return literalExpr{lit}, nil
	}
	return parseLookup(s)
}

// parseLiteral recognises the Liquid scalar literals: quoted strings, integers,
// floats, true/false, nil/null, empty, blank.
func parseLiteral(s string) (any, bool) {
	switch s {
	case "true":
		return true, true
	case "false":
		return false, true
	case "nil", "null":
		return nil, true
	case "empty":
		return emptySentinel{}, true
	case "blank":
		return blankSentinel{}, true
	}
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1], true
		}
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return int(i), true
	}
	if isFloatLiteral(s) {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, true
		}
	}
	return nil, false
}

// isFloatLiteral reports whether s is a plain decimal float (digits with a single
// dot), excluding scientific notation which Liquid does not accept as a literal.
func isFloatLiteral(s string) bool {
	if s == "" {
		return false
	}
	body := s
	if body[0] == '-' || body[0] == '+' {
		body = body[1:]
	}
	dots := 0
	hasDigit := false
	for _, r := range body {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r == '.':
			dots++
		default:
			return false
		}
	}
	return hasDigit && dots == 1
}

// parseLookup parses a variable lookup: name then .key or [expr] accessors.
func parseLookup(s string) (expr, error) {
	lk := &lookupExpr{}
	i := 0
	n := len(s)
	// Base name.
	start := i
	for i < n && s[i] != '.' && s[i] != '[' {
		i++
	}
	lk.name = strings.TrimSpace(s[start:i])
	if lk.name == "" {
		return nil, syntaxErr("Variable '" + s + "' was not properly terminated")
	}
	// The base-name scan above stops only at '.' or '[', so every accessor here
	// begins with one of those two characters.
	for i < n {
		if s[i] == '.' {
			i++
			ks := i
			for i < n && s[i] != '.' && s[i] != '[' {
				i++
			}
			key := strings.TrimSpace(s[ks:i])
			if key == "" {
				return nil, syntaxErr("Variable '" + s + "' was not properly terminated")
			}
			lk.keys = append(lk.keys, lookupKey{static: key})
			continue
		}
		// s[i] == '['
		depth := 1
		i++
		ks := i
		for i < n && depth > 0 {
			if s[i] == '[' {
				depth++
			} else if s[i] == ']' {
				depth--
				if depth == 0 {
					break
				}
			}
			i++
		}
		if depth != 0 {
			return nil, syntaxErr("Variable '" + s + "' was not properly terminated")
		}
		inner := strings.TrimSpace(s[ks:i])
		i++ // consume ]
		if lit, ok := parseLiteral(inner); ok {
			lk.keys = append(lk.keys, lookupKey{static: lit})
		} else {
			dyn, err := parseExpr(inner)
			if err != nil {
				return nil, err
			}
			lk.keys = append(lk.keys, lookupKey{dyn: dyn, dynamic: true})
		}
	}
	return lk, nil
}
