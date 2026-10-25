package a2ui

// ValidationResult is the structured outcome of a check condition.
type ValidationResult struct {
	Valid    bool   `json:"valid"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
	Severity string `json:"severity,omitempty"`
}

func invalid(msg string) ValidationResult {
	return ValidationResult{Valid: false, Message: msg, Severity: "error"}
}

// Checks evaluates a component's check rules in order.
func (e *Evaluator) Checks(c *Component, s *Scope) []ValidationResult {
	raw, ok := c.Prop("checks")
	if !ok {
		return nil
	}
	rules, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]ValidationResult, 0, len(rules))
	for _, r := range rules {
		rule, ok := r.(map[string]any)
		if !ok {
			continue
		}
		fallback, _ := rule["message"].(string)
		out = append(out, e.Check(rule["condition"], fallback, s))
	}
	return out
}

// Check evaluates one condition and coerces it to a ValidationResult using
// fallback as the message when the condition supplies none.
func (e *Evaluator) Check(condition any, fallback string, s *Scope) ValidationResult {
	v, err := e.Resolve(condition, s)
	if err != nil {
		return invalid(firstNonEmpty(fallback, err.Error()))
	}
	var res ValidationResult
	switch t := v.(type) {
	case ValidationResult:
		res = t
	case *ValidationResult:
		if t != nil {
			res = *t
		}
	case map[string]any:
		if valid, ok := t["valid"].(bool); ok {
			res.Valid = valid
			res.Code, _ = t["code"].(string)
			res.Message, _ = t["message"].(string)
			res.Severity, _ = t["severity"].(string)
		} else {
			res.Valid = ToBool(t)
		}
	default:
		res.Valid = ToBool(v)
	}
	if res.Valid {
		return ValidationResult{Valid: true}
	}
	res.Message = firstNonEmpty(res.Message, fallback, "Invalid value")
	res.Severity = firstNonEmpty(res.Severity, "error")
	return res
}

// Failures filters results down to the failed ones.
func Failures(results []ValidationResult) []ValidationResult {
	var out []ValidationResult
	for _, r := range results {
		if !r.Valid {
			out = append(out, r)
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
