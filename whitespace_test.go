// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import "testing"

// TestWhitespaceControl is the deterministic (ruby-free) battery pinning
// {%-/-%} and {{-/-}} trimming to Shopify Liquid's exact byte behaviour. Each
// want was captured from the liquid gem (see oracle_test.go for the live
// differential run). It holds the trim paths at 100% coverage off-ruby.
func TestWhitespaceControl(t *testing.T) {
	x := map[string]any{"x": "X"}
	for _, c := range []struct {
		name, src, want string
		assigns         map[string]any
	}{
		// Trimming strips the whole whitespace run, including newlines, on the
		// marked side — matching Ruby String#rstrip/#lstrip.
		{"both", "a  \n  {{- x -}}  \n  b", "aXb", x},
		{"multiline", "a\n\n\n{{- x -}}\n\n\nb", "aXb", x},
		{"tabs", "a\t\t{{- x -}}\t\tb", "aXb", x},
		{"crlf", "a\r\n\r\n{{- x -}}\r\n\r\nb", "aXb", x},
		{"left_only", "  {{- x }}  ", "X  ", x},
		{"right_only", "  {{ x -}}  ", "  X", x},
		{"plain_output", "  {{ x }}  ", "  X  ", x},
		// The strip set matches Ruby's: NUL, tab, LF, VT, FF, CR, space.
		{"vt_ff", "p\n\v\f{{- x -}}\v\f q", "pXq", x},
		{"null", "p\x00{{- x -}}\x00q", "pXq", x},
		{"tag_vt_ff", "p\v\f{%- assign y=1 -%}\v\fq", "pq", nil},
		// Adjacent tags: the between-run is consumed once, then nothing remains
		// for the following tag to trim.
		{"adjacent", "a\n{%- assign x=1 -%}{%- assign y=2 -%}\nb", "ab", nil},
		// A skipped (false) branch still trims its neighbouring text at parse
		// time — the minima home.html shape.
		{"false_branch", "<div>\n  {%- if t -%}A{%- endif -%}\n\n  {{ c }}\n\n  {%- if p -%}B{%- endif -%}\n</div>",
			"<div>B</div>", map[string]any{"c": "", "p": true}},

		// raw: the body is verbatim — inner {{-/-%} markers stay literal and are
		// never trimmed.
		{"raw_inner_markers", "{% raw %}a{{- b -}}c{%- d -%}e{% endraw %}", "a{{- b -}}c{%- d -%}e", nil},
		{"raw_plain", "A\n{% raw %}\n keep \n{% endraw %}\nB", "A\n\n keep \n\nB", nil},
		{"raw_empty", "x{% raw %}{% endraw %}y", "xy", nil},
		{"raw_fake_output", "{% raw %}{{ y }}{% endraw %}", "{{ y }}", nil},
		// raw open's leading {%- trims the preceding text; its trailing -%} does
		// not reach the body.
		{"raw_trim_open", "A\n{%- raw %}\n keep \n{% endraw %}\nB", "A\n keep \n\nB", nil},
		{"raw_open_right_ignored", "A\n{% raw -%}\n keep \n{% endraw %}\nB", "A\n\n keep \n\nB", nil},
		// endraw's trailing -%} trims the following text; its leading {%- does
		// not reach the body.
		{"raw_trim_close", "A\n{% raw %}\n keep \n{% endraw -%}\nB", "A\n\n keep \nB", nil},
		{"raw_endraw_left_ignored", "A\n{% raw %}\n keep \n{%- endraw %}\nB", "A\n\n keep \n\nB", nil},
		{"raw_trim_all", "A\n{%- raw -%}\n keep \n{%- endraw -%}\nB", "A\n keep \nB", nil},
	} {
		if got := render(t, c.src, c.assigns); got != c.want {
			t.Errorf("%s: render(%q) = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}

// TestRawUnterminated covers the scanner's two unterminated-raw exits: a body
// with no further markup at all, and a body holding a stray {% that never
// closes with %}. Both surface as the "never closed" parse error.
func TestRawUnterminated(t *testing.T) {
	errParse(t, "{% raw %}unterminated")
	errParse(t, "{% raw %}foo {% bar")
}
