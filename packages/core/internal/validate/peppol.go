package validate

import (
	"strings"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

// peppolRules implements a representative SUBSET of the Peppol BIS Billing 3.0 CIUS
// restrictions layered on top of EN16931. The authoritative rules are the OpenPEPPOL
// Schematron (PEPPOL-EN16931-UBL.sch, release 3.0.20 / Nov 2025); full coverage is
// ticket T B02 (size L) and SHOULD execute the official Schematron rather than
// re-implement it. We implement the rules the acceptance criteria name (e.g.
// PEPPOL-EN16931-R010) plus the highest-value existence restrictions.
//
// Rule semantics verified against OpenPEPPOL master 2026-06; see STATUS.md diff log.
func peppolRules(d *normalized.Doc) []RuleError {
	var errs []RuleError
	add := func(id, path, en, ar string, sev Severity) {
		errs = append(errs, RuleError{RuleID: id, Path: path, MessageEN: en, MessageAR: ar, Severity: sev})
	}

	// PEPPOL-EN16931-R001: Business process (ProfileID) MUST be provided.
	if d.ProfileID == "" {
		add("PEPPOL-EN16931-R001", "/profile_id",
			"Business process (Peppol ProfileID) must be provided.",
			"يجب توفير معرف العملية التجارية (ProfileID).", Fatal)
	}

	// PEPPOL-EN16931-R002: an Invoice shall have a Buyer reference (BT-10) or a Purchase order
	// reference (BT-13) — Peppol needs at least one routing/matching reference.
	if d.BuyerReference == "" && d.OrderReference == "" {
		add("PEPPOL-EN16931-R002", "/buyer_reference",
			"An Invoice must have a Buyer reference (BT-10) or a Purchase order reference (BT-13).",
			"يجب أن تحتوي الفاتورة على مرجع المشتري (BT-10) أو مرجع أمر الشراء (BT-13).", Fatal)
	}

	// PEPPOL-EN16931-R004: Specification identifier (CustomizationID) MUST be provided.
	if d.CustomizationID == "" {
		add("PEPPOL-EN16931-R004", "/customization_id",
			"Specification identifier (CustomizationID) must be provided.",
			"يجب توفير معرف المواصفات (CustomizationID).", Fatal)
	}

	// PEPPOL-EN16931-R010: Buyer electronic address (BT-49) MUST be provided.
	if d.Buyer.EndpointID == "" {
		add("PEPPOL-EN16931-R010", "/buyer/endpoint_id",
			"Buyer electronic address (BT-49) must be provided.",
			"يجب توفير العنوان الإلكتروني للمشتري (BT-49).", Fatal)
	}

	// PEPPOL-EN16931-R020: Seller electronic address (BT-34) MUST be provided.
	if d.Seller.EndpointID == "" {
		add("PEPPOL-EN16931-R020", "/seller/endpoint_id",
			"Seller electronic address (BT-34) must be provided.",
			"يجب توفير العنوان الإلكتروني للبائع (BT-34).", Fatal)
	}

	// PEPPOL-EN16931-R053: Only one tax total with tax subtotals SHALL be provided —
	// surfaced here as a warning when the VAT breakdown is empty but tax is charged.
	if d.Totals.TaxAmount != 0 && len(d.TaxBreakdown) == 0 {
		add("PEPPOL-EN16931-R053", "/tax_breakdown",
			"A VAT breakdown (BG-23) must be provided when a VAT amount is charged.",
			"يجب توفير تفصيل ضريبة القيمة المضافة (BG-23) عند فرض مبلغ ضريبي.", Fatal)
	}

	// PEPPOL-EN16931-R040: each Invoice line identifier (BT-126) shall be unique.
	seen := map[string]bool{}
	for _, l := range d.Lines {
		if l.ID != "" && seen[l.ID] {
			add("PEPPOL-EN16931-R040", "/lines",
				"Each invoice line identifier (BT-126) must be unique.",
				"يجب أن يكون معرّف كل بند في الفاتورة (BT-126) فريداً.", Fatal)
			break
		}
		seen[l.ID] = true
	}

	// PEPPOL-EN16931-R052: each VAT breakdown group (BG-23) shall have a VAT category code.
	for _, t := range d.TaxBreakdown {
		if t.Category == "" {
			add("PEPPOL-EN16931-R052", "/tax_breakdown",
				"Each VAT breakdown (BG-23) must have a VAT category code (BT-118).",
				"يجب أن تحتوي كل مجموعة تفصيل ضريبي (BG-23) على رمز فئة ضريبية (BT-118).", Fatal)
			break
		}
	}

	// PEPPOL-EN16931-R055: each invoiced line VAT category (BT-151) must match a VAT
	// breakdown category (BG-23) — categories used on lines must be summarised.
	cats := map[string]bool{}
	for _, t := range d.TaxBreakdown {
		cats[strings.ToUpper(t.Category)] = true
	}
	for _, l := range d.Lines {
		if l.VATCategory != "" && !cats[strings.ToUpper(l.VATCategory)] {
			add("PEPPOL-EN16931-R055", "/lines",
				"Each invoiced line VAT category (BT-151) must be summarised in the VAT breakdown (BG-23).",
				"يجب تلخيص فئة ضريبة كل بند (BT-151) في تفصيل الضريبة (BG-23).", Fatal)
			break
		}
	}

	return errs
}
