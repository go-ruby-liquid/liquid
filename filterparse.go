// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import "strings"

// filterCall is one `| name: arg1, arg2` step in a filter chain.
type filterCall struct {
	name string
	args []expr
}

// parseFiltered splits a "{{ expr | f1: a | f2 }}" body into the leading
// expression and the filter chain.
func parseFiltered(body string) (expr, []filterCall, error) {
	parts := splitPipes(body)
	e, err := parseExpr(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, nil, err
	}
	var filters []filterCall
	for _, fp := range parts[1:] {
		fc, err := parseFilter(fp)
		if err != nil {
			return nil, nil, err
		}
		filters = append(filters, fc)
	}
	return e, filters, nil
}

// parseFilter parses one filter segment "name: a, b" into a filterCall.
func parseFilter(seg string) (filterCall, error) {
	seg = strings.TrimSpace(seg)
	name := seg
	rest := ""
	if i := strings.Index(seg, ":"); i >= 0 {
		name = strings.TrimSpace(seg[:i])
		rest = seg[i+1:]
	}
	fc := filterCall{name: name}
	for _, a := range splitTopLevel(rest, []string{","}) {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		// Named filter arguments (key: value, e.g. date filters) collapse to the
		// value; the standard set used here does not consume the key.
		if j := topColon(a); j >= 0 {
			a = strings.TrimSpace(a[j+1:])
		}
		e, err := parseExpr(a)
		if err != nil {
			return filterCall{}, err
		}
		fc.args = append(fc.args, e)
	}
	return fc, nil
}

// apply evaluates the filter's args and dispatches to the implementation.
func (fc filterCall) apply(ctx *context, in any) (any, *Error) {
	args := make([]any, len(fc.args))
	for i, ae := range fc.args {
		args[i] = ae.eval(ctx)
	}
	if fn, ok := ctx.filters[fc.name]; ok {
		out, err := fn(in, args)
		if err != nil {
			return nil, &Error{Type: "ArgumentError", Message: err.Error()}
		}
		return out, nil
	}
	return applyFilter(fc.name, in, args)
}

// splitPipes splits a body on top-level `|`, ignoring pipes inside quotes,
// brackets, or parentheses.
func splitPipes(s string) []string {
	return splitTopLevel(s, []string{"|"})
}

// splitTopLevel splits s on any of the given separators that appear at bracket
// depth zero and outside quotes. Multi-char separators (like " or ") match
// literally.
func splitTopLevel(s string, seps []string) []string {
	var out []string
	var cur strings.Builder
	depth := 0
	var quote byte
	i := 0
	for i < len(s) {
		c := s[i]
		if quote != 0 {
			cur.WriteByte(c)
			if c == quote {
				quote = 0
			}
			i++
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			cur.WriteByte(c)
			i++
			continue
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		}
		if depth == 0 {
			matched := false
			for _, sep := range seps {
				if strings.HasPrefix(s[i:], sep) {
					out = append(out, cur.String())
					cur.Reset()
					i += len(sep)
					matched = true
					break
				}
			}
			if matched {
				continue
			}
		}
		cur.WriteByte(c)
		i++
	}
	out = append(out, cur.String())
	return out
}

// splitAttrs separates a collection expression from trailing attribute words,
// e.g. "(1..5) limit:2 offset:1 reversed" -> "(1..5)", ["limit:2","offset:1",
// "reversed"]. It respects quotes and brackets so a quoted/ranged collection
// stays intact.
func splitAttrs(s string) (head string, attrs []string) {
	words := splitTopLevel(s, []string{" "})
	var coll []string
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		if isAttrWord(w) {
			attrs = append(attrs, w)
		} else if len(attrs) == 0 {
			coll = append(coll, w)
		} else {
			// A bare word after attributes started (e.g. "reversed").
			attrs = append(attrs, w)
		}
	}
	return strings.Join(coll, " "), attrs
}

// isAttrWord reports whether a word is a loop attribute (key:value or reversed).
func isAttrWord(w string) bool {
	if w == "reversed" {
		return true
	}
	i := strings.Index(w, ":")
	if i <= 0 {
		return false
	}
	key := w[:i]
	switch key {
	case "limit", "offset", "cols":
		return true
	}
	return false
}

// parseAttr parses a "key:valueExpr" attribute into its key and value expr.
func parseAttr(a string) (key string, ve expr, err error) {
	i := strings.Index(a, ":")
	if i < 0 {
		return "", nil, syntaxErr("invalid attribute: " + a)
	}
	key = strings.TrimSpace(a[:i])
	ve, err = parseExpr(strings.TrimSpace(a[i+1:]))
	return key, ve, err
}

// topColon returns the index of the first top-level ':' in s, or -1.
func topColon(s string) int { return scanTopColon(s) }

// indexTopColon is topColon for the cycle group separator.
func indexTopColon(s string) int { return scanTopColon(s) }

func scanTopColon(s string) int {
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case ':':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
