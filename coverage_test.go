// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import (
	"strings"
	"testing"
	"time"
)

// errParse asserts that strict-mode Parse fails for src.
func errParse(t *testing.T, src string) {
	t.Helper()
	if _, err := Parse(src, WithErrorMode(Strict)); err == nil {
		t.Errorf("Parse(%q) expected a syntax error", src)
	}
}

// TestParseErrorBranches drives every parser error path.
func TestParseErrorBranches(t *testing.T) {
	errParse(t, "{% bogus %}")           // unknown tag
	errParse(t, "{% endfor %}")          // leftover ender
	errParse(t, "{% endcase %}")         // leftover ender
	errParse(t, "{% for %}{% endfor %}") // for missing parts
	errParse(t, "{% for x of y %}{% endfor %}")
	errParse(t, "{% tablerow %}{% endtablerow %}")
	errParse(t, "{% assign %}")
	errParse(t, "{% assign = 1 %}")
	errParse(t, "{% capture %}{% endcapture %}")
	errParse(t, "{% comment %}unterminated")
	errParse(t, "{% raw %}unterminated")
	errParse(t, "{% capture x %}unterminated")
	errParse(t, "{% for x in (1..2) %}unterminated")
	errParse(t, "{% tablerow x in (1..2) %}unterminated")
	errParse(t, "{{ a. }}")        // malformed lookup (trailing dot)
	errParse(t, "{{ a[ }}")        // unterminated bracket
	errParse(t, "{{ a.. }}")       // empty dotted key
	errParse(t, "{{ . }}")         // empty base name
	errParse(t, "{{ a | f: a. }}") // filter arg lookup malformed
	errParse(t, "{% if a. %}x{% endif %}")
	errParse(t, "{% case a. %}{% when 1 %}x{% endcase %}")
	errParse(t, "{% for i in a. %}{% endfor %}")
	errParse(t, "{% assign x = a. %}")
	errParse(t, "{% cycle a. %}")
	errParse(t, "{% if a < %}x{% endif %}")     // comparison missing rhs handled as 2 tokens
	errParse(t, "{% if a b c d %}x{% endif %}") // too many tokens
	errParse(t, "{% if a $ b %}x{% endif %}")   // bad operator
}

// TestLaxSwallowsParseIssues verifies that in lax (default) mode, a number of
// these still parse (the gem is permissive); we only assert no panic.
func TestLaxParse(t *testing.T) {
	for _, src := range []string{
		"{{ a | upcase | downcase }}",
		"{% if a %}x{% endif %}",
		"plain text only",
		"{% assign x = 1 %}",
	} {
		if _, err := Parse(src); err != nil {
			t.Errorf("lax Parse(%q) = %v", src, err)
		}
	}
}

// TestStringFilterEdges covers the remaining string-filter branches.
func TestStringFilterEdges(t *testing.T) {
	eq(t, "{{ 'abc' | replace }}", nil, "abc")           // no args
	eq(t, "{{ 'abc' | replace: '' }}", nil, "abc")       // empty target
	eq(t, "{{ 'abc' | replace: 'a' }}", nil, "bc")       // replace, no repl
	eq(t, "{{ 'aaa' | replace_first: 'a' }}", nil, "aa") // first, no repl
	eq(t, "{{ 'hello' | truncate }}", nil, "hello")      // default len 50, short
	eq(t, "{{ s | truncate: 3 }}", map[string]any{"s": strings.Repeat("x", 10)}, "...")
	eq(t, "{{ 'one two' | truncatewords }}", nil, "one two") // default 15, short
	eq(t, "{{ 'a b c' | truncatewords: 0 }}", nil, "a...")   // n<1 clamps to 1
	eq(t, "{{ '' | split: '' }}", nil, "")                   // empty input, empty sep
	eq(t, "{{ 'abc' | split: '' | size }}", nil, "3")        // char split
	eq(t, "{{ 'abc' | slice: 1 }}", nil, "b")                // single char
	eq(t, "{{ 'abc' | slice: 1, 0 }}", nil, "")              // zero length
	eq(t, "{{ x | upcase }}", nil, "")                       // nil to string filter
	eq(t, "{{ 'a b' | url_encode }}", nil, "a+b")
}

// TestArrayFilterEdges covers remaining array-filter branches.
func TestArrayFilterEdges(t *testing.T) {
	eq(t, "{{ x | first }}", nil, "")                                          // first of non-collection
	eq(t, "{{ x | last }}", nil, "")                                           // last of non-collection
	eq(t, "{{ 5 | first }}", nil, "")                                          // first of number
	eq(t, "{{ x | join }}", map[string]any{"x": []any{1, 2}}, "1 2")           // default sep
	eq(t, "{{ x | map: 'k' | join: ',' }}", map[string]any{"x": "scalar"}, "") // map over scalar->[scalar], index nil
	eq(t, "{{ x | concat: y | join: ',' }}", map[string]any{"x": []any{1}, "y": []any{}}, "1")
	eq(t, "{{ x | size }}", map[string]any{"x": map[string]any{"a": 1}}, "1")
	eq(t, "{{ x | reverse | join: ',' }}", map[string]any{"x": []any{}}, "")
}

