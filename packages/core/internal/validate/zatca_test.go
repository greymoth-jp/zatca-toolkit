package validate

import (
	"testing"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

func validKSADoc() *normalized.Doc {
	return &normalized.Doc{
		ProfileID: "reporting:1.0", ID: "INV-1", IssueDate: "2026-06-14", IssueTime: "10:30:00", TypeCode: "388",
		Currency: "SAR", TaxCurrency: "SAR", BuyerReference: "PO-1",
		Seller: normalized.Party{Name: "Acme", NameAr: "أكمي", VATID: "300000000000003", CountryCode: "SA"},
		Buyer:  normalized.Party{Name: "Beta", VATID: "311111111111113", CountryCode: "SA"},
		Lines:  []normalized.Line{{ID: "1", Quantity: 2, UnitCode: "PCE", ItemName: "Widget", NetPrice: 50, NetAmount: 100, VATCategory: "S", VATRate: 15}},
		TaxBreakdown: []normalized.TaxSubtotal{{Category: "S", Rate: 15, TaxableAmount: 100, TaxAmount: 15}},
		Totals: normalized.Totals{LineExtensionAmount: 100, TaxExclusiveAmount: 100, TaxAmount: 15, TaxInclusiveAmount: 115, PayableAmount: 115},
	}
}

func ids(r Report) map[string]bool {
	m := map[string]bool{}
	for _, e := range r.Errors {
		m[e.RuleID] = true
	}
	return m
}

func TestZATCAValidDocPasses(t *testing.T) {
	r := Validate(validKSADoc(), ProfileZATCA)
	if !r.Valid {
		t.Fatalf("expected valid KSA invoice to pass; errors=%v", ids(r))
	}
}

func TestZATCASellerVATFormat(t *testing.T) {
	for _, bad := range []string{"123456789012345", "30000000000000", "3000000000000003", "30000000000000X", "300000000000005"} {
		d := validKSADoc()
		d.Seller.VATID = bad
		if !ids(Validate(d, ProfileZATCA))["BR-KSA-39"] {
			t.Fatalf("VAT %q should fail BR-KSA-39", bad)
		}
	}
	// the canonical good one passes
	if ids(Validate(validKSADoc(), ProfileZATCA))["BR-KSA-39"] {
		t.Fatal("valid VAT flagged BR-KSA-39")
	}
}

func TestZATCACurrencyMustBeSAR(t *testing.T) {
	d := validKSADoc()
	d.Currency = "USD"
	if !ids(Validate(d, ProfileZATCA))["BR-KSA-05"] {
		t.Fatal("non-SAR currency should fail BR-KSA-05")
	}
}

func TestZATCAArabicSellerNameMandatory(t *testing.T) {
	d := validKSADoc()
	d.Seller.NameAr = ""
	if !ids(Validate(d, ProfileZATCA))["BR-KSA-27"] {
		t.Fatal("missing Arabic seller name should fail BR-KSA-27")
	}
}

func TestZATCAStandardInvoiceRequiresBuyerVAT(t *testing.T) {
	d := validKSADoc()
	d.Buyer.VATID = "311" // too short
	if !ids(Validate(d, ProfileZATCA))["BR-KSA-40"] {
		t.Fatal("standard invoice with invalid buyer VAT should fail BR-KSA-40")
	}
}

func TestZATCAVATCategoryMath(t *testing.T) {
	d := validKSADoc()
	d.TaxBreakdown[0].TaxAmount = 99 // 100 * 15% = 15, not 99
	if !ids(Validate(d, ProfileZATCA))["BR-KSA-S-MATH"] {
		t.Fatal("wrong category VAT amount should fail BR-KSA-S-MATH")
	}
}

func TestZATCALineMath(t *testing.T) {
	d := validKSADoc()
	d.Lines[0].NetAmount = 999 // 2 * 50 = 100, not 999
	if !ids(Validate(d, ProfileZATCA))["BR-KSA-LINE"] {
		t.Fatal("wrong line net should fail BR-KSA-LINE")
	}
}

// Price base quantity (BT-149): a price quoted per 100 units must not false-positive.
func TestZATCALineBaseQuantity(t *testing.T) {
	d := validKSADoc()
	// price 500 per 100 units, qty 2 -> net = 2 * 500 / 100 = 10
	d.Lines = []normalized.Line{{ID: "1", Quantity: 2, NetPrice: 500, BaseQuantity: 100, NetAmount: 10, VATCategory: "S", VATRate: 15}}
	d.TaxBreakdown = []normalized.TaxSubtotal{{Category: "S", Rate: 15, TaxableAmount: 10, TaxAmount: 1.5}}
	d.Totals = normalized.Totals{LineExtensionAmount: 10, TaxExclusiveAmount: 10, TaxAmount: 1.5, TaxInclusiveAmount: 11.5, PayableAmount: 11.5}
	if ids(Validate(d, ProfileZATCA))["BR-KSA-LINE"] {
		t.Fatal("base-quantity line must not false-positive BR-KSA-LINE")
	}
}

func TestZATCAFindingsCarryFixGuidance(t *testing.T) {
	d := validKSADoc()
	d.Currency = "USD"
	r := Validate(d, ProfileZATCA)
	for _, e := range r.Errors {
		if e.RuleID == "BR-KSA-05" {
			if e.FixEN == "" || e.FixAR == "" {
				t.Fatal("ZATCA findings must carry EN+AR fix guidance")
			}
			return
		}
	}
	t.Fatal("expected BR-KSA-05")
}

func TestZATCALineVATCategoryMandatory(t *testing.T) {
	d := validKSADoc()
	d.Lines[0].VATCategory = "" // strip the category
	if !ids(Validate(d, ProfileZATCA))["BR-KSA-LINE-CAT"] {
		t.Fatal("line without VAT category should fail BR-KSA-LINE-CAT")
	}
}

func TestZATCAZeroRatedMustBeZeroPercent(t *testing.T) {
	d := validKSADoc()
	d.Lines[0].VATCategory = "Z"
	d.Lines[0].VATRate = 15 // zero-rated but 15% — contradiction
	if !ids(Validate(d, ProfileZATCA))["BR-KSA-ZE-RATE"] {
		t.Fatal("zero-rated line with non-zero rate should fail BR-KSA-ZE-RATE")
	}
}

func TestZATCALineCategoryMustBeInBreakdown(t *testing.T) {
	d := validKSADoc()
	d.Lines[0].VATCategory = "Z"
	d.Lines[0].VATRate = 0 // valid Z line, but no Z group in the breakdown
	if !ids(Validate(d, ProfileZATCA))["BR-KSA-CAT-BRK"] {
		t.Fatal("line category absent from breakdown should fail BR-KSA-CAT-BRK")
	}
}

func TestZATCASimplifiedRelaxesBuyerVAT(t *testing.T) {
	// Standard B2B without a buyer VAT -> BR-KSA-40 fires.
	std := validKSADoc()
	std.Buyer.VATID = ""
	if !ids(Validate(std, ProfileZATCA))["BR-KSA-40"] {
		t.Fatal("standard invoice without buyer VAT should fail BR-KSA-40")
	}
	// Same invoice marked simplified (B2C consumer) -> buyer VAT not required.
	simp := validKSADoc()
	simp.Simplified = true
	simp.Buyer = normalized.Party{Name: "Walk-in customer"} // consumer: name only, no VAT
	if ids(Validate(simp, ProfileZATCA))["BR-KSA-40"] {
		t.Fatal("simplified (B2C) invoice must NOT require a buyer VAT (BR-KSA-40)")
	}
	// Seller VAT is still mandatory on a simplified invoice.
	simp.Seller.VATID = "bad"
	if !ids(Validate(simp, ProfileZATCA))["BR-KSA-39"] {
		t.Fatal("seller VAT is mandatory even on a simplified invoice (BR-KSA-39)")
	}
}

func TestZATCAIssueTimeRequired(t *testing.T) {
	d := validKSADoc()
	d.IssueTime = ""
	if !ids(Validate(d, ProfileZATCA))["BR-KSA-IT"] {
		t.Fatal("missing issue time should fail BR-KSA-IT")
	}
}

func TestZATCACreditNoteNeedsBillingRef(t *testing.T) {
	// A credit note without a BillingReference fails; with one it passes that rule.
	cn := validKSADoc()
	cn.TypeCode = "381"
	if !ids(Validate(cn, ProfileZATCA))["BR-KSA-CN-REF"] {
		t.Fatal("credit note without BillingReference should fail BR-KSA-CN-REF")
	}
	cn.BillingRefID = "INV-1"
	if ids(Validate(cn, ProfileZATCA))["BR-KSA-CN-REF"] {
		t.Fatal("credit note with a BillingReference must not fail BR-KSA-CN-REF")
	}
	// A standard invoice (388) must not require a BillingReference.
	if ids(Validate(validKSADoc(), ProfileZATCA))["BR-KSA-CN-REF"] {
		t.Fatal("a standard invoice must not require a BillingReference")
	}
}

func TestZATCALineWithAllowance(t *testing.T) {
	// Line net = qty*price - line allowance: 2*50 - 10 = 90. Must pass BR-KSA-LINE.
	d := validKSADoc()
	d.Lines[0].NetAmount = 90
	d.Lines[0].AllowanceCharges = []normalized.AllowanceCharge{{Amount: 10, Reason: "discount"}}
	d.TaxBreakdown = []normalized.TaxSubtotal{{Category: "S", Rate: 15, TaxableAmount: 90, TaxAmount: 13.5}}
	d.Totals = normalized.Totals{LineExtensionAmount: 90, TaxExclusiveAmount: 90, TaxAmount: 13.5, TaxInclusiveAmount: 103.5, PayableAmount: 103.5}
	if ids(Validate(d, ProfileZATCA))["BR-KSA-LINE"] {
		t.Fatalf("a line net that accounts for its allowance must not fail BR-KSA-LINE: %v", ids(Validate(d, ProfileZATCA)))
	}
	// Without accounting for the allowance (net still 100), it must fail.
	d.Lines[0].NetAmount = 100
	if !ids(Validate(d, ProfileZATCA))["BR-KSA-LINE"] {
		t.Fatal("a line net ignoring its allowance should fail BR-KSA-LINE")
	}
}

func TestZATCAExemptReasonRequired(t *testing.T) {
	d := validKSADoc()
	d.TaxBreakdown = []normalized.TaxSubtotal{{Category: "Z", Rate: 0, TaxableAmount: 100, TaxAmount: 0}}
	d.Lines = []normalized.Line{{ID: "1", Quantity: 1, NetPrice: 100, NetAmount: 100, VATCategory: "Z", VATRate: 0}}
	d.Totals = normalized.Totals{LineExtensionAmount: 100, TaxExclusiveAmount: 100, TaxAmount: 0, TaxInclusiveAmount: 100, PayableAmount: 100}
	if !ids(Validate(d, ProfileZATCA))["BR-KSA-EXEMPT-REASON"] {
		t.Fatal("Z breakdown without an exemption reason should fail BR-KSA-EXEMPT-REASON")
	}
	d.TaxBreakdown[0].ExemptionReason = "Exempt per Article 30 of the VAT Law"
	if ids(Validate(d, ProfileZATCA))["BR-KSA-EXEMPT-REASON"] {
		t.Fatal("Z breakdown with an exemption reason must not fail BR-KSA-EXEMPT-REASON")
	}
	// A standard (S-only) invoice must never trip it.
	if ids(Validate(validKSADoc(), ProfileZATCA))["BR-KSA-EXEMPT-REASON"] {
		t.Fatal("a standard S invoice must not require an exemption reason")
	}
}

func TestZATCAStandardLineRate(t *testing.T) {
	d := validKSADoc()
	d.Lines[0].VATRate = 10 // S line but not 15%
	if !ids(Validate(d, ProfileZATCA))["BR-KSA-S-LINE-RATE"] {
		t.Fatal("S line with non-15% rate should fail BR-KSA-S-LINE-RATE")
	}
}

func TestZATCALineRateMatchesBreakdown(t *testing.T) {
	d := validKSADoc()
	d.Lines[0].VATRate = 20 // breakdown S group is 15
	if !ids(Validate(d, ProfileZATCA))["BR-KSA-LINE-RATE-BRK"] {
		t.Fatal("line rate differing from its breakdown rate should fail BR-KSA-LINE-RATE-BRK")
	}
}

func TestZATCAZEOBreakdownRate(t *testing.T) {
	d := validKSADoc()
	d.TaxBreakdown = []normalized.TaxSubtotal{{Category: "Z", Rate: 15, TaxableAmount: 100, TaxAmount: 15}} // Z must be 0%
	if !ids(Validate(d, ProfileZATCA))["BR-KSA-ZEO-BRK-RATE"] {
		t.Fatal("Z/E/O breakdown with non-zero rate should fail BR-KSA-ZEO-BRK-RATE")
	}
}

func TestZATCANewRulesNoFalsePositive(t *testing.T) {
	r := Validate(validKSADoc(), ProfileZATCA)
	for _, id := range []string{"BR-KSA-S-LINE-RATE", "BR-KSA-LINE-RATE-BRK", "BR-KSA-ZEO-BRK-RATE"} {
		if ids(r)[id] {
			t.Fatalf("%s fired on a valid KSA invoice (false positive)", id)
		}
	}
}

func TestEN16931CategoryRules(t *testing.T) {
	// Zero-rated breakdown with a non-zero tax amount -> BR-Z-09.
	z := validKSADoc()
	z.TaxBreakdown = []normalized.TaxSubtotal{{Category: "Z", Rate: 0, TaxableAmount: 100, TaxAmount: 15}}
	z.Lines = []normalized.Line{{ID: "1", Quantity: 1, NetPrice: 100, NetAmount: 100, VATCategory: "Z", VATRate: 0}}
	z.Totals = normalized.Totals{LineExtensionAmount: 100, TaxExclusiveAmount: 100, TaxAmount: 15, TaxInclusiveAmount: 115, PayableAmount: 115}
	if !ids(Validate(z, ProfileEN16931))["BR-Z-09"] {
		t.Fatal("zero-rated with non-zero VAT amount should fail BR-Z-09")
	}
	// Exempt with a non-zero rate -> BR-E-10.
	e := validKSADoc()
	e.TaxBreakdown = []normalized.TaxSubtotal{{Category: "E", Rate: 15, TaxableAmount: 100, TaxAmount: 0}}
	if !ids(Validate(e, ProfileEN16931))["BR-E-10"] {
		t.Fatal("exempt with non-zero rate should fail BR-E-10")
	}
	// Standard (the good doc) must NOT trip any Z/E/O rule.
	bad := ids(Validate(validKSADoc(), ProfileEN16931))
	for _, r := range []string{"BR-Z-09", "BR-Z-10", "BR-E-09", "BR-E-10", "BR-O-09", "BR-O-11", "BR-S-05"} {
		if bad[r] {
			t.Fatalf("a clean standard invoice must not trip %s", r)
		}
	}
}

func TestPeppolAdditionalRules(t *testing.T) {
	// R040: duplicate line IDs.
	d := validKSADoc()
	d.Lines = []normalized.Line{
		{ID: "1", Quantity: 1, NetPrice: 50, NetAmount: 50, VATCategory: "S", VATRate: 15},
		{ID: "1", Quantity: 1, NetPrice: 50, NetAmount: 50, VATCategory: "S", VATRate: 15},
	}
	if !ids(Validate(d, ProfilePeppol))["PEPPOL-EN16931-R040"] {
		t.Fatal("duplicate line IDs should fail PEPPOL-EN16931-R040")
	}
	// R055: a line category not present in the breakdown.
	d2 := validKSADoc()
	d2.Lines[0].VATCategory = "Z"
	d2.Lines[0].VATRate = 0 // (the breakdown only has S)
	if !ids(Validate(d2, ProfilePeppol))["PEPPOL-EN16931-R055"] {
		t.Fatal("line category absent from breakdown should fail PEPPOL-EN16931-R055")
	}
	// A clean doc must trip none of the new rules.
	clean := ids(Validate(validKSADoc(), ProfilePeppol))
	for _, r := range []string{"PEPPOL-EN16931-R040", "PEPPOL-EN16931-R052", "PEPPOL-EN16931-R055"} {
		if clean[r] {
			t.Fatalf("clean doc must not trip %s", r)
		}
	}
}

// EN16931 base profile must NOT apply KSA rules (layering correctness).
func TestEN16931ProfileDoesNotApplyKSA(t *testing.T) {
	d := validKSADoc()
	d.Currency = "USD" // valid ISO, just not SAR
	r := Validate(d, ProfileEN16931)
	if ids(r)["BR-KSA-05"] {
		t.Fatal("EN16931 profile must not apply KSA SAR rule")
	}
}
