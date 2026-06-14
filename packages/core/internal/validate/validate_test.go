package validate

import (
	"testing"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

// validDoc returns a fully EN16931 + Peppol-valid invoice. Tests mutate one field at
// a time so each rule is exercised in isolation.
func validDoc() *normalized.Doc {
	return &normalized.Doc{
		ProfileID:       "urn:fdc:peppol.eu:2017:poacc:billing:01:1.0",
		CustomizationID: "urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0",
		ID:              "INV-0001",
		IssueDate:       "2026-06-14",
		TypeCode:        "380",
		Currency:        "SAR",
		BuyerReference:  "PO-2026-0001",
		Seller: normalized.Party{
			Name: "Acme Trading LLC", NameAr: "شركة أكمي للتجارة",
			VATID: "300000000000003", CountryCode: "SA",
			EndpointID: "300000000000003", EndpointScheme: "0235",
		},
		Buyer: normalized.Party{
			Name: "Beta Retail Co", VATID: "311111111111113", CountryCode: "SA",
			EndpointID: "311111111111113", EndpointScheme: "0235",
		},
		Lines: []normalized.Line{
			{ID: "1", Quantity: 2, ItemName: "Widget", NetPrice: 50, NetAmount: 100, VATCategory: "S", VATRate: 15},
		},
		TaxBreakdown: []normalized.TaxSubtotal{
			{Category: "S", Rate: 15, TaxableAmount: 100, TaxAmount: 15},
		},
		Totals: normalized.Totals{
			LineExtensionAmount: 100,
			TaxExclusiveAmount:  100,
			TaxAmount:           15,
			TaxInclusiveAmount:  115,
			PayableAmount:       115,
		},
	}
}

// findRule returns the matching RuleError (or zero value + false).
func findRule(r Report, id string) (RuleError, bool) {
	for _, e := range r.Errors {
		if e.RuleID == id {
			return e, true
		}
	}
	return RuleError{}, false
}

func TestValidDocumentPasses(t *testing.T) {
	r := Validate(validDoc(), ProfilePeppol)
	if !r.Valid {
		t.Fatalf("expected valid doc, got errors: %+v", r.Errors)
	}
	if len(r.Errors) != 0 {
		t.Fatalf("expected zero findings, got %d: %+v", len(r.Errors), r.Errors)
	}
}

// FR-B01 acceptance: Given BR-CO-10 (line sum mismatch) / When validate / Then 422
// with errors[] containing {rule_id:"BR-CO-10", path:"/lines", message_en, message_ar}.
func TestFR_B01_BR_CO_10_LineSumMismatch(t *testing.T) {
	d := validDoc()
	d.Totals.LineExtensionAmount = 999 // break BT-106 vs Σ BT-131
	r := Validate(d, ProfileEN16931)

	if r.Valid {
		t.Fatal("expected invalid (fatal) when line sum mismatches")
	}
	e, ok := findRule(r, "BR-CO-10")
	if !ok {
		t.Fatalf("expected BR-CO-10, got: %+v", r.Errors)
	}
	if e.Path != "/lines" {
		t.Errorf("BR-CO-10 path = %q, want /lines", e.Path)
	}
	if e.MessageEN == "" || e.MessageAR == "" {
		t.Errorf("BR-CO-10 must carry both message_en and message_ar; got en=%q ar=%q", e.MessageEN, e.MessageAR)
	}
	if e.Severity != Fatal {
		t.Errorf("BR-CO-10 severity = %q, want fatal", e.Severity)
	}
}

// FR-B02 acceptance: Given a Peppol BIS 3.0 violation (PEPPOL-EN16931-R010) /
// When validate / Then 422 + rule id. R010 = buyer electronic address (BT-49) required.
func TestFR_B02_PEPPOL_R010_BuyerEndpoint(t *testing.T) {
	d := validDoc()
	d.Buyer.EndpointID = "" // violate R010
	r := Validate(d, ProfilePeppol)

	if r.Valid {
		t.Fatal("expected invalid when buyer electronic address missing")
	}
	e, ok := findRule(r, "PEPPOL-EN16931-R010")
	if !ok {
		t.Fatalf("expected PEPPOL-EN16931-R010, got: %+v", r.Errors)
	}
	if e.Path != "/buyer/endpoint_id" {
		t.Errorf("R010 path = %q, want /buyer/endpoint_id", e.Path)
	}
}

// Peppol restrictions must NOT fire under the bare EN16931 profile.
func TestPeppolR002BuyerOrOrderReference(t *testing.T) {
	d := validDoc()
	d.BuyerReference = ""
	d.OrderReference = ""
	if _, ok := findRule(Validate(d, ProfilePeppol), "PEPPOL-EN16931-R002"); !ok {
		t.Fatal("an invoice with no buyer reference and no order reference should fail R002")
	}
	// A purchase order reference alone satisfies it.
	d.OrderReference = "PO-9"
	if _, ok := findRule(Validate(d, ProfilePeppol), "PEPPOL-EN16931-R002"); ok {
		t.Fatal("a purchase order reference must satisfy R002")
	}
}

func TestPeppolRulesScopedToProfile(t *testing.T) {
	d := validDoc()
	d.Buyer.EndpointID = ""
	r := Validate(d, ProfileEN16931)
	if _, ok := findRule(r, "PEPPOL-EN16931-R010"); ok {
		t.Fatal("R010 must not run under en16931 profile")
	}
}

// Reports must be deterministic (rule order stable) for audit reproducibility.
func TestDeterministicOrdering(t *testing.T) {
	d := validDoc()
	d.ID = ""
	d.Currency = ""
	d.Buyer.EndpointID = ""
	first := Validate(d, ProfilePeppol)
	second := Validate(d, ProfilePeppol)
	if len(first.Errors) != len(second.Errors) {
		t.Fatal("non-deterministic error count")
	}
	for i := range first.Errors {
		if first.Errors[i].RuleID != second.Errors[i].RuleID {
			t.Fatalf("non-deterministic order at %d: %s vs %s", i, first.Errors[i].RuleID, second.Errors[i].RuleID)
		}
	}
}

// allowanceDoc is a fully-consistent invoice carrying a document-level allowance.
func allowanceDoc() *normalized.Doc {
	d := validDoc()
	d.Lines = []normalized.Line{{ID: "1", Quantity: 2, ItemName: "Widget", NetPrice: 50, NetAmount: 100, VATCategory: "S", VATRate: 15}}
	d.AllowanceCharges = []normalized.AllowanceCharge{{Amount: 10, Reason: "Volume discount", VATCategory: "S", VATRate: 15}}
	d.TaxBreakdown = []normalized.TaxSubtotal{{Category: "S", Rate: 15, TaxableAmount: 90, TaxAmount: 13.5}}
	d.Totals = normalized.Totals{LineExtensionAmount: 100, AllowanceTotal: 10, TaxExclusiveAmount: 90, TaxAmount: 13.5, TaxInclusiveAmount: 103.5, PayableAmount: 103.5}
	return d
}

func TestAllowanceDocValid(t *testing.T) {
	r := Validate(allowanceDoc(), ProfileEN16931)
	if !r.Valid {
		t.Fatalf("a consistent allowance invoice should be valid: %+v", r.Errors)
	}
}

func TestBR_CO_11_AllowanceSum(t *testing.T) {
	d := allowanceDoc()
	d.Totals.AllowanceTotal = 99 // sum of allowances is 10
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-CO-11"); !ok {
		t.Fatal("allowance sum mismatch should fail BR-CO-11")
	}
}