// TestMathFilterEdges covers numeric branches.
func TestMathFilterEdges(t *testing.T) {
	eq(t, "{{ 5 | plus: 'x' }}", nil, "5") // string arg ->0
	eq(t, "{{ 5 | minus: 2 }}", nil, "3")
	eq(t, "{{ 2 | times: 3 }}", nil, "6")
	eq(t, "{{ 5.0 | times: 2 }}", nil, "10.0")
	eq(t, "{{ 5.0 | minus: 2 }}", nil, "3.0")
	eq(t, "{{ -3.2 | abs }}", nil, "3.2")
	eq(t, "{{ 3 | abs }}", nil, "3")
	eq(t, "{{ 3.4 | round: 0 }}", nil, "3")
	eq(t, "{{ 3.456 | round: 1 }}", nil, "3.5")
	eq(t, "{{ 5 | at_least: 3 }}", nil, "5") // int>=int keep
	eq(t, "{{ 5 | at_most: 9 }}", nil, "5")
	eq(t, "{{ 'x' | plus: 1.0 }}", nil, "1.0") // ratOf string fallback
}

// TestDateAllDirectives exercises every strftime directive branch.
func TestDateAllDirectives(t *testing.T) {
	old := nowFunc
	nowFunc = func() time.Time { return time.Date(2026, 6, 30, 9, 7, 3, 0, time.UTC) }
	defer func() { nowFunc = old }()
	eq(t, "{{ 'now' | date: '%Y %y %m %d %e %H %I %M %S %p %P %A %a %B %b %h %j %w %% q' }}",
		nil, "2026 26 06 30 30 09 09 07 03 AM am Tuesday Tue June Jun Jun 181 2 % q")
	// %e space-pads a single-digit day; an afternoon hour exercises PM/I.
	nowFunc = func() time.Time { return time.Date(2026, 1, 5, 15, 0, 0, 0, time.UTC) }
	eq(t, "{{ 'now' | date: '%e %I %p %P' }}", nil, " 5 03 PM pm")
	// "today" keyword and integer-string input.
	eq(t, "{{ 'today' | date: '%Y' }}", nil, "2026")
	eq(t, "{{ '1700000000' | date: '%Y' }}", map[string]any{}, "2023")
	// int64 input.
	eq(t, "{{ x | date: '%Y' }}", map[string]any{"x": int64(0)}, "1970")
	// blank (nil) returns input unchanged (renders empty).
	eq(t, "{{ x | date: '%Y' }}", nil, "")
	// various parseable layouts.
	for _, s := range []string{
		"2026/06/30", "06/30/2026", "June 30, 2026", "Jun 30, 2026",
		"Mon Jun 30 09:00:00 2026", "2026-06-30T09:00:00Z",
	} {
		if out := render(t, "{{ d | date: '%Y' }}", map[string]any{"d": s}); out != "2026" {
			t.Errorf("date parse %q -> %q", s, out)
		}
	}
}

// TestConditionEdges covers comparison/condition branches.
func TestConditionEdges(t *testing.T) {
	eq(t, "{% if a == b %}y{% else %}n{% endif %}", map[string]any{"a": nil, "b": nil}, "y")
	eq(t, "{% if a == b %}y{% else %}n{% endif %}", map[string]any{"a": nil}, "y") // nil==nil
	eq(t, "{% if a == b %}y{% else %}n{% endif %}", map[string]any{"a": true, "b": true}, "y")
	eq(t, "{% if a == b %}y{% else %}n{% endif %}", map[string]any{"a": true, "b": false}, "n")
	eq(t, "{% if a == b %}y{% else %}n{% endif %}", map[string]any{"a": []any{1}, "b": []any{1, 2}}, "n")
	eq(t, "{% if a == 1 %}y{% else %}n{% endif %}", map[string]any{"a": "s"}, "n") // string!=number
	eq(t, "{% if 1.0 == 1 %}y{% endif %}", nil, "y")                               // number eq cross-type
	eq(t, "{% if a > b %}y{% else %}n{% endif %}", map[string]any{"a": 1.0, "b": 2}, "n")
	eq(t, "{% if a <= b %}y{% endif %}", map[string]any{"a": "a", "b": "b"}, "y")
	eq(t, "{% if x contains 1 %}y{% else %}n{% endif %}", map[string]any{"x": 5}, "n") // contains on number
	eq(t, "{% if x != empty %}y{% else %}n{% endif %}", map[string]any{"x": "z"}, "y")
	eq(t, "{% if x != blank %}y{% else %}n{% endif %}", map[string]any{"x": []any{}}, "n")
	// blank/empty on the left side of != .
	eq(t, "{% if empty != x %}y{% else %}n{% endif %}", map[string]any{"x": "z"}, "y")
	// `and`/`or` chains of three.
	eq(t, "{% if true and true and true %}y{% endif %}", nil, "y")
	eq(t, "{% if false or false or true %}y{% endif %}", nil, "y")
}

