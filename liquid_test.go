// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import (
	"testing"
	"time"
)

// render is a test helper: parse in Lax mode and render, failing on error.
func render(t *testing.T, src string, assigns map[string]any) string {
	t.Helper()
	out, err := MustParse(src).Render(assigns)
	if err != nil {
		t.Fatalf("Render(%q): %v", src, err)
	}
	return out
}

func eq(t *testing.T, src string, assigns map[string]any, want string) {
	t.Helper()
	if got := render(t, src, assigns); got != want {
		t.Errorf("render(%q) = %q, want %q", src, got, want)
	}
}

// TestRenderTable is the deterministic (ruby-free) corpus that, together with
// the error/edge tests, holds coverage at 100% without the gem.
func TestRenderTable(t *testing.T) {
	A := map[string]any{
		"name": "World",
		"arr":  []any{3, 1, 2},
		"h":    map[string]any{"a": 1, "b": 2},
		"deep": map[string]any{"x": map[string]any{"y": "z"}},
	}
	cases := []struct{ src, want string }{
		{"Hello {{ name }}!", "Hello World!"},
		{"{{ 'HI' | downcase }}", "hi"},
		{"{{ 'hi' | upcase }}", "HI"},
		{"{{ 'hELLO' | capitalize }}", "Hello"},
		{"{{ '' | capitalize }}", ""},
		{"{{ '  x  ' | strip }}", "x"},
		{"[{{ '  x  ' | lstrip }}]", "[x  ]"},
		{"[{{ '  x  ' | rstrip }}]", "[  x]"},
		{"{{ 'a\nb' | strip_newlines }}", "ab"},
		{"{{ 'a\nb' | newline_to_br }}", "a<br />\nb"},
		{"{{ '<a>&' | escape }}", "&lt;a&gt;&amp;"},
		{"{{ '&lt; <' | escape_once }}", "&lt; &lt;"},
		{"{{ '<b>hi</b>' | strip_html }}", "hi"},
		{"{{ 'a b' | url_encode }}", "a+b"},
		{"{{ 'hello world' | url_decode }}", "hello world"},
		{"{{ 'a' | append: 'b' | prepend: 'z' }}", "zab"},
		{"{{ 'hello' | replace: 'l', 'L' }}", "heLLo"},
		{"{{ 'aaa' | replace_first: 'a', 'b' }}", "baa"},
		{"{{ 'hello' | remove: 'l' }}", "heo"},
		{"{{ 'a a a' | remove_first: 'a ' }}", "a a"},
		{"{{ 'Hello World' | truncate: 5 }}", "He..."},
		{"{{ 'Hello World' | truncate: 8, '..' }}", "Hello .."},
		{"{{ 'one two three' | truncatewords: 2 }}", "one two..."},
		{"{{ 'Hello' | slice: 1, 3 }}", "ell"},
		{"{{ 'Hello' | slice: -2, 2 }}", "lo"},
		{"{{ 'a,b,c' | split: ',' | join: '-' }}", "a-b-c"},
		{"{{ '' | split: ',' | size }}", "1"},
		{"{{ arr | join: ',' }}", "3,1,2"},
		{"{{ arr | first }}/{{ arr | last }}", "3/2"},
		{"{{ arr | sort | join: ',' }}", "1,2,3"},
		{"{{ arr | reverse | join: ',' }}", "2,1,3"},
		{"{{ arr | size }}", "3"},
		{"{{ 'hi' | size }}", "2"},
		{"{{ h | size }}", "2"},
		{"{{ 5 | plus: 3 }}", "8"},
		{"{{ 10 | minus: 3 }}", "7"},
		{"{{ 3 | times: 4 }}", "12"},
		{"{{ 10 | divided_by: 3 }}", "3"},
		{"{{ 10 | divided_by: 4.0 }}", "2.5"},
		{"{{ 17 | modulo: 5 }}", "2"},
		{"{{ 3.14159 | round: 2 }}", "3.14"},
		{"{{ 2.567 | round }}", "3"},
		{"{{ 1.2 | ceil }}/{{ 1.8 | floor }}", "2/1"},
		{"{{ -3 | abs }}", "3"},
		{"{{ -3.5 | abs }}", "3.5"},
		{"{{ 5 | at_least: 8 }}/{{ 5 | at_most: 3 }}", "8/3"},
		{"{{ 0.1 | plus: 0.2 }}", "0.3"},
		{"{{ '3' | plus: '4' }}", "7"},
		{"{{ x | default: 'd' }}", "d"},
		{"{{ false | default: 'd', allow_false: true }}", "false"},
		{"{{ '2026-06-30' | date: '%Y/%m/%d' }}", "2026/06/30"},
		{"{% if true %}y{% else %}n{% endif %}", "y"},
		{"{% unless false %}u{% endunless %}", "u"},
		{"{% if 0 %}t{% endif %}{% if '' %}u{% endif %}{% if nil %}n{% endif %}", "tu"},
		{"{% for i in (1..3) %}{{ i }}{% endfor %}", "123"},
		{"{% for i in (1..5) limit:2 offset:1 %}{{ i }}{% endfor %}", "23"},
		{"{% for i in (1..3) reversed %}{{ i }}{% endfor %}", "321"},
		{"{% for i in arr %}{% if i == 1 %}{% break %}{% endif %}{{ i }}{% endfor %}", "3"},
		{"{% for i in arr %}{% if i == 1 %}{% continue %}{% endif %}{{ i }}{% endfor %}", "32"},
		{"{% for i in empty %}{{ i }}{% else %}none{% endfor %}", "none"},
		{"{% assign x = 5 %}{{ x | plus: 3 }}", "8"},
		{"{% capture g %}hi {{ name }}{% endcapture %}{{ g }}", "hi World"},
		{"{% increment c %}{% increment c %}", "01"},
		{"{% decrement c %}{% decrement c %}", "-1-2"},
		{"{% raw %}{{ x }}{% endraw %}", "{{ x }}"},
		{"{% comment %}x{% endcomment %}y", "y"},
		{"{% cycle 'a','b' %}{% cycle 'a','b' %}{% cycle 'a','b' %}", "aba"},
		{"a {%- if true -%} b {%- endif -%} c", "abc"},
		{"{{ deep.x.y }}", "z"},
		{"{{ deep['x']['y'] }}", "z"},
		{"{{ h }}", `{"a"=>1, "b"=>2}`},
		{"{{ arr }}", "312"},
	}
	for _, c := range cases {
		eq(t, c.src, A, c.want)
	}
}

