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
