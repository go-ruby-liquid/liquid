// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import (
	"strconv"
	"strings"
)

// node is one renderable element of a parsed template.
type node interface {
	render(sb *strings.Builder, ctx *context) error
}

// interrupt is the control-flow signal raised by {% break %} / {% continue %}.
type interrupt struct{ kind string } // "break" or "continue"

func (i *interrupt) Error() string { return "interrupt:" + i.kind }

// blockNode is a sequence of child nodes.
type blockNode struct{ children []node }

func (b *blockNode) render(sb *strings.Builder, ctx *context) error {
	for _, c := range b.children {
		if err := c.render(sb, ctx); err != nil {
			return err
		}
	}
	return nil
}

// textNode is literal template text.
type textNode struct{ text string }

func (t *textNode) render(sb *strings.Builder, _ *context) error {
	sb.WriteString(t.text)
	return nil
}

// outputNode is {{ expr | filter: arg | ... }}.
type outputNode struct {
	expr    expr
	filters []filterCall
}

func (o *outputNode) render(sb *strings.Builder, ctx *context) error {
	v := o.expr.eval(ctx)
	for _, f := range o.filters {
		var err *Error
		v, err = f.apply(ctx, v)
		if err != nil {
			return o.emitErr(sb, ctx, err)
		}
	}
	sb.WriteString(toStr(v))
	return nil
}

func (o *outputNode) emitErr(sb *strings.Builder, ctx *context, le *Error) error {
	inline, rerr := ctx.fail(le)
	if rerr != nil {
		return rerr
	}
	sb.WriteString(inline)
	return nil
}

// assignNode is {% assign name = expr | filters %}.
type assignNode struct {
	name    string
	expr    expr
	filters []filterCall
}

func (a *assignNode) render(_ *strings.Builder, ctx *context) error {
	v := a.expr.eval(ctx)
	for _, f := range a.filters {
		var err *Error
		v, err = f.apply(ctx, v)
		if err != nil {
			return runtimeOrNil(ctx, err)
		}
	}
	ctx.setGlobal(a.name, v)
	return nil
}

// runtimeOrNil collects a recoverable error (Lax/Warn) or returns it (Strict);
// used by tags that have no inline output slot.
func runtimeOrNil(ctx *context, le *Error) error {
	_, rerr := ctx.fail(le)
	return rerr
}

// emitRuntime writes a recoverable error's inline message to sb (Lax/Warn) or
// returns it (Strict); used by tags whose condition evaluation errors must
// surface inline in the output, like the gem.
func emitRuntime(sb *strings.Builder, ctx *context, le *Error) error {
	inline, rerr := ctx.fail(le)
	if rerr != nil {
		return rerr
	}
	sb.WriteString(inline)
	return nil
}

// captureNode is {% capture name %}…{% endcapture %}.
type captureNode struct {
	name string
	body *blockNode
}

func (c *captureNode) render(_ *strings.Builder, ctx *context) error {
	var inner strings.Builder
	if err := c.body.render(&inner, ctx); err != nil {
		return err
	}
	ctx.setGlobal(c.name, inner.String())
	return nil
}

// incrementNode / decrementNode keep a per-name counter independent of assigns.
type incrementNode struct {
	name string
	step int // +1 or -1
}

func (n *incrementNode) render(sb *strings.Builder, ctx *context) error {
	if n.step > 0 {
		v := ctx.counters[n.name]
		sb.WriteString(strconv.Itoa(v))
		ctx.counters[n.name] = v + 1
	} else {
		v := ctx.counters[n.name] - 1
		ctx.counters[n.name] = v
		sb.WriteString(strconv.Itoa(v))
	}
	return nil
}

// ifNode is {% if/unless %} with elsif/else branches.
type ifNode struct {
	branches []ifBranch
	els      *blockNode
}

type ifBranch struct {
	cond   *condition
	body   *blockNode
	negate bool // {% unless %} inverts the condition
}

func (n *ifNode) render(sb *strings.Builder, ctx *context) error {
	for _, br := range n.branches {
		ok, err := br.cond.eval(ctx)
		if err != nil {
			return emitRuntime(sb, ctx, err)
		}
		if br.negate {
			ok = !ok
		}
		if ok {
			return br.body.render(sb, ctx)
		}
	}
	if n.els != nil {
		return n.els.render(sb, ctx)
	}
	return nil
}

// caseNode is {% case expr %}{% when … %}…{% else %}…{% endcase %}.
type caseNode struct {
	subject expr
	whens   []whenBranch
	els     *blockNode
}

type whenBranch struct {
	values []expr // a when may list several comma/or-separated values
	body   *blockNode
}

func (n *caseNode) render(sb *strings.Builder, ctx *context) error {
	subj := n.subject.eval(ctx)
	matched := false
	for _, w := range n.whens {
		for _, ve := range w.values {
			wv := ve.eval(ctx)
			if valueEqual(subj, wv) {
				if err := w.body.render(sb, ctx); err != nil {
					return err
				}
				matched = true
				break
			}
		}
	}
	if !matched && n.els != nil {
		return n.els.render(sb, ctx)
	}
	return nil
}

// rawNode emits its body verbatim ({% raw %}…{% endraw %}).
type rawNode struct{ text string }

func (r *rawNode) render(sb *strings.Builder, _ *context) error {
	sb.WriteString(r.text)
	return nil
}

// breakNode / continueNode raise the loop interrupt.
type breakNode struct{}

func (breakNode) render(*strings.Builder, *context) error { return &interrupt{"break"} }

type continueNode struct{}

func (continueNode) render(*strings.Builder, *context) error { return &interrupt{"continue"} }

// cycleNode rotates through its values, keyed by an optional group name.
type cycleNode struct {
	group  expr // optional group key (nil => keyed by the value list)
	values []expr
	key    string // stable identity of the value list (for default group)
}

func (n *cycleNode) render(sb *strings.Builder, ctx *context) error {
	key := n.key
	if n.group != nil {
		key = "g:" + toStr(n.group.eval(ctx))
	}
	idx := ctx.cycles[key]
	if len(n.values) == 0 {
		return nil
	}
	v := n.values[idx%len(n.values)].eval(ctx)
	sb.WriteString(toStr(v))
	ctx.cycles[key] = (idx + 1) % len(n.values)
	return nil
}
