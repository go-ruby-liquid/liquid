// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import "testing"

// TestNestedBlockParseErrors drives the parseBlock error returns inside nested
// blocks: a malformed (unknown) tag inside an elsif/else/when/for-else body and
// a malformed expression inside a tablerow attribute, all under strict mode.
func TestNestedBlockParseErrors(t *testing.T) {
	bad := []string{
		"{% if a %}x{% elsif b %}{% bogus %}{% endif %}",       // elsif body
		"{% if a %}x{% else %}{% bogus %}{% endif %}",          // else body
		"{% case x %}{% when 1 %}{% bogus %}{% endcase %}",     // when body
		"{% case x %}{% else %}{% bogus %}{% endcase %}",       // case else body
		"{% for i in a %}x{% else %}{% bogus %}{% endfor %}",   // for else body
		"{% capture c %}{% bogus %}{% endcapture %}",           // capture body
		"{% tablerow i in (1..2) cols:a. %}x{% endtablerow %}", // tablerow attr
		"{% for i in (1..2) limit:a. %}x{% endfor %}",          // for attr
	}
	for _, src := range bad {
		if _, err := Parse(src, WithErrorMode(Strict)); err == nil {
			t.Errorf("Parse(%q) expected error", src)
		}
	}
}

// TestWhenEmptyField covers parseWhenValues' empty-field skip (a trailing comma).
func TestWhenEmptyField(t *testing.T) {
	eq(t, "{% case x %}{% when 1, , 2 %}m{% endcase %}", map[string]any{"x": 2}, "m")
}

// TestIsAttrWordFalse covers isAttrWord's false return for a colon word whose key
// is not a known attribute: such a word stays part of the collection.
func TestIsAttrWordFalse(t *testing.T) {
	// "foo:bar" is not a loop attribute; it is treated as the collection (which
	// resolves to nil here, so the loop is empty and the else branch renders).
	eq(t, "{% for i in foo:bar %}x{% else %}none{% endfor %}", nil, "none")
}

// TestSplitAttrsBareWordAfterAttr2 covers the bare-word-after-attr arm: a
// `reversed` keyword appearing after a key:value attribute on a for loop.
func TestSplitAttrsBareAfterAttr(t *testing.T) {
	// limit then reversed: reversed is a bare word recorded after attrs started.
	eq(t, "{% for i in (1..4) limit:3 reversed %}{{ i }}{% endfor %}", nil, "123")
}

// TestTablerowReversedAttrError covers parseAttr's no-colon error path: tablerow
// does not accept `reversed`, so the bare word reaches parseAttr and errors.
func TestTablerowReversedAttrError(t *testing.T) {
	if _, err := Parse("{% tablerow i in (1..2) reversed %}x{% endtablerow %}", WithErrorMode(Strict)); err == nil {
		t.Error("tablerow reversed should error")
	}
}

// TestAppendTextEmpty covers appendText's empty-string short-circuit: a template
// that begins with markup (no leading text) and whose trailing text is trimmed
// to empty produces empty text runs the tokenizer must drop.
func TestAppendTextEmpty(t *testing.T) {
	// Leading {{ }} means no preceding text token; trailing -}} trims to empty.
	eq(t, "{{ 'a' }}{{ 'b' -}}   ", nil, "ab")
}