func TestForloopMembers(t *testing.T) {
	out := render(t, "{% for i in a %}{{forloop.index0}}/{{forloop.rindex}}/{{forloop.rindex0}}/{{forloop.first}}/{{forloop.last}} {% endfor %}",
		map[string]any{"a": []any{"x", "y"}})
	want := "0/2/1/true/false 1/1/0/false/true "
	if out != want {
		t.Errorf("forloop members = %q want %q", out, want)
	}
}

func TestNestedParentloop(t *testing.T) {
	out := render(t, "{% for i in (1..2) %}{% for j in (1..2) %}{{forloop.parentloop.index}}{{forloop.index}} {% endfor %}{% endfor %}", nil)
	if out != "11 12 21 22 " {
		t.Errorf("nested = %q", out)
	}
	// parentloop at top level is nil.
	out = render(t, "{% for i in (1..1) %}{{ forloop.parentloop }}{% endfor %}", nil)
	if out != "" {
		t.Errorf("top parentloop = %q", out)
	}
}

func TestCaseTag(t *testing.T) {
	eq(t, "{% case x %}{% when 1 %}one{% when 2 %}two{% else %}o{% endcase %}", map[string]any{"x": 2}, "two")
	eq(t, "{% case x %}{% when 'a', 'b' %}ab{% endcase %}", map[string]any{"x": "b"}, "ab")
	eq(t, "{% case x %}{% when 'a' or 'b' %}ab{% endcase %}", map[string]any{"x": "a"}, "ab")
	eq(t, "{% case x %}{% when 9 %}nine{% endcase %}", map[string]any{"x": 1}, "")
}

func TestElsif(t *testing.T) {
	eq(t, "{% if x==1 %}1{% elsif x==2 %}2{% else %}o{% endif %}", map[string]any{"x": 2}, "2")
	eq(t, "{% if x==1 %}1{% elsif x==2 %}2{% else %}o{% endif %}", map[string]any{"x": 9}, "o")
	eq(t, "{% if x==1 %}1{% elsif x==2 %}2{% endif %}", map[string]any{"x": 9}, "")
}

