package validate

import (
	"sort"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

// Profile selects which rule layers run. EN16931 is always the base; Peppol adds the
// BIS Billing 3.0 CIUS restrictions. Jurisdiction adapters (e.g. KSA ZATCA) add a
// third layer via their own validate() (fr-catalog FR-B03 adapter delegation).
type Profile string

const (
	ProfileEN16931 Profile = "en16931"   // base semantic rules only
	ProfilePeppol  Profile = "peppol-bis" // EN16931 + Peppol BIS 3.0
	ProfileZATCA   Profile = "zatca-ksa"  // EN16931 + KSA ZATCA (Fatoora Phase 2) CIUS
)

// Validate runs the deterministic rule layers for the given profile and returns a
// Report. EN16931 is always the base; Peppol and ZATCA add their CIUS layers. errors are
// sorted by rule id so the output is byte-stable across runs (audit reproducibility, FR-B01/B02).
func Validate(d *normalized.Doc, p Profile) Report {
	errs := en16931Rules(d)
	switch p {
	case ProfilePeppol:
		errs = append(errs, peppolRules(d)...)
	case ProfileZATCA:
		errs = append(errs, zatcaRules(d)...)
	}

	sort.SliceStable(errs, func(i, j int) bool {
		return errs[i].RuleID < errs[j].RuleID
	})

	return Report{
		Valid:  !hasFatal(errs),
		Errors: errs,
	}
}
