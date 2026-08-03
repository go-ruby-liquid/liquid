// Copyright (c) the go-ruby-liquid/liquid authors
//
// SPDX-License-Identifier: BSD-3-Clause

package liquid

import (
	"strconv"
	"strings"
	"time"
)

// nowFunc is the clock used by the date filter for "now"/"today"; tests override
// it for determinism.
var nowFunc = time.Now

// dateFilter formats a value with a Ruby strftime pattern, like the gem's date
// filter. It accepts a time, a unix integer, the strings "now"/"today", and the
// common ISO/date string forms.
func dateFilter(in any, args []any) (any, *Error) {
	if isBlank(in) && in != 0 {
		// nil/"" with no parseable date renders empty (gem returns input).
		if _, ok := in.(string); !ok {
			return in, nil
		}
	}
	t, ok := toTime(in)
	if !ok {
		return in, nil
	}
	format := arg0Str(args)
	if format == "" {
		return in, nil
	}
	return strftime(t, format), nil
}

// toTime coerces a value to time.Time for the date filter.
func toTime(in any) (time.Time, bool) {
	switch x := in.(type) {
	case time.Time:
		return x, true
	case int:
		return time.Unix(int64(x), 0).UTC(), true
	case int64:
		return time.Unix(x, 0).UTC(), true
	case string:
		s := strings.TrimSpace(x)
		switch strings.ToLower(s) {
		case "now", "today":
			return nowFunc(), true
		}
		return parseDateString(s)
	}
	return time.Time{}, false
}

// parseDateString tries a series of common layouts the gem (via Ruby's
// Time.parse) accepts.
func parseDateString(s string) (time.Time, bool) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006/01/02",
		"01/02/2006",
		time.RFC1123Z,
		time.RFC1123,
		"Mon Jan 2 15:04:05 2006",
		"January 2, 2006",
		"Jan 2, 2006",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	// Bare integer string -> unix seconds.
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(n, 0).UTC(), true
	}
	return time.Time{}, false
}

// strftime renders the Ruby strftime directives the date filter commonly uses,
// including the glibc padding flags a conversion may carry: `-` (no padding),
// `_` (space padding), `0` (zero padding) and `^` (upcase). Jekyll's minima
// theme, for example, formats post dates with "%b %-d, %Y".
func strftime(t time.Time, format string) string {
	var sb strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			sb.WriteByte(format[i])
			continue
		}
		// Consume optional flags (-, _, 0, ^, #) between the % and the directive.
		j := i + 1
		var flag byte
		for j < len(format) {
			switch format[j] {
			case '-', '_', '0', '^', '#':
				flag = format[j]
				j++
				continue
			}
			break
		}
		if j >= len(format) {
			// A trailing % (with any flags) is emitted literally.
			sb.WriteString(format[i:])
			break
		}
		sb.WriteString(strftimeDirective(t, format[j], flag))
		i = j
	}
	return sb.String()
}

// strftimeDirective renders one directive character honoring a leading padding
// flag. Numeric directives respect -/_/0; textual directives respect ^ (upcase).
func strftimeDirective(t time.Time, d, flag byte) string {
	switch d {
	case 'Y':
		return padNum(t.Year(), 0, flag)
	case 'y':
		return padNum(t.Year()%100, 2, flag)
	case 'm':
		return padNum(int(t.Month()), 2, flag)
	case 'd':
		return padNum(t.Day(), 2, flag)
	case 'e':
		// %e is space-padded day; equivalent to %_d.
		return padNum(t.Day(), 2, '_')
	case 'H':
		return padNum(t.Hour(), 2, flag)
	case 'I':
		h := t.Hour() % 12
		if h == 0 {
			h = 12
		}
		return padNum(h, 2, flag)
	case 'M':
		return padNum(t.Minute(), 2, flag)
	case 'S':
		return padNum(t.Second(), 2, flag)
	case 'j':
		return padNum(t.YearDay(), 3, flag)
	case 'w':
		return padNum(int(t.Weekday()), 0, flag)
	case 'p':
		return upcaseFlag(ampm(t, "AM", "PM"), flag)
	case 'P':
		return upcaseFlag(ampm(t, "am", "pm"), flag)
	case 'A':
		return upcaseFlag(t.Weekday().String(), flag)
	case 'a':
		return upcaseFlag(t.Weekday().String()[:3], flag)
	case 'B':
		return upcaseFlag(t.Month().String(), flag)
	case 'b', 'h':
		return upcaseFlag(t.Month().String()[:3], flag)
	case 'Z':
		return upcaseFlag(t.Format("MST"), flag)
	case 'z':
		return t.Format("-0700")
	case '%':
		return "%"
	default:
		if flag != 0 {
			return "%" + string(flag) + string(d)
		}
		return "%" + string(d)
	}
}

func ampm(t time.Time, am, pm string) string {
	if t.Hour() < 12 {
		return am
	}
	return pm
}

func upcaseFlag(s string, flag byte) string {
	if flag == '^' {
		return strings.ToUpper(s)
	}
	return s
}

// padNum renders v in the requested minimum width using the padding implied by
// flag: '-' none, '_' spaces, '0' or default zeros. A zero width never pads.
func padNum(v, width int, flag byte) string {
	s := strconv.Itoa(v)
	if flag == '-' {
		return s
	}
	fill := byte('0')
	if flag == '_' {
		fill = ' '
	}
	for len(s) < width {
		s = string(fill) + s
	}
	return s
}
