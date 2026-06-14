package validate

import (
	"strings"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

// en16931Rules implements a representative, deterministic SUBSET of the EN16931
// semantic business rules. The authoritative and exhaustive rule set lives in the
// official EN16931 / OpenPEPPOL Schematron; full coverage is ticket T B01 (size L).
// This subset covers the existence rules (BR-*) and the arithmetic integrity rules
// (BR-CO-*) that the acceptance criteria (FR-B01, e.g. BR-CO-10) name explicitly.
//
// Each rule appends at most one RuleError. Order is fixed → reproducible reports.
func en16931Rules(d *normalized.Doc) []RuleError {
	var errs []RuleError
	add := func(id, path, en, ar string, sev Severity) {
		errs = append(errs, RuleError{RuleID: id, Path: path, MessageEN: en, MessageAR: ar, Severity: sev})
	}

	// --- Existence rules (subset) ---
	// NOTE: BR-01 (Specification identifier BT-24) is intentionally NOT enforced in the base
	// layer: the ZATCA-KSA profile does not always carry an EN16931 CustomizationID, so a base
	// BR-01 would false-positive on valid KSA invoices. Peppol enforces its own R004 instead.
	if d.ID == "" {
		add("BR-02", "/id",
			"An Invoice shall have an Invoice number (BT-1).",
			"يجب أن يحتوي الفاتورة على رقم فاتورة (BT-1).", Fatal)
	}
	if d.IssueDate == "" {
		add("BR-03", "/issue_date",
			"An Invoice shall have an Invoice issue date (BT-2).",
			"يجب أن يحتوي الفاتورة على تاريخ إصدار (BT-2).", Fatal)
	}
	if d.TypeCode == "" {
		add("BR-04", "/type_code",
			"An Invoice shall have an Invoice type code (BT-3).",
			"يجب أن يحتوي الفاتورة على رمز نوع الفاتورة (BT-3).", Fatal)
	}
	if d.Currency == "" {
		add("BR-05", "/currency",
			"An Invoice shall have an Invoice currency code (BT-5).",
			"يجب أن يحتوي الفاتورة على رمز العملة (BT-5).", Fatal)
	}
	if d.Seller.Name == "" {
		add("BR-06", "/seller/name",
			"An Invoice shall contain the Seller name (BT-27).",
			"يجب أن يحتوي الفاتورة على اسم البائع (BT-27).", Fatal)
	}
	if d.Buyer.Name == "" {
		add("BR-07", "/buyer/name",
			"An Invoice shall contain the Buyer name (BT-44).",
			"يجب أن يحتوي الفاتورة على اسم المشتري (BT-44).", Fatal)
	}
	// BR-09: the Seller postal address (BG-5) shall contain a Seller country code (BT-40).
	// Gated on the seller block being present (name set) so an entirely missing seller is
	// reported once by BR-06 rather than twice.
	if d.Seller.Name != "" && d.Seller.CountryCode == "" {
		add("BR-09", "/seller/country_code",
			"The Seller postal address (BG-5) shall contain a Seller country code (BT-40).",
			"يجب أن يحتوي عنوان البائع (BG-5) على رمز بلد البائع (BT-40).", Fatal)
	}
	// BR-11: the Buyer postal address (BG-8) shall contain a Buyer country code (BT-55).
	if d.Buyer.Name != "" && d.Buyer.CountryCode == "" {
		add("BR-11", "/buyer/country_code",
			"The Buyer postal address (BG-8) shall contain a Buyer country code (BT-55).",
			"يجب أن يحتوي عنوان المشتري (BG-8) على رمز بلد المشتري (BT-55).", Fatal)
	}
	if len(d.Lines) == 0 {
		add("BR-16", "/lines",
			"An Invoice shall have at least one Invoice line (BG-25).",
			"يجب أن يحتوي الفاتورة على بند واحد على الأقل (BG-25).", Fatal)
	}
	// BR-21 / BR-24: each Invoice line (BG-25) shall have a line identifier (BT-126) and an
	// Item name (BT-153). Reported once each so a malformed line list stays readable.
	for _, l := range d.Lines {
		if l.ID == "" {
			add("BR-21", "/lines",
				"Each Invoice line (BG-25) shall have an Invoice line identifier (BT-126).",
				"يجب أن يحتوي كل بند في الفاتورة (BG-25) على معرّف بند (BT-126).", Fatal)
			break
		}
	}
	for _, l := range d.Lines {
		if l.ItemName == "" {
			add("BR-24", "/lines",
				"Each Invoice line (BG-25) shall have an Item name (BT-153).",
				"يجب أن يحتوي كل بند في الفاتورة (BG-25) على اسم صنف (BT-153).", Fatal)
			break
		}
	}

	// BR-CO-26: so the Buyer can identify the Seller, at least one Seller identifier shall be
	// present — VAT identifier (BT-31), tax registration (BT-32) or legal/party registration
	// (BT-30/BT-29). The parser maps the latter into CompanyID (see parse.go).
	if d.Seller.Name != "" && d.Seller.VATID == "" && d.Seller.CompanyID == "" {
		add("BR-CO-26", "/seller",
			"The Seller must be identifiable: a Seller VAT identifier (BT-31), tax registration (BT-32) or legal registration (BT-30) shall be present.",
			"يجب أن يكون البائع قابلاً للتعريف: رقم ضريبي (BT-31) أو تسجيل ضريبي (BT-32) أو تسجيل نظامي (BT-30).", Fatal)
	}
	// BR-S-02: an Invoice with a Standard-rated (S) VAT breakdown shall contain the Seller VAT
	// identifier (BT-31) (or a tax representative VAT identifier, not modelled here).
	hasStandard := false
	for _, t := range d.TaxBreakdown {
		if strings.ToUpper(t.Category) == "S" {
			hasStandard = true
			break
		}
	}
	if hasStandard && d.Seller.VATID == "" {
		add("BR-S-02", "/seller/vat_id",
			"An Invoice with a Standard-rated (S) VAT breakdown shall contain the Seller VAT identifier (BT-31).",
			"يجب أن تحتوي الفاتورة ذات الفئة القياسية (S) على الرقم الضريبي للبائع (BT-31).", Fatal)
	}

	// --- Arithmetic integrity rules (BR-CO-*) ---

	// BR-CO-10: Sum of Invoice line net amount (BT-106) = Σ Invoice line net amount (BT-131).
	var lineSum float64
	for _, l := range d.Lines {
		lineSum += l.NetAmount
	}
	if len(d.Lines) > 0 && !approxEqual(d.Totals.LineExtensionAmount, lineSum) {
		add("BR-CO-10", "/lines",
			"Sum of Invoice line net amount (BT-106) must equal the sum of line net amounts (BT-131).",
			"يجب أن يساوي مجموع صافي مبالغ البنود (BT-106) مجموع صافي مبالغ السطور (BT-131).", Fatal)
	}

	// BR-CO-13: Invoice total amount without VAT (BT-109) =
	//   Σ line net (BT-106) - allowances (BT-107) + charges (BT-108).
	expectedExcl := d.Totals.LineExtensionAmount - d.Totals.AllowanceTotal + d.Totals.ChargeTotal
	if !approxEqual(d.Totals.TaxExclusiveAmount, expectedExcl) {
		add("BR-CO-13", "/totals/tax_exclusive_amount",
			"Invoice total without VAT (BT-109) must equal Σ line net - allowances + charges.",
			"يجب أن يساوي إجمالي الفاتورة بدون ضريبة (BT-109) مجموع البنود ناقص الخصومات زائد الرسوم.", Fatal)
	}

	// BR-CO-15: Invoice total with VAT (BT-112) =
	//   Invoice total without VAT (BT-109) + Invoice total VAT amount (BT-110).
	if !approxEqual(d.Totals.TaxInclusiveAmount, d.Totals.TaxExclusiveAmount+d.Totals.TaxAmount) {
		add("BR-CO-15", "/totals/tax_inclusive_amount",
			"Invoice total with VAT (BT-112) must equal total without VAT (BT-109) + total VAT (BT-110).",
			"يجب أن يساوي إجمالي الفاتورة شامل الضريبة (BT-112) الإجمالي بدون ضريبة زائد إجمالي الضريبة.", Fatal)
	}

	// BR-CO-16: Amount due for payment (BT-115) =
	//   Invoice total with VAT (BT-112) - paid amount (BT-113).
	if d.Totals.PayableAmount != 0 || d.Totals.PrepaidAmount != 0 {
		if !approxEqual(d.Totals.PayableAmount, d.Totals.TaxInclusiveAmount-d.Totals.PrepaidAmount) {
			add("BR-CO-16", "/totals/payable_amount",
				"Amount due for payment (BT-115) must equal total with VAT (BT-112) - prepaid amount (BT-113).",
				"يجب أن يساوي المبلغ المستحق للدفع (BT-115) الإجمالي شامل الضريبة ناقص المبلغ المدفوع مقدماً.", Fatal)
		}
	}

	// BR-CO-14: Invoice total VAT amount (BT-110) = Σ VAT category tax amount (BT-117).
	var taxSum float64
	for _, t := range d.TaxBreakdown {
		taxSum += t.TaxAmount
	}
	if len(d.TaxBreakdown) > 0 && !approxEqual(d.Totals.TaxAmount, taxSum) {
		add("BR-CO-14", "/totals/tax_amount",
			"Invoice total VAT amount (BT-110) must equal the sum of VAT category tax amounts (BT-117).",
			"يجب أن يساوي إجمالي ضريبة الفاتورة (BT-110) مجموع مبالغ ضريبة الفئات (BT-117).", Fatal)
	}

	// --- VAT category rules (BR-S / BR-Z / BR-E / BR-O), per VAT breakdown group ---
	for _, t := range d.TaxBreakdown {
		switch strings.ToUpper(t.Category) {
		case "S":
			// BR-S-05: a Standard-rated VAT breakdown must have a VAT rate greater than zero.
			if t.Rate <= 0 {
				add("BR-S-05", "/tax_breakdown",
					"A Standard-rated (S) VAT breakdown must have a VAT rate greater than zero.",
					"يجب أن تكون نسبة الضريبة أكبر من صفر في فئة الضريبة القياسية (S).", Fatal)
			}
		case "Z":
			// BR-Z-09: zero-rated VAT category tax amount must be 0; BR-Z-10: rate must be 0.
			if !approxEqual(t.TaxAmount, 0) {
				add("BR-Z-09", "/tax_breakdown",
					"A Zero-rated (Z) VAT breakdown must have a VAT category tax amount of 0.",
					"يجب أن يكون مبلغ ضريبة الفئة صفراً في الفئة الصفرية (Z).", Fatal)
			}
			if !approxEqual(t.Rate, 0) {
				add("BR-Z-10", "/tax_breakdown",
					"A Zero-rated (Z) VAT breakdown must have a VAT rate of 0.",
					"يجب أن تكون نسبة الضريبة صفراً في الفئة الصفرية (Z).", Fatal)
			}
		case "E":
			// BR-E-09 / BR-E-10: exempt category has 0 tax amount and 0 rate.
			if !approxEqual(t.TaxAmount, 0) {
				add("BR-E-09", "/tax_breakdown",
					"An Exempt (E) VAT breakdown must have a VAT category tax amount of 0.",
					"يجب أن يكون مبلغ ضريبة الفئة صفراً في الفئة المعفاة (E).", Fatal)
			}
			if !approxEqual(t.Rate, 0) {
				add("BR-E-10", "/tax_breakdown",
					"An Exempt (E) VAT breakdown must have a VAT rate of 0.",
					"يجب أن تكون نسبة الضريبة صفراً في الفئة المعفاة (E).", Fatal)
			}
		case "O":
			// BR-O-09 / BR-O-11: out-of-scope category has 0 tax amount and 0 rate.
			if !approxEqual(t.TaxAmount, 0) {
				add("BR-O-09", "/tax_breakdown",
					"An Out-of-scope (O) VAT breakdown must have a VAT category tax amount of 0.",
					"يجب أن يكون مبلغ ضريبة الفئة صفراً في الفئة خارج النطاق (O).", Fatal)
			}
			if !approxEqual(t.Rate, 0) {
				add("BR-O-11", "/tax_breakdown",
					"An Out-of-scope (O) VAT breakdown must have a VAT rate of 0.",
					"يجب أن تكون نسبة الضريبة صفراً في الفئة خارج النطاق (O).", Fatal)
			}
		}
	}

	// BR-CO-18: an Invoice with lines shall have at least one VAT breakdown group (BG-23).
	if len(d.Lines) > 0 && len(d.TaxBreakdown) == 0 {
		add("BR-CO-18", "/tax_breakdown",
			"An Invoice shall have at least one VAT breakdown group (BG-23).",
			"يجب أن تحتوي الفاتورة على مجموعة تفصيل ضريبي واحدة على الأقل (BG-23).", Fatal)
	}

	// BR-CO-17: in each VAT breakdown, the category tax amount (BT-117) = the category
	// taxable amount (BT-116) x (rate / 100), rounded. approxEqual tolerates 2-decimal rounding.
	for _, t := range d.TaxBreakdown {
		if !approxEqual(t.TaxAmount, round2(t.TaxableAmount*t.Rate/100)) {
			add("BR-CO-17", "/tax_breakdown",
				"VAT category tax amount (BT-117) must equal the category taxable amount (BT-116) times the rate.",
				"يجب أن يساوي مبلغ ضريبة الفئة (BT-117) المبلغ الخاضع للفئة (BT-116) مضروباً في النسبة.", Fatal)
			break
		}
	}

	// BR-S-08 / BR-Z-08 / BR-E-08 / BR-O-08 / BR-AE-08: in each VAT breakdown, the category
	// taxable amount (BT-116) must equal the sum of the line net amounts (BT-131) of that VAT
	// category, plus document charges (BT-99) of that category, minus document allowances (BT-92)
	// of that category. Line net amounts already include any line-level allowances/charges.
	//
	// FP-safety: only checkable when every line and every document allowance/charge carries a VAT
	// category (else amounts cannot be attributed). We also skip any category that appears in more
	// than one breakdown (e.g. split rates), which this category-level sum cannot disambiguate.
	categoriesAttributable := len(d.Lines) > 0
	for _, l := range d.Lines {
		if l.VATCategory == "" {
			categoriesAttributable = false
			break
		}
	}
	if categoriesAttributable {
		for _, ac := range d.AllowanceCharges {
			if ac.VATCategory == "" {
				categoriesAttributable = false
				break
			}
		}
	}
	if categoriesAttributable {
		catCount := map[string]int{}
		for _, t := range d.TaxBreakdown {
			catCount[strings.ToUpper(t.Category)]++
		}
		for _, t := range d.TaxBreakdown {
			cat := strings.ToUpper(t.Category)
			id := brCat08ID(cat)
			if id == "" || catCount[cat] > 1 {
				continue
			}
			var expected float64
			for _, l := range d.Lines {
				if strings.ToUpper(l.VATCategory) == cat {
					expected += l.NetAmount
				}
			}
			for _, ac := range d.AllowanceCharges {
				if strings.ToUpper(ac.VATCategory) != cat {
					continue
				}
				if ac.Charge {
					expected += ac.Amount
				} else {
					expected -= ac.Amount
				}
			}
			if !approxEqual(t.TaxableAmount, round2(expected)) {
				add(id, "/tax_breakdown",
					"The VAT category taxable amount (BT-116) must equal the sum of line net amounts of that category, plus category charges, minus category allowances.",
					"يجب أن يساوي المبلغ الخاضع للفئة الضريبية (BT-116) مجموع صافي مبالغ البنود لتلك الفئة زائد رسوم الفئة ناقص خصوماتها.", Fatal)
			}
		}
	}

	// --- Codelist + decimal rules ---

	// BR-CL-05: Document currency (BT-5) must be a 3-letter ISO 4217 code.
	if d.Currency != "" && !isCurrencyCode(d.Currency) {
		add("BR-CL-05", "/currency",
			"Document currency code (BT-5) must be a valid 3-letter ISO 4217 code.",
			"يجب أن يكون رمز عملة المستند (BT-5) رمز ISO 4217 صحيحاً من ثلاثة أحرف.", Fatal)
	}

	// BR-CL-14: the Seller country code (BT-40), when present, must be a 2-letter ISO 3166-1 code.
	if d.Seller.CountryCode != "" && !isCountryCode(d.Seller.CountryCode) {
		add("BR-CL-14", "/seller/country_code",
			"Seller country code (BT-40) must be a valid 2-letter ISO 3166-1 code.",
			"يجب أن يكون رمز بلد البائع (BT-40) رمز ISO 3166-1 صحيحاً من حرفين.", Fatal)
	}

	// BR-CL-17: each VAT category code (BT-118/BT-151) must be in the EN16931 code list
	// (S/Z/E/AE/K/G/O/L/M). NOTE: ZATCA further restricts this to S/Z/E/O (BR-KSA-CL-01).
	for _, t := range d.TaxBreakdown {
		if t.Category != "" && !enVATCategory(t.Category) {
			add("BR-CL-17", "/tax_breakdown",
				"VAT category code must be a valid EN16931 code (S, Z, E, AE, K, G, O, L, M).",
				"يجب أن يكون رمز فئة الضريبة من قائمة EN16931 الصحيحة (S, Z, E, AE, K, G, O, L, M).", Fatal)
			break
		}
	}

	// BR-DEC-14: monetary totals must be expressed with at most two decimal places.
	decAmts := map[string]float64{
		"/totals/line_extension_amount": d.Totals.LineExtensionAmount,
		"/totals/tax_exclusive_amount":  d.Totals.TaxExclusiveAmount,
		"/totals/tax_amount":            d.Totals.TaxAmount,
		"/totals/tax_inclusive_amount":  d.Totals.TaxInclusiveAmount,
		"/totals/payable_amount":        d.Totals.PayableAmount,
	}
	for path, v := range decAmts {
		if v != round2(v) {
			add("BR-DEC-14", path,
				"Monetary amounts must not have more than two decimal places.",
				"يجب ألا تحتوي المبالغ النقدية على أكثر من خانتين عشريتين.", Fatal)
			break
		}
	}

	// BR-DEC-23: VAT category taxable amount (BT-116) must have at most two decimal places.
	for _, t := range d.TaxBreakdown {
		if t.TaxableAmount != round2(t.TaxableAmount) {
			add("BR-DEC-23", "/tax_breakdown",
				"The VAT category taxable amount (BT-116) must not have more than two decimal places.",
				"يجب ألا يحتوي المبلغ الخاضع للفئة الضريبية (BT-116) على أكثر من خانتين عشريتين.", Fatal)
			break
		}
	}
	// BR-DEC-24: VAT category tax amount (BT-117) must have at most two decimal places.
	for _, t := range d.TaxBreakdown {
		if t.TaxAmount != round2(t.TaxAmount) {
			add("BR-DEC-24", "/tax_breakdown",
				"The VAT category tax amount (BT-117) must not have more than two decimal places.",
				"يجب ألا يحتوي مبلغ ضريبة الفئة (BT-117) على أكثر من خانتين عشريتين.", Fatal)
			break
		}
	}

	// --- Invoice period (BG-14) rules ---
	// All gated on InvoicePeriod being present, so an invoice without a period never trips them
	// (FP-safe: the official EN16931 fixtures with no/valid periods stay green).
	if d.InvoicePeriod != nil {
		// BR-CO-19: if the Invoicing period (BG-14) is used, the period start date (BT-73) or the
		// period end date (BT-74) shall be filled.
		if d.InvoicePeriod.StartDate == "" && d.InvoicePeriod.EndDate == "" {
			add("BR-CO-19", "/invoice_period",
				"If the Invoicing period (BG-14) is used, the period start date (BT-73) or the period end date (BT-74) shall be filled.",
				"إذا استُخدمت فترة الفوترة (BG-14) فيجب تعبئة تاريخ بداية الفترة (BT-73) أو تاريخ نهايتها (BT-74).", Fatal)
		}
		// BR-29: when both the period start date (BT-73) and end date (BT-74) are given, the end
		// date shall be later than or equal to the start date. ISO-8601 (YYYY-MM-DD) of equal
		// length compares lexically == chronologically.
		if d.InvoicePeriod.StartDate != "" && d.InvoicePeriod.EndDate != "" &&
			len(d.InvoicePeriod.StartDate) == len(d.InvoicePeriod.EndDate) &&
			d.InvoicePeriod.EndDate < d.InvoicePeriod.StartDate {
			add("BR-29", "/invoice_period/end_date",
				"The Invoicing period end date (BT-74) shall be later than or equal to the Invoicing period start date (BT-73).",
				"يجب أن يكون تاريخ نهاية فترة الفوترة (BT-74) مساوياً أو لاحقاً لتاريخ بدايتها (BT-73).", Fatal)
		}
	}

	// --- Invoice line period (BG-26) rules ---
	// BR-CO-20: if an Invoice line period (BG-26) is used, its start (BT-134) or end (BT-135) date
	// shall be filled. BR-30: when both are given, the end date shall be >= the start date. Both are
	// gated on the line carrying a period, so an ordinary line never trips them (FP-safe).
	var badLinePeriodEmpty, badLinePeriodOrder bool
	for _, l := range d.Lines {
		if l.Period == nil {
			continue
		}
		if l.Period.StartDate == "" && l.Period.EndDate == "" {
			badLinePeriodEmpty = true
			continue
		}
		if l.Period.StartDate != "" && l.Period.EndDate != "" &&
			len(l.Period.StartDate) == len(l.Period.EndDate) &&
			l.Period.EndDate < l.Period.StartDate {
			badLinePeriodOrder = true
		}
	}
	if badLinePeriodEmpty {
		add("BR-CO-20", "/lines/period",
			"If an Invoice line period (BG-26) is used, the line period start date (BT-134) or end date (BT-135) shall be filled.",
			"إذا استُخدمت فترة بند الفاتورة (BG-26) فيجب تعبئة تاريخ بدايتها (BT-134) أو نهايتها (BT-135).", Fatal)
	}
	if badLinePeriodOrder {
		add("BR-30", "/lines/period/end_date",
			"The Invoice line period end date (BT-135) shall be later than or equal to the line period start date (BT-134).",
			"يجب أن يكون تاريخ نهاية فترة بند الفاتورة (BT-135) مساوياً أو لاحقاً لتاريخ بدايتها (BT-134).", Fatal)
	}

	// --- Document-level allowance (BG-20) / charge (BG-21) rules ---
	var allowSum, chargeSum float64
	hasAllow, hasCharge, badAllowReason, badChargeReason, badCalc := false, false, false, false, false
	var badAllowAmtDec, badAllowBaseDec, badChargeAmtDec, badChargeBaseDec bool
	for _, ac := range d.AllowanceCharges {
		if ac.Charge {
			chargeSum += ac.Amount
			hasCharge = true
		} else {
			allowSum += ac.Amount
			hasAllow = true
		}
		// BR-33 (allowance) / BR-38 (charge): a reason (BT-97/BT-104) or reason code
		// (BT-98/BT-105) shall be provided.
		if ac.Reason == "" && ac.ReasonCode == "" {
			if ac.Charge {
				badChargeReason = true
			} else {
				badAllowReason = true
			}
		}
		// BR-CO-05: when a base amount and percentage are both present, the amount must equal
		// base amount x percentage / 100.
		if ac.BaseAmount != 0 && ac.Percent != 0 && !approxEqual(ac.Amount, round2(ac.BaseAmount*ac.Percent/100)) {
			badCalc = true
		}
		// BR-DEC-01/02 (allowance) and BR-DEC-05/06 (charge): the amount (BT-92/BT-99) and the
		// base amount (BT-93/BT-100) shall have at most two decimal places. The base check is
		// gated on a non-zero base so an omitted base does not trip it.
		if ac.Amount != round2(ac.Amount) {
			if ac.Charge {
				badChargeAmtDec = true
			} else {
				badAllowAmtDec = true
			}
		}
		if ac.BaseAmount != 0 && ac.BaseAmount != round2(ac.BaseAmount) {
			if ac.Charge {
				badChargeBaseDec = true
			} else {
				badAllowBaseDec = true
			}
		}
	}
	if badAllowReason {
		add("BR-33", "/allowance_charges",
			"Each document-level allowance (BG-20) shall have a reason (BT-97) or a reason code (BT-98).",
			"يجب أن يكون لكل خصم على مستوى المستند (BG-20) سبب (BT-97) أو رمز سبب (BT-98).", Fatal)
	}
	if badChargeReason {
		add("BR-38", "/allowance_charges",
			"Each document-level charge (BG-21) shall have a reason (BT-104) or a reason code (BT-105).",
			"يجب أن يكون لكل رسم على مستوى المستند (BG-21) سبب (BT-104) أو رمز سبب (BT-105).", Fatal)
	}
	if badCalc {
		add("BR-CO-05", "/allowance_charges",
			"An allowance/charge amount must equal its base amount times its percentage.",
			"يجب أن يساوي مبلغ الخصم/الرسم المبلغ الأساسي مضروباً في النسبة المئوية.", Fatal)
	}
	if badAllowAmtDec {
		add("BR-DEC-01", "/allowance_charges",
			"The document level allowance amount (BT-92) must not have more than two decimal places.",
			"يجب ألا يحتوي مبلغ الخصم على مستوى المستند (BT-92) على أكثر من خانتين عشريتين.", Fatal)
	}
	if badAllowBaseDec {
		add("BR-DEC-02", "/allowance_charges",
			"The document level allowance base amount (BT-93) must not have more than two decimal places.",
			"يجب ألا يحتوي المبلغ الأساسي للخصم على مستوى المستند (BT-93) على أكثر من خانتين عشريتين.", Fatal)
	}
	if badChargeAmtDec {
		add("BR-DEC-05", "/allowance_charges",
			"The document level charge amount (BT-99) must not have more than two decimal places.",
			"يجب ألا يحتوي مبلغ الرسم على مستوى المستند (BT-99) على أكثر من خانتين عشريتين.", Fatal)
	}
	if badChargeBaseDec {
		add("BR-DEC-06", "/allowance_charges",
			"The document level charge base amount (BT-100) must not have more than two decimal places.",
			"يجب ألا يحتوي المبلغ الأساسي للرسم على مستوى المستند (BT-100) على أكثر من خانتين عشريتين.", Fatal)
	}
	// BR-CO-11: sum of document-level allowance amounts (BT-92) = allowance total (BT-107).
	if hasAllow && !approxEqual(d.Totals.AllowanceTotal, round2(allowSum)) {
		add("BR-CO-11", "/totals/allowance_total",
			"Sum of document-level allowance amounts (BT-92) must equal the allowance total (BT-107).",
			"يجب أن يساوي مجموع مبالغ الخصومات على مستوى المستند (BT-92) إجمالي الخصومات (BT-107).", Fatal)
	}
	// BR-CO-12: sum of document-level charge amounts (BT-99) = charge total (BT-108).
	if hasCharge && !approxEqual(d.Totals.ChargeTotal, round2(chargeSum)) {
		add("BR-CO-12", "/totals/charge_total",
			"Sum of document-level charge amounts (BT-99) must equal the charge total (BT-108).",
			"يجب أن يساوي مجموع مبالغ الرسوم على مستوى المستند (BT-99) إجمالي الرسوم (BT-108).", Fatal)
	}

	// --- Line-level allowance (BG-27) / charge (BG-28) rules ---
	// BR-41 / BR-42: each Invoice line allowance/charge shall carry a reason (BT-139/BT-144) or a
	// reason code (BT-140/BT-145). BR-DEC-24/25 (line allowance amount BT-136 / base BT-137) and
	// BR-DEC-27/28 (line charge amount BT-141 / base BT-142): at most two decimal places. All
	// gated on a line allowance/charge (and non-zero base) being present (FP-safe).
	badLineAllowReason, badLineChargeReason := false, false
	var badLAllowAmtDec, badLAllowBaseDec, badLChargeAmtDec, badLChargeBaseDec bool
	for _, l := range d.Lines {
		for _, ac := range l.AllowanceCharges {
			if ac.Reason == "" && ac.ReasonCode == "" {
				if ac.Charge {
					badLineChargeReason = true
				} else {
					badLineAllowReason = true
				}
			}
			if ac.Amount != round2(ac.Amount) {
				if ac.Charge {
					badLChargeAmtDec = true
				} else {
					badLAllowAmtDec = true
				}
			}
			if ac.BaseAmount != 0 && ac.BaseAmount != round2(ac.BaseAmount) {
				if ac.Charge {
					badLChargeBaseDec = true
				} else {
					badLAllowBaseDec = true
				}
			}
		}
	}
	if badLineAllowReason {
		add("BR-41", "/lines/allowance_charges",
			"Each Invoice line allowance (BG-27) shall have an Invoice line allowance reason (BT-139) or reason code (BT-140).",
			"يجب أن يكون لكل خصم على مستوى بند الفاتورة (BG-27) سبب (BT-139) أو رمز سبب (BT-140).", Fatal)
	}
	if badLineChargeReason {
		add("BR-42", "/lines/allowance_charges",
			"Each Invoice line charge (BG-28) shall have an Invoice line charge reason (BT-144) or reason code (BT-145).",
			"يجب أن يكون لكل رسم على مستوى بند الفاتورة (BG-28) سبب (BT-144) أو رمز سبب (BT-145).", Fatal)
	}
	if badLAllowAmtDec {
		add("BR-DEC-24", "/lines/allowance_charges",
			"The Invoice line allowance amount (BT-136) must not have more than two decimal places.",
			"يجب ألا يحتوي مبلغ الخصم على مستوى البند (BT-136) على أكثر من خانتين عشريتين.", Fatal)
	}
	if badLAllowBaseDec {
		add("BR-DEC-25", "/lines/allowance_charges",
			"The Invoice line allowance base amount (BT-137) must not have more than two decimal places.",
			"يجب ألا يحتوي المبلغ الأساسي للخصم على مستوى البند (BT-137) على أكثر من خانتين عشريتين.", Fatal)
	}
	if badLChargeAmtDec {
		add("BR-DEC-27", "/lines/allowance_charges",
			"The Invoice line charge amount (BT-141) must not have more than two decimal places.",
			"يجب ألا يحتوي مبلغ الرسم على مستوى البند (BT-141) على أكثر من خانتين عشريتين.", Fatal)
	}
	if badLChargeBaseDec {
		add("BR-DEC-28", "/lines/allowance_charges",
			"The Invoice line charge base amount (BT-142) must not have more than two decimal places.",
			"يجب ألا يحتوي المبلغ الأساسي للرسم على مستوى البند (BT-142) على أكثر من خانتين عشريتين.", Fatal)
	}

	return errs
}

// isCountryCode reports whether s is a 2-letter uppercase ISO 3166-1 alpha-2 country code.
func isCountryCode(s string) bool {
	if len(s) != 2 {
		return false
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// brCat08ID maps a VAT category code to its EN16931 per-category taxable-amount rule id
// (BR-{cat}-08). Returns "" for categories without a defined per-category rule in this layer.
func brCat08ID(cat string) string {
	switch cat {
	case "S":
		return "BR-S-08"
	case "Z":
		return "BR-Z-08"
	case "E":
		return "BR-E-08"
	case "O":
		return "BR-O-08"
	case "AE":
		return "BR-AE-08"
	}
	return ""
}

// enVATCategory reports whether c is a valid EN16931 VAT category code (broader than KSA).
func enVATCategory(c string) bool {
	switch strings.ToUpper(c) {
	case "S", "Z", "E", "AE", "K", "G", "O", "L", "M":
		return true
	}
	return false
}

// isCurrencyCode reports whether s is a 3-letter uppercase ISO 4217-shaped currency code.
func isCurrencyCode(s string) bool {
	if len(s) != 3 {
		return false
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}
