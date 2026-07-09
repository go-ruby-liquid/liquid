# Examples

Runnable pure-Ruby usage of the `Liquid` template engine, verified under the [rbgo](https://github.com/go-embedded-ruby) interpreter.

```sh
rbgo examples/liquid_usage.rb
```

| File | Shows |
| --- | --- |
| `liquid_usage.rb` | `Liquid::Template.parse`/`render` with an assigns Hash, `{{ }}` variables and filter chains (`upcase`, `times`, `join`, `default`), dotted lookups, `if`/`case`/`for` tags, `assign` and `capture`, `{%- -%}` whitespace control, and `error_mode: :strict` raising `Liquid::SyntaxError` |