func TestBR_33_AllowanceReason(t *testing.T) {
	d := allowanceDoc()
	d.AllowanceCharges[0].Reason = ""
	d.AllowanceCharges[0].ReasonCode = ""
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-33"); !ok {
		t.Fatal("allowance without reason/code should fail BR-33")
	}
}

func TestBR_CO_12_ChargeSum(t *testing.T) {
	d := allowanceDoc()
	d.AllowanceCharges = append(d.AllowanceCharges, normalized.AllowanceCharge{Charge: true, Amount: 5, Reason: "Freight"})
	d.Totals.ChargeTotal = 99 // sum of charges is 5
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-CO-12"); !ok {
		t.Fatal("charge sum mismatch should fail BR-CO-12")
	}
}

func TestBR_CO_15_TotalWithVAT(t *testing.T) {
	d := validDoc()
	d.Totals.TaxInclusiveAmount = 200 // != 100 + 15
	r := Validate(d, ProfileEN16931)
	if _, ok := findRule(r, "BR-CO-15"); !ok {
		t.Fatalf("expected BR-CO-15, got: %+v", r.Errors)
	}
}

// Existence rules added for deeper EN16931 coverage. Each fires under the bare EN16931
// profile (so it is not Peppol/ZATCA-specific) when its Business Term is missing.
func TestBR_09_SellerCountryRequired(t *testing.T) {
	d := validDoc()
	d.Seller.CountryCode = ""
	r := Validate(d, ProfileEN16931)
	e, ok := findRule(r, "BR-09")
	if !ok {
		t.Fatalf("expected BR-09, got: %+v", r.Errors)
	}
	if e.Path != "/seller/country_code" || e.MessageAR == "" {
		t.Errorf("BR-09 path/ar wrong: %+v", e)
	}
}