func TestTablerow(t *testing.T) {
	out := render(t, "{% tablerow i in (1..4) cols:2 %}{{i}}{% endtablerow %}", nil)
	want := "<tr class=\"row1\">\n<td class=\"col1\">1</td><td class=\"col2\">2</td></tr>\n<tr class=\"row2\"><td class=\"col1\">3</td><td class=\"col2\">4</td></tr>\n"
	if out != want {
		t.Errorf("tablerow =\n%q\nwant\n%q", out, want)
	}
	// limit/offset
	render(t, "{% tablerow i in (1..6) cols:2 limit:3 offset:1 %}{{i}}{% endtablerow %}", nil)
	// cols defaults to all, zero cols clamps to 1.
	render(t, "{% tablerow i in (1..2) cols:0 %}{{i}}{% endtablerow %}", nil)
	// break inside tablerow.
	render(t, "{% tablerow i in (1..3) %}{% break %}{% endtablerow %}", nil)
}

func TestCycleGroups(t *testing.T) {
	eq(t, "{% cycle 'g': 'a','b' %}{% cycle 'g': 'a','b' %}{% cycle 'h': 'a','b' %}", nil, "aba")
	// empty cycle.
	render(t, "{% cycle %}", nil)
}

func TestContainsAndOperators(t *testing.T) {
	eq(t, "{% if 'hello' contains 'ell' %}y{% endif %}", nil, "y")
	eq(t, "{% if arr contains 2 %}y{% endif %}", map[string]any{"arr": []any{1, 2}}, "y")
	eq(t, "{% if h contains 'a' %}y{% endif %}", map[string]any{"h": map[string]any{"a": 1}}, "y")
	eq(t, "{% if x contains 'a' %}y{% else %}n{% endif %}", map[string]any{"x": 5}, "n")
	eq(t, "{% if 1 < 2 %}y{% endif %}", nil, "y")
	eq(t, "{% if 'a' < 'b' %}y{% endif %}", nil, "y")
	eq(t, "{% if 2 >= 2 %}y{% endif %}", nil, "y")
	eq(t, "{% if 2 <= 1 %}y{% else %}n{% endif %}", nil, "n")
	eq(t, "{% if 1 != 2 %}y{% endif %}", nil, "y")
	eq(t, "{% if a == empty %}y{% endif %}", map[string]any{"a": []any{}}, "y")
	eq(t, "{% if empty == a %}y{% endif %}", map[string]any{"a": ""}, "y")
	eq(t, "{% if blank == a %}y{% endif %}", map[string]any{"a": "  "}, "y")
	eq(t, "{% if a != blank %}y{% else %}n{% endif %}", map[string]any{"a": "x"}, "y")
	eq(t, "{% if a == b %}y{% else %}n{% endif %}", map[string]any{"a": []any{1}, "b": []any{1}}, "y")
	eq(t, "{% if a == b %}y{% else %}n{% endif %}", map[string]any{"a": []any{1}, "b": []any{2}}, "n")
	// Liquid and/or are right-associative: false and (true or true) => false.
	eq(t, "{% if false and true or true %}t{% else %}f{% endif %}", nil, "f")
	eq(t, "{% if true and false %}t{% else %}f{% endif %}", nil, "f")
}

func TestLookupVariants(t *testing.T) {
	eq(t, "{{ arr[0] }}/{{ arr[-1] }}", map[string]any{"arr": []any{10, 20}}, "10/20")
	eq(t, "{{ arr[9] }}", map[string]any{"arr": []any{1}}, "")
	eq(t, "{{ arr.first }}/{{ arr.last }}/{{ arr.size }}", map[string]any{"arr": []any{1, 2}}, "1/2/2")
	eq(t, "{{ empty.first }}{{ empty.last }}", map[string]any{"empty": []any{}}, "")
	eq(t, "{{ h.size }}", map[string]any{"h": map[string]any{"a": 1}}, "1")
	eq(t, "{{ h.missing }}", map[string]any{"h": map[string]any{}}, "")
	eq(t, "{{ s.size }}", map[string]any{"s": "abc"}, "3")
	eq(t, "{{ s.bogus }}", map[string]any{"s": "abc"}, "")
	eq(t, "{{ n.x }}", map[string]any{"n": 5}, "")
	// dynamic index a[b]
	eq(t, "{{ arr[i] }}", map[string]any{"arr": []any{1, 2, 3}, "i": 1}, "2")
}

