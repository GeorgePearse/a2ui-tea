package a2ui

import (
	"fmt"
	"strconv"
	"strings"
)

// ParsePointer splits an RFC 6901 JSON Pointer into unescaped tokens. The
// empty pointer and "/" both address the whole document.
func ParsePointer(p string) ([]string, error) {
	if p == "" || p == "/" {
		return nil, nil
	}
	if p[0] != '/' {
		return nil, fmt.Errorf("a2ui: pointer %q must start with /", p)
	}
	parts := strings.Split(p[1:], "/")
	for i, t := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(t, "~1", "/"), "~0", "~")
	}
	return parts, nil
}

func pointerGet(root any, tokens []string) (any, bool) {
	cur := root
	for _, t := range tokens {
		switch c := cur.(type) {
		case map[string]any:
			v, ok := c[t]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(t)
			if err != nil || i < 0 || i >= len(c) {
				return nil, false
			}
			cur = c[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

// pointerSet returns root with v stored at tokens, creating intermediate
// objects as needed. Arrays accept an existing index, the next index or "-"
// to append.
func pointerSet(root any, tokens []string, v any) (any, error) {
	if len(tokens) == 0 {
		return v, nil
	}
	t, rest := tokens[0], tokens[1:]
	switch c := root.(type) {
	case nil:
		child, err := pointerSet(nil, rest, v)
		if err != nil {
			return nil, err
		}
		return map[string]any{t: child}, nil
	case map[string]any:
		child, err := pointerSet(c[t], rest, v)
		if err != nil {
			return nil, err
		}
		c[t] = child
		return c, nil
	case []any:
		i := len(c)
		if t != "-" {
			n, err := strconv.Atoi(t)
			if err != nil || n < 0 || n > len(c) {
				return nil, fmt.Errorf("a2ui: array index %q out of range (len %d)", t, len(c))
			}
			i = n
		}
		if i == len(c) {
			child, err := pointerSet(nil, rest, v)
			if err != nil {
				return nil, err
			}
			return append(c, child), nil
		}
		child, err := pointerSet(c[i], rest, v)
		if err != nil {
			return nil, err
		}
		c[i] = child
		return c, nil
	default:
		return nil, fmt.Errorf("a2ui: cannot set %q inside a %T", t, root)
	}
}

func pointerDelete(root any, tokens []string) any {
	if len(tokens) == 0 {
		return nil
	}
	t, rest := tokens[0], tokens[1:]
	switch c := root.(type) {
	case map[string]any:
		if len(rest) == 0 {
			delete(c, t)
		} else if child, ok := c[t]; ok {
			c[t] = pointerDelete(child, rest)
		}
		return c
	case []any:
		i, err := strconv.Atoi(t)
		if err != nil || i < 0 || i >= len(c) {
			return c
		}
		if len(rest) == 0 {
			return append(c[:i:i], c[i+1:]...)
		}
		c[i] = pointerDelete(c[i], rest)
		return c
	}
	return root
}

// JoinPointer appends a relative path to an absolute pointer.
func JoinPointer(base, rel string) string {
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return base
	}
	return base + "/" + rel
}
