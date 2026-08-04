// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import "strings"

// parser walks the token stream and builds the node tree.
type parser struct {
	toks []token
	pos  int
	mode ErrorMode
}

// blockEnders are the tag names that terminate a block; parseBlock stops
// (without consuming) when it sees one in its stop set.
func (p *parser) parseBlock(stop []string) (*blockNode, error) {
	blk := &blockNode{}
	for p.pos < len(p.toks) {
		t := p.toks[p.pos]
		switch t.kind {
		case tokText:
			blk.children = append(blk.children, &textNode{t.body})
			p.pos++
		case tokOutput:
			n, err := p.parseOutput(t.body)
			if err != nil {
				return nil, err
			}
			blk.children = append(blk.children, n)
			p.pos++
		case tokTag:
			if contains_(stop, t.name) {
				return blk, nil // leave the ender for the caller
			}
			n, err := p.parseTag(t)
			if err != nil {
				return nil, err
			}
			if n != nil {
				blk.children = append(blk.children, n)
			}
		}
	}
	if len(stop) > 0 {
		// Reached EOF while a block was still open: its end tag is missing.
		return nil, syntaxErr("tag was never closed, expected one of: " + strings.Join(stop, ", "))
	}
	return blk, nil
}

// parseOutput parses the body of a {{ }} into an expression + filter chain.
func (p *parser) parseOutput(body string) (node, error) {
	e, filters, err := parseFiltered(body)
	if err != nil {
		return nil, err
	}
	return &outputNode{expr: e, filters: filters}, nil
}

// parseTag dispatches on the tag name. It consumes the opening tag token and,
// for block tags, the body up to and including the matching end tag.
func (p *parser) parseTag(t token) (node, error) {
	p.pos++ // consume the opening tag
	switch t.name {
	case "assign":
		return p.parseAssign(t.args)
	case "capture":
		return p.parseCapture(t.args)
	case "increment":
		return &incrementNode{name: strings.TrimSpace(t.args), step: +1}, nil
	case "decrement":
		return &incrementNode{name: strings.TrimSpace(t.args), step: -1}, nil
	case "if":
		return p.parseIf(t.args, false)
	case "unless":
		return p.parseIf(t.args, true)
	case "case":
		return p.parseCase(t.args)
	case "for":
		return p.parseFor(t.args)
	case "tablerow":
		return p.parseTablerow(t.args)
	case "break":
		return breakNode{}, nil
	case "continue":
		return continueNode{}, nil
	case "cycle":
		return p.parseCycle(t.args)
	case "comment":
		return p.parseComment()
	case "raw":
		return p.parseRaw()
	default:
		return nil, syntaxErr("Unknown tag '" + t.name + "'")
	}
}

func (p *parser) parseAssign(args string) (node, error) {
	i := strings.Index(args, "=")
	if i < 0 {
		return nil, syntaxErr("Syntax Error in 'assign' - Valid syntax: assign [var] = [source]")
	}
	name := strings.TrimSpace(args[:i])
	if name == "" {
		return nil, syntaxErr("Syntax Error in 'assign' - Valid syntax: assign [var] = [source]")
	}
	e, filters, err := parseFiltered(strings.TrimSpace(args[i+1:]))
	if err != nil {
		return nil, err
	}
	return &assignNode{name: name, expr: e, filters: filters}, nil
}

func (p *parser) parseCapture(args string) (node, error) {
	name := strings.TrimSpace(args)
	if name == "" {
		return nil, syntaxErr("Syntax Error in 'capture' - Valid syntax: capture [var]")
	}
	body, err := p.parseBlock([]string{"endcapture"})
	if err != nil {
		return nil, err
	}
	p.consumeEnd()
	return &captureNode{name: name, body: body}, nil
}

func (p *parser) parseIf(args string, unless bool) (node, error) {
	n := &ifNode{}
	cond, err := parseCondition(args)
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock([]string{"elsif", "else", "endif", "endunless"})
	if err != nil {
		return nil, err
	}
	n.branches = append(n.branches, ifBranch{cond: cond, body: body, negate: unless})
	endTag := "endif"
	if unless {
		endTag = "endunless"
	}
	for {
		cur := p.toks[p.pos]
		switch cur.name {
		case "elsif":
			p.pos++
			c, err := parseCondition(cur.args)
			if err != nil {
				return nil, err
			}
			b, err := p.parseBlock([]string{"elsif", "else", endTag})
			if err != nil {
				return nil, err
			}
			n.branches = append(n.branches, ifBranch{cond: c, body: b})
		case "else":
			p.pos++
			b, err := p.parseBlock([]string{endTag})
			if err != nil {
				return nil, err
			}
			n.els = b
		case endTag, "endif", "endunless":
			p.pos++
			return n, nil
		}
	}
}

