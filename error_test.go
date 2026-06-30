// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import (
	"math"
	"strings"
	"testing"
)

func TestErrorModes(t *testing.T) {
	// Lax: a runtime error renders inline and is collected as a warning.
	tmpl := MustParse("{{ 5 | divided_by: 0 }}")
	out, err := tmpl.Render(nil)
	if err != nil {
		t.Fatalf("lax render err: %v", err)
	}
	if out != "Liquid error: divided by 0" {
		t.Errorf("lax inline = %q", out)
	}
	// Render also collects the error.
	if len(tmpl.Errors()) != 1 {
		t.Errorf("expected 1 collected error, got %d", len(tmpl.Errors()))
	}

	// Strict: the same template returns the error and the Error string formats.
	_, err = MustParse("{{ 5 | divided_by: 0 }}").RenderStrict(nil)
	if err == nil {
		t.Fatal("strict render should error")
	}
	le, ok := err.(*Error)
	if !ok || le.Type != "ZeroDivisionError" {
		t.Fatalf("strict err = %#v", err)
	}
	if !strings.Contains(le.Error(), "divided by 0") {
		t.Errorf("Error() = %q", le.Error())
	}

	// Warn mode behaves like Lax but also records the errors.
	wt := MustParse("{{ 1 | divided_by: 0 }}", WithErrorMode(Warn))
	if _, err := wt.Render(nil); err != nil {
		t.Fatalf("warn render err: %v", err)
	}
}

func TestSyntaxErrorClassFormat(t *testing.T) {
	e := syntaxErr("oops")
	if e.Type != "SyntaxError" || !strings.Contains(e.Error(), "syntax error: oops") {
		t.Errorf("syntaxErr Error() = %q", e.Error())
	}
	a := argErr("bad")
	if a.Type != "ArgumentError" || !strings.Contains(a.Error(), "error: bad") {
		t.Errorf("argErr Error() = %q", a.Error())
	}
}

func TestStrictParseErrors(t *testing.T) {
	// An unknown tag, malformed for, malformed assign, and an unterminated block
	// all surface as parse errors. (Lax mode swallows most of these.)
	bad := []string{
		"{% bogus %}",
		"{% for %}{% endfor %}",
		"{% for x y z %}{% endfor %}",
		"{% assign %}",
		"{% assign = 5 %}",
		"{% capture %}{% endcapture %}",
		"{% comment %}",
		"{% raw %}",
		"{{ a. }}",
		"{{ a[ }}",
		"{% endif %}",
	}
	for _, src := range bad {
		if _, err := Parse(src, WithErrorMode(Strict)); err == nil {
			t.Errorf("Parse(%q) expected error", src)
		}
	}
}

func TestMustParsePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustParse should panic on a syntax error")
		}
	}()
	MustParse("{% bogus %}", WithErrorMode(Strict))
}

func TestUnknownFilterPassthrough(t *testing.T) {
	// An undefined filter is a silent no-op even under render! (gem behaviour).
	out, err := MustParse("{{ a | nope }}").RenderStrict(map[string]any{"a": "x"})
	if err != nil || out != "x" {
		t.Errorf("unknown filter = %q, %v", out, err)
	}
}

func TestStrictRuntimeStopsAtFirstError(t *testing.T) {
	// In strict mode an assign whose source errors halts rendering.
	_, err := MustParse("{% assign y = 1 | divided_by: 0 %}{{ y }}").RenderStrict(nil)
	if err == nil {
		t.Error("strict assign error should propagate")
	}
	// Output strict propagation (filters are valid in {{ }} but not in tag
	// conditions, matching the gem).
	if _, err := MustParse("{{ 1 | divided_by: 0 }}").RenderStrict(nil); err == nil {
		t.Error("strict output should error")
	}
	// A capture whose body errors propagates in strict mode.
	if _, err := MustParse("{% capture c %}{{ 1 | divided_by: 0 }}{% endcapture %}{{ c }}").RenderStrict(nil); err == nil {
		t.Error("strict capture should error")
	}
}

