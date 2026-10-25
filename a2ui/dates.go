package a2ui

import (
	"strings"
	"time"
)

var dateLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
	"15:04:05",
	"15:04",
}

func parseDateTime(v any, e *Evaluator) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case float64:
		if t > 1e11 {
			return time.UnixMilli(int64(t)).UTC(), true
		}
		return time.Unix(int64(t), 0).UTC(), true
	case string:
		str := strings.TrimSpace(t)
		if str == "" {
			return time.Time{}, false
		}
		if strings.EqualFold(str, "now") {
			return e.Now(), true
		}
		for _, layout := range dateLayouts {
			if ts, err := time.Parse(layout, str); err == nil {
				return ts, true
			}
		}
	}
	return time.Time{}, false
}

// FormatDate renders t with a Unicode TR35 pattern (yyyy, MM, MMM, d, EEEE,
// HH, h, mm, ss, a and single-quoted literals).
func FormatDate(t time.Time, pattern string) string {
	var b strings.Builder
	runes := []rune(pattern)
	for i := 0; i < len(runes); {
		r := runes[i]
		if r == '\'' {
			if i+1 < len(runes) && runes[i+1] == '\'' {
				b.WriteRune('\'')
				i += 2
				continue
			}
			j := i + 1
			for j < len(runes) {
				if runes[j] == '\'' {
					if j+1 < len(runes) && runes[j+1] == '\'' {
						b.WriteRune('\'')
						j += 2
						continue
					}
					break
				}
				b.WriteRune(runes[j])
				j++
			}
			i = j + 1
			continue
		}
		if !isPatternLetter(r) {
			b.WriteRune(r)
			i++
			continue
		}
		j := i
		for j < len(runes) && runes[j] == r {
			j++
		}
		b.WriteString(dateToken(t, r, j-i))
		i = j
	}
	return b.String()
}

func isPatternLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func dateToken(t time.Time, r rune, n int) string {
	switch r {
	case 'y':
		if n == 2 {
			return t.Format("06")
		}
		return t.Format("2006")
	case 'M', 'L':
		switch {
		case n >= 4:
			return t.Format("January")
		case n == 3:
			return t.Format("Jan")
		case n == 2:
			return t.Format("01")
		}
		return t.Format("1")
	case 'd':
		if n >= 2 {
			return t.Format("02")
		}
		return t.Format("2")
	case 'E', 'c':
		if n >= 4 {
			return t.Format("Monday")
		}
		return t.Format("Mon")
	case 'H':
		if n >= 2 {
			return t.Format("15")
		}
		return FormatFloat(float64(t.Hour()))
	case 'h':
		if n >= 2 {
			return t.Format("03")
		}
		return t.Format("3")
	case 'm':
		if n >= 2 {
			return t.Format("04")
		}
		return FormatFloat(float64(t.Minute()))
	case 's':
		if n >= 2 {
			return t.Format("05")
		}
		return FormatFloat(float64(t.Second()))
	case 'a':
		return t.Format("PM")
	case 'S':
		return fracSeconds(t, n)
	case 'z', 'Z', 'X', 'x', 'O', 'v', 'V':
		return t.Format("MST")
	}
	return strings.Repeat(string(r), n)
}

func fracSeconds(t time.Time, n int) string {
	s := t.Format(".000000000")[1:]
	if n < len(s) {
		return s[:n]
	}
	return s
}
