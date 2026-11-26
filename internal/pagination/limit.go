package pagination

import "fmt"

// LimitError preserves the domain validation sentinel while exposing safe,
// actionable pagination details to both transports.
type LimitError struct {
	Maximum             int
	Default             int
	NegativeUsesDefault bool
	Cause               error
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("limit must be at most %d (default %d)", e.Maximum, e.Default)
}
func (e *LimitError) Unwrap() error { return e.Cause }
func (e *LimitError) Details() map[string]any {
	return map[string]any{"field": "limit", "minimum": 1, "maximum": e.Maximum, "default": e.Default, "zero_uses_default": true, "negative_uses_default": e.NegativeUsesDefault}
}