func TestBR_11_BuyerCountryRequired(t *testing.T) {
	d := validDoc()
	d.Buyer.CountryCode = ""
	r := Validate(d, ProfileEN16931)
	if _, ok := findRule(r, "BR-11"); !ok {
		t.Fatalf("expected BR-11, got: %+v", r.Errors)
	}
}

func TestBR_21_LineIDRequired(t *testing.T) {
	d := validDoc()
	d.Lines[0].ID = ""
	r := Validate(d, ProfileEN16931)
	if _, ok := findRule(r, "BR-21"); !ok {
		t.Fatalf("expected BR-21, got: %+v", r.Errors)
	}
}

func TestBR_24_ItemNameRequired(t *testing.T) {
	d := validDoc()
	d.Lines[0].ItemName = ""
	r := Validate(d, ProfileEN16931)
	if _, ok := findRule(r, "BR-24"); !ok {
		t.Fatalf("expected BR-24, got: %+v", r.Errors)
	}
}

// The new existence rules must NOT fire on the fully-valid document (false-positive guard,
// in addition to TestValidDocumentPasses which checks the whole set).
func TestNewExistenceRulesNoFalsePositive(t *testing.T) {
	r := Validate(validDoc(), ProfileEN16931)
	for _, id := range []string{"BR-09", "BR-11", "BR-21", "BR-24", "BR-CO-26", "BR-S-02", "BR-DEC-23", "BR-DEC-24"} {
		if _, ok := findRule(r, id); ok {
			t.Fatalf("%s fired on a valid document (false positive): %+v", id, r.Errors)
		}
	}
}

func TestBR_CO_26_SellerIdentifierRequired(t *testing.T) {
	d := validDoc()
	d.Seller.VATID = ""
	d.Seller.CompanyID = ""
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-CO-26"); !ok {
		t.Fatal("expected BR-CO-26 when seller has neither VAT id nor company id")
	}
	// A non-VAT identifier (legal/party registration in CompanyID) satisfies BR-CO-26.
	d.Seller.CompanyID = "5532331183"
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-CO-26"); ok {
		t.Fatal("BR-CO-26 must not fire when a company/party identifier is present")
	}
}

func TestBR_S_02_StandardRatedSellerVAT(t *testing.T) {
	d := validDoc()
	d.Seller.VATID = "" // S breakdown present but no seller VAT id
	r := Validate(d, ProfileEN16931)
	if _, ok := findRule(r, "BR-S-02"); !ok {
		t.Fatalf("expected BR-S-02, got: %+v", r.Errors)
	}
}

func TestBR_DEC_23_24_BreakdownDecimals(t *testing.T) {
	d := validDoc()
	d.TaxBreakdown[0].TaxableAmount = 100.123 // > 2 decimals
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-DEC-23"); !ok {
		t.Fatalf("expected BR-DEC-23 for BT-116 with >2 decimals")
	}
	d2 := validDoc()
	d2.TaxBreakdown[0].TaxAmount = 15.001 // > 2 decimals
	if _, ok := findRule(Validate(d2, ProfileEN16931), "BR-DEC-24"); !ok {
		t.Fatalf("expected BR-DEC-24 for BT-117 with >2 decimals")
	}
}

