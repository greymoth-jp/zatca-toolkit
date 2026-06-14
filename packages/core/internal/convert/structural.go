package convert

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"

	"github.com/beevik/etree"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/validate"
)

var errInvoiceRoot = errors.New("convert: root element is not a UBL Invoice")

// structural.go — verify the ZATCA-mandatory STRUCTURAL elements of a submitted/cleared
// invoice: the document UUID, the ICV (Invoice Counter Value), the PIH (Previous Invoice
// Hash), the QR, and the XAdES signature in UBLExtensions. This is distinct from the
// pre-submission semantic audit (validate.Validate): those elements are added at signing /
// clearance, so this check is for verifying an invoice that claims to be cleared — e.g. an
// inbound/received document, or a self-check of a produced-and-signed file.
//
// Returns validate.RuleError findings (rule_id + EN/AR message + fix), so the SDK/CLI speak
// one finding shape everywhere.
func ZatcaStructuralIssues(xmlBytes []byte) ([]validate.RuleError, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(xmlBytes); err != nil {
		return nil, err
	}
	root := doc.Root()
	if root == nil || root.Tag != "Invoice" {
		return nil, errInvoiceRoot
	}

	var errs []validate.RuleError
	add := func(id, path, en, ar, fixEN, fixAR string) {
		errs = append(errs, validate.RuleError{RuleID: id, Path: path, MessageEN: en, MessageAR: ar, FixEN: fixEN, FixAR: fixAR, Severity: validate.Fatal})
	}

	// UUID
	if stext(root, "cbc:UUID") == "" {
		add("BR-KSA-ST-UUID", "/uuid",
			"A cleared ZATCA invoice must carry a document UUID (cbc:UUID).",
			"يجب أن تحمل فاتورة زاتكا المُخلَّصة معرّف مستند UUID.",
			"Assign a UUID when generating the invoice (one per document, reused on retry).",
			"عيّن UUID عند توليد الفاتورة (واحد لكل مستند، يُعاد استخدامه عند الإعادة).")
	}

	// ICV — AdditionalDocumentReference[ID=ICV]/cbc:UUID, must be a positive integer.
	if icv := addlRefValue(root, "ICV"); icv == "" {
		add("BR-KSA-ST-ICV", "/icv",
			"Missing the Invoice Counter Value (ICV) reference.",
			"ينقص مرجع قيمة عدّاد الفاتورة (ICV).",
			"Add an AdditionalDocumentReference with ID 'ICV' carrying the sequential counter.",
			"أضف AdditionalDocumentReference بالمعرّف 'ICV' يحمل العدّاد التسلسلي.")
	} else if n, err := strconv.Atoi(strings.TrimSpace(icv)); err != nil || n < 1 {
		add("BR-KSA-ST-ICV", "/icv",
			"The ICV must be a positive sequential integer.",
			"يجب أن تكون قيمة ICV عدداً صحيحاً تسلسلياً موجباً.",
			"Set the ICV to a 1-based counter that increments per issued document.",
			"اضبط ICV على عدّاد يبدأ من 1 ويزيد مع كل مستند صادر.")
	}

	// PIH — AdditionalDocumentReference[ID=PIH]/.../EmbeddedDocumentBinaryObject, base64.
	if pih := addlRefBinary(root, "PIH"); pih == "" {
		add("BR-KSA-ST-PIH", "/pih",
			"Missing the Previous Invoice Hash (PIH) reference.",
			"ينقص مرجع بصمة الفاتورة السابقة (PIH).",
			"Add an AdditionalDocumentReference with ID 'PIH' carrying the prior document hash (Base64).",
			"أضف AdditionalDocumentReference بالمعرّف 'PIH' يحمل بصمة المستند السابق (Base64).")
	} else if _, err := base64.StdEncoding.DecodeString(strings.TrimSpace(pih)); err != nil {
		add("BR-KSA-ST-PIH", "/pih",
			"The PIH must be Base64-encoded.",
			"يجب أن تكون قيمة PIH مُرمَّزة Base64.",
			"Encode the previous invoice hash as Base64 (genesis = Base64 of SHA-256 of '0').",
			"رمّز بصمة الفاتورة السابقة بصيغة Base64 (الأصل = Base64 لـ SHA-256 للقيمة '0').")
	}

	// QR
	if addlRefBinary(root, "QR") == "" {
		add("BR-KSA-ST-QR", "/qr",
			"A cleared invoice must embed the QR (AdditionalDocumentReference ID 'QR').",
			"يجب أن تتضمّن الفاتورة المُخلَّصة رمز QR (AdditionalDocumentReference بالمعرّف 'QR').",
			"Embed the Base64 TLV QR after signing.",
			"ضمّن رمز QR (Base64 TLV) بعد التوقيع.")
	}

	// Signature — ds:Signature inside ext:UBLExtensions.
	if root.FindElement("ext:UBLExtensions//ds:Signature") == nil {
		add("BR-KSA-ST-SIG", "/signature",
			"Missing the XAdES signature in UBLExtensions.",
			"ينقص التوقيع XAdES داخل UBLExtensions.",
			"Sign the invoice (secp256k1 / CSID) and embed the XAdES signature before submission.",
			"وقّع الفاتورة (secp256k1 / CSID) وضمّن توقيع XAdES قبل الإرسال.")
	}

	return errs, nil
}

// addlRefValue returns the cbc:UUID of the AdditionalDocumentReference whose cbc:ID == id.
func addlRefValue(root *etree.Element, id string) string {
	for _, r := range root.FindElements("cac:AdditionalDocumentReference") {
		if idEl := r.SelectElement("cbc:ID"); idEl != nil && idEl.Text() == id {
			return stext(r, "cbc:UUID")
		}
	}
	return ""
}

// addlRefBinary returns the embedded binary object of the AdditionalDocumentReference ID == id.
func addlRefBinary(root *etree.Element, id string) string {
	for _, r := range root.FindElements("cac:AdditionalDocumentReference") {
		if idEl := r.SelectElement("cbc:ID"); idEl != nil && idEl.Text() == id {
			return stext(r, "cac:Attachment/cbc:EmbeddedDocumentBinaryObject")
		}
	}
	return ""
}
