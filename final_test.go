// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import "testing"

// TestConditionRangeAndRightError covers the parenthesised-range token branch in
// splitConditionTokens and the right-hand error propagation in condition.eval.
func TestConditionRangeAndRightError(t *testing.T) {
	// A range literal on the right of contains (membership test).
	eq(t, "{% if (1..3) contains 2 %}y{% else %}n{% endif %}", nil, "y")
	eq(t, "{% if (1..3) contains 9 %}y{% else %}n{% endif %}", nil, "n")
	// Nested parens inside the range token exercise the depth counter.
	eq(t, "{% if x == (1..3) %}y{% else %}n{% endif %}", map[string]any{"x": []any{1, 2, 3}}, "y")
	// A comparison error on the RIGHT side of `and` propagates (inline in lax).
	out := render(t, "{% if true and a > 'x' %}y{% endif %}", map[string]any{"a": 5})
	if out == "" || out[:6] != "Liquid" {
		t.Errorf("right-side error = %q", out)
	}
}

// TestStrictConditionSubParseErrors covers buildCondition/parseComparison error
// returns: a malformed operand in a 1-token and a 3-token comparison, and in the
// left and right of an and/or chain.
func TestStrictConditionSubParseErrors(t *testing.T) {
	for _, src := range []string{
		"{% if a. %}x{% endif %}",        // 1-token, malformed
		"{% if a. == 1 %}x{% endif %}",   // 3-token, left malformed
		"{% if 1 == a. %}x{% endif %}",   // 3-token, right malformed
		"{% if a. and b %}x{% endif %}",  // and: left malformed
		"{% if a and b. %}x{% endif %}",  // and: right malformed
		"{% if a badop b %}x{% endif %}", // invalid operator
		"{% if a b c d e %}x{% endif %}", // too many tokens
	} {
		if _, err := Parse(src, WithErrorMode(Strict)); err == nil {
			t.Errorf("Parse(%q) expected error", src)
		}
	}
}

// TestCmpBlankEmptyDefault covers the non-==/!= branch of cmpBlankEmpty.
func TestCmpBlankEmptyDefault(t *testing.T) {
	eq(t, "{% if x < blank %}y{% else %}n{% endif %}", map[string]any{"x": ""}, "n")
	eq(t, "{% if x > empty %}y{% else %}n{% endif %}", map[string]any{"x": []any{}}, "n")
}

// TestValueEqualDefault covers valueEqual's fall-through (a map compared by ==).
func TestValueEqualDefault(t *testing.T) {
	eq(t, "{% if x == y %}y{% else %}n{% endif %}",
		map[string]any{"x": map[string]any{"a": 1}, "y": map[string]any{"a": 1}}, "n")
}

// TestContainsDefault covers contains on an unsupported left type.
func TestContainsDefault(t *testing.T) {
	eq(t, "{% if x contains 1 %}y{% else %}n{% endif %}", map[string]any{"x": 5}, "n")
}

// TestDateBlankAndUnparseable covers toTime's blank/non-string/unparseable arms.
func TestDateEdgeInputs(t *testing.T) {
	eq(t, "{{ x | date: '%Y' }}", map[string]any{"x": nil}, "")              // nil -> input
	eq(t, "{{ x | date: '%Y' }}", map[string]any{"x": false}, "false")       // false (non-date) -> input, renders "false"
	eq(t, "{{ x | date: '%Y' }}", map[string]any{"x": "zzz"}, "zzz")         // unparseable string -> input
	eq(t, "{{ x | date }}", map[string]any{"x": "2026-06-30"}, "2026-06-30") // empty format -> input
	eq(t, "{{ x | date: '%Q' }}", map[string]any{"x": "2026-06-30"}, "%Q")   // unknown directive
}

// TestFilterArgDefaults covers truncate/truncatewords custom ellipsis and slice.
func TestFilterArgDefaults(t *testing.T) {
	eq(t, "{{ 'hello world' | truncate: 8, '--' }}", nil, "hello --")
	eq(t, "{{ 'a b c d' | truncatewords: 2, '!' }}", nil, "a b!")
	eq(t, "{{ a | slice: -1 | join: ',' }}", map[string]any{"a": []any{1, 2, 3}}, "3")
}

// TestSliceSeqArrayAndEmpty covers sliceSeq's array-return and empty-return arms.
func TestSliceSeqArms(t *testing.T) {
	eq(t, "{{ a | slice: 0, 2 | join: ',' }}", map[string]any{"a": []any{9, 8, 7}}, "9,8")
	eq(t, "{{ a | slice: 9 | join: ',' }}", map[string]any{"a": []any{1}}, "") // out of range array
}

// TestConcatNonArrayArg covers concat's error arm.
func TestConcatNonArray(t *testing.T) {
	out := render(t, "{{ a | concat: b }}", map[string]any{"a": []any{1}, "b": 5})
	if out[:6] != "Liquid" {
		t.Errorf("concat error = %q", out)
	}
}

// TestRatOfArms covers ratOf for int64 and an unparseable string.
func TestRatOfArms(t *testing.T) {
	eq(t, "{{ x | plus: 1.0 }}", map[string]any{"x": int64(2)}, "3.0")
	eq(t, "{{ 'abc' | plus: 1.5 }}", nil, "1.5") // string -> 0 via float fallback
}

// TestForOffsetBeyondLength covers the offset>len clamp arm.
func TestForOffsetBeyondLength(t *testing.T) {
	eq(t, "{% for i in (1..2) offset:5 %}{{ i }}{% else %}none{% endfor %}", nil, "none")
}

// TestCaseElse covers the case else arm and a no-match-no-else.
func TestCaseElseArm(t *testing.T) {
	eq(t, "{% case x %}{% when 1 %}a{% else %}b{% endcase %}", map[string]any{"x": 9}, "b")
	eq(t, "{% case x %}{% when 1 %}a{% endcase %}", map[string]any{"x": 9}, "")
}

// TestTokenAppendTextEmpty covers appendText's empty-string short-circuit via a
// template that produces an empty trailing text run after trimming.
func TestTrailingTrimEmpty(t *testing.T) {
	eq(t, "{{ 'a' -}}   ", nil, "a")
}

// TestIndexNegativeAndPseudo covers index() array negative wrap and size pseudo.
func TestIndexArms(t *testing.T) {
	eq(t, "{{ a[-1] }}", map[string]any{"a": []any{1, 2, 3}}, "3")
	eq(t, "{{ a[-9] }}", map[string]any{"a": []any{1}}, "") // out of range after wrap
	eq(t, "{{ a.size }}", map[string]any{"a": []any{1, 2}}, "2")
}
