// Package validate runs deterministic, offline (LLM-free) semantic validation of
// the normalized invoice against EN16931 business rules and the Peppol BIS Billing
// 3.0 CIUS. Determinism is a hard requirement (fr-catalog FR-B01/B02): the same
// document must always yield the same rule outcomes so results are audit-reproducible.
package validate

// Severity follows EN16931 / Peppol: a fatal rule blocks the document, a warning
// is recorded but does not block (mirrors ZATCA "CLEARED with WARNINGS").
type Severity string

const (
	Fatal   Severity = "fatal"
	Warning Severity = "warning"
)

// RuleError is one validation finding. The shape matches the public API contract
// (api.md): {rule_id, path, message_en, message_ar}. message_ar is mandatory because
// Arabic error messages are a ZATCA requirement and an i18n NFR.
type RuleError struct {
	RuleID    string   `json:"rule_id"`
	Path      string   `json:"path"`
	MessageEN string   `json:"message_en"`
	MessageAR string   `json:"message_ar"`
	Severity  Severity `json:"severity"`
	// FixEN/FixAR: the "how to fix it" guidance (fr-catalog: rule_id -> meaning(ar/en) -> fix).
	// This is a deliberate differentiator: competitors return cryptic rule failures; we tell
	// the user exactly what to change. Optional (omitted when the message is self-explanatory).
	FixEN string `json:"fix_en,omitempty"`
	FixAR string `json:"fix_ar,omitempty"`
}

// Report is the full validation outcome.
type Report struct {
	Valid  bool        `json:"valid"`  // true only when there are zero fatal errors
	Errors []RuleError `json:"errors"` // fatal + warning findings, in deterministic rule order
}

// hasFatal reports whether any finding is fatal.
func hasFatal(errs []RuleError) bool {
	for _, e := range errs {
		if e.Severity == Fatal {
			return true
		}
	}
	return false
}

// round2 rounds to 2 decimals, half-up, for monetary comparisons (FR-A03 rounding rule).
func round2(v float64) float64 {
	if v < 0 {
		return -round2(-v)
	}
	return float64(int64(v*100+0.5)) / 100
}

// approxEqual compares two monetary amounts after half-up 2-decimal rounding.
// Tolerance guards against binary float noise while honoring the 2-decimal rule.
func approxEqual(a, b float64) bool {
	return round2(a) == round2(b) || diffAbs(a, b) < 0.005
}

func diffAbs(a, b float64) float64 {
	d := a - b
	if d < 0 {
		return -d
	}
	return d
}
