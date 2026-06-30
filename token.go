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
)

// token is one lexical chunk of the template. For tokOutput/tokTag, body is the
// trimmed inside of the markup; trimLeft/trimRight record the {%- / -%} markers.
type token struct {
	kind      tokenKind
	body      string // raw text (tokText) or inner markup (tokOutput/tokTag)
	name      string // tag name (tokTag only)
	args      string // remainder after the tag name (tokTag only)
	trimLeft  bool   // {{- or {%-  : strip trailing whitespace of preceding text
	trimRight bool   // -}} or -%}  : strip leading whitespace of following text
}

// tokenize splits src into text / output / tag tokens, applying whitespace
// control: a {{-/{%- strips the immediately-preceding text token's trailing
// whitespace, and a -}}/-%% strips the following text token's leading
// whitespace. This matches Liquid's behaviour exactly.
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
		} else {
			t.kind = tokTag
			t.name, t.args = splitTag(inner)
			t.body = inner
		}
		toks = append(toks, t)
		i = end + len(closeSeq)
	}
	return applyTrim(toks)
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

// applyTrim rewrites adjacent text tokens for {{-/-}} whitespace control.
func applyTrim(toks []token) []token {
	for i := range toks {
		if toks[i].kind == tokText {
			continue
		}
		if toks[i].trimLeft && i > 0 && toks[i-1].kind == tokText {
			toks[i-1].body = strings.TrimRight(toks[i-1].body, " \t\r\n")
		}
		if toks[i].trimRight && i+1 < len(toks) && toks[i+1].kind == tokText {
			toks[i+1].body = strings.TrimLeft(toks[i+1].body, " \t\r\n")
		}
	}
	return toks
}
