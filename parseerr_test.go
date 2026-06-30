// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import "testing"

// TestEveryParseErrorSlot puts a malformed expression ("a." — a trailing dot)
// into every parse position that evaluates a sub-expression, exercising each
// "if err != nil { return nil, err }" branch under strict mode.
func TestEveryParseErrorSlot(t *testing.T) {
	bad := []string{
		// output: leading expr and filter argument
		"{{ a. }}",
		"{{ x | append: a. }}",
		// assign source expr and assign filter
		"{% assign y = a. %}",
		"{% assign y = x | append: a. %}",
		// if / elsif / else conditions (comparison operands)
		"{% if a. %}x{% endif %}",
		"{% if a. == 1 %}x{% endif %}",
		"{% if 1 == a. %}x{% endif %}",
		"{% if a %}x{% elsif b. %}y{% endif %}",
		// unless condition
		"{% unless a. %}x{% endunless %}",
		// case subject and when values
		"{% case a. %}{% when 1 %}x{% endcase %}",
		"{% case x %}{% when a. %}y{% endcase %}",
		"{% case x %}{% when 1, a. %}y{% endcase %}",
		// for: collection, limit, offset
		"{% for i in a. %}{% endfor %}",
		"{% for i in (1..3) limit:a. %}{% endfor %}",
		"{% for i in (1..3) offset:a. %}{% endfor %}",
		// range lo and hi bounds malformed
		"{% for i in (a[..3) %}{% endfor %}",
		"{% for i in (1..a.) %}{% endfor %}",
		// tablerow: collection, cols, limit, offset
		"{% tablerow i in a. %}{% endtablerow %}",
		"{% tablerow i in (1..3) cols:a. %}{% endtablerow %}",
		"{% tablerow i in (1..3) limit:a. %}{% endtablerow %}",
		"{% tablerow i in (1..3) offset:a. %}{% endtablerow %}",
		// cycle: group and values
		"{% cycle a.: 'x' %}",
		"{% cycle a. %}",
		// dynamic index whose key sub-expression is malformed
		"{{ arr[a.] }}",
	}
	for _, src := range bad {
		if _, err := Parse(src, WithErrorMode(Strict)); err == nil {
			t.Errorf("Parse(%q) expected a syntax error", src)
		}
	}
}

// TestComparisonAndContainsEdges fills the remaining compare/contains/valueEqual
// branches.
func TestComparisonAndContainsEdges(t *testing.T) {
	// blank/empty on the left with == and != .
	eq(t, "{% if blank == x %}y{% else %}n{% endif %}", map[string]any{"x": ""}, "y")
	eq(t, "{% if blank != x %}y{% else %}n{% endif %}", map[string]any{"x": "z"}, "y")
	eq(t, "{% if empty == x %}y{% else %}n{% endif %}", map[string]any{"x": []any{}}, "y")
	eq(t, "{% if empty != x %}y{% else %}n{% endif %}", map[string]any{"x": []any{1}}, "y")
	// blank/empty comparison with an operator other than ==/!= yields false.
	eq(t, "{% if x > blank %}y{% else %}n{% endif %}", map[string]any{"x": "z"}, "n")
	// valueEqual: bool vs non-bool, array length mismatch already covered; add
	// nil != value and string vs non-string.
	eq(t, "{% if x == y %}y{% else %}n{% endif %}", map[string]any{"x": nil, "y": "a"}, "n")
	eq(t, "{% if x == y %}y{% else %}n{% endif %}", map[string]any{"x": true, "y": 1}, "n")
	// contains on an unsupported type (nil) is false.
	eq(t, "{% if x contains 'a' %}y{% else %}n{% endif %}", map[string]any{"x": nil}, "n")
	// order compare <= / >= on strings.
	eq(t, "{% if 'b' >= 'a' %}y{% endif %}", nil, "y")
}

// TestToStrToIntBranches fills toStr/toInt remaining branches.
func TestToStrToIntBranches(t *testing.T) {
	if toStr(struct{}{}) != "" {
		t.Error("toStr unknown")
	}
	if toInt(struct{}{}) != 0 {
		t.Error("toInt unknown")
	}
	if toFloat(struct{}{}) != 0 {
		t.Error("toFloat unknown")
	}
}

// TestParseExprEmptyAndWhitespace covers parseExpr trimming and empty bounds.
func TestParseExprEdges(t *testing.T) {
	eq(t, "{{   }}", nil, "")                                   // all-whitespace body
	eq(t, "{% for i in (1..1) %}{{ i }}{% endfor %}", nil, "1") // single-element range
}

// TestIsFloatLiteralEdges covers the float-literal recogniser branches.
func TestIsFloatLiteralEdges(t *testing.T) {
	// "+.5" and "-3.0" parse as floats; ".." and "1.2.3" do not.
	eq(t, "{{ x | plus: 0 }}", map[string]any{"x": 0.5}, "0.5")
	if isFloatLiteral("") || isFloatLiteral("1.2.3") || isFloatLiteral("abc") || isFloatLiteral(".") {
		t.Error("isFloatLiteral false positives")
	}
	if !isFloatLiteral("-3.0") || !isFloatLiteral("3.14") {
		t.Error("isFloatLiteral false negatives")
	}
}
