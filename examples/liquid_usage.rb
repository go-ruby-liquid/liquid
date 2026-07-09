# frozen_string_literal: true

require "liquid"

# Parse a template once, then render it against an assigns Hash. Variables use
# {{ ... }} (with filters after a pipe) and control flow uses {% ... %} tags.
hello = Liquid::Template.parse("Hello {{ name | upcase }}!")
puts hello.render("name" => "world")

# Filters chain left to right, and dotted lookups reach into nested Hashes.
row = Liquid::Template.parse("{% if u.admin %}[admin]{% else %}[user]{% endif %} {{ u.name }}")
puts row.render("u" => { "admin" => true, "name" => "Ada" })

# Loops, arithmetic filters, and `assign` for intermediate values.
order = Liquid::Template.parse(
  "{% assign total = price | times: qty %}{{ qty }} x {{ price }} = {{ total }}"
)
puts order.render("price" => 3, "qty" => 4)

# case/when, the `join` filter, and the `default` filter for nil values.
tags = Liquid::Template.parse("{{ items | join: ', ' }} ({{ label | default: 'untitled' }})")
puts tags.render("items" => %w[go ruby liquid], "label" => nil)

# `capture` builds a string into a variable; `{%- ... -%}` trims adjacent whitespace.
card = Liquid::Template.parse("{%- capture title -%} {{ who | capitalize }}'s page {%- endcapture -%}[{{ title }}]")
puts card.render("who" => "ada")

# render (lax) never raises; render!/strict mode surfaces errors as Liquid exceptions.
begin
  Liquid::Template.parse("{% if %}", error_mode: :strict)
rescue Liquid::SyntaxError => e
  puts "strict parse rejected bad template: #{e.class}"
end
