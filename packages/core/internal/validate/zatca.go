package validate

import (
	"strings"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

// zatca.go — KSA ZATCA (Fatoora Phase 2) business rules layered on EN16931. These are the
// KSA CIUS rules that make the difference between "valid EN16931" and "would actually clear
// ZATCA": VAT-number format, SAR currency, mandatory Arabic seller name, invoice type and VAT
// category code lists, the KSA standard 15% rate, and the VAT arithmetic per category.
//
// Each finding carries a fix hint (FixEN/FixAR) — telling the user exactly what to change is a
// deliberate differentiator. Rule ids use the BR-KSA-* space (vendor numbering; ZATCA does not
// publish stable public ids for all of these, so they are clearly namespaced and
// fixture-reconcilable). This is a representative-but-substantial KSA layer; the authoritative
// set is the ZATCA validation rules doc + official Schematron (P0a-4, fixture-gated).
func zatcaRules(d *normalized.Doc) []RuleError {
	var errs []RuleError
	add := func(id, path, en, ar, fixEN, fixAR string, sev Severity) {
		errs = append(errs, RuleError{RuleID: id, Path: path, MessageEN: en, MessageAR: ar, FixEN: fixEN, FixAR: fixAR, Severity: sev})
	}

	// Standard (B2B/B2G clearance) vs simplified (B2C reporting). Simplified relaxes the
	// buyer-VAT requirement (the buyer is a consumer) but still requires the QR tag-9 stamp
	// (verified at the QR/structural layer).
	standard := !d.Simplified && (d.TypeCode == "388" || d.TypeCode == "")

	// BR-KSA-02: Invoice type code must be a ZATCA value (388 invoice, 381 credit, 383 debit).
	switch d.TypeCode {
	case "388", "381", "383", "":
	default:
		add("BR-KSA-02", "/type_code",
			"Invoice type code must be 388 (invoice), 381 (credit note) or 383 (debit note) for ZATCA.",
			"يجب أن يكون رمز نوع الفاتورة 388 (فاتورة) أو 381 (إشعار دائن) أو 383 (إشعار مدين) في زاتكا.",
			"Set the invoice type code to 388, 381 or 383.",
			"اضبط رمز نوع الفاتورة على 388 أو 381 أو 383.", Fatal)
	}

	// BR-KSA-05: Document currency (BT-5) must be SAR.
	if d.Currency != "" && d.Currency != "SAR" {
		add("BR-KSA-05", "/currency",
			"ZATCA invoices must be issued in SAR (BT-5).",
			"يجب إصدار فواتير زاتكا بالريال السعودي (SAR).",
			"Set the document currency to SAR.",
			"اضبط عملة المستند على SAR.", Fatal)
	}
	// BR-KSA-06: Tax currency (BT-6), when present, must be SAR.
	if d.TaxCurrency != "" && d.TaxCurrency != "SAR" {
		add("BR-KSA-06", "/tax_currency",
			"The VAT accounting currency (BT-6) must be SAR.",
			"يجب أن تكون عملة احتساب الضريبة (BT-6) بالريال السعودي.",
			"Set the tax currency to SAR.",
			"اضبط عملة الضريبة على SAR.", Fatal)
	}

	// BR-KSA-39: Seller VAT identifier (BT-31) must be a 15-digit KSA VAT number (starts and
	// ends with 3) — mandatory for both standard and simplified (the seller is VAT-registered).
	if !validKSAVAT(d.Seller.VATID) {
		add("BR-KSA-39", "/seller/vat_id",
			"Seller VAT number (BT-31) must be 15 digits beginning and ending with 3 (KSA format).",
			"يجب أن يكون الرقم الضريبي للبائع (BT-31) من 15 رقماً يبدأ وينتهي بالرقم 3 (صيغة المملكة).",
			"Provide a valid 15-digit KSA VAT number, e.g. 3XXXXXXXXXXXX03.",
			"أدخل رقماً ضريبياً سعودياً صحيحاً من 15 خانة، مثل 3XXXXXXXXXXXX03.", Fatal)
	}

	// BR-KSA-40: For a standard tax invoice (388), the buyer VAT number must be present and valid.
	if standard && !validKSAVAT(d.Buyer.VATID) {
		add("BR-KSA-40", "/buyer/vat_id",
			"A standard tax invoice (B2B) requires a valid 15-digit buyer VAT number (BT-48).",
			"تتطلب الفاتورة الضريبية القياسية (B2B) رقماً ضريبياً صحيحاً للمشتري من 15 خانة (BT-48).",
			"Add the buyer VAT number, or issue a simplified invoice if the buyer is a consumer.",
			"أضف الرقم الضريبي للمشتري، أو أصدر فاتورة مبسطة إذا كان المشتري مستهلكاً.", Fatal)
	}

	// BR-KSA-27: Seller name in Arabic (BT-27) is mandatory.
	if d.Seller.NameAr == "" {
		add("BR-KSA-27", "/seller/name_ar",
			"The seller name must be provided in Arabic (BT-27).",
			"يجب توفير اسم البائع باللغة العربية (BT-27).",
			"Add the Arabic seller registration name.",
			"أضف اسم البائع المسجّل بالعربية.", Fatal)
	}

	// BR-KSA-IT: a ZATCA invoice must carry the issue time (cbc:IssueTime) in addition to the date.
	if d.IssueTime == "" {
		add("BR-KSA-IT", "/issue_time",
			"A ZATCA invoice must carry an issue time (cbc:IssueTime), not only the issue date.",
			"يجب أن تحتوي فاتورة زاتكا على وقت الإصدار (cbc:IssueTime) بالإضافة إلى التاريخ.",
			"Add the issue time as HH:MM:SS (e.g. 10:30:00).",
			"أضف وقت الإصدار بصيغة HH:MM:SS (مثل 10:30:00).", Fatal)
	}

	// BR-KSA-CN-REF: a credit (381) or debit (383) note must reference the original invoice via a
	// BillingReference (BT-25). Without it, the correction cannot be tied to the cleared invoice.
	if (d.TypeCode == "381" || d.TypeCode == "383") && d.BillingRefID == "" {
		add("BR-KSA-CN-REF", "/billing_ref_id",
			"A credit or debit note must reference the original invoice via a BillingReference (BT-25).",
			"يجب أن يشير الإشعار الدائن أو المدين إلى الفاتورة الأصلية عبر مرجع الفوترة (BT-25).",
			"Add the original invoice number (and issue date) as the BillingReference.",
			"أضف رقم الفاتورة الأصلية (وتاريخ إصدارها) كمرجع فوترة.", Fatal)
	}

	// BR-KSA-CL-VATCAT: Each line VAT category code must be S/Z/E/O.
	for _, l := range d.Lines {
		if l.VATCategory != "" && !validVATCategory(l.VATCategory) {
			add("BR-KSA-CL-01", "/lines",
				"Line VAT category code must be one of S, Z, E, O (BT-151).",
				"يجب أن يكون رمز فئة الضريبة للسطر أحد القيم S أو Z أو E أو O (BT-151).",
				"Use S (standard 15%), Z (zero-rated), E (exempt) or O (out of scope).",
				"استخدم S (قياسي ١٥٪) أو Z (صفري) أو E (معفى) أو O (خارج النطاق).", Fatal)
			break
		}
	}

	// BR-KSA-S-RATE: standard-rated (S) tax categories must use the KSA standard 15% rate.
	for _, t := range d.TaxBreakdown {
		if strings.EqualFold(t.Category, "S") && !approxEqual(t.Rate, 15) {
			add("BR-KSA-S-RATE", "/tax_breakdown",
				"Standard-rated (S) VAT must use the KSA standard rate of 15%.",
				"يجب أن تستخدم الضريبة القياسية (S) النسبة القياسية في المملكة وهي ١٥٪.",
				"Set the rate to 15 for standard-rated items, or use Z/E for zero/exempt.",
				"اضبط النسبة على ١٥ للأصناف القياسية، أو استخدم Z/E للصفري/المعفى.", Warning)
		}
		// BR-KSA-S-MATH: per-category VAT amount must equal taxable x rate.
		if !approxEqual(t.TaxAmount, round2(t.TaxableAmount*t.Rate/100)) {
			add("BR-KSA-S-MATH", "/tax_breakdown",
				"VAT category tax amount (BT-117) must equal taxable amount x rate.",
				"يجب أن يساوي مبلغ ضريبة الفئة (BT-117) المبلغ الخاضع مضروباً في النسبة.",
				"Recompute the category VAT as taxable_amount * rate / 100, rounded to 2 decimals.",
				"أعد حساب ضريبة الفئة = المبلغ الخاضع × النسبة ÷ ١٠٠ مقرّباً لخانتين.", Fatal)
		}
		// BR-KSA-ZEO-BRK-RATE: zero-rated (Z), exempt (E) and out-of-scope (O) VAT breakdown
		// groups must carry a 0% rate (breakdown-level companion to the line-level BR-KSA-ZE-RATE).
		switch strings.ToUpper(t.Category) {
		case "Z", "E", "O":
			if !approxEqual(t.Rate, 0) {
				add("BR-KSA-ZEO-BRK-RATE", "/tax_breakdown",
					"Zero-rated (Z), exempt (E) and out-of-scope (O) VAT breakdown groups must have a 0% rate.",
					"يجب أن تكون نسبة الضريبة 0٪ لمجموعات التفصيل الصفرية (Z) والمعفاة (E) وخارج النطاق (O).",
					"Set the breakdown rate to 0 for Z/E/O groups, or use S (15%) for standard-rated.",
					"اضبط نسبة المجموعة على 0 لفئات Z/E/O، أو استخدم S (١٥٪) للقياسي.", Fatal)
			}
			// BR-KSA-EXEMPT-REASON: a Z/E/O breakdown must state a VAT exemption reason
			// (BT-120 text or BT-121 code) — ZATCA requires the legal basis for not charging VAT.
			if t.ExemptionReasonCode == "" && t.ExemptionReason == "" {
				add("BR-KSA-EXEMPT-REASON", "/tax_breakdown",
					"A zero-rated/exempt/out-of-scope VAT breakdown must state an exemption reason (BT-120) or reason code (BT-121).",
					"يجب أن تذكر مجموعة الضريبة الصفرية/المعفاة/خارج النطاق سبب الإعفاء (BT-120) أو رمز السبب (BT-121).",
					"Add the VAT exemption reason text or code for the Z/E/O breakdown group.",
					"أضف نص أو رمز سبب الإعفاء الضريبي لمجموعة Z/E/O.", Fatal)
			}
		}
	}

	// BR-KSA-LINE-MATH: line net amount (BT-131) must equal quantity x (net price / base
	// quantity). The price base quantity (BT-149) defaults to 1; real invoices price per N units.
	for _, l := range d.Lines {
		base := l.BaseQuantity
		if base <= 0 {
			base = 1
		}
		// Line net (BT-131) = quantity x (net price / base qty) - line allowances + line charges.
		var lineAllow, lineCharge float64
		for _, ac := range l.AllowanceCharges {
			if ac.Charge {
				lineCharge += ac.Amount
			} else {
				lineAllow += ac.Amount
			}
		}
		if !approxEqual(l.NetAmount, round2(l.Quantity*l.NetPrice/base-lineAllow+lineCharge)) {
			add("BR-KSA-LINE", "/lines",
				"Invoice line net amount (BT-131) must equal quantity (BT-129) x item net price (BT-146) minus line allowances plus line charges.",
				"يجب أن يساوي صافي مبلغ سطر الفاتورة (BT-131) الكمية × سعر الوحدة الصافي ناقص خصومات السطر زائد رسوم السطر.",
				"Set line net = quantity * net price - line allowances + line charges (2-decimal half-up).",
				"اضبط صافي السطر = الكمية × السعر الصافي − خصومات السطر + رسوم السطر (تقريب خانتين).", Fatal)
			break
		}
	}

	// BR-KSA-CR: Seller country (BT-40) should be SA.
	if d.Seller.CountryCode != "" && !strings.EqualFold(d.Seller.CountryCode, "SA") {
		add("BR-KSA-CR", "/seller/country_code",
			"For KSA invoices the seller country (BT-40) should be SA.",
			"بالنسبة لفواتير المملكة يجب أن يكون بلد البائع (BT-40) هو SA.",
			"Set the seller country code to SA.",
			"اضبط رمز بلد البائع على SA.", Warning)
	}

	// --- Line VAT category rules (BT-151/BT-152) ---
	breakdownCats := map[string]bool{}
	breakdownRate := map[string]float64{}
	for _, t := range d.TaxBreakdown {
		breakdownCats[strings.ToUpper(t.Category)] = true
		breakdownRate[strings.ToUpper(t.Category)] = t.Rate
	}
	// BR-KSA-S-LINE-RATE: a Standard-rated (S) line must carry the KSA 15% rate (BT-152).
	for _, l := range d.Lines {
		if strings.EqualFold(l.VATCategory, "S") && !approxEqual(l.VATRate, 15) {
			add("BR-KSA-S-LINE-RATE", "/lines",
				"A Standard-rated (S) line must use the KSA standard VAT rate of 15% (BT-152).",
				"يجب أن يستخدم السطر القياسي (S) النسبة القياسية في المملكة وهي ١٥٪ (BT-152).",
				"Set the line VAT rate to 15 for S lines, or use Z/E/O for zero/exempt/out-of-scope.",
				"اضبط نسبة ضريبة السطر على ١٥ لسطور S، أو استخدم Z/E/O.", Warning)
			break
		}
	}
	// BR-KSA-LINE-RATE-BRK: each line VAT rate (BT-152) must equal the rate of its category in
	// the VAT breakdown (BG-23) — the line and the summary must agree.
	for _, l := range d.Lines {
		c := strings.ToUpper(l.VATCategory)
		if c == "" {
			continue
		}
		if r, ok := breakdownRate[c]; ok && !approxEqual(l.VATRate, r) {
			add("BR-KSA-LINE-RATE-BRK", "/lines",
				"Each line VAT rate (BT-152) must equal the rate of its category in the VAT breakdown (BT-119).",
				"يجب أن تساوي نسبة ضريبة كل سطر (BT-152) نسبة فئتها في تفصيل الضريبة (BT-119).",
				"Align the line rate with its VAT breakdown group rate.",
				"اجعل نسبة السطر مطابقة لنسبة مجموعة التفصيل الضريبي.", Fatal)
			break
		}
	}
	for _, l := range d.Lines {
		// BR-KSA-LINE-CAT: every line must declare a VAT category code.
		if l.VATCategory == "" {
			add("BR-KSA-LINE-CAT", "/lines",
				"Every invoice line must declare a VAT category code (BT-151).",
				"يجب أن يحدد كل سطر في الفاتورة رمز فئة ضريبة القيمة المضافة (BT-151).",
				"Tag each line with S (standard 15%), Z (zero-rated), E (exempt) or O (out of scope).",
				"حدّد لكل سطر S (قياسي ١٥٪) أو Z (صفري) أو E (معفى) أو O (خارج النطاق).", Fatal)
			break
		}
	}
	for _, l := range d.Lines {
		c := strings.ToUpper(l.VATCategory)
		// BR-KSA-ZE-RATE: zero-rated / exempt / out-of-scope lines carry a 0% rate.
		if (c == "Z" || c == "E" || c == "O") && !approxEqual(l.VATRate, 0) {
			add("BR-KSA-ZE-RATE", "/lines",
				"Zero-rated (Z), exempt (E) and out-of-scope (O) lines must have a 0% VAT rate.",
				"يجب أن تكون نسبة الضريبة 0٪ للسطور الصفرية (Z) والمعفاة (E) وخارج النطاق (O).",
				"Set the rate to 0 for Z/E/O lines, or change the category to S for 15%.",
				"اضبط النسبة على 0 لسطور Z/E/O، أو غيّر الفئة إلى S لتطبيق ١٥٪.", Fatal)
			break
		}
	}
	for _, l := range d.Lines {
		// BR-KSA-CAT-BRK: each line VAT category must appear in the VAT breakdown.
		if l.VATCategory != "" && !breakdownCats[strings.ToUpper(l.VATCategory)] {
			add("BR-KSA-CAT-BRK", "/tax_breakdown",
				"Every line VAT category must have a matching entry in the VAT breakdown (BG-23).",
				"يجب أن يكون لكل فئة ضريبية في السطور قيد مطابق في تفصيل الضريبة (BG-23).",
				"Add a VAT breakdown group for every category used on the lines.",
				"أضف مجموعة تفصيل ضريبي لكل فئة مستخدمة في السطور.", Fatal)
			break
		}
	}

	return errs
}

// validKSAVAT reports whether v is a KSA VAT number: 15 digits, first and last digit 3.
func validKSAVAT(v string) bool {
	if len(v) != 15 {
		return false
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return false
		}
	}
	return v[0] == '3' && v[14] == '3'
}

func validVATCategory(c string) bool {
	switch strings.ToUpper(c) {
	case "S", "Z", "E", "O":
		return true
	}
	return false
}
