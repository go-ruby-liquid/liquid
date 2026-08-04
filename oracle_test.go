// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// rubyBin locates a usable `ruby` with the liquid gem once. The oracle tests
// skip themselves when it is absent (the qemu cross-arch lanes and the Windows
// lane), so the deterministic suite alone drives the 100% gate there.
func rubyBin(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping liquid-gem oracle")
	}
	// Require the liquid gem; skip if unavailable.
	if err := exec.Command(path, "-rliquid", "-e", "").Run(); err != nil {
		t.Skip("liquid gem not installed; skipping oracle")
	}
	return path
}

// gemRender renders src+assigns through the liquid gem and returns its output.
// The script $stdout.binmode's so Windows text-mode never pollutes the bytes
// (the go-ruby-erb lesson), though the oracle only runs on the unix lanes.
func gemRender(t *testing.T, bin, src string, assigns map[string]any) string {
	t.Helper()
	aj, err := json.Marshal(assigns)
	if err != nil {
		t.Fatalf("marshal assigns: %v", err)
	}
	script := `$stdout.binmode
require 'liquid'
require 'json'
src = STDIN.read
assigns = JSON.parse(ENV['ASSIGNS'])
print Liquid::Template.parse(src).render(assigns)
`
	cmd := exec.Command(bin, "-e", script)
	cmd.Env = append(cmd.Environ(), "ASSIGNS="+string(aj))
	cmd.Stdin = strings.NewReader(src)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gem render error: %v\nsrc:%s\nout:%s", err, src, out)
	}
	return string(out)
}

