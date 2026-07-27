// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

// Filter is a host-supplied filter implementation. It receives the piped input
// value and the already-evaluated positional arguments, and returns the
// transformed value. Returning a non-nil error surfaces through the active
// [ErrorMode] exactly like a built-in filter failure.
//
// Custom filters let a host (for example a Jekyll front-end) extend the filter
// vocabulary — relative_url, jsonify, slugify, … — without forking the engine.
// A registered name takes precedence over a built-in of the same name.
type Filter func(input any, args []any) (any, error)

// WithFilter registers a single custom [Filter] under name. It may be passed to
// [Parse] more than once and combines with [WithFilters].
func WithFilter(name string, fn Filter) Option {
	return func(c *parseConfig) {
		if c.filters == nil {
			c.filters = map[string]Filter{}
		}
		c.filters[name] = fn
	}
}

// WithFilters registers a set of custom filters in one call. Later registrations
// (including a subsequent [WithFilter]) override earlier ones for the same name.
func WithFilters(m map[string]Filter) Option {
	return func(c *parseConfig) {
		if c.filters == nil {
			c.filters = map[string]Filter{}
		}
		for k, v := range m {
			c.filters[k] = v
		}
	}
}
