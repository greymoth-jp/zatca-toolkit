// Package check is the shared logic behind the zatca-check CLI / CI Action: validate a UBL
// invoice (ZATCA semantic rules, optionally the structural elements of a submitted invoice)
// and report whether it would block clearance. Kept separate from cmd/ so it is unit-testable
// (the CLI main is a thin wrapper).
package check

import (
	"github.com/greymoth-jp/zatca-toolkit/core/internal/convert"
	"github.com/greymoth-jp/zatca-toolkit/core/internal/validate"
)

// Result is the outcome of checking one invoice.
type Result struct {
	Findings []validate.RuleError
	Fatal    bool // true if any finding is fatal (would not clear)
}

// CheckXML parses a UBL/ZATCA invoice and validates it against the ZATCA CIUS profile.
// When structural is true it also verifies the submitted-invoice structural elements
// (UUID/ICV/PIH/QR/signature) — only meaningful for an already-signed/cleared document.
func CheckXML(xmlBytes []byte, structural bool) (Result, error) {
	doc, err := convert.ParseUBL(xmlBytes)
	if err != nil {
		return Result{}, err
	}
	var findings []validate.RuleError
	findings = append(findings, validate.Validate(doc, validate.ProfileZATCA).Errors...)
	if structural {
		if st, e := convert.ZatcaStructuralIssues(xmlBytes); e == nil {
			findings = append(findings, st...)
		}
	}
	fatal := false
	for _, f := range findings {
		if f.Severity == validate.Fatal {
			fatal = true
			break
		}
	}
	return Result{Findings: findings, Fatal: fatal}, nil
}
