package a2ui

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
)

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// BasicFunctions returns the functions of the A2UI basic catalog plus the
// universal @index function.
func BasicFunctions() Functions {
	return Functions{
		"required":       fnRequired,
		"regex":          fnRegex,
		"length":         fnLength,
		"numeric":        fnNumeric,
		"email":          fnEmail,
		"formatString":   fnFormatString,
		"formatNumber":   fnFormatNumber,
		"formatCurrency": fnFormatCurrency,
		"formatDate":     fnFormatDate,
		"pluralize":      fnPluralize,
		"openUrl":        fnOpenURL,
		"and":            fnAnd,
		"or":             fnOr,
		"not":            fnNot,
		"@index":         fnIndex,
	}
}

func fnRequired(e *Evaluator, args map[string]any, s *Scope) (any, error) {
	v, err := e.Arg(args, "value", s)
	if err != nil {
		return nil, err
	}
	switch t := v.(type) {
	case nil:
		return invalid(""), nil
	case string:
		if t == "" {
			return invalid(""), nil
		}
	case []any:
		if len(t) == 0 {
			return invalid(""), nil
		}
	}
	return ValidationResult{Valid: true}, nil
}

func fnRegex(e *Evaluator, args map[string]any, s *Scope) (any, error) {
	pattern, err := e.Arg(args, "pattern", s)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(ToString(pattern))
	if err != nil {
		return nil, fmt.Errorf("a2ui: regex: %w", err)
	}
	v, err := e.Arg(args, "value", s)
	if err != nil {
		return nil, err
	}
	if re.MatchString(ToString(v)) {
		return ValidationResult{Valid: true}, nil
	}
	return invalid(""), nil
}

func fnLength(e *Evaluator, args map[string]any, s *Scope) (any, error) {
	v, err := e.Arg(args, "value", s)
	if err != nil {
		return nil, err
	}
	n := float64(len([]rune(ToString(v))))
	if list, ok := v.([]any); ok {
		n = float64(len(list))
	}
	return rangeResult(e, args, s, n)
}

func fnNumeric(e *Evaluator, args map[string]any, s *Scope) (any, error) {
	v, err := e.Arg(args, "value", s)
	if err != nil {
		return nil, err
	}
	n, ok := ToNumber(v)
	if !ok {
		return invalid(""), nil
	}
	return rangeResult(e, args, s, n)
}

func rangeResult(e *Evaluator, args map[string]any, s *Scope, n float64) (any, error) {
	if minV, ok := e.Number(args["min"], s); ok && args["min"] != nil && n < minV {
		return invalid(""), nil
	}
	if maxV, ok := e.Number(args["max"], s); ok && args["max"] != nil && n > maxV {
		return invalid(""), nil
	}
	return ValidationResult{Valid: true}, nil
}

func fnEmail(e *Evaluator, args map[string]any, s *Scope) (any, error) {
	v, err := e.Arg(args, "value", s)
	if err != nil {
		return nil, err
	}
	if emailPattern.MatchString(ToString(v)) {
		return ValidationResult{Valid: true}, nil
	}
	return invalid(""), nil
}

func fnFormatString(e *Evaluator, args map[string]any, s *Scope) (any, error) {
	raw, ok := args["value"]
	if !ok {
		return "", nil
	}
	template, ok := raw.(string)
	if !ok {
		return e.String(raw, s), nil
	}
	return FormatString(e, template, s)
}

func fnFormatNumber(e *Evaluator, args map[string]any, s *Scope) (any, error) {
	v, err := e.Arg(args, "value", s)
	if err != nil {
		return nil, err
	}
	n, ok := ToNumber(v)
	if !ok {
		return ToString(v), nil
	}
	return formatNumber(n, decimalsArg(e, args, s, n), groupingArg(e, args, s)), nil
}

func fnFormatCurrency(e *Evaluator, args map[string]any, s *Scope) (any, error) {
	v, err := e.Arg(args, "value", s)
	if err != nil {
		return nil, err
	}
	n, ok := ToNumber(v)
	if !ok {
		return ToString(v), nil
	}
	code := strings.ToUpper(e.String(args["currency"], s))
	decimals := 2
	if _, zero := zeroDecimalCurrencies[code]; zero {
		decimals = 0
	}
	if d, ok := e.Number(args["decimals"], s); ok && args["decimals"] != nil {
		decimals = int(d)
	}
	body := formatNumber(math.Abs(n), decimals, groupingArg(e, args, s))
	sign := ""
	if n < 0 {
		sign = "-"
	}
	if sym, ok := currencySymbols[code]; ok {
		return sign + sym + body, nil
	}
	if code == "" {
		return sign + body, nil
	}
	return sign + code + " " + body, nil
}

