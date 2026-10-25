package a2ui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// FormatString interpolates ${...} expressions in template: absolute or
// relative data paths, quoted/number/boolean literals and function calls
// with named arguments, which may themselves contain ${...} expressions.
// "\${" yields a literal "${".
func FormatString(e *Evaluator, template string, s *Scope) (string, error) {
	var b strings.Builder
	for i := 0; i < len(template); {
		if strings.HasPrefix(template[i:], `\${`) {
			b.WriteString("${")
			i += 3
			continue
		}
		if !strings.HasPrefix(template[i:], "${") {
			b.WriteByte(template[i])
			i++
			continue
		}
		end, err := matchBrace(template, i+2)
		if err != nil {
			return "", err
		}
		expr, err := parseExpression(strings.TrimSpace(template[i+2 : end]))
		if err != nil {
			return "", err
		}
		v, err := e.Resolve(expr, s)
		if err != nil {
			return "", err
		}
		b.WriteString(ToString(v))
		i = end + 1
	}
	return b.String(), nil
}

// matchBrace returns the index of the "}" closing an expression that starts
// at from, skipping nested ${...} and quoted strings.
func matchBrace(src string, from int) (int, error) {
	depth := 1
	var quote byte
	for i := from; i < len(src); i++ {
		c := src[i]
		switch {
		case quote != 0:
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '$' && i+1 < len(src) && src[i+1] == '{':
			depth++
			i++
		case c == '}':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, errors.New("a2ui: formatString: unterminated ${ expression")
}

// parseExpression converts an interpolation expression into the same JSON
// shapes the evaluator resolves: literals, {"@path": ...} and {"@call": ...}.
func parseExpression(expr string) (any, error) {
	if expr == "" {
		return "", nil
	}
	if strings.HasPrefix(expr, "${") && strings.HasSuffix(expr, "}") {
		return parseExpression(strings.TrimSpace(expr[2 : len(expr)-1]))
	}
	if lit, ok := parseLiteral(expr); ok {
		return lit, nil
	}
	if open := strings.IndexByte(expr, '('); open > 0 && strings.HasSuffix(expr, ")") {
		name := strings.TrimSpace(expr[:open])
		if isIdentifier(name) {
			args, err := parseArgs(expr[open+1 : len(expr)-1])
			if err != nil {
				return nil, err
			}
			return map[string]any{"@call": name, "args": args}, nil
		}
	}
	return map[string]any{"@path": expr}, nil
}

func parseLiteral(expr string) (any, bool) {
	if len(expr) >= 2 && (expr[0] == '\'' || expr[0] == '"') && expr[len(expr)-1] == expr[0] {
		return unescape(expr[1 : len(expr)-1]), true
	}
	switch expr {
	case "true":
		return true, true
	case "false":
		return false, true
	case "null":
		return nil, true
	}
	if f, err := strconv.ParseFloat(expr, 64); err == nil {
		return f, true
	}
	return nil, false
}

func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_' || r == '@' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

func parseArgs(src string) (map[string]any, error) {
	args := map[string]any{}
	for _, part := range splitTopLevel(src, ',') {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		colon := strings.IndexByte(part, ':')
		if colon <= 0 {
			return nil, fmt.Errorf("a2ui: formatString: argument %q must be name: value", part)
		}
		name := strings.TrimSpace(part[:colon])
		value, err := parseExpression(strings.TrimSpace(part[colon+1:]))
		if err != nil {
			return nil, err
		}
		args[name] = value
	}
	return args, nil
}

// splitTopLevel splits on sep outside quotes, parentheses and ${...}.
func splitTopLevel(src string, sep byte) []string {
	var parts []string
	depth := 0
	var quote byte
	start := 0
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case quote != 0:
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(' || c == '{':
			depth++
		case c == ')' || c == '}':
			depth--
		case c == sep && depth == 0:
			parts = append(parts, src[start:i])
			start = i + 1
		}
	}
	return append(parts, src[start:])
}