func (p *parser) parseCase(args string) (node, error) {
	subj, err := parseExpr(strings.TrimSpace(args))
	if err != nil {
		return nil, err
	}
	n := &caseNode{subject: subj}
	// Skip any text/markup before the first when (the gem discards it).
	body, err := p.parseBlock([]string{"when", "else", "endcase"})
	if err != nil {
		return nil, err
	}
	_ = body
	for {
		cur := p.toks[p.pos]
		switch cur.name {
		case "when":
			p.pos++
			vals, err := parseWhenValues(cur.args)
			if err != nil {
				return nil, err
			}
			b, err := p.parseBlock([]string{"when", "else", "endcase"})
			if err != nil {
				return nil, err
			}
			n.whens = append(n.whens, whenBranch{values: vals, body: b})
		case "else":
			p.pos++
			b, err := p.parseBlock([]string{"endcase"})
			if err != nil {
				return nil, err
			}
			n.els = b
		case "endcase":
			p.pos++
			return n, nil
		}
	}
}

// parseWhenValues splits a when's argument into one or more values separated by
// commas or the `or` keyword.
func parseWhenValues(args string) ([]expr, error) {
	fields := splitTopLevel(args, []string{",", " or "})
	var out []expr
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		e, err := parseExpr(f)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func (p *parser) parseFor(args string) (node, error) {
	varName, rest, err := splitForArgs(args)
	if err != nil {
		return nil, err
	}
	colStr, attrs := splitAttrs(rest)
	col, err := parseExpr(colStr)
	if err != nil {
		return nil, err
	}
	n := &forNode{varName: varName, collection: col}
	if err := applyLoopAttrs(n, attrs); err != nil {
		return nil, err
	}
	body, err := p.parseBlock([]string{"else", "endfor"})
	if err != nil {
		return nil, err
	}
	n.body = body
	if p.toks[p.pos].name == "else" {
		p.pos++
		els, err := p.parseBlock([]string{"endfor"})
		if err != nil {
			return nil, err
		}
		n.els = els
	}
	p.consumeEnd()
	return n, nil
}

func (p *parser) parseTablerow(args string) (node, error) {
	varName, rest, err := splitForArgs(args)
	if err != nil {
		return nil, err
	}
	colStr, attrs := splitAttrs(rest)
	col, err := parseExpr(colStr)
	if err != nil {
		return nil, err
	}
	n := &tablerowNode{varName: varName, collection: col}
	for _, a := range attrs {
		k, ve, err := parseAttr(a)
		if err != nil {
			return nil, err
		}
		switch k {
		case "cols":
			n.cols = ve
		case "limit":
			n.limit = ve
		case "offset":
			n.offset = ve
		}
	}
	body, err := p.parseBlock([]string{"endtablerow"})
	if err != nil {
		return nil, err
	}
	n.body = body
	p.consumeEnd()
	return n, nil
}

// applyLoopAttrs sets limit/offset/reversed on a forNode.
func applyLoopAttrs(n *forNode, attrs []string) error {
	for _, a := range attrs {
		if a == "reversed" {
			n.reversed = true
			continue
		}
		k, ve, err := parseAttr(a)
		if err != nil {
			return err
		}
		switch k {
		case "limit":
			n.limit = ve
		case "offset":
			n.offset = ve
		}
	}
	return nil
}

func (p *parser) parseCycle(args string) (node, error) {
	n := &cycleNode{key: args}
	rest := args
	// Optional group: `cycle "groupname": a, b, c`.
	if i := indexTopColon(rest); i >= 0 {
		ge, err := parseExpr(strings.TrimSpace(rest[:i]))
		if err != nil {
			return nil, err
		}
		n.group = ge
		rest = rest[i+1:]
	}
	for _, part := range splitTopLevel(rest, []string{","}) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		e, err := parseExpr(part)
		if err != nil {
			return nil, err
		}
		n.values = append(n.values, e)
	}
	return n, nil
}

// parseComment swallows everything up to {% endcomment %}.
func (p *parser) parseComment() (node, error) {
	for p.pos < len(p.toks) {
		t := p.toks[p.pos]
		p.pos++
		if t.kind == tokTag && t.name == "endcomment" {
			return nil, nil
		}
	}
	return nil, syntaxErr("'comment' tag was never closed")
}

// parseRaw emits the literal {% raw %}…{% endraw %} body. The tokenizer captures
// that body verbatim as a single tokRaw (see scanRaw), so the parser only has to
// take it and consume the trailing endraw tag; a missing endraw is an error.
func (p *parser) parseRaw() (node, error) {
	var body string
	if p.pos < len(p.toks) && p.toks[p.pos].kind == tokRaw {
		body = p.toks[p.pos].body
		p.pos++
	}
	if p.pos < len(p.toks) && p.toks[p.pos].kind == tokTag && p.toks[p.pos].name == "endraw" {
		p.pos++
		return &rawNode{text: body}, nil
	}
	return nil, syntaxErr("'raw' tag was never closed")
}

// consumeEnd consumes the matching end tag. parseBlock only returns with its
// stop tag present (it errors on EOF), so the current token is guaranteed to be
// the ender; this just advances past it.
func (p *parser) consumeEnd() { p.pos++ }

func contains_(set []string, s string) bool {
	for _, x := range set {
		if x == s {
			return true
		}
	}
	return false
}