// oracleCases is the differential corpus: representative templates spanning the
// implemented tag/filter surface, modelled on the gem's integration suite.
var oracleCases = []struct {
	name    string
	src     string
	assigns map[string]any
}{
	{"plain", "Hello {{ name }}!", map[string]any{"name": "World"}},
	{"upcase", "{{ 'hi' | upcase }}", nil},
	{"chain", "{{ 'a,b,c' | split: ',' | last | upcase }}", nil},
	{"truncate", "{{ 'Hello World' | truncate: 5 }}", nil},
	{"truncatewords", "{{ 'one two three four' | truncatewords: 2 }}", nil},
	{"capitalize", "{{ 'hELLO world' | capitalize }}", nil},
	{"replace", "{{ 'hello' | replace: 'l', 'L' }}", nil},
	{"remove", "{{ 'hello' | remove: 'l' }}", nil},
	{"append", "{{ 'a' | append: 'b' | prepend: 'z' }}", nil},
	{"slice_s", "{{ 'Hello' | slice: 1, 3 }}", nil},
	{"slice_neg", "{{ 'Hello' | slice: -2, 2 }}", nil},
	{"escape", "{{ '1 < 2 & 3' | escape }}", nil},
	{"escape_once", "{{ '1 &lt; 2 &amp; 3 < 4' | escape_once }}", nil},
	{"strip_html", "{{ 'Have <em>you</em>' | strip_html }}", nil},
	{"newline_br", "{{ x | newline_to_br }}", map[string]any{"x": "a\nb"}},
	{"strip_newlines", "{{ x | strip_newlines }}", map[string]any{"x": "a\nb\nc"}},
	{"url_encode", "{{ '<p> a' | url_encode }}", nil},
	{"strips", "[{{ '  x  ' | lstrip }}][{{ '  x  ' | rstrip }}][{{ '  x  ' | strip }}]", nil},

	{"join", "{{ arr | join: '-' }}", map[string]any{"arr": []any{1, 2, 3}}},
	{"first_last", "{{ arr | first }}/{{ arr | last }}", map[string]any{"arr": []any{3, 1, 2}}},
	{"sort", "{{ arr | sort | join: ',' }}", map[string]any{"arr": []any{3, 1, 2}}},
	{"sort_nat", "{{ arr | sort_natural | join: ',' }}", map[string]any{"arr": []any{"B", "a", "C"}}},
	{"reverse", "{{ arr | reverse | join: ',' }}", map[string]any{"arr": []any{1, 2, 3}}},
	{"uniq", "{{ arr | uniq | join: ',' }}", map[string]any{"arr": []any{1, 1, 2, 3, 3}}},
	{"map", "{{ arr | map: 'name' | join: ',' }}", map[string]any{"arr": []any{map[string]any{"name": "a"}, map[string]any{"name": "b"}}}},
	{"where", "{{ arr | where: 'on', true | size }}", map[string]any{"arr": []any{map[string]any{"on": true}, map[string]any{"on": false}}}},
	{"compact", "{{ arr | compact | join: ',' }}", map[string]any{"arr": []any{1, nil, 2}}},
	{"concat", "{{ a | concat: b | join: ',' }}", map[string]any{"a": []any{1, 2}, "b": []any{3, 4}}},
	{"size_arr", "{{ arr | size }}", map[string]any{"arr": []any{1, 2, 3}}},
	{"size_str", "{{ 'hello' | size }}", nil},

	{"plus", "{{ 5 | plus: 3 }}", nil},
	{"minus", "{{ 10 | minus: 3 }}", nil},
	{"times", "{{ 3 | times: 4 }}", nil},
	{"divint", "{{ 10 | divided_by: 3 }}", nil},
	{"divfloat", "{{ 10 | divided_by: 3.0 }}", nil},
	{"modulo", "{{ 17 | modulo: 5 }}", nil},
	{"round", "{{ 3.14159 | round: 2 }}", nil},
	{"round0", "{{ 2.567 | round }}", nil},
	{"ceilfloor", "{{ 1.2 | ceil }}/{{ 1.8 | floor }}", nil},
	{"abs", "{{ -3 | abs }}", nil},
	{"atleast", "{{ 5 | at_least: 8 }}/{{ 5 | at_most: 3 }}", nil},
	{"floatplus", "{{ 0.1 | plus: 0.2 }}", nil},
	{"strplus", "{{ '3' | plus: '4' }}/{{ '3.5' | plus: 1 }}", nil},

	{"default", "{{ x | default: 'none' }}/{{ '' | default: 'none' }}/{{ 0 | default: 'none' }}", nil},
	{"default_false", "{{ false | default: 'x' }}/{{ false | default: 'x', allow_false: true }}", nil},
	{"date", "{{ '2026-06-30' | date: '%Y/%m/%d' }}", nil},

	{"if", "{% if true %}yes{% else %}no{% endif %}", nil},
	{"elsif", "{% if x == 1 %}1{% elsif x == 2 %}2{% else %}o{% endif %}", map[string]any{"x": float64(2)}},
	{"unless", "{% unless false %}u{% endunless %}", nil},
	{"case", "{% case x %}{% when 1 %}one{% when 2 %}two{% else %}other{% endcase %}", map[string]any{"x": float64(2)}},
	{"case_or", "{% case x %}{% when 'a', 'b' %}ab{% endcase %}", map[string]any{"x": "b"}},
	{"and_or", "{% if false and true or true %}t{% else %}f{% endif %}", nil},
	{"contains_s", "{% if x contains 'ell' %}c{% endif %}", map[string]any{"x": "hello"}},
	{"contains_a", "{% if arr contains 2 %}c{% endif %}", map[string]any{"arr": []any{1, 2, 3}}},
	{"gt", "{% if x > 5 %}big{% endif %}", map[string]any{"x": float64(10)}},
	{"blank_cmp", "{% if x == blank %}blank{% endif %}", map[string]any{"x": ""}},
	{"empty_cmp", "{% if x == empty %}empty{% endif %}", map[string]any{"x": []any{}}},
	{"truthy", "{% if 0 %}t{% endif %}{% if '' %}u{% endif %}{% if nil %}n{% endif %}", nil},

	{"for_range", "{% for i in (1..3) %}{{ i }}{% endfor %}", nil},
	{"for_limit", "{% for i in (1..5) limit:2 offset:1 %}{{ i }}{% endfor %}", nil},
	{"for_rev", "{% for i in (1..3) reversed %}{{ i }}{% endfor %}", nil},
	{"for_rev_limit", "{% for i in (1..5) limit:2 offset:1 reversed %}{{ i }}{% endfor %}", nil},
	{"forloop", "{% for i in arr %}{{ forloop.index }}:{{ forloop.first }}:{{ forloop.last }}:{{ forloop.length }} {% endfor %}", map[string]any{"arr": []any{"a", "b"}}},
	{"nested_loop", "{% for i in (1..2) %}{% for j in (1..2) %}{{ forloop.parentloop.index }}{{ forloop.index }} {% endfor %}{% endfor %}", nil},
	{"break", "{% for i in arr %}{% if i == 2 %}{% break %}{% endif %}{{ i }}{% endfor %}", map[string]any{"arr": []any{1, 2, 3}}},
	{"continue", "{% for i in arr %}{% if i == 2 %}{% continue %}{% endif %}{{ i }}{% endfor %}", map[string]any{"arr": []any{1, 2, 3}}},
	{"for_else", "{% for i in arr %}{{ i }}{% else %}none{% endfor %}", map[string]any{"arr": []any{}}},
	{"for_hash", "{% for kv in h %}{{ kv[0] }}={{ kv[1] }} {% endfor %}", map[string]any{"h": map[string]any{"a": 1, "b": 2}}},
	{"for_var_range", "{% assign n = 3 %}{% for i in (1..n) %}{{ i }}{% endfor %}", nil},

	{"assign", "{% assign x = 5 %}{{ x | plus: 3 }}", nil},
	{"assign_filter", "{% assign x = 'a,b' | split: ',' %}{{ x | last }}", nil},
	{"capture", "{% capture g %}Hello {{ name }}{% endcapture %}{{ g }}", map[string]any{"name": "X"}},
	{"increment", "{% increment c %}{% increment c %}{% increment c %}", nil},
	{"decrement", "{% decrement c %}{% decrement c %}", nil},
	{"raw", "{% raw %}{{ not_rendered }}{% endraw %}", nil},
	{"comment", "{% comment %}ignored{% endcomment %}after", nil},
	{"cycle", "{% cycle 'a','b','c' %}{% cycle 'a','b','c' %}{% cycle 'a','b','c' %}{% cycle 'a','b','c' %}", nil},
	{"cycle_group", "{% cycle 'g': 'a','b' %}{% cycle 'g': 'a','b' %}{% cycle 'h': 'a','b' %}", nil},
	{"tablerow", "{% tablerow i in (1..2) %}{{ i }}{% endtablerow %}", nil},

	{"ws_control", "a {%- if true -%} b {%- endif -%} c", nil},
	{"ws_output", "a  \n  {{- x -}}  \n  b", map[string]any{"x": "X"}},
	{"ws_multiline", "a\n\n\n{{- x -}}\n\n\nb", map[string]any{"x": "X"}},
	{"ws_tabs", "a\t\t{{- x -}}\t\tb", map[string]any{"x": "X"}},
	{"ws_crlf", "a\r\n\r\n{{- x -}}\r\n\r\nb", map[string]any{"x": "X"}},
	{"ws_vtff", "p\n\v\f{{- x -}}\v\f q", map[string]any{"x": "X"}},
	{"ws_null", "p\x00{{- x -}}\x00q", map[string]any{"x": "X"}},
	{"ws_left_only", "  {{- x }}  ", map[string]any{"x": "X"}},
	{"ws_right_only", "  {{ x -}}  ", map[string]any{"x": "X"}},
	{"ws_false_if", "<div>\n  {%- if t -%}A{%- endif -%}\n\n  {{ c }}\n\n  {%- if p -%}B{%- endif -%}\n</div>", map[string]any{"c": "", "p": true}},
	{"ws_for_loop", "<ul>\n  {%- for i in (1..3) -%}\n    <li>{{ i }}</li>\n  {%- endfor -%}\n</ul>", nil},
	{"ws_adjacent", "a\n{%- assign x=1 -%}{%- assign y=2 -%}\nb", nil},
	{"raw_plain", "A\n{% raw %}\n keep \n{% endraw %}\nB", nil},
	{"raw_inner_markers", "{% raw %}a{{- b -}}c{%- d -%}e{% endraw %}", nil},
	{"raw_trim_open", "A\n{%- raw %}\n keep \n{% endraw %}\nB", nil},
	{"raw_trim_close", "A\n{% raw %}\n keep \n{% endraw -%}\nB", nil},
	{"raw_trim_all", "A\n{%- raw -%}\n keep \n{%- endraw -%}\nB", nil},
	{"raw_trim_lead_endraw", "A\n{% raw %}\n keep \n{%- endraw %}\nB", nil},
	{"raw_empty", "x{% raw %}{% endraw %}y", nil},
	{"raw_fake_tags", "{% raw %}{% if x %}{{ y }}{% endraw %}", nil},
	{"dotted", "{{ a.b.c }}", map[string]any{"a": map[string]any{"b": map[string]any{"c": "deep"}}}},
	{"indexed", "{{ arr[1] }}", map[string]any{"arr": []any{10, 20, 30}}},
	{"bracket_str", "{{ a['b'] }}", map[string]any{"a": map[string]any{"b": "x"}}},
	{"hash_render", "{{ h }}", map[string]any{"h": map[string]any{"a": 1}}},
	{"arr_render", "{{ a }}", map[string]any{"a": []any{1, 2, 3}}},
	{"float_render", "{{ x }}", map[string]any{"x": float64(1.5)}},
	{"unknown_filter", "{{ a | nonexistent }}", map[string]any{"a": "x"}},
	{"div_zero", "{{ 5 | divided_by: 0 }}", nil},
	{"missing_var", "[{{ missing }}]", nil},
}

// TestOracleDifferential renders each corpus case through both this engine and
// the liquid gem and asserts byte-identical output.
func TestOracleDifferential(t *testing.T) {
	bin := rubyBin(t)
	for _, c := range oracleCases {
		t.Run(c.name, func(t *testing.T) {
			want := gemRender(t, bin, c.src, c.assigns)
			got, err := MustParse(c.src).Render(c.assigns)
			if err != nil {
				t.Fatalf("render error: %v", err)
			}
			if got != want {
				t.Errorf("mismatch for %q\nassigns=%v\n got=%q\nwant=%q", c.src, c.assigns, got, want)
			}
		})
	}
}
