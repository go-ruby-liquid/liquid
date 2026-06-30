// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import (
	"math"
	"math/big"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// applyFilter dispatches a standard filter by name. An unknown filter is a
// no-op that returns its input unchanged, matching the gem (an undefined filter
// is silently ignored, even under render!).
func applyFilter(name string, in any, args []any) (any, *Error) {
	switch name {
	// --- string ---
	case "upcase":
		return strings.ToUpper(toStr(in)), nil
	case "downcase":
		return strings.ToLower(toStr(in)), nil
	case "capitalize":
		return capitalize(toStr(in)), nil
	case "strip":
		return strings.TrimSpace(toStr(in)), nil
	case "lstrip":
		return strings.TrimLeft(toStr(in), " \t\r\n\f\v"), nil
	case "rstrip":
		return strings.TrimRight(toStr(in), " \t\r\n\f\v"), nil
	case "strip_newlines":
		return strings.NewReplacer("\r\n", "", "\n", "", "\r", "").Replace(toStr(in)), nil
	case "newline_to_br":
		return strings.ReplaceAll(toStr(in), "\n", "<br />\n"), nil
	case "escape":
		return htmlEscape(toStr(in)), nil
	case "escape_once":
		return escapeOnce(toStr(in)), nil
	case "url_encode":
		return urlEncode(toStr(in)), nil
	case "url_decode":
		return urlDecode(toStr(in))
	case "strip_html":
		return stripHTML(toStr(in)), nil
	case "append":
		return toStr(in) + arg0Str(args), nil
	case "prepend":
		return arg0Str(args) + toStr(in), nil
	case "replace":
		return replace(toStr(in), args, false, true), nil
	case "replace_first":
		return replace(toStr(in), args, true, true), nil
	case "remove":
		return replace(toStr(in), args, false, false), nil
	case "remove_first":
		return replace(toStr(in), args, true, false), nil
	case "truncate":
		return truncate(toStr(in), args), nil
	case "truncatewords":
		return truncatewords(toStr(in), args), nil
	case "slice":
		return slice(in, args), nil
	case "split":
		return split(toStr(in), args), nil

	// --- array ---
	case "join":
		return join(in, args), nil
	case "first":
		return first(in), nil
	case "last":
		return last(in), nil
	case "concat":
		return concat(in, args)
	case "map":
		return mapField(in, args), nil
	case "where":
		return where(in, args), nil
	case "sort":
		return sortFilter(in, args, false), nil
	case "sort_natural":
		return sortFilter(in, args, true), nil
	case "uniq":
		return uniq(in), nil
	case "reverse":
		return reverse(in), nil
	case "size":
		return size(in), nil
	case "compact":
		return compact(in), nil

	// --- math ---
	case "plus":
		return arith(in, args, '+')
	case "minus":
		return arith(in, args, '-')
	case "times":
		return arith(in, args, '*')
	case "divided_by":
		return dividedBy(in, args)
	case "modulo":
		return modulo(in, args)
	case "round":
		return round(in, args), nil
	case "ceil":
		return int(math.Ceil(toFloat(in))), nil
	case "floor":
		return int(math.Floor(toFloat(in))), nil
	case "abs":
		return absFilter(in), nil
	case "at_least":
		return atLeast(in, args), nil
	case "at_most":
		return atMost(in, args), nil

	// --- misc ---
	case "default":
		return defaultFilter(in, args), nil
	case "date":
		return dateFilter(in, args)
	}
	return in, nil
}

func arg0(args []any) any {
	if len(args) == 0 {
		return nil
	}
	return args[0]
}

func arg0Str(args []any) string { return toStr(arg0(args)) }

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	head := strings.ToUpper(string(r[0]))
	tail := strings.ToLower(string(r[1:]))
	return head + tail
}

func htmlEscape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	).Replace(s)
}

// escapeOnce escapes HTML but leaves already-escaped entities intact.
func escapeOnce(s string) string {
	// First unescape the entities we manage, then escape — net effect is one pass.
	return htmlEscape(unescapeBasic(s))
}

func unescapeBasic(s string) string {
	return strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
	).Replace(s)
}

func urlEncode(s string) string {
	// Ruby CGI.escape encodes spaces as '+'.
	return url.QueryEscape(s)
}

