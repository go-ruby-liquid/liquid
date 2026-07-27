// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import (
	"errors"
	"testing"
)

func TestWithFilterSingle(t *testing.T) {
	tpl := MustParse(`{{ "/a" | relative_url }}`,
		WithFilter("relative_url", func(in any, _ []any) (any, error) {
			return "/blog" + in.(string), nil
		}))
	got, err := tpl.Render(nil)
	if err != nil || got != "/blog/a" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestWithFilterInsideLoop(t *testing.T) {
	// The Jekyll case: a custom filter applied per iteration inside {% for %}.
	tpl := MustParse(`{% for p in xs %}{{ p | shout: "!" }}{% endfor %}`,
		WithFilters(map[string]Filter{
			"shout": func(in any, args []any) (any, error) {
				return in.(string) + args[0].(string), nil
			},
		}))
	got, err := tpl.Render(map[string]any{"xs": []any{"a", "b"}})
	if err != nil || got != "a!b!" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestWithFilterOverridesBuiltin(t *testing.T) {
	tpl := MustParse(`{{ "x" | upcase }}`,
		WithFilter("upcase", func(in any, _ []any) (any, error) {
			return "OVERRIDDEN", nil
		}))
	got, _ := tpl.Render(nil)
	if got != "OVERRIDDEN" {
		t.Fatalf("got %q", got)
	}
}

func TestWithFilterError(t *testing.T) {
	tpl := MustParse(`{{ "x" | boom }}`,
		WithFilter("boom", func(_ any, _ []any) (any, error) {
			return nil, errors.New("kaboom")
		}))
	// Lax mode renders the error inline.
	got, err := tpl.Render(nil)
	if err != nil {
		t.Fatalf("lax should not return err: %v", err)
	}
	if got != "Liquid error: kaboom" {
		t.Fatalf("got %q", got)
	}
	// Strict mode surfaces it.
	if _, err := tpl.RenderStrict(nil); err == nil {
		t.Fatal("strict should return err")
	}
}

func TestNoCustomFiltersStillWorks(t *testing.T) {
	// Nil filter map: built-in dispatch unaffected.
	tpl := MustParse(`{{ "x" | upcase }}`)
	if got, _ := tpl.Render(nil); got != "X" {
		t.Fatalf("got %q", got)
	}
}