func TestNumberFormatting(t *testing.T) {
	eq(t, "{{ x }}", map[string]any{"x": 1.5}, "1.5")
	eq(t, "{{ x }}", map[string]any{"x": int64(7)}, "7")
	eq(t, "{{ x }}", map[string]any{"x": true}, "true")
	eq(t, "{{ x }}", map[string]any{"x": false}, "false")
	if formatFloat(2.0) != "2.0" {
		t.Error("2.0 format")
	}
}

func TestDateFilter(t *testing.T) {
	// fixed clock so "now" is deterministic.
	old := nowFunc
	nowFunc = func() time.Time { return time.Date(2026, 6, 30, 14, 5, 9, 0, time.UTC) }
	defer func() { nowFunc = old }()
	eq(t, "{{ 'now' | date: '%Y-%m-%d %H:%M' }}", nil, "2026-06-30 14:05")
	eq(t, "{{ t | date: '%A %B %d, %Y' }}", map[string]any{"t": "2026-06-30"}, "Tuesday June 30, 2026")
	eq(t, "{{ t | date: '%a %b %e %I%p %j %w %% z' }}", map[string]any{"t": "2026-06-30 14:05:09"}, "Tue Jun 30 02PM 181 2 % z")
	eq(t, "{{ t | date: '%y %P' }}", map[string]any{"t": "2026-01-02 03:00:00"}, "26 am")
	// unix integer input
	eq(t, "{{ 0 | date: '%Y' }}", nil, "1970")
	// nil / non-date input passes through.
	eq(t, "{{ x | date: '%Y' }}", nil, "")
	eq(t, "{{ 'notadate' | date: '%Y' }}", nil, "notadate")
	eq(t, "{{ '2026-06-30' | date }}", nil, "2026-06-30")
	eq(t, "{{ 'now' | date: '%Z %z' }}", nil, "UTC +0000")
}

func TestMapWhereCompactUniqConcat(t *testing.T) {
	eq(t, "{{ a | map: 'k' | join: ',' }}", map[string]any{"a": []any{
		map[string]any{"k": 1}, map[string]any{"k": 2}}}, "1,2")
	eq(t, "{{ a | where: 'on' | size }}", map[string]any{"a": []any{
		map[string]any{"on": true}, map[string]any{"on": false}}}, "1")
	eq(t, "{{ a | where: 'x', 9 | size }}", map[string]any{"a": []any{map[string]any{"x": 1}}}, "0")
	eq(t, "{{ a | compact | size }}", map[string]any{"a": []any{1, nil, 2}}, "2")
	eq(t, "{{ a | compact | size }}", map[string]any{"a": []any{nil}}, "0")
	eq(t, "{{ a | uniq | join: ',' }}", map[string]any{"a": []any{1, 1, 2}}, "1,2")
	eq(t, "{{ a | uniq | size }}", map[string]any{"a": []any{}}, "0")
	eq(t, "{{ a | concat: b | join: ',' }}", map[string]any{"a": []any{1}, "b": []any{2}}, "1,2")
	eq(t, "{{ a | sort_natural | join: ',' }}", map[string]any{"a": []any{"B", "a"}}, "a,B")
	eq(t, "{{ a | sort: 'k' | map: 'k' | join: ',' }}", map[string]any{"a": []any{
		map[string]any{"k": 3}, map[string]any{"k": 1}}}, "1,3")
}

func TestSliceArray(t *testing.T) {
	eq(t, "{{ a | slice: 1, 2 | join: ',' }}", map[string]any{"a": []any{1, 2, 3, 4}}, "2,3")
	eq(t, "{{ a | slice: -2, 1 | join: ',' }}", map[string]any{"a": []any{1, 2, 3}}, "2")
	eq(t, "{{ a | slice: 9 | join: ',' }}", map[string]any{"a": []any{1}}, "")
	eq(t, "{{ 'h' | slice: 9 }}", nil, "")
	eq(t, "{{ a | first }}/{{ a | last }}", map[string]any{"a": []any{}}, "/")
	eq(t, "{{ 'abc' | first }}/{{ 'abc' | last }}", nil, "a/c")
	eq(t, "{{ '' | first }}/{{ '' | last }}", nil, "/")
}
