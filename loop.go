// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import (
	"strconv"
	"strings"
)

// forNode is {% for var in collection (limit/offset/reversed) %}…{% else %}…
// {% endfor %}.
type forNode struct {
	varName    string
	collection expr
	limit      expr // optional
	offset     expr // optional
	reversed   bool
	body       *blockNode
	els        *blockNode // {% else %} when the collection is empty
}

func (n *forNode) render(sb *strings.Builder, ctx *context) error {
	items := toSlice(n.collection.eval(ctx))

	if n.offset != nil {
		off := toInt(n.offset.eval(ctx))
		if off < 0 {
			off = 0
		}
		if off > len(items) {
			off = len(items)
		}
		items = items[off:]
	}
	if n.limit != nil {
		lim := toInt(n.limit.eval(ctx))
		if lim < 0 {
			lim = 0
		}
		if lim < len(items) {
			items = items[:lim]
		}
	}
	// Liquid quirk: `reversed` reverses the whole collection, but when combined
	// with limit/offset the slice is taken first and the reversal is effectively
	// suppressed (the gem yields the forward slice). So only reverse when neither
	// limit nor offset is present.
	if n.reversed && n.limit == nil && n.offset == nil {
		rev := make([]any, len(items))
		for i, v := range items {
			rev[len(items)-1-i] = v
		}
		items = rev
	}

	if len(items) == 0 {
		if n.els != nil {
			return n.els.render(sb, ctx)
		}
		return nil
	}

	parent := currentForloop(ctx)
	ctx.push()
	defer ctx.pop()
	total := len(items)
	for i, item := range items {
		ctx.set(n.varName, item)
		ctx.set("forloop", newForloop(i, total, parent))
		err := n.body.render(sb, ctx)
		if err != nil {
			if it, ok := err.(*interrupt); ok {
				if it.kind == "break" {
					break
				}
				continue // "continue"
			}
			return err
		}
	}
	return nil
}

// currentForloop returns the forloop drop in scope (for parentloop), or nil.
func currentForloop(ctx *context) *forloop {
	if v, ok := ctx.get("forloop"); ok {
		if fl, ok := v.(*forloop); ok {
			return fl
		}
	}
	return nil
}

// forloop is the loop status object exposed inside a for body.
type forloop struct {
	index0, length int
	parent         *forloop
}

func newForloop(i, length int, parent *forloop) *forloop {
	return &forloop{index0: i, length: length, parent: parent}
}

// LiquidGet exposes the forloop members to lookups (forloop.index, .first, …).
func (f *forloop) LiquidGet(name string) (any, bool) {
	switch name {
	case "index":
		return f.index0 + 1, true
	case "index0":
		return f.index0, true
	case "rindex":
		return f.length - f.index0, true
	case "rindex0":
		return f.length - f.index0 - 1, true
	case "first":
		return f.index0 == 0, true
	case "last":
		return f.index0 == f.length-1, true
	case "length":
		return f.length, true
	case "parentloop":
		if f.parent == nil {
			return nil, true
		}
		return f.parent, true
	}
	return nil, false
}

// tablerowNode is {% tablerow var in collection (cols/limit/offset) %}…
// {% endtablerow %}, emitting <tr>/<td> grid markup like the gem.
type tablerowNode struct {
	varName    string
	collection expr
	cols       expr
	limit      expr
	offset     expr
	body       *blockNode
}

func (n *tablerowNode) render(sb *strings.Builder, ctx *context) error {
	items := toSlice(n.collection.eval(ctx))
	if n.offset != nil {
		off := clamp(toInt(n.offset.eval(ctx)), 0, len(items))
		items = items[off:]
	}
	if n.limit != nil {
		lim := toInt(n.limit.eval(ctx))
		if lim >= 0 && lim < len(items) {
			items = items[:lim]
		}
	}
	cols := len(items)
	if n.cols != nil {
		cols = toInt(n.cols.eval(ctx))
	}
	if cols < 1 {
		cols = 1
	}

	ctx.push()
	defer ctx.pop()
	row := 1
	sb.WriteString("<tr class=\"row1\">\n")
	for i, item := range items {
		colIdx := i % cols
		if i > 0 && colIdx == 0 {
			row++
			sb.WriteString("</tr>\n<tr class=\"row")
			sb.WriteString(strconv.Itoa(row))
			sb.WriteString("\">")
		}
		sb.WriteString("<td class=\"col")
		sb.WriteString(strconv.Itoa(colIdx + 1))
		sb.WriteString("\">")
		ctx.set(n.varName, item)
		if err := n.body.render(sb, ctx); err != nil {
			if _, ok := err.(*interrupt); ok {
				// tablerow ignores break/continue mid-cell in the gem; close cleanly.
				sb.WriteString("</td>")
				continue
			}
			return err
		}
		sb.WriteString("</td>")
	}
	sb.WriteString("</tr>\n")
	return nil
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// splitForArgs separates a for/tablerow argument string into its "var in
// collection" head and the trailing attribute words (limit:/offset:/cols:/
// reversed).
func splitForArgs(args string) (varName, collectionAndAttrs string, err error) {
	parts := strings.Fields(args)
	if len(parts) < 3 || parts[1] != "in" {
		return "", "", syntaxErr("for loop malformed: " + args)
	}
	varName = parts[0]
	// The collection plus attribute tail is everything after "in".
	idx := strings.Index(args, parts[1])
	rest := strings.TrimSpace(args[idx+len(parts[1]):])
	return varName, rest, nil
}