var currencySymbols = map[string]string{
	"USD": "$", "EUR": "€", "GBP": "£", "JPY": "¥", "CNY": "¥", "INR": "₹",
	"KRW": "₩", "AUD": "A$", "CAD": "C$", "NZD": "NZ$", "HKD": "HK$", "SGD": "S$",
	"CHF": "CHF ", "BRL": "R$", "MXN": "MX$", "ILS": "₪", "NGN": "₦", "PHP": "₱",
	"VND": "₫", "UAH": "₴", "TRY": "₺", "RUB": "₽", "PLN": "zł", "SEK": "kr", "NOK": "kr", "DKK": "kr",
}

var zeroDecimalCurrencies = map[string]struct{}{"JPY": {}, "KRW": {}, "VND": {}, "CLP": {}, "ISK": {}, "HUF": {}}

func decimalsArg(e *Evaluator, args map[string]any, s *Scope, n float64) int {
	if d, ok := e.Number(args["decimals"], s); ok && args["decimals"] != nil {
		return int(d)
	}
	if n == math.Trunc(n) {
		return 0
	}
	return 2
}

func groupingArg(e *Evaluator, args map[string]any, s *Scope) bool {
	if v, ok := args["grouping"]; ok {
		return e.Bool(v, s)
	}
	return true
}

func formatNumber(n float64, decimals int, grouping bool) string {
	if decimals < 0 {
		decimals = 0
	}
	str := fmt.Sprintf("%.*f", decimals, n)
	sign := ""
	if strings.HasPrefix(str, "-") {
		sign, str = "-", str[1:]
	}
	intPart, frac := str, ""
	if i := strings.IndexByte(str, '.'); i >= 0 {
		intPart, frac = str[:i], str[i:]
	}
	if grouping && len(intPart) > 3 {
		var b strings.Builder
		for i, r := range intPart {
			if i > 0 && (len(intPart)-i)%3 == 0 {
				b.WriteByte(',')
			}
			b.WriteRune(r)
		}
		intPart = b.String()
	}
	return sign + intPart + frac
}

func fnFormatDate(e *Evaluator, args map[string]any, s *Scope) (any, error) {
	v, err := e.Arg(args, "value", s)
	if err != nil {
		return nil, err
	}
	t, ok := parseDateTime(v, e)
	if !ok {
		return ToString(v), nil
	}
	return FormatDate(t, e.String(args["format"], s)), nil
}

func fnPluralize(e *Evaluator, args map[string]any, s *Scope) (any, error) {
	n, _ := e.Number(args["value"], s)
	pick := func(key string) (string, bool) {
		if v, ok := args[key]; ok {
			return e.String(v, s), true
		}
		return "", false
	}
	switch {
	case n == 0:
		if str, ok := pick("zero"); ok {
			return str, nil
		}
	case n == 1:
		if str, ok := pick("one"); ok {
			return str, nil
		}
	case n == 2:
		if str, ok := pick("two"); ok {
			return str, nil
		}
	}
	other, _ := pick("other")
	return other, nil
}

func fnOpenURL(e *Evaluator, args map[string]any, s *Scope) (any, error) {
	raw := e.String(args["url"], s)
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("a2ui: openUrl: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("a2ui: openUrl: scheme %q not allowed", u.Scheme)
	}
	if e.OpenURL == nil {
		return nil, nil
	}
	return nil, e.OpenURL(raw)
}

func boolValues(e *Evaluator, args map[string]any, s *Scope) ([]any, error) {
	raw, ok := args["values"]
	if !ok {
		return nil, errors.New("a2ui: values argument required")
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, errors.New("a2ui: values must be an array")
	}
	return list, nil
}

func fnAnd(e *Evaluator, args map[string]any, s *Scope) (any, error) {
	list, err := boolValues(e, args, s)
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		if !e.Bool(v, s) {
			return false, nil
		}
	}
	return true, nil
}

func fnOr(e *Evaluator, args map[string]any, s *Scope) (any, error) {
	list, err := boolValues(e, args, s)
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		if e.Bool(v, s) {
			return true, nil
		}
	}
	return false, nil
}

func fnNot(e *Evaluator, args map[string]any, s *Scope) (any, error) {
	return !e.Bool(args["value"], s), nil
}

func fnIndex(e *Evaluator, args map[string]any, s *Scope) (any, error) {
	i, ok := s.ItemIndex()
	if !ok {
		return nil, errors.New("a2ui: @index used outside a template")
	}
	offset, _ := e.Number(args["offset"], s)
	return float64(i) + offset, nil
}
