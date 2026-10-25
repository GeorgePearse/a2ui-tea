package a2ui

import (
	"testing"
	"time"
)

func testScope(data map[string]any) *Scope {
	d := NewDataModel()
	_ = d.Set("/", data)
	return RootScope(d)
}

func TestResolve(t *testing.T) {
	e := NewEvaluator()
	s := testScope(map[string]any{"user": map[string]any{"name": "Ada"}, "n": 2.0, "ok": true})
	cases := []struct {
		in   any
		want string
	}{
		{"literal", "literal"},
		{map[string]any{"@path": "/user/name"}, "Ada"},
		{map[string]any{"path": "/user/name"}, "Ada"},
		{map[string]any{"@path": "/missing"}, ""},
		{2.0, "2"},
		{0.45, "0.45"},
		{true, "true"},
		{nil, ""},
		{map[string]any{"@call": "formatString", "args": map[string]any{"value": "Hi ${/user/name}"}}, "Hi Ada"},
		{map[string]any{"@call": "not", "args": map[string]any{"value": map[string]any{"@path": "/ok"}}}, "false"},
		{[]any{"a", map[string]any{"@path": "/n"}}, `["a",2]`},
		{map[string]any{"k": map[string]any{"@path": "/n"}}, `{"k":2}`},
	}
	for _, c := range cases {
		if got := e.String(c.in, s); got != c.want {
			t.Errorf("String(%v) = %q, want %q", c.in, got, c.want)
		}
	}
	if _, err := e.Resolve(map[string]any{"@call": "nope"}, s); err == nil {
		t.Error("unknown function must error")
	}
}

func TestFormatString(t *testing.T) {
	e := NewEvaluator()
	s := testScope(map[string]any{
		"user":  map[string]any{"firstName": "Jane"},
		"count": 3.0,
		"date":  "2026-01-16T14:30:00Z",
		"items": []any{map[string]any{"name": "A"}, map[string]any{"name": "B"}},
	})
	cases := map[string]string{
		"Hello, ${/user/firstName}!":                                  "Hello, Jane!",
		`Price: \${/not/a/path}`:                                      "Price: ${/not/a/path}",
		"${formatDate(value: ${/date}, format: 'MMM dd, yyyy')}":      "Jan 16, 2026",
		"${formatDate(value: ${/date}, format: 'EEEE, d MMMM')}":      "Friday, 16 January",
		"${formatDate(value: ${/date}, format: 'h:mm a')}":            "2:30 PM",
		"${formatDate(value: ${/date}, format: 'HH:mm')}":             "14:30",
		"${formatNumber(value: 1234567.891, decimals: 1)}":            "1,234,567.9",
		"${formatNumber(value: ${/count}, grouping: false)}":          "3",
		"${pluralize(value: ${/count}, one: 'item', other: 'items')}": "items",
		"${pluralize(value: 1, one: 'item', other: 'items')}":         "item",
		"${formatCurrency(value: 1500, currency: 'USD')}":             "$1,500.00",
		"${formatCurrency(value: -42.5, currency: 'EUR')}":            "-€42.50",
		"${formatCurrency(value: 1200, currency: 'JPY')}":             "¥1,200",
		"plain":                     "plain",
		"${'quoted'} ${42} ${true}": "quoted 42 true",
		"${not(value: ${required(value: ${/missing})})}":                              "true",
		"${formatString(value: 'nested ${/user/firstName}')}":                         "nested Jane",
		"${formatNumber(value: ${formatNumber(value: 5, decimals: 0)}, decimals: 2)}": "5.00",
	}
	for in, want := range cases {
		got, err := FormatString(e, in, s)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("%q = %q, want %q", in, got, want)
		}
	}
	if _, err := FormatString(e, "${unterminated", s); err == nil {
		t.Error("unterminated expression must fail")
	}
	item := s.Item("/items/1", 1)
	got, err := FormatString(e, "#${@index(offset: 1)} ${name}", item)
	if err != nil || got != "#2 B" {
		t.Errorf("template scope: %q %v", got, err)
	}
	if _, err := FormatString(e, "${@index()}", s); err == nil {
		t.Error("@index outside a template must fail")
	}
}

