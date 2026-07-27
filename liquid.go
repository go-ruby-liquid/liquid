// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package liquid is a pure-Go (CGO-free) reimplementation of Shopify's Ruby
// Liquid template engine. It parses a Liquid source template and renders it
// against a set of assigns, matching the rendered output of the `liquid` gem.
//
// The entry point mirrors the gem's API:
//
//	tmpl, err := liquid.Parse(src)        // Liquid::Template.parse(src)
//	out, err  := tmpl.Render(assigns)     // tmpl.render(assigns)
//	out, err  := tmpl.RenderStrict(assigns) // tmpl.render!(assigns) — raises
//
// # Value model
//
// Assigns are an ordinary Go value tree; the engine accepts and produces the
// small, fixed set of Go types a host (such as go-embedded-ruby) maps to and
// from its own object graph:
//
//	Ruby            Go
//	----            --
//	nil             nil
//	true / false    bool
//	Integer         int, int64
//	Float           float64
//	String          string
//	Array           []any
//	Hash            map[string]any
//	Drop            Drop (LiquidValue / context-aware lookups)
//
// Render walks the parsed tree and writes a string, so a host binds
// object.Value to and from these Go shapes.
package liquid

import "strings"

// Template is a parsed Liquid template ready to render.
type Template struct {
	root    *blockNode
	errors  []error           // errors collected during a lax/warn render
	filters map[string]Filter // host-supplied custom filters (may be nil)
}

// ErrorMode selects how parse and render errors are surfaced.
type ErrorMode int

const (
	// Lax swallows recoverable errors, rendering an inline "Liquid error: …"
	// message in their place (the gem's default at render time).
	Lax ErrorMode = iota
	// Warn behaves like Lax but also collects the errors on the template.
	Warn
	// Strict turns recoverable parse/render errors into a returned error.
	Strict
)

// Option configures Parse.
type Option func(*parseConfig)

type parseConfig struct {
	mode    ErrorMode
	filters map[string]Filter
}

// WithErrorMode selects the parse/render error mode (default Lax).
func WithErrorMode(m ErrorMode) Option {
	return func(c *parseConfig) { c.mode = m }
}

// Parse compiles a Liquid source template. It mirrors
// Liquid::Template.parse(src). A syntax error is returned only in Strict mode;
// in Lax/Warn mode parse never fails and malformed constructs render as inline
// errors.
func Parse(src string, opts ...Option) (*Template, error) {
	cfg := &parseConfig{}
	for _, o := range opts {
		o(cfg)
	}
	toks := tokenize(src)
	p := &parser{toks: toks, mode: cfg.mode}
	root, err := p.parseBlock(nil)
	if err != nil {
		return nil, err
	}
	// parseBlock(nil) consumes the whole token stream: a stray end-tag (e.g.
	// {% endif %} with no opener) is reported by parseTag as an unknown tag, so
	// there is never a leftover token here.
	return &Template{root: root, filters: cfg.filters}, nil
}

// MustParse is Parse without error handling, for tests and trusted templates.
func MustParse(src string, opts ...Option) *Template {
	t, err := Parse(src, opts...)
	if err != nil {
		panic(err)
	}
	return t
}

// Render renders the template against assigns in Lax mode: recoverable runtime
// errors are written inline as "Liquid error: …" and the returned error is nil.
func (t *Template) Render(assigns map[string]any) (string, error) {
	return t.render(assigns, Lax)
}

// RenderStrict renders the template like the gem's render!: the first
// recoverable runtime error stops rendering and is returned.
func (t *Template) RenderStrict(assigns map[string]any) (string, error) {
	return t.render(assigns, Strict)
}

func (t *Template) render(assigns map[string]any, mode ErrorMode) (string, error) {
	var sb strings.Builder
	ctx := newContext(assigns, mode)
	ctx.filters = t.filters
	err := t.root.render(&sb, ctx)
	t.errors = ctx.errors
	if err != nil {
		if _, ok := err.(*interrupt); ok {
			// A top-level break/continue is a no-op (the gem ignores it).
			return sb.String(), nil
		}
		return sb.String(), err
	}
	return sb.String(), nil
}

// Errors returns the errors collected during the most recent Render (Lax/Warn
// mode), mirroring template.errors in the gem.
func (t *Template) Errors() []error { return t.errors }
