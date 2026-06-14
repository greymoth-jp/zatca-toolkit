package ksa

import (
	"encoding/base64"
	"strconv"
	"time"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/validate"
)

// qr_validate.go — validate a ZATCA QR (TLV/Base64) against the tag rules. The engine can
// generate a QR (qr.go); a real toolkit must also VERIFY one — e.g. an inbound/received
// invoice, or a self-check of a produced document. Returns validate.RuleError findings
// (rule_id + EN/AR message + fix) so the SDK/audit can surface them uniformly.
//
// ZATCA QR tags: 1 seller name, 2 VAT number, 3 timestamp (ISO8601), 4 invoice total
// (incl VAT), 5 VAT total, 6 invoice hash (base64 SHA-256), 7 ECDSA signature, 8 public key,
// 9 stamp signature (simplified only). Tags 1-6 are always required; 7-8 once signed;
// 9 only for simplified.
func ValidateQR(qrB64 string, signed, simplified bool) []validate.RuleError {
	var errs []validate.RuleError
	add := func(id, en, ar, fixEN, fixAR string) {
		errs = append(errs, validate.RuleError{RuleID: id, Path: "/qr", MessageEN: en, MessageAR: ar, FixEN: fixEN, FixAR: fixAR, Severity: validate.Fatal})
	}

	tags, err := DecodeQR(qrB64)
	if err != nil {
		add("BR-KSA-QR-01",
			"The QR code is not a valid Base64 TLV string.",
			"رمز الاستجابة السريعة ليس سلسلة TLV مُرمَّزة Base64 صحيحة.",
			"Regenerate the QR from the signed invoice (Base64 of the TLV bytes).",
			"أعد توليد رمز QR من الفاتورة الموقّعة (Base64 لبايتات TLV).")
		return errs
	}

	byTag := map[byte][]byte{}
	for _, t := range tags {
		byTag[t.Tag] = t.Value
	}
	required := []byte{1, 2, 3, 4, 5, 6}
	if signed {
		required = append(required, 7, 8)
	}
	for _, tag := range required {
		if _, ok := byTag[tag]; !ok {
			add("BR-KSA-QR-02",
				"QR is missing a mandatory tag ("+strconv.Itoa(int(tag))+").",
				"رمز QR ينقصه وسم إلزامي ("+strconv.Itoa(int(tag))+").",
				"Include QR tags 1-6 (and 7-8 once signed): seller, VAT, timestamp, totals, hash, signature, key.",
				"أدرج وسوم QR من 1 إلى 6 (و7-8 بعد التوقيع): البائع، الرقم الضريبي، الوقت، الإجماليات، البصمة، التوقيع، المفتاح.")
			break
		}
	}

	// tag 2: 15-digit VAT.
	if v, ok := byTag[2]; ok && !validKSAVATBytes(v) {
		add("BR-KSA-QR-VAT",
			"QR tag 2 (VAT number) must be a 15-digit KSA VAT number.",
			"يجب أن يكون الوسم 2 (الرقم الضريبي) رقماً ضريبياً سعودياً من 15 خانة.",
			"Set QR tag 2 to the seller 15-digit VAT number.",
			"اضبط الوسم 2 على الرقم الضريبي للبائع من 15 خانة.")
	}
	// tag 3: ISO8601 timestamp.
	if v, ok := byTag[3]; ok {
		if _, e := time.Parse(time.RFC3339, string(v)); e != nil {
			if _, e2 := time.Parse("2006-01-02T15:04:05", string(v)); e2 != nil {
				add("BR-KSA-QR-TS",
					"QR tag 3 (timestamp) must be ISO 8601.",
					"يجب أن يكون الوسم 3 (الطابع الزمني) بصيغة ISO 8601.",
					"Use an ISO 8601 timestamp, e.g. 2026-06-14T10:30:00Z.",
					"استخدم طابعاً زمنياً بصيغة ISO 8601 مثل 2026-06-14T10:30:00Z.")
			}
		}
	}
	// tags 4,5: decimal amounts.
	for _, tag := range []byte{4, 5} {
		if v, ok := byTag[tag]; ok {
			if _, e := strconv.ParseFloat(string(v), 64); e != nil {
				add("BR-KSA-QR-AMT",
					"QR tags 4 and 5 (totals) must be decimal numbers.",
					"يجب أن يكون الوسمان 4 و5 (الإجماليات) أرقاماً عشرية.",
					"Format totals as plain decimals, e.g. 115.00.",
					"نسّق الإجماليات كأرقام عشرية مثل 115.00.")
				break
			}
		}
	}
	// tag 6: base64 SHA-256 (32 bytes).
	if v, ok := byTag[6]; ok {
		if raw, e := base64.StdEncoding.DecodeString(string(v)); e != nil || len(raw) != 32 {
			add("BR-KSA-QR-HASH",
				"QR tag 6 (invoice hash) must be the Base64 of a 32-byte SHA-256 digest.",
				"يجب أن يكون الوسم 6 (بصمة الفاتورة) ترميز Base64 لبصمة SHA-256 بطول 32 بايت.",
				"Set QR tag 6 to the canonical invoice hash (Base64 SHA-256).",
				"اضبط الوسم 6 على بصمة الفاتورة المعيارية (Base64 SHA-256).")
		}
	}
	// tag 9 only for simplified.
	if _, has9 := byTag[9]; has9 && !simplified {
		add("BR-KSA-QR-09",
			"QR tag 9 (stamp) is only allowed on simplified invoices.",
			"الوسم 9 (الختم) مسموح فقط في الفواتير المبسطة.",
			"Remove tag 9 for standard invoices; it applies to simplified (B2C) only.",
			"احذف الوسم 9 للفواتير القياسية؛ فهو للفواتير المبسطة (B2C) فقط.")
	}

	return errs
}

func validKSAVATBytes(v []byte) bool {
	if len(v) != 15 {
		return false
	}
	for _, b := range v {
		if b < '0' || b > '9' {
			return false
		}
	}
	return v[0] == '3' && v[14] == '3'
}
