// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

// Drop is the interface a host object may implement to expose context-aware
// member lookups to templates (the gem's Liquid::Drop). LiquidGet returns the
// value for a member name and whether it is defined.
type Drop interface {
	LiquidGet(name string) (any, bool)
}

// index resolves obj[key] for the Liquid value model: maps by string key,
// arrays by integer index (with negative-from-end and the size/first/last
// pseudo-properties), strings by size/first/last, and Drops via LiquidGet.
func index(obj, key any) any {
	switch o := obj.(type) {
	case map[string]any:
		ks := toStr(key)
		if v, ok := o[ks]; ok {
			return v
		}
		switch ks {
		case "size":
			return len(o)
		case "first":
			return firstPair(o)
		case "last":
			return lastPair(o)
		}
		return nil
	case []any:
		switch k := key.(type) {
		case string:
			switch k {
			case "size":
				return len(o)
			case "first":
				if len(o) == 0 {
					return nil
				}
				return o[0]
			case "last":
				if len(o) == 0 {
					return nil
				}
				return o[len(o)-1]
			}
			return nil
		default:
			i := toInt(key)
			if i < 0 {
				i += len(o)
			}
			if i < 0 || i >= len(o) {
				return nil
			}
			return o[i]
		}
	case string:
		switch toStr(key) {
		case "size":
			return len([]rune(o))
		}
		return nil
	case Drop:
		if v, ok := o.LiquidGet(toStr(key)); ok {
			return v
		}
		return nil
	}
	return nil
}

// firstPair / lastPair model Hash#first / values for the .first/.last access on
// maps; the gem returns a [key, value] pair in insertion order. Go maps have no
// order, so we use sorted keys for determinism (a documented divergence only for
// the rarely-used hash.first/last access).
func firstPair(m map[string]any) any {
	ks := sortedKeys(m)
	if len(ks) == 0 {
		return nil
	}
	return []any{ks[0], m[ks[0]]}
}

func lastPair(m map[string]any) any {
	ks := sortedKeys(m)
	if len(ks) == 0 {
		return nil
	}
	k := ks[len(ks)-1]
	return []any{k, m[k]}
}

func sortedKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// truthy implements Liquid truthiness: only nil and false are falsy.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	default:
		return true
	}
}

// isBlank reports whether v equals Liquid `blank`: nil, false, empty string,
// whitespace-only string, empty array, or empty map.
func isBlank(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case bool:
		return !x
	case string:
		return strings.TrimSpace(x) == ""
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	}
	return false
}

// isEmpty reports whether v equals Liquid `empty`: empty string/array/map.
func isEmpty(v any) bool {
	switch x := v.(type) {
	case string:
		return x == ""
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	}
	return false
}

// toStr renders a value to its Liquid string form (used by output and string
// filters). Numbers use Ruby-style formatting; arrays concatenate; maps inspect.
func toStr(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return formatFloat(x)
	case []any:
		var sb strings.Builder
		for _, e := range x {
			sb.WriteString(toStr(e))
		}
		return sb.String()
	case map[string]any:
		return inspectMap(x)
	}
	return ""
}

// formatFloat renders a float the way Ruby's Float#to_s does: an integral value
// keeps a trailing ".0", and the shortest round-tripping decimal is used.
func formatFloat(f float64) string {
	if math.IsInf(f, 1) {
		return "Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}
	if math.IsNaN(f) {
		return "NaN"
	}
	return rubyFloatString(f)
}

// rubyFloatString reproduces Ruby's Float#to_s: a fixed-point decimal for
// magnitudes in [1e-4, 1e16), and the "d.dddde±NN" exponent form (mantissa
// always carrying a decimal point, exponent at least two digits) outside it.
func rubyFloatString(f float64) string {
	if f == 0 {
		if math.Signbit(f) {
			return "-0.0"
		}
		return "0.0"
	}
	abs := math.Abs(f)
	if abs >= 1e16 || abs < 1e-4 {
		// Shortest mantissa in scientific form, then normalise to Ruby's shape.
		s := strconv.FormatFloat(f, 'e', -1, 64)
		return normalizeSci(s)
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// normalizeSci turns Go's "1e+06" / "1.5e-07" into Ruby's "1.0e+06" /
// "1.5e-07": the mantissa always has a decimal point and the exponent keeps its
// sign with at least two digits.
func normalizeSci(s string) string {
	i := strings.IndexAny(s, "eE")
	mant, exp := s[:i], s[i+1:]
	if !strings.Contains(mant, ".") {
		mant += ".0"
	}
	// Go's 'e' format always emits a sign and at least two exponent digits, which
	// is exactly Ruby's Float#to_s shape, so the exponent is used verbatim.
	return mant + "e" + exp
}

// inspectMap mimics Ruby Hash#inspect ({"a"=>1}) used when a hash is output.
func inspectMap(m map[string]any) string {
	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range sortedKeys(m) {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(rubyInspect(k))
		sb.WriteString("=>")
		sb.WriteString(rubyInspect(m[k]))
	}
	sb.WriteByte('}')
	return sb.String()
}

func rubyInspect(v any) string {
	switch x := v.(type) {
	case string:
		return strconv.Quote(x)
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = rubyInspect(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		return inspectMap(x)
	default:
		return toStr(v)
	}
}

// toInt coerces a value to an int for ranges and integer filters (Ruby to_i).
func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case bool:
		return 0
	case string:
		// Leading integer prefix, like Ruby String#to_i.
		i := 0
		neg := false
		if i < len(x) && (x[i] == '-' || x[i] == '+') {
			neg = x[i] == '-'
			i++
		}
		n := 0
		got := false
		for i < len(x) && x[i] >= '0' && x[i] <= '9' {
			n = n*10 + int(x[i]-'0')
			i++
			got = true
		}
		if !got {
			return 0
		}
		if neg {
			return -n
		}
		return n
	}
	return 0
}

// toFloat coerces a value to float64 (Ruby to_f) for math filters.
func toFloat(v any) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case float64:
		return x
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil {
			return f
		}
		return 0
	}
	return 0
}

// isFloat reports whether v is a float64 (math filters keep int vs float type).
func isFloat(v any) bool {
	_, ok := v.(float64)
	return ok
}

// toSlice coerces a value to an array for array filters and iteration. A map is
// iterated as its [key, value] pairs (sorted, for determinism); a scalar
// becomes a single-element slice; nil becomes empty.
func toSlice(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case nil, blankSentinel, emptySentinel:
		// nil and the empty/blank literals iterate as an empty collection (the
		// gem's `for i in empty` yields nothing).
		return nil
	case map[string]any:
		ks := sortedKeys(x)
		out := make([]any, len(ks))
		for i, k := range ks {
			out[i] = []any{k, x[k]}
		}
		return out
	default:
		return []any{v}
	}
}
