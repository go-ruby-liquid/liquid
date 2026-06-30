// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import "testing"

// TestParseBlockErrorPropagation covers the error-propagation arms of parseIf
// and parseCase: a malformed output expression inside the block body must
// surface as a syntax error from the enclosing tag's parseBlock call.
func TestParseBlockErrorPropagation(t *testing.T) {
	cases := []string{
		// if body with an unterminated bracket variable -> parseIf parseBlock err
		"{% if true %}{{ a[ }}{% endif %}",
		// unless shares the same path
		"{% unless false %}{{ a[ }}{% endunless %}",
		// case body before the first when -> parseCase parseBlock err
		"{% case x %}{{ a[ }}{% when 1 %}y{% endcase %}",
	}
	for _, src := range cases {
		if _, err := Parse(src, WithErrorMode(Strict)); err == nil {
			t.Errorf("Parse(%q) expected error, got nil", src)
		}
	}
}

// TestSplitAttrsBareWordAfterAttrs covers the splitAttrs arm where a non
// key:value, non-"reversed" bare word appears after attribute parsing has
// already begun: it is treated as a (later-rejected) attribute, not as part of
// the collection expression.
func TestSplitAttrsBareWordAfterAttrs(t *testing.T) {
	if _, err := Parse("{% for i in arr limit:2 bareword %}{{ i }}{% endfor %}",
		WithErrorMode(Strict)); err == nil {
		t.Error("Parse(for with bare word after attrs) expected error, got nil")
	}
}

// TestForOverHash covers toSlice's map[string]any arm deterministically (the
// gem oracle also checks this, but the oracle skips where the liquid gem is
// absent — the qemu, Windows, and gem-less unix lanes — so this keeps the arm
// covered there). Iterating a hash yields [key, value] pairs in sorted-key
// order, matching the gem.
func TestForOverHash(t *testing.T) {
	eq(t, "{% for kv in h %}{{ kv[0] }}={{ kv[1] }} {% endfor %}",
		map[string]any{"h": map[string]any{"b": 2, "a": 1}}, "a=1 b=2 ")
}

// TestWhereArms covers where's two arms deterministically — the matching
// two-argument form (field == value) and the single-argument truthiness form.
// The oracle exercises both too but skips where the liquid gem is absent (the
// qemu, Windows, and gem-less unix lanes), so these keep the arms covered there.
func TestWhereArms(t *testing.T) {
	// Two-argument, value matches: keeps the element.
	eq(t, "{{ a | where: 'on', true | map: 'n' | join: ',' }}",
		map[string]any{"a": []any{
			map[string]any{"n": "x", "on": true},
			map[string]any{"n": "y", "on": false},
		}}, "x")
	// Single-argument, filter by truthiness of the field.
	eq(t, "{{ a | where: 'ok' | map: 'n' | join: ',' }}",
		map[string]any{"a": []any{
			map[string]any{"n": "x", "ok": true},
			map[string]any{"n": "y", "ok": false},
			map[string]any{"n": "z", "ok": true},
		}}, "x,z")
}

// TestAppendTextEmptyDirect covers appendText's empty-string short-circuit
// directly: the tokenizer never feeds it an empty run, so the guard is exercised
// here at the package level to prove it returns the slice unchanged.
func TestAppendTextEmptyDirect(t *testing.T) {
	in := []token{{kind: tokText, body: "x"}}
	out := appendText(in, "")
	if len(out) != len(in) {
		t.Fatalf("appendText(_, \"\") changed length: got %d want %d", len(out), len(in))
	}
}