func TestFloatFormatting(t *testing.T) {
	cases := map[float64]string{
		2.0:          "2.0",
		1.5:          "1.5",
		math.Inf(1):  "Infinity",
		math.Inf(-1): "-Infinity",
		math.NaN():   "NaN",
		1000000.0:    "1000000.0",
		0.0001:       "0.0001",
		-3.5:         "-3.5",
		100.0:        "100.0",
		1e20:         "1.0e+20",
		0.0000001:    "1.0e-07",
		0.0:          "0.0",
	}
	for f, want := range cases {
		if got := formatFloat(f); got != want {
			t.Errorf("formatFloat(%v) = %q, want %q", f, got, want)
		}
	}
}

func TestToIntCoercions(t *testing.T) {
	cases := []struct {
		in   any
		want int
	}{
		{int64(5), 5},
		{3.9, 3},
		{true, 0},
		{"42abc", 42},
		{"-7", -7},
		{"+7", 7},
		{"abc", 0},
		{"", 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := toInt(c.in); got != c.want {
			t.Errorf("toInt(%#v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestToFloatCoercions(t *testing.T) {
	if toFloat(int64(3)) != 3 || toFloat("2.5") != 2.5 || toFloat("nan!") != 0 || toFloat(nil) != 0 {
		t.Error("toFloat coercion")
	}
}

func TestRubyInspectShapes(t *testing.T) {
	m := map[string]any{"a": []any{1, "x"}, "b": map[string]any{"c": 2}}
	got := toStr(m)
	want := `{"a"=>[1, "x"], "b"=>{"c"=>2}}`
	if got != want {
		t.Errorf("inspectMap = %q want %q", got, want)
	}
}

func TestStripHTMLScriptStyleComments(t *testing.T) {
	in := "a<script>var x=1;</script>b<style>p{}</style>c<!--note-->d<p>e</p>"
	if got := stripHTML(in); got != "abcde" {
		t.Errorf("stripHTML = %q", got)
	}
	// Unterminated script/style/comment truncate.
	if got := stripHTML("x<script>tail"); got != "x" {
		t.Errorf("stripHTML unterminated script = %q", got)
	}
	if got := stripHTML("x<style>tail"); got != "x" {
		t.Errorf("stripHTML unterminated style = %q", got)
	}
	if got := stripHTML("x<!--tail"); got != "x" {
		t.Errorf("stripHTML unterminated comment = %q", got)
	}
}

func TestRawReconstructsMarkup(t *testing.T) {
	// A raw block containing both output and tag markup is emitted verbatim.
	out := render(t, "{% raw %}a {{ x }} b {% assign y = 1 %} c{% endraw %}", nil)
	if out != "a {{ x }} b {% assign y = 1 %} c" {
		t.Errorf("raw reconstruct = %q", out)
	}
}

func TestUrlDecodeError(t *testing.T) {
	// A malformed percent-escape is an inline error in lax mode.
	out := render(t, "{{ '%zz' | url_decode }}", nil)
	if !strings.HasPrefix(out, "Liquid error:") {
		t.Errorf("url_decode bad = %q", out)
	}
}

func TestModuloAndDivideEdges(t *testing.T) {
	eq(t, "{{ -7 | modulo: 3 }}", nil, "2")
	eq(t, "{{ 7.5 | modulo: 2 }}", nil, "1.5")
	eq(t, "{{ -7 | divided_by: 2 }}", nil, "-4")
	eq(t, "{{ 7 | modulo: 0 }}", nil, "Liquid error: divided by 0")
	eq(t, "{{ 7.0 | modulo: 0 }}", nil, "Liquid error: divided by 0")
	eq(t, "{{ 7.0 | divided_by: 0 }}", nil, "Liquid error: divided by 0")
}

func TestClampFloatPaths(t *testing.T) {
	eq(t, "{{ 5.0 | at_least: 8 }}", nil, "8.0")
	eq(t, "{{ 5.0 | at_most: 3 }}", nil, "3.0")
	eq(t, "{{ 9.0 | at_least: 3 }}", nil, "9.0")
	eq(t, "{{ 1.0 | at_most: 9 }}", nil, "1.0")
}

func TestConcatError(t *testing.T) {
	out := render(t, "{{ a | concat: b }}", map[string]any{"a": []any{1}, "b": "notarray"})
	if !strings.HasPrefix(out, "Liquid error:") {
		t.Errorf("concat non-array = %q", out)
	}
}

func TestRatOfStringFloat(t *testing.T) {
	eq(t, "{{ '1.5' | plus: '2.5' }}", nil, "4.0")
	eq(t, "{{ 'x' | plus: 1.0 }}", nil, "1.0")
}

func TestSizeOfNilAndUnknown(t *testing.T) {
	eq(t, "{{ x | size }}", nil, "0")
	// size of a number is 0 (Ruby has no Integer#size in Liquid's filter).
	eq(t, "{{ 5 | size }}", nil, "0")
}

func TestHashFirstLast(t *testing.T) {
	eq(t, "{{ h.first | join: '=' }}", map[string]any{"h": map[string]any{"a": 1, "b": 2}}, "a=1")
	eq(t, "{{ h.last | join: '=' }}", map[string]any{"h": map[string]any{"a": 1, "b": 2}}, "b=2")
	eq(t, "{{ h.first }}", map[string]any{"h": map[string]any{}}, "")
	eq(t, "{{ h.last }}", map[string]any{"h": map[string]any{}}, "")
}

func TestOrderCompareErrors(t *testing.T) {
	// Comparing a number with a string raises ArgumentError (inline in lax).
	out := render(t, "{% if x > 'a' %}y{% else %}n{% endif %}", map[string]any{"x": 5})
	if !strings.HasPrefix(out, "Liquid error:") {
		t.Errorf("bad compare = %q", out)
	}
	// className covers each shape via the error message path.
	for _, v := range []any{nil, true, []any{}, map[string]any{}, 1.0} {
		_ = className(v)
	}
	if className(struct{}{}) != "Object" {
		t.Error("className object")
	}
}

func TestTopLevelInterruptIgnored(t *testing.T) {
	// A break with no enclosing loop is a no-op at the top level.
	out, err := MustParse("a{% break %}b").Render(nil)
	if err != nil {
		t.Fatalf("top break err: %v", err)
	}
	if out != "a" {
		t.Errorf("top break out = %q", out)
	}
}

func TestWhitespaceTrimRightLeft(t *testing.T) {
	eq(t, "{{- x -}}", map[string]any{"x": "v"}, "v")
	eq(t, "a  {{- 'b' }}", nil, "ab")
	eq(t, "{{ 'a' -}}  b", nil, "ab")
}

func TestUnterminatedMarkupIsText(t *testing.T) {
	// An unterminated {{ or {% is rendered as literal text.
	eq(t, "before {{ x", nil, "before {{ x")
	eq(t, "before {% x", nil, "before {% x")
}

func TestRangeReversedEmpty(t *testing.T) {
	// A descending range yields nothing.
	eq(t, "{% for i in (5..1) %}{{ i }}{% endfor %}", nil, "")
}

func TestForOffsetLimitClamp(t *testing.T) {
	eq(t, "{% for i in (1..3) offset:9 %}{{ i }}{% else %}none{% endfor %}", nil, "none")
	eq(t, "{% for i in (1..3) limit:0 %}{{ i }}{% else %}none{% endfor %}", nil, "none")
	eq(t, "{% for i in (1..3) offset:-1 %}{{ i }}{% endfor %}", nil, "123")
}

func TestDecrementNegativeStart(t *testing.T) {
	// decrement and increment share one per-name counter; decrement stores -1 and
	// the following increment reads it (the gem renders "-1-1").
	eq(t, "{% decrement c %}{% increment c %}", nil, "-1-1")
}