// Invoice period (BG-14): BR-29 (end >= start) and BR-CO-19 (at least one date), both gated on
// the period being present so an ordinary invoice never trips them (FP-safe).
func TestBR_29_InvoicePeriodOrder(t *testing.T) {
	// A valid period (start <= end) must pass.
	d := validDoc()
	d.InvoicePeriod = &normalized.InvoicePeriod{StartDate: "2026-05-01", EndDate: "2026-05-31"}
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-29"); ok {
		t.Fatal("a chronologically valid invoice period must not trip BR-29")
	}
	// End before start must fail BR-29.
	d.InvoicePeriod = &normalized.InvoicePeriod{StartDate: "2026-05-31", EndDate: "2026-05-01"}
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-29"); !ok {
		t.Fatalf("expected BR-29 when period end precedes start")
	}
	// An empty period (both dates blank) must fail BR-CO-19.
	d.InvoicePeriod = &normalized.InvoicePeriod{}
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-CO-19"); !ok {
		t.Fatalf("expected BR-CO-19 when an invoice period carries neither date")
	}
	// No period at all: neither rule fires.
	d.InvoicePeriod = nil
	r := Validate(d, ProfileEN16931)
	if _, ok := findRule(r, "BR-29"); ok {
		t.Fatal("BR-29 must not fire when there is no invoice period")
	}
	if _, ok := findRule(r, "BR-CO-19"); ok {
		t.Fatal("BR-CO-19 must not fire when there is no invoice period")
	}
}

// BR-DEC-01/02 (allowance) and BR-DEC-05/06 (charge): the document-level allowance/charge
// amount and base amount must have at most two decimal places. Gated on presence (FP-safe).
func TestBR_DEC_AllowanceChargeDecimals(t *testing.T) {
	// Allowance amount with >2 decimals -> BR-DEC-01.
	d := validDoc()
	d.AllowanceCharges = []normalized.AllowanceCharge{{Amount: 10.123, Reason: "promo"}}
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-DEC-01"); !ok {
		t.Errorf("expected BR-DEC-01 for an allowance amount with >2 decimals")
	}
	// Charge base amount with >2 decimals -> BR-DEC-06.
	d2 := validDoc()
	d2.AllowanceCharges = []normalized.AllowanceCharge{{Charge: true, Amount: 5, BaseAmount: 100.001, Reason: "freight"}}
	if _, ok := findRule(Validate(d2, ProfileEN16931), "BR-DEC-06"); !ok {
		t.Errorf("expected BR-DEC-06 for a charge base amount with >2 decimals")
	}
	// A clean (<=2 decimal) allowance must NOT trip any of the decimal rules.
	d3 := validDoc()
	d3.AllowanceCharges = []normalized.AllowanceCharge{{Amount: 10.50, BaseAmount: 100.00, Percent: 10.5, Reason: "promo"}}
	d3.Totals.AllowanceTotal = 10.50
	r := Validate(d3, ProfileEN16931)
	for _, id := range []string{"BR-DEC-01", "BR-DEC-02", "BR-DEC-05", "BR-DEC-06"} {
		if _, ok := findRule(r, id); ok {
			t.Errorf("%s must not fire on a 2-decimal allowance", id)
		}
	}
}

// BR-41 (line allowance) / BR-42 (line charge): a line-level allowance/charge must carry a
// reason or a reason code. Gated on presence (FP-safe).
func TestBR_41_42_LineAllowanceChargeReason(t *testing.T) {
	// A line allowance with no reason/code -> BR-41.
	d := validDoc()
	d.Lines[0].AllowanceCharges = []normalized.AllowanceCharge{{Amount: 3}}
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-41"); !ok {
		t.Errorf("expected BR-41 for a line allowance without a reason")
	}
	// A line charge with no reason/code -> BR-42.
	d2 := validDoc()
	d2.Lines[0].AllowanceCharges = []normalized.AllowanceCharge{{Charge: true, Amount: 2}}
	if _, ok := findRule(Validate(d2, ProfileEN16931), "BR-42"); !ok {
		t.Errorf("expected BR-42 for a line charge without a reason")
	}
	// With a reason present, neither fires.
	d3 := validDoc()
	d3.Lines[0].AllowanceCharges = []normalized.AllowanceCharge{{Amount: 3, Reason: "line discount"}}
	r := Validate(d3, ProfileEN16931)
	if _, ok := findRule(r, "BR-41"); ok {
		t.Error("BR-41 must not fire when a line allowance has a reason")
	}
	// No line allowances at all -> neither fires.
	r2 := Validate(validDoc(), ProfileEN16931)
	if _, ok := findRule(r2, "BR-41"); ok {
		t.Error("BR-41 must not fire when there are no line allowances")
	}
	if _, ok := findRule(r2, "BR-42"); ok {
		t.Error("BR-42 must not fire when there are no line charges")
	}
}