// TestBlankEmptyValues covers the isBlank/isEmpty branches.
func TestBlankEmptyValues(t *testing.T) {
	eq(t, "{% if x == blank %}y{% else %}n{% endif %}", map[string]any{"x": nil}, "y")
	eq(t, "{% if x == blank %}y{% else %}n{% endif %}", map[string]any{"x": false}, "y")
	eq(t, "{% if x == blank %}y{% else %}n{% endif %}", map[string]any{"x": []any{}}, "y")
	eq(t, "{% if x == blank %}y{% else %}n{% endif %}", map[string]any{"x": map[string]any{}}, "y")
	eq(t, "{% if x == blank %}y{% else %}n{% endif %}", map[string]any{"x": 5}, "n")
	eq(t, "{% if x == empty %}y{% else %}n{% endif %}", map[string]any{"x": map[string]any{}}, "y")
	eq(t, "{% if x == empty %}y{% else %}n{% endif %}", map[string]any{"x": "a"}, "n")
	eq(t, "{% if x == empty %}y{% else %}n{% endif %}", map[string]any{"x": 5}, "n")
}

// TestDefaultAllowFalseBranches covers defaultFilter paths.
func TestDefaultAllowFalseBranches(t *testing.T) {
	eq(t, "{{ x | default: 'd', allow_false: true }}", map[string]any{"x": nil}, "d")
	eq(t, "{{ x | default: 'd', allow_false: true }}", map[string]any{"x": ""}, "d")
	eq(t, "{{ x | default: 'd', allow_false: true }}", map[string]any{"x": false}, "false")
	eq(t, "{{ x | default: 'd', allow_false: true }}", map[string]any{"x": "v"}, "v")
	eq(t, "{{ x | default: 'd' }}", map[string]any{"x": []any{1}}, "1")
}

// TestLookupAndIndexEdges covers index() and lookup branches.
func TestLookupAndIndexEdges(t *testing.T) {
	eq(t, "{{ a.first }}", map[string]any{"a": []any{}}, "") // empty array .first
	eq(t, "{{ a.last }}", map[string]any{"a": []any{}}, "")  // empty array .last
	eq(t, "{{ a.first }}", map[string]any{"a": []any{1, 2}}, "1")
	eq(t, "{{ a.last }}", map[string]any{"a": []any{1, 2}}, "2")
	eq(t, "{{ a.bogus }}", map[string]any{"a": []any{1}}, "")
	eq(t, "{{ a[0][1] }}", map[string]any{"a": []any{[]any{"x", "y"}}}, "y")
	// Drop lookups: a forloop in scope reached via a missing member returns nil.
	eq(t, "{% for i in (1..1) %}{{ forloop.bogus }}{% endfor %}", nil, "")
	// undefined base name with accessors.
	eq(t, "{{ missing.x.y }}", nil, "")
}

// TestNumberToStr covers toStr int64 and other branches.
func TestNumberToStr(t *testing.T) {
	eq(t, "{{ x }}", map[string]any{"x": int64(99)}, "99")
	eq(t, "{{ x }}", map[string]any{"x": int(42)}, "42")
	// nested array render concatenates.
	eq(t, "{{ x }}", map[string]any{"x": []any{[]any{1, 2}, "z"}}, "12z")
}

// TestWhitespaceAndComments covers tokenizer trim branches and comment text.
func TestWhitespaceAndComments(t *testing.T) {
	eq(t, "x  {%- assign y = 1 -%}  z", nil, "xz")
	eq(t, "{% comment %}a {{b}} {%c%}{% endcomment %}!", nil, "!")
	// trimRight with no following text token, trimLeft with no preceding token.
	eq(t, "{{- 'a' -}}", nil, "a")
}

// TestTablerowBranches covers tablerow limit/offset/cols and break.
func TestTablerowBranches(t *testing.T) {
	render(t, "{% tablerow i in (1..6) cols:2 limit:4 offset:1 %}{{i}}{% endtablerow %}", nil)
	render(t, "{% tablerow i in (1..3) cols:0 %}{{i}}{% endtablerow %}", nil) // cols<1 -> 1
	render(t, "{% tablerow i in (1..3) %}{% if i == 2 %}{% break %}{% endif %}{{i}}{% endtablerow %}", nil)
}

// TestStripBlankString covers isBlank for whitespace-only strings.
func TestStripBlankString(t *testing.T) {
	eq(t, "{{ x | default: 'd' }}", map[string]any{"x": "   "}, "d")
}
