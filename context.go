// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

// context holds the mutable state of a single render: the scope stack of
// assigns, the increment/decrement counters, the cycle position table, and the
// error mode.
type context struct {
	scopes     []map[string]any // innermost scope last
	counters   map[string]int   // increment/decrement state
	cycles     map[string]int   // cycle group -> next index
	mode       ErrorMode
	errors     []error
	depthGuard int
}

func newContext(assigns map[string]any, mode ErrorMode) *context {
	base := map[string]any{}
	for k, v := range assigns {
		base[k] = v
	}
	return &context{
		scopes:   []map[string]any{base},
		counters: map[string]int{},
		cycles:   map[string]int{},
		mode:     mode,
	}
}

// push adds a new innermost scope (for/tablerow/capture-free blocks that
// introduce loop variables).
func (c *context) push() { c.scopes = append(c.scopes, map[string]any{}) }

// pop removes the innermost scope.
func (c *context) pop() { c.scopes = c.scopes[:len(c.scopes)-1] }

// get resolves a base variable name across the scope stack (innermost first).
func (c *context) get(name string) (any, bool) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if v, ok := c.scopes[i][name]; ok {
			return v, true
		}
	}
	return nil, false
}

// set binds name in the innermost scope.
func (c *context) set(name string, v any) {
	c.scopes[len(c.scopes)-1][name] = v
}

// setGlobal binds name in the outermost scope (assign/capture persist across
// loop iterations, matching the gem's single shared assigns hash).
func (c *context) setGlobal(name string, v any) {
	c.scopes[0][name] = v
}

// fail handles a recoverable runtime error per the active mode: Strict returns
// it; Lax/Warn record it and signal the caller to emit the inline message.
func (c *context) fail(e *Error) (inline string, err error) {
	if c.mode == Strict {
		return "", e
	}
	c.errors = append(c.errors, e)
	return e.inlineMessage(), nil
}