func urlDecode(s string) (string, *Error) {
	out, err := url.QueryUnescape(s)
	if err != nil {
		return "", argErr("invalid byte sequence")
	}
	return out, nil
}

// stripHTML removes tags and script/style contents, like the gem's regexp set.
func stripHTML(s string) string {
	s = stripBetween(s, "<script", "</script>")
	s = stripBetween(s, "<style", "</style>")
	s = stripComments(s)
	var sb strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func stripBetween(s, open, close string) string {
	for {
		lo := strings.Index(strings.ToLower(s), open)
		if lo < 0 {
			return s
		}
		hi := strings.Index(strings.ToLower(s[lo:]), close)
		if hi < 0 {
			return s[:lo]
		}
		s = s[:lo] + s[lo+hi+len(close):]
	}
}

func stripComments(s string) string {
	for {
		lo := strings.Index(s, "<!--")
		if lo < 0 {
			return s
		}
		hi := strings.Index(s[lo:], "-->")
		if hi < 0 {
			return s[:lo]
		}
		s = s[:lo] + s[lo+hi+3:]
	}
}

func replace(s string, args []any, firstOnly, withRepl bool) string {
	if len(args) == 0 {
		return s
	}
	target := toStr(args[0])
	repl := ""
	if withRepl && len(args) > 1 {
		repl = toStr(args[1])
	}
	if target == "" {
		return s
	}
	if firstOnly {
		return strings.Replace(s, target, repl, 1)
	}
	return strings.ReplaceAll(s, target, repl)
}

func truncate(s string, args []any) string {
	n := 50
	if len(args) > 0 {
		n = toInt(args[0])
	}
	ell := "..."
	if len(args) > 1 {
		ell = toStr(args[1])
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	cut := n - len([]rune(ell))
	if cut < 0 {
		cut = 0
	}
	return string(r[:cut]) + ell
}

func truncatewords(s string, args []any) string {
	n := 15
	if len(args) > 0 {
		n = toInt(args[0])
	}
	if n < 1 {
		n = 1
	}
	ell := "..."
	if len(args) > 1 {
		ell = toStr(args[1])
	}
	words := strings.Fields(s)
	if len(words) <= n {
		return s
	}
	return strings.Join(words[:n], " ") + ell
}

func slice(in any, args []any) any {
	start := 0
	if len(args) > 0 {
		start = toInt(args[0])
	}
	length := 1
	if len(args) > 1 {
		length = toInt(args[1])
	}
	if arr, ok := in.([]any); ok {
		return sliceSeq(len(arr), start, length, func(i int) any { return arr[i] }, func(s []any) any { return s }, []any{})
	}
	r := []rune(toStr(in))
	res := sliceSeq(len(r), start, length, func(i int) any { return r[i] }, nil, nil)
	out := res.([]rune)
	return string(out)
}

// sliceSeq computes Ruby's Array/String#slice(start, len) index math and returns
// either an []any (arrays) or []rune (strings, when collect is nil).
func sliceSeq(n, start, length int, at func(int) any, asArr func([]any) any, empty any) any {
	if start < 0 {
		start += n
	}
	if start < 0 || start >= n || length <= 0 {
		if asArr != nil {
			return empty
		}
		return []rune{}
	}
	end := start + length
	if end > n {
		end = n
	}
	if asArr != nil {
		out := make([]any, 0, end-start)
		for i := start; i < end; i++ {
			out = append(out, at(i))
		}
		return asArr(out)
	}
	out := make([]rune, 0, end-start)
	for i := start; i < end; i++ {
		out = append(out, at(i).(rune))
	}
	return out
}

func split(s string, args []any) any {
	sep := arg0Str(args)
	var parts []string
	if sep == "" {
		for _, r := range s {
			parts = append(parts, string(r))
		}
	} else {
		parts = strings.Split(s, sep)
	}
	out := make([]any, len(parts))
	for i, p := range parts {
		out[i] = p
	}
	return out
}

func join(in any, args []any) string {
	sep := " "
	if len(args) > 0 {
		sep = toStr(args[0])
	}
	items := toSlice(in)
	parts := make([]string, len(items))
	for i, e := range items {
		parts[i] = toStr(e)
	}
	return strings.Join(parts, sep)
}

func first(in any) any {
	switch x := in.(type) {
	case []any:
		if len(x) == 0 {
			return nil
		}
		return x[0]
	case string:
		r := []rune(x)
		if len(r) == 0 {
			return nil
		}
		return string(r[0])
	}
	return nil
}

func last(in any) any {
	switch x := in.(type) {
	case []any:
		if len(x) == 0 {
			return nil
		}
		return x[len(x)-1]
	case string:
		r := []rune(x)
		if len(r) == 0 {
			return nil
		}
		return string(r[len(r)-1])
	}
	return nil
}

func concat(in any, args []any) (any, *Error) {
	a := append([]any{}, toSlice(in)...)
	if len(args) == 0 {
		return a, nil
	}
	other, ok := args[0].([]any)
	if !ok {
		return nil, argErr("concat filter requires an array argument")
	}
	return append(a, other...), nil
}

func mapField(in any, args []any) any {
	field := arg0Str(args)
	items := toSlice(in)
	out := make([]any, len(items))
	for i, e := range items {
		out[i] = index(e, field)
	}
	return out
}

func where(in any, args []any) any {
	items := toSlice(in)
	field := arg0Str(args)
	var out []any
	for _, e := range items {
		v := index(e, field)
		if len(args) >= 2 {
			if valueEqual(v, args[1]) {
				out = append(out, e)
			}
		} else if truthy(v) {
			out = append(out, e)
		}
	}
	if out == nil {
		return []any{}
	}
	return out
}

func sortFilter(in any, args []any, natural bool) any {
	items := append([]any{}, toSlice(in)...)
	field := ""
	if len(args) > 0 {
		field = arg0Str(args)
	}
	keyOf := func(e any) any {
		if field != "" {
			return index(e, field)
		}
		return e
	}
	sort.SliceStable(items, func(i, j int) bool {
		return lessValue(keyOf(items[i]), keyOf(items[j]), natural)
	})
	return items
}

// lessValue orders two values for sort: numbers numerically, otherwise by their
// string form (case-insensitively for sort_natural).
func lessValue(a, b any, natural bool) bool {
	if isNumber(a) && isNumber(b) {
		return toFloat(a) < toFloat(b)
	}
	sa, sb := toStr(a), toStr(b)
	if natural {
		sa, sb = strings.ToLower(sa), strings.ToLower(sb)
	}
	return sa < sb
}

func uniq(in any) any {
	items := toSlice(in)
	var out []any
	for _, e := range items {
		dup := false
		for _, k := range out {
			if valueEqual(k, e) {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, e)
		}
	}
	if out == nil {
		return []any{}
	}
	return out
}

func reverse(in any) any {
	items := toSlice(in)
	out := make([]any, len(items))
	for i, e := range items {
		out[len(items)-1-i] = e
	}
	return out
}

func size(in any) any {
	switch x := in.(type) {
	case string:
		return len([]rune(x))
	case []any:
		return len(x)
	case map[string]any:
		return len(x)
	case nil:
		return 0
	}
	return 0
}

func compact(in any) any {
	items := toSlice(in)
	var out []any
	for _, e := range items {
		if e != nil {
			out = append(out, e)
		}
	}
	if out == nil {
		return []any{}
	}
	return out
}

// numericIsFloat reports whether a value should drive float arithmetic: an
// actual float, or a string whose numeric form has a fractional part (Ruby
// coerces "3" to an Integer but "3.5" to a Float in math filters).
func numericIsFloat(v any) bool {
	if isFloat(v) {
		return true
	}
	if s, ok := v.(string); ok {
		return strings.Contains(strings.TrimSpace(s), ".")
	}
	return false
}

// arith implements plus/minus/times preserving integer vs float type: the
// result is a float when either operand is a float, else an integer. Float
// arithmetic is done in exact decimal (via big.Rat over the shortest decimal of
// each operand), matching the gem's BigDecimal model: 0.1 + 0.2 == 0.3, not the
// binary float 0.30000000000000004.
func arith(in any, args []any, op byte) (any, *Error) {
	b := arg0(args)
	if numericIsFloat(in) || numericIsFloat(b) {
		x, y := ratOf(in), ratOf(b)
		r := new(big.Rat)
		switch op {
		case '+':
			r.Add(x, y)
		case '-':
			r.Sub(x, y)
		default:
			r.Mul(x, y)
		}
		return ratToFloat(r), nil
	}
	x, y := toInt(in), toInt(b)
	switch op {
	case '+':
		return x + y, nil
	case '-':
		return x - y, nil
	default:
		return x * y, nil
	}
}

// ratOf builds an exact rational from a numeric value, parsing the float's
// shortest decimal string so 0.1 becomes 1/10 (not the binary approximation).
func ratOf(v any) *big.Rat {
	switch x := v.(type) {
	case int:
		return new(big.Rat).SetInt64(int64(x))
	case int64:
		return new(big.Rat).SetInt64(x)
	case float64:
		r := new(big.Rat)
		r.SetString(strconv.FormatFloat(x, 'g', -1, 64))
		return r
	case string:
		r := new(big.Rat)
		if _, ok := r.SetString(strings.TrimSpace(x)); ok {
			return r
		}
		return new(big.Rat).SetFloat64(toFloat(x))
	}
	return new(big.Rat)
}

// ratToFloat renders an exact rational back to the nearest float64; the result
// formats via formatFloat, reproducing Ruby's BigDecimal#to_f.to_s output.
func ratToFloat(r *big.Rat) float64 {
	f, _ := r.Float64()
	return f
}

func dividedBy(in any, args []any) (any, *Error) {
	b := arg0(args)
	if numericIsFloat(in) || numericIsFloat(b) {
		y := toFloat(b)
		if y == 0 {
			return nil, &Error{Type: "ZeroDivisionError", Message: "divided by 0"}
		}
		return toFloat(in) / y, nil
	}
	y := toInt(b)
	if y == 0 {
		return nil, &Error{Type: "ZeroDivisionError", Message: "divided by 0"}
	}
	return floorDivInt(toInt(in), y), nil
}

// floorDivInt is Ruby Integer division (floored toward negative infinity).
func floorDivInt(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func modulo(in any, args []any) (any, *Error) {
	b := arg0(args)
	if numericIsFloat(in) || numericIsFloat(b) {
		y := toFloat(b)
		if y == 0 {
			return nil, &Error{Type: "ZeroDivisionError", Message: "divided by 0"}
		}
		return math.Mod(math.Mod(toFloat(in), y)+y, y), nil
	}
	y := toInt(b)
	if y == 0 {
		return nil, &Error{Type: "ZeroDivisionError", Message: "divided by 0"}
	}
	m := toInt(in) % y
	if m != 0 && (m < 0) != (y < 0) {
		m += y
	}
	return m, nil
}

func round(in any, args []any) any {
	prec := 0
	if len(args) > 0 {
		prec = toInt(args[0])
	}
	f := toFloat(in)
	mult := math.Pow(10, float64(prec))
	r := math.Round(f*mult) / mult
	if prec <= 0 {
		return int(r)
	}
	return r
}

func absFilter(in any) any {
	if isFloat(in) {
		return math.Abs(toFloat(in))
	}
	v := toInt(in)
	if v < 0 {
		return -v
	}
	return v
}

func atLeast(in any, args []any) any {
	return clampNum(in, arg0(args), true)
}

func atMost(in any, args []any) any {
	return clampNum(in, arg0(args), false)
}

// clampNum returns max(in,b) when least, else min(in,b), preserving int/float.
func clampNum(in, b any, least bool) any {
	if numericIsFloat(in) || numericIsFloat(b) {
		x, y := toFloat(in), toFloat(b)
		if least {
			return math.Max(x, y)
		}
		return math.Min(x, y)
	}
	x, y := toInt(in), toInt(b)
	if least {
		if x > y {
			return x
		}
		return y
	}
	if x < y {
		return x
	}
	return y
}

// defaultFilter returns the argument when in is nil/false/blank (the gem treats
// "", [], {} as blank for default), else in. With allow_false: true (passed as a
// truthy second argument), an explicit false is kept instead of defaulted.
func defaultFilter(in any, args []any) any {
	allowFalse := len(args) > 1 && truthy(args[1])
	if allowFalse {
		if in == nil || isEmptyForDefault(in) {
			return arg0(args)
		}
		return in
	}
	if isBlank(in) {
		return arg0(args)
	}
	return in
}

// isEmptyForDefault reports the blank-but-not-false cases (nil already handled):
// empty string/array/map. Used by default with allow_false.
func isEmptyForDefault(in any) bool { return isEmpty(in) }