func TestFormatDate(t *testing.T) {
	ts := time.Date(2026, time.March, 5, 9, 7, 3, 0, time.UTC)
	cases := map[string]string{
		"yyyy-MM-dd":   "2026-03-05",
		"yy/M/d":       "26/3/5",
		"EEEE E":       "Thursday Thu",
		"H:m:s":        "9:7:3",
		"hh:mm:ss a":   "09:07:03 AM",
		"'at' HH:mm":   "at 09:07",
		"MMMM yyyy":    "March 2026",
		"d 'o''clock'": "5 o'clock",
	}
	for p, want := range cases {
		if got := FormatDate(ts, p); got != want {
			t.Errorf("FormatDate(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestChecks(t *testing.T) {
	e := NewEvaluator()
	s := testScope(map[string]any{"form": map[string]any{"email": "", "phone": "123", "agree": false, "zip": "1234"}})
	c := &Component{ID: "f", Type: "TextField", Props: map[string]any{
		"checks": []any{
			map[string]any{"condition": map[string]any{"@call": "required", "args": map[string]any{"value": map[string]any{"@path": "/form/email"}}}, "message": "Email required"},
			map[string]any{"condition": map[string]any{"@call": "regex", "args": map[string]any{"value": map[string]any{"@path": "/form/zip"}, "pattern": "^[0-9]{5}$"}}, "message": "5 digits"},
			map[string]any{"condition": map[string]any{"@call": "email", "args": map[string]any{"value": "a@b.co"}}},
			map[string]any{"condition": map[string]any{"@path": "/form/agree"}, "message": "Agree first"},
			map[string]any{"condition": map[string]any{"@call": "length", "args": map[string]any{"value": map[string]any{"@path": "/form/phone"}, "min": 2.0, "max": 5.0}}},
			map[string]any{"condition": map[string]any{"@call": "numeric", "args": map[string]any{"value": map[string]any{"@path": "/form/zip"}, "min": 0.0, "max": 100.0}}, "message": "too big"},
		},
	}}
	results := e.Checks(c, s)
	if len(results) != 6 {
		t.Fatalf("got %d results", len(results))
	}
	wantValid := []bool{false, false, true, false, true, false}
	wantMsg := []string{"Email required", "5 digits", "", "Agree first", "", "too big"}
	for i, r := range results {
		if r.Valid != wantValid[i] || r.Message != wantMsg[i] {
			t.Errorf("check %d = %+v", i, r)
		}
		if !r.Valid && r.Severity != "error" {
			t.Errorf("check %d severity = %q", i, r.Severity)
		}
	}
	if len(Failures(results)) != 4 {
		t.Errorf("failures = %d", len(Failures(results)))
	}

	combo := map[string]any{"@call": "and", "args": map[string]any{"values": []any{
		map[string]any{"@path": "/form/agree"},
		map[string]any{"@call": "or", "args": map[string]any{"values": []any{
			map[string]any{"@call": "required", "args": map[string]any{"value": map[string]any{"@path": "/form/email"}}},
			map[string]any{"@call": "required", "args": map[string]any{"value": map[string]any{"@path": "/form/phone"}}},
		}}},
	}}}
	if r := e.Check(combo, "combo", s); r.Valid || r.Message != "combo" {
		t.Errorf("and/or: %+v", r)
	}
	_ = s.Set("/form/agree", true)
	if r := e.Check(combo, "combo", s); !r.Valid {
		t.Errorf("and/or after agree: %+v", r)
	}
	if r := e.Check(map[string]any{"valid": false, "message": "custom", "severity": "warning"}, "fallback", s); r.Message != "custom" || r.Severity != "warning" {
		t.Errorf("validation result passthrough: %+v", r)
	}
}

func TestOpenURL(t *testing.T) {
	e := NewEvaluator()
	var opened string
	e.OpenURL = func(u string) error { opened = u; return nil }
	s := testScope(nil)
	if _, err := e.Call("openUrl", map[string]any{"url": "https://a2ui.org"}, s); err != nil || opened != "https://a2ui.org" {
		t.Errorf("openUrl: %v %q", err, opened)
	}
	if _, err := e.Call("openUrl", map[string]any{"url": "javascript:alert(1)"}, s); err == nil {
		t.Error("non-http scheme must be rejected")
	}
}
