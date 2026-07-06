<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-liquid/brand/main/social/go-ruby-liquid-liquid.png" alt="go-ruby-liquid/liquid" width="720"></p>

# liquid — go-ruby-liquid

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-liquid.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of Shopify's [Liquid](https://shopify.github.io/liquid/)
template engine** — the deterministic, interpreter-independent core of MRI's
`Liquid::Template.parse(src).render(assigns)`. It parses a Liquid template and
renders it against a tree of Go values, matching the gem's output byte-for-byte
across the standard tag and filter set — **without any Ruby runtime**.

It is the Liquid backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module with no dependency on the Ruby runtime — a sibling
of [go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) (the Onigmo engine),
[go-ruby-erb](https://github.com/go-ruby-erb/erb) (the ERB compiler) and
[go-ruby-yaml](https://github.com/go-ruby-yaml/yaml) (the Psych emitter/loader).

> **What it is — and isn't.** Parsing and rendering the Liquid markup language
> (tags, filters, the variable/lookup grammar, truthiness, operators, whitespace
> control, lax/strict/warn error modes) is fully deterministic and needs **no
> interpreter**, so it lives here as pure Go. Templates render against a small,
> explicit value model — Go scalars, `[]any`, `map[string]any`, and any type
> implementing the `Drop` interface — that the host maps to and from its own
> objects.

## Features

Faithful port of Liquid's parser and renderer, validated against the `liquid`
gem on every supported platform:

- **Tags** — `if` / `elsif` / `else`, `unless`, `case` / `when` / `else`,
  `for` (with `limit:`, `offset:` / `offset:continue`, `reversed`, ranges,
  `forloop` drop, `else`, and `break` / `continue`), `tablerow` (with `cols:` /
  `limit:` / `offset:` and the `tablerowloop` drop), `assign`, `capture`,
  `increment` / `decrement`, `cycle` (named groups), `raw`, and `comment`.
- **Filters** — the complete standard set: `upcase` `downcase` `capitalize`
  `strip` `lstrip` `rstrip` `strip_newlines` `newline_to_br` `escape`
  `escape_once` `url_encode` `url_decode` `strip_html` `append` `prepend`
  `replace` `replace_first` `remove` `remove_first` `truncate` `truncatewords`
  `slice` `split` `join` `first` `last` `concat` `map` `where` `sort`
  `sort_natural` `uniq` `reverse` `size` `compact` `plus` `minus` `times`
  `divided_by` `modulo` `round` `ceil` `floor` `abs` `at_least` `at_most`
  `default`, and `date` (strftime).
- **Whitespace control** — `{%-` / `-%}` and `{{-` / `-}}` strip the adjacent
  text run exactly as the gem does.
- **Variable lookup** — dotted (`a.b.c`), bracketed (`a[0]`, `a["k"]`,
  `a[var]`), the `size` / `first` / `last` pseudo-properties, negative array
  indexing, and the `Drop` interface for host objects.
- **Operators** — `==` `!=` `<` `>` `<=` `>=` `contains`, plus `and` / `or`
  with Liquid's right-to-left, no-parentheses precedence.
- **Literals** — strings, numbers, `true` / `false` / `nil` / `empty` / `blank`,
  and `(a..b)` ranges.
- **Truthiness** — only `nil` and `false` are falsy (everything else, including
  `0` and `""`, is truthy), matching Ruby/Liquid semantics.
- **Error modes** — `Lax` (inline `Liquid error: …`, never fails), `Warn` (lax
  plus collected errors), and `Strict` (`render!` — first error returned).

CGO-free, dependency-free, **100% test coverage**, `gofmt` + `go vet` clean, and
green across the six 64-bit Go targets (amd64, arm64, riscv64, loong64, ppc64le,
s390x) and three OSes (Linux, macOS, Windows).

## Install

```sh
go get github.com/go-ruby-liquid/liquid
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-liquid/liquid"
)

func main() {
	tmpl, err := liquid.Parse(`{% for p in products %}{{ p.name | upcase }}: {{ p.price | times: 2 }}
{% endfor %}`)
	if err != nil {
		panic(err)
	}

	out, _ := tmpl.Render(map[string]any{
		"products": []any{
			map[string]any{"name": "shirt", "price": 10},
			map[string]any{"name": "hat", "price": 5},
		},
	})
	fmt.Print(out)
	// SHIRT: 20
	// HAT: 10
}
```

## API

```go
// Parse compiles a template (Liquid::Template.parse). In Lax/Warn mode parse
// never fails; in Strict mode a syntax error is returned.
func Parse(src string, opts ...Option) (*Template, error)

// MustParse is Parse but panics on error (convenient for static templates).
func MustParse(src string, opts ...Option) *Template

// Render renders against assigns in Lax mode (Liquid::Template#render).
func (t *Template) Render(assigns map[string]any) (string, error)

// RenderStrict renders like the gem's render!: the first error is returned.
func (t *Template) RenderStrict(assigns map[string]any) (string, error)

// Errors returns the errors collected in Warn mode.
func (t *Template) Errors() []error

type ErrorMode int
const (
	Lax    ErrorMode = iota // inline "Liquid error: …", never fails
	Warn                    // lax, but collects errors on the template
	Strict                  // render!: first error returned
)

func WithErrorMode(m ErrorMode) Option // Liquid error_mode:

// Drop lets a host object expose lookups to the template (Liquid::Drop); the
// bool reports whether the key resolved.
type Drop interface {
	LiquidGet(name string) (any, bool)
}
```

## Tests & coverage

The suite pairs deterministic, ruby-free tests (which alone hold coverage at
100%, so the qemu cross-arch and Windows lanes pass the gate) with a
**differential gem oracle**: a wide corpus of templates is rendered here and by
the system `liquid` gem and the outputs compared byte-for-byte — across tags,
filters, ranges, `forloop`, whitespace control, operators, and error rendering.
The oracle script `$stdout.binmode`s so Windows text-mode never pollutes the
bytes, and skips itself where `ruby` / the `liquid` gem is absent.

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-liquid/liquid authors.

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```
