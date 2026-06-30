// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import (
	"testing"
	"time"
)

// TestRangeTokenInCondition covers splitConditionTokens' parenthesised-range
// token scan (a range literal as a comparison operand).
func TestRangeTokenInCondition(t *testing.T) {
	eq(t, "{% if x == (1..3) %}y{% else %}n{% endif %}", map[string]any{"x": []any{1, 2, 3}}, "y")
	eq(t, "{% if (1..3) contains 2 %}y{% else %}n{% endif %}", nil, "y")
}

// TestBlankConditionToken covers parseComparison's 0-token (blank) case under
// strict mode (a condition that tokenises to nothing on one side of and).
func TestBlankConditionToken(t *testing.T) {
	if _, err := Parse("{% if a and %}x{% endif %}", WithErrorMode(Strict)); err == nil {
		t.Error("blank condition should error in strict mode")
	}
}

// TestTimeTimeInput covers toTime's time.Time arm and date.go's float fall-through.
func TestTimeAndFloatDate(t *testing.T) {
	tm := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	eq(t, "{{ t | date: '%Y' }}", map[string]any{"t": tm}, "2026")
	// A float is not a date type: toTime returns false, the value passes through.
	eq(t, "{{ x | date: '%Y' }}", map[string]any{"x": 3.5}, "3.5")
}

// TestMidnightHour covers strftime %I at hour 0 (-> 12).
func TestMidnightHour(t *testing.T) {
	eq(t, "{{ t | date: '%I %p' }}", map[string]any{"t": "2026-06-30 00:30:00"}, "12 AM")
}

// TestDynamicNestedIndex covers expr.go's nested-bracket depth in a[b[0]].
func TestDynamicNestedIndex(t *testing.T) {
	eq(t, "{{ a[b[0]] }}", map[string]any{"a": []any{"x", "y", "z"}, "b": []any{2}}, "z")
}

// TestSplitAttrsBlankAndScanColon covers splitAttrs' blank-word skip, isAttrWord
// false path, parseAttr, and scanTopColon bracket skipping.
func TestSplitAttrsArms(t *testing.T) {
	// Extra spaces produce blank words that splitAttrs skips.
	eq(t, "{% for i in  (1..3)   limit:2 %}{{ i }}{% endfor %}", nil, "12")
	// A cycle group whose name contains a bracketed colon is scanned correctly.
	eq(t, "{% cycle x[0]: 'a', 'b' %}{% cycle x[0]: 'a', 'b' %}", map[string]any{"x": []any{"g"}}, "ab")
}

// TestTruncateNegativeCut covers truncate when the ellipsis is longer than n.
func TestTruncateNegativeCut(t *testing.T) {
	eq(t, "{{ 'hello world' | truncate: 1, '......' }}", nil, "......")
}

// TestSliceEndClamp covers sliceSeq's end>n clamp.
func TestSliceEndClamp(t *testing.T) {
	eq(t, "{{ 'abc' | slice: 1, 100 }}", nil, "bc")
}

// TestConcatNoArgs covers concat with no argument (returns the input array).
func TestConcatNoArgs(t *testing.T) {
	eq(t, "{{ a | concat | join: ',' }}", map[string]any{"a": []any{1, 2}}, "1,2")
}

// TestRatOfNonNumeric covers ratOf's default arm (a bool operand).
func TestRatOfNonNumeric(t *testing.T) {
	eq(t, "{{ x | plus: 1.0 }}", map[string]any{"x": true}, "1.0")
}

// TestForLimitNegative covers the for limit<0 clamp.
func TestForLimitNegative(t *testing.T) {
	eq(t, "{% for i in (1..3) limit:-1 %}{{ i }}{% else %}none{% endfor %}", nil, "none")
}

// TestTablerowStrictBodyError covers tablerow's non-interrupt error propagation.
func TestTablerowStrictBodyError(t *testing.T) {
	if _, err := MustParse("{% tablerow i in (1..2) %}{{ 1 | divided_by: 0 }}{% endtablerow %}").RenderStrict(nil); err == nil {
		t.Error("tablerow strict body error should propagate")
	}
}

// TestFloatNegativeZero covers formatFloat(-0.0).
func TestFloatNegativeZero(t *testing.T) {
	if got := formatFloat(negZero()); got != "-0.0" {
		t.Errorf("formatFloat(-0.0) = %q", got)
	}
}

func negZero() float64 {
	z := 0.0
	return -z
}

// TestNormalizeSciNoMantissaDot covers the mantissa "no dot -> .0" branch (e.g.
// an exact power of ten in exponent range).
func TestNormalizeSciMantissa(t *testing.T) {
	eq(t, "{{ x | plus: 0.0 }}", map[string]any{"x": 1e17}, "1.0e+17")
}