// BR-DEC-24/25 (line allowance) and BR-DEC-27/28 (line charge): line-level allowance/charge
// amount and base amount must have at most two decimals. Gated on presence (FP-safe).
func TestBR_DEC_LineAllowanceChargeDecimals(t *testing.T) {
	d := validDoc()
	d.Lines[0].AllowanceCharges = []normalized.AllowanceCharge{{Amount: 1.234, Reason: "x"}}
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-DEC-24"); !ok {
		t.Errorf("expected BR-DEC-24 for a line allowance amount with >2 decimals")
	}
	d2 := validDoc()
	d2.Lines[0].AllowanceCharges = []normalized.AllowanceCharge{{Charge: true, Amount: 2, BaseAmount: 10.005, Reason: "x"}}
	if _, ok := findRule(Validate(d2, ProfileEN16931), "BR-DEC-28"); !ok {
		t.Errorf("expected BR-DEC-28 for a line charge base amount with >2 decimals")
	}
	// Clean (<=2 dec) line allowance/charge must not trip any of them.
	d3 := validDoc()
	d3.Lines[0].AllowanceCharges = []normalized.AllowanceCharge{{Amount: 1.25, BaseAmount: 10.00, Reason: "x"}}
	r := Validate(d3, ProfileEN16931)
	for _, id := range []string{"BR-DEC-24", "BR-DEC-25", "BR-DEC-27", "BR-DEC-28"} {
		if _, ok := findRule(r, id); ok {
			t.Errorf("%s must not fire on a 2-decimal line allowance", id)
		}
	}
}

// BR-S-08 (and Z/E/O variants): per-category VAT taxable amount (BT-116) must equal the sum of
// that category's line nets + category charges - category allowances. FP-safe: skipped unless
// every line and document allowance/charge is categorised.
func TestBR_S_08_CategoryTaxableConsistency(t *testing.T) {
	// validDoc: line S net 100, breakdown S taxable 100 -> consistent, no fire.
	if _, ok := findRule(Validate(validDoc(), ProfileEN16931), "BR-S-08"); ok {
		t.Fatal("BR-S-08 must not fire on a consistent single-category invoice")
	}
	// Break the breakdown taxable so it no longer matches the line sum -> BR-S-08.
	d := validDoc()
	d.TaxBreakdown[0].TaxableAmount = 250 // lines sum to 100
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-S-08"); !ok {
		t.Fatalf("expected BR-S-08 when category taxable != sum of category line nets")
	}
	// An uncategorised line disables the check (attribution impossible) -> no fire even if mismatched.
	d2 := validDoc()
	d2.TaxBreakdown[0].TaxableAmount = 250
	d2.Lines[0].VATCategory = ""
	if _, ok := findRule(Validate(d2, ProfileEN16931), "BR-S-08"); ok {
		t.Fatal("BR-S-08 must be skipped when a line has no VAT category (FP-safe guard)")
	}
	// A document allowance attributed to S reduces the expected category taxable.
	d3 := validDoc()
	d3.AllowanceCharges = []normalized.AllowanceCharge{{Amount: 10, Reason: "promo", VATCategory: "S"}}
	d3.TaxBreakdown[0].TaxableAmount = 90 // 100 line net - 10 allowance
	if _, ok := findRule(Validate(d3, ProfileEN16931), "BR-S-08"); ok {
		t.Fatalf("BR-S-08 must account for category allowances: %+v", Validate(d3, ProfileEN16931).Errors)
	}
}

// BR-CO-20 (line period present -> needs a date) and BR-30 (end >= start), both gated on a line
// carrying a period (FP-safe).
func TestBR_CO_20_30_LinePeriod(t *testing.T) {
	d := validDoc()
	d.Lines[0].Period = &normalized.InvoicePeriod{StartDate: "2026-05-01", EndDate: "2026-05-31"}
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-30"); ok {
		t.Fatal("a valid line period must not trip BR-30")
	}
	d.Lines[0].Period = &normalized.InvoicePeriod{StartDate: "2026-05-31", EndDate: "2026-05-01"}
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-30"); !ok {
		t.Fatal("expected BR-30 when line period end precedes start")
	}
	d.Lines[0].Period = &normalized.InvoicePeriod{}
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-CO-20"); !ok {
		t.Fatal("expected BR-CO-20 for a line period with neither date")
	}
	d.Lines[0].Period = nil
	if _, ok := findRule(Validate(d, ProfileEN16931), "BR-CO-20"); ok {
		t.Fatal("BR-CO-20 must not fire when a line has no period")
	}
}
