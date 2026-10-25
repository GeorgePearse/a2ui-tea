package a2ui

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"
)

// Func is a renderer-side catalog function. It receives its raw, unresolved
// arguments so it can resolve them lazily (and short-circuit) through ev.
type Func func(ev *Evaluator, args map[string]any, s *Scope) (any, error)

// Functions is a registry of catalog functions by name.
type Functions map[string]Func

// Evaluator resolves dynamic values (literals, data bindings and function
// calls) against a scope.
type Evaluator struct {
	Funcs Functions
	// Now supplies the current time to date functions; defaults to time.Now.
	Now func() time.Time
	// OpenURL is called by the openUrl function; nil makes it a no-op.
	OpenURL func(url string) error
}

// NewEvaluator returns an evaluator with the basic catalog functions.
func NewEvaluator() *Evaluator {
	return &Evaluator{Funcs: BasicFunctions(), Now: time.Now}
}

// Register adds or replaces a function.
func (e *Evaluator) Register(name string, fn Func) {
	if e.Funcs == nil {
		e.Funcs = Functions{}
	}
	e.Funcs[name] = fn
}

// IsBinding reports whether v is a {"@path": ...} data binding and returns
// the path. The v0.9 spelling {"path": ...} is accepted when it is the only
// key.
func IsBinding(v any) (string, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return "", false
	}
	if p, ok := m["@path"].(string); ok {
		return p, true
	}
	if p, ok := m["path"].(string); ok && len(m) == 1 {
		return p, true
	}
	return "", false
}

// IsCall reports whether v is a {"@call": ..., "args": {...}} function call.
// The v0.9 spelling {"call": ...} is accepted.
func IsCall(v any) (name string, args map[string]any, ok bool) {
	m, isMap := v.(map[string]any)
	if !isMap {
		return "", nil, false
	}
	name, ok = m["@call"].(string)
	if !ok {
		name, ok = m["call"].(string)
		if !ok {
			return "", nil, false
		}
	}
	args, _ = m["args"].(map[string]any)
	if args == nil {
		args = map[string]any{}
	}
	return name, args, true
}

// Resolve evaluates a dynamic value. Objects and arrays that are not
// bindings or calls are resolved element-wise so nested bindings work.
func (e *Evaluator) Resolve(v any, s *Scope) (any, error) {
	switch t := v.(type) {
	case map[string]any:
		if p, ok := IsBinding(t); ok {
			val, _ := s.Get(p)
			return val, nil
		}
		if name, args, ok := IsCall(t); ok {
			return e.Call(name, args, s)
		}
		out := make(map[string]any, len(t))
		for k, child := range t {
			r, err := e.Resolve(child, s)
			if err != nil {
				return nil, err
			}
			out[k] = r
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, child := range t {
			r, err := e.Resolve(child, s)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	}
	return v, nil
}

// Call invokes a registered function.
func (e *Evaluator) Call(name string, args map[string]any, s *Scope) (any, error) {
	fn, ok := e.Funcs[name]
	if !ok {
		return nil, fmt.Errorf("a2ui: unknown function %q", name)
	}
	return fn(e, args, s)
}

// Arg resolves one argument.
func (e *Evaluator) Arg(args map[string]any, name string, s *Scope) (any, error) {
	v, ok := args[name]
	if !ok {
		return nil, nil
	}
	return e.Resolve(v, s)
}

// String resolves v and converts it to text; errors become empty strings.
func (e *Evaluator) String(v any, s *Scope) string {
	r, err := e.Resolve(v, s)
	if err != nil {
		return ""
	}
	return ToString(r)
}

// Bool resolves v and coerces it to a boolean.
func (e *Evaluator) Bool(v any, s *Scope) bool {
	r, err := e.Resolve(v, s)
	if err != nil {
		return false
	}
	return ToBool(r)
}

// Number resolves v and coerces it to a number.
func (e *Evaluator) Number(v any, s *Scope) (float64, bool) {
	r, err := e.Resolve(v, s)
	if err != nil {
		return 0, false
	}
	return ToNumber(r)
}

// Strings resolves v to a list of strings (a DynamicStringList).
func (e *Evaluator) Strings(v any, s *Scope) []string {
	r, err := e.Resolve(v, s)
	if err != nil {
		return nil
	}
	return ToStrings(r)
}

// ToString converts a resolved value to text following the protocol's type
// conversion rules: nil is "", numbers and booleans print plainly, objects and
// arrays are JSON.
func ToString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return FormatFloat(t)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return t.String()
	case ValidationResult:
		b, _ := json.Marshal(t)
		return string(b)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

// FormatFloat prints a number without a trailing ".0" for integers.
func FormatFloat(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return fmt.Sprint(f)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// ToBool coerces a resolved value: validation results use their valid flag,
// strings are true when non-empty (except "false"), numbers when non-zero.
func ToBool(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case ValidationResult:
		return t.Valid
	case *ValidationResult:
		return t != nil && t.Valid
	case string:
		return t != "" && t != "false"
	case float64:
		return t != 0
	case int:
		return t != 0
	case map[string]any:
		if valid, ok := t["valid"].(bool); ok {
			return valid
		}
		return len(t) > 0
	case []any:
		return len(t) > 0
	}
	return true
}

// ToNumber coerces a resolved value to a float64.
func ToNumber(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case bool:
		if t {
			return 1, true
		}
		return 0, true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		return f, err == nil
	}
	return 0, false
}

// ToStrings coerces a resolved value to a string list; a lone string becomes
// a one-element list.
func ToStrings(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			out = append(out, ToString(e))
		}
		return out
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	}
	return []string{ToString(v)}
}
