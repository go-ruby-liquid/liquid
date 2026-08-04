// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import "strings"

type tokenKind int

const (
	tokText   tokenKind = iota // raw template text
	tokOutput                  // {{ ... }}
	tokTag                     // {% ... %}
	tokRaw                     // literal body between {% raw %} and {% endraw %}
)

// wsCutset is the set of characters Ruby's String#lstrip / #rstrip strip, which
// Liquid's whitespace control ({%-/-%} and {{-/-}}) uses to trim adjacent text:
// null, horizontal tab, line feed, vertical tab, form feed, carriage return and
// space. Matching it exactly is what makes trimming byte-identical to the gem.
const wsCutset = "\x00\t\n\v\f\r "

// token is one lexical chunk of the template. For tokOutput/tokTag, body is the
// trimmed inside of the markup; trimLeft/trimRight record the {%- / -%} markers.
type token struct {
	kind      tokenKind
	body      string // raw text (tokText/tokRaw) or inner markup (tokOutput/tokTag)
	name      string // tag name (tokTag only)
	args      string // remainder after the tag name (tokTag only)
	trimLeft  bool   // {{- or {%-  : strip trailing whitespace of preceding text
	trimRight bool   // -}} or -%}  : strip leading whitespace of following text
}

// tokenize splits src into text / output / tag tokens, applying whitespace
// control: a {{-/{%- strips the immediately-preceding text token's trailing
// whitespace, and a -}}/-%} strips the following text token's leading
// whitespace. The body of a {% raw %}…{% endraw %} block is captured verbatim as
// a single tokRaw so its contents (including any {{-/-%} markers) stay literal
// and untrimmed, exactly as Shopify Liquid treats them.
func tokenize(src string) []token {
	var toks []token
	i := 0
	n := len(src)
	for i < n {
		open := nextMarker(src, i) // index of next {{ or {%
		if open < 0 {
			toks = appendText(toks, src[i:])
			break
		}
		if open > i {
			toks = appendText(toks, src[i:open])
		}
		isOutput := src[open+1] == '{'
		var closeSeq string
		if isOutput {
			closeSeq = "}}"
		} else {
			closeSeq = "%}"
		}
		end := strings.Index(src[open+2:], closeSeq)
		if end < 0 {
			// Unterminated markup: treat the rest as text (the gem renders it raw).
			toks = appendText(toks, src[open:])
			break
		}
		end += open + 2
		inner := src[open+2 : end]
		trimL := strings.HasPrefix(inner, "-")
		if trimL {
			inner = inner[1:]
		}
		trimR := strings.HasSuffix(inner, "-")
		if trimR {
			inner = inner[:len(inner)-1]
		}
		inner = strings.TrimSpace(inner)
		t := token{trimLeft: trimL, trimRight: trimR}
		if isOutput {
			t.kind = tokOutput
			t.body = inner
			toks = append(toks, t)
			i = end + len(closeSeq)
			continue
		}
		t.kind = tokTag
		t.name, t.args = splitTag(inner)
		t.body = inner
		toks = append(toks, t)
		i = end + len(closeSeq)
		if t.name == "raw" {
			toks, i = scanRaw(src, i, toks)
		}
	}
	return applyTrim(toks)
}

// scanRaw captures the literal text following a {% raw %} tag (at index i) up to
// the matching {% endraw %}, emitting it as a single tokRaw plus the endraw tag
// token. Whitespace-control markers inside the body are left literal; the raw
// tag's own -%} and the endraw's leading {%- do not reach the body (tokRaw is
// invisible to applyTrim). If no endraw is found, the remainder is emitted as a
// tokRaw and the parser reports the unterminated block.
func scanRaw(src string, i int, toks []token) ([]token, int) {
	j := i
	for {
		open := strings.Index(src[j:], "{%")
		if open < 0 {
			toks = append(toks, token{kind: tokRaw, body: src[i:]})
			return toks, len(src)
		}
		open += j
		end := strings.Index(src[open+2:], "%}")
		if end < 0 {
			toks = append(toks, token{kind: tokRaw, body: src[i:]})
			return toks, len(src)
		}
		end += open + 2
		inner := src[open+2 : end]
		tl := strings.HasPrefix(inner, "-")
		if tl {
			inner = inner[1:]
		}
		tr := strings.HasSuffix(inner, "-")
		if tr {
			inner = inner[:len(inner)-1]
		}
		name, _ := splitTag(strings.TrimSpace(inner))
		if name == "endraw" {
			toks = append(toks, token{kind: tokRaw, body: src[i:open]})
			toks = append(toks, token{kind: tokTag, name: "endraw", trimLeft: tl, trimRight: tr})
			return toks, end + 2
		}
		j = end + 2
	}
}

// nextMarker returns the index at or after i of the next "{{" or "{%", or -1.
func nextMarker(src string, i int) int {
	for j := i; j+1 < len(src); j++ {
		if src[j] == '{' && (src[j+1] == '{' || src[j+1] == '%') {
			return j
		}
	}
	return -1
}

// appendText appends a text token (merging is unnecessary as the scanner emits
// contiguous runs already).
func appendText(toks []token, s string) []token {
	if s == "" {
		return toks
	}
	return append(toks, token{kind: tokText, body: s})
}

// splitTag separates a tag's name from its argument string.
func splitTag(inner string) (name, args string) {
	j := strings.IndexFunc(inner, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	if j < 0 {
		return inner, ""
	}
	return inner[:j], strings.TrimSpace(inner[j:])
}

// applyTrim rewrites adjacent text tokens for {{-/-}} whitespace control. Only
// tokText neighbours are trimmed, so a tokRaw body is never touched — which is
// how the raw tag's -%} and the endraw's {%- are kept from reaching it.
func applyTrim(toks []token) []token {
	for i := range toks {
		if toks[i].kind != tokOutput && toks[i].kind != tokTag {
			continue
		}
		if toks[i].trimLeft && i > 0 && toks[i-1].kind == tokText {
			toks[i-1].body = strings.TrimRight(toks[i-1].body, wsCutset)
		}
		if toks[i].trimRight && i+1 < len(toks) && toks[i+1].kind == tokText {
			toks[i+1].body = strings.TrimLeft(toks[i+1].body, wsCutset)
		}
	}
	return toks
}
