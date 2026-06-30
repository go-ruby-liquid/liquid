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

// strftime renders the Ruby strftime directives the date filter commonly uses.
func strftime(t time.Time, format string) string {
	var sb strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 >= len(format) {
			sb.WriteByte(format[i])
			continue
		}
		i++
		switch format[i] {
		case 'Y':
			sb.WriteString(strconv.Itoa(t.Year()))
		case 'y':
			sb.WriteString(pad2(t.Year() % 100))
		case 'm':
			sb.WriteString(pad2(int(t.Month())))
		case 'd':
			sb.WriteString(pad2(t.Day()))
		case 'e':
			sb.WriteString(padSpace2(t.Day()))
		case 'H':
			sb.WriteString(pad2(t.Hour()))
		case 'I':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			sb.WriteString(pad2(h))
		case 'M':
			sb.WriteString(pad2(t.Minute()))
		case 'S':
			sb.WriteString(pad2(t.Second()))
		case 'p':
			if t.Hour() < 12 {
				sb.WriteString("AM")
			} else {
				sb.WriteString("PM")
			}
		case 'P':
			if t.Hour() < 12 {
				sb.WriteString("am")
			} else {
				sb.WriteString("pm")
			}
		case 'A':
			sb.WriteString(t.Weekday().String())
		case 'a':
			sb.WriteString(t.Weekday().String()[:3])
		case 'B':
			sb.WriteString(t.Month().String())
		case 'b', 'h':
			sb.WriteString(t.Month().String()[:3])
		case 'j':
			sb.WriteString(pad3(t.YearDay()))
		case 'w':
			sb.WriteString(strconv.Itoa(int(t.Weekday())))
		case 'Z':
			sb.WriteString(t.Format("MST"))
		case 'z':
			sb.WriteString(t.Format("-0700"))
		case '%':
			sb.WriteByte('%')
		default:
			sb.WriteByte('%')
			sb.WriteByte(format[i])
		}
	}
	return sb.String()
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func padSpace2(n int) string {
	if n < 10 {
		return " " + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func pad3(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}
