package convert

import (
	"encoding/xml"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

// FR-Z01 / Z-T0 — ZATCA-flavoured UBL 2.1. This is the EN16931 UBL with the KSA CIUS
// additions required by ZATCA, per the repo field-mapping.md:
//   - ext:UBLExtensions      placeholder for the XAdES (CSID) signature (filled by signer)
//   - cbc:UUID               document UUID
//   - AdditionalDocumentReference "ICV"  (Invoice Counter Value, sequential)
//   - AdditionalDocumentReference "PIH"  (Previous Invoice Hash — anti-tamper chain)
//   - AdditionalDocumentReference "QR"   (TLV/Base64, filled after signing)
//   - seller RegistrationName in Arabic (BT-27, ZATCA mandatory)
//
// Field choices follow field-mapping.md; ZATCA's exact XSD/Schematron must be re-validated
// before production (STATUS Open Questions).

type ZatcaUBLOpts struct {
	UUID    string // document UUID
	ICV     int64  // invoice counter value
	PIH     string // previous invoice hash (base64)
	QR      string // base64 TLV QR (optional; injected post-sign)
	IssueTime string // HH:MM:SS (ZATCA requires IssueTime in addition to IssueDate)
}

type zatcaUBLInvoice struct {
	XMLName      xml.Name `xml:"Invoice"`
	XmlnsInvoice string   `xml:"xmlns,attr"`
	XmlnsCBC     string   `xml:"xmlns:cbc,attr"`
	XmlnsCAC     string   `xml:"xmlns:cac,attr"`
	XmlnsExt     string   `xml:"xmlns:ext,attr"`

	// Signature extension placeholder — the signer replaces the empty content with XAdES.
	Extensions zatcaExtensions `xml:"ext:UBLExtensions"`

	ProfileID       string `xml:"cbc:ProfileID"`
	ID              string `xml:"cbc:ID"`
	UUID            string `xml:"cbc:UUID"`
	IssueDate       string `xml:"cbc:IssueDate"`
	IssueTime       string `xml:"cbc:IssueTime"`
	InvoiceTypeCode ublTypeCode `xml:"cbc:InvoiceTypeCode"`
	CurrencyCode    string `xml:"cbc:DocumentCurrencyCode"`
	TaxCurrency     string `xml:"cbc:TaxCurrencyCode"`

	AdditionalRefs []zatcaAddlRef `xml:"cac:AdditionalDocumentReference"`

	Supplier party    `xml:"cac:AccountingSupplierParty>cac:Party"`
	Customer party    `xml:"cac:AccountingCustomerParty>cac:Party"`
	TaxTotal taxTotal `xml:"cac:TaxTotal"`
	Monetary monetary `xml:"cac:LegalMonetaryTotal"`
	Lines    []ublLine `xml:"cac:InvoiceLine"`
}

type zatcaExtensions struct {
	// Minimal UBLExtension carrying a placeholder; the CSID signer fills SignatureInformation.
	Extension struct {
		URI     string `xml:"ext:ExtensionURI"`
		Content struct {
			Placeholder string `xml:"sig:UBLDocumentSignatures>cac:SignatureInformation>cbc:ID,omitempty"`
		} `xml:"ext:ExtensionContent"`
	} `xml:"ext:UBLExtension"`
}

type zatcaAddlRef struct {
	ID         string         `xml:"cbc:ID"`
	UUID       string         `xml:"cbc:UUID,omitempty"`
	Attachment *zatcaAttach   `xml:"cac:Attachment,omitempty"`
}

type zatcaAttach struct {
	Binary zatcaBinary `xml:"cbc:EmbeddedDocumentBinaryObject"`
}

type zatcaBinary struct {
	MimeCode string `xml:"mimeCode,attr"`
	Value    string `xml:",chardata"`
}

const (
	nsExt = "urn:oasis:names:specification:ubl:schema:xsd:CommonExtensionComponents-2"
	zatcaProfileID = "reporting:1.0"
)

// ToZatcaUBL renders the normalized doc as ZATCA UBL with ICV/PIH/QR refs + signature
// placeholder. The QR ref is included only if opts.QR is set (post-sign injection).
func ToZatcaUBL(d *normalized.Doc, opts ZatcaUBLOpts) ([]byte, error) {
	inv := zatcaUBLInvoice{
		XmlnsInvoice: nsInvoice, XmlnsCBC: nsCBC, XmlnsCAC: nsCAC, XmlnsExt: nsExt,
		ProfileID:       zatcaProfileID,
		ID:              d.ID,
		UUID:            opts.UUID,
		IssueDate:       d.IssueDate,
		IssueTime:       opts.IssueTime,
		InvoiceTypeCode: ublTypeCode{Name: typeCodeName(d), Value: d.TypeCode},
		CurrencyCode:    d.Currency,
		TaxCurrency:     "SAR",
		Supplier:        toZatcaParty(d.Seller),
		Customer:        toParty(d.Buyer),
	}
	inv.Extensions.Extension.URI = "urn:oasis:names:specification:ubl:dsig:enveloped:xades"

	// ICV (Invoice Counter Value).
	inv.AdditionalRefs = append(inv.AdditionalRefs, zatcaAddlRef{
		ID: "ICV", UUID: itoa(opts.ICV),
	})
	// PIH (Previous Invoice Hash).
	inv.AdditionalRefs = append(inv.AdditionalRefs, zatcaAddlRef{
		ID: "PIH", Attachment: &zatcaAttach{Binary: zatcaBinary{MimeCode: "text/plain", Value: opts.PIH}},
	})
	// QR (only after signing).
	if opts.QR != "" {
		inv.AdditionalRefs = append(inv.AdditionalRefs, zatcaAddlRef{
			ID: "QR", Attachment: &zatcaAttach{Binary: zatcaBinary{MimeCode: "text/plain", Value: opts.QR}},
		})
	}

	inv.TaxTotal.TaxAmount = amount{d.Currency, d.Totals.TaxAmount}
	for _, t := range d.TaxBreakdown {
		var s taxSub
		s.Taxable = amount{d.Currency, t.TaxableAmount}
		s.TaxAmt = amount{d.Currency, t.TaxAmount}
		s.Category.ID = t.Category
		s.Category.Percent = t.Rate
		s.Category.Scheme = "VAT"
		inv.TaxTotal.Subtotals = append(inv.TaxTotal.Subtotals, s)
	}
	inv.Monetary = monetary{
		LineExtension: amount{d.Currency, d.Totals.LineExtensionAmount},
		TaxExclusive:  amount{d.Currency, d.Totals.TaxExclusiveAmount},
		TaxInclusive:  amount{d.Currency, d.Totals.TaxInclusiveAmount},
		Payable:       amount{d.Currency, d.Totals.PayableAmount},
	}
	for _, l := range d.Lines {
		var ln ublLine
		ln.ID = l.ID
		ln.Quantity = qty{l.UnitCode, l.Quantity}
		ln.LineAmount = amount{d.Currency, l.NetAmount}
		ln.Item.Name = l.ItemName
		ln.Item.ClassifiedTaxCategory = classifyLine(l)
		ln.Price.Amount = amount{d.Currency, l.NetPrice}
		inv.Lines = append(inv.Lines, ln)
	}

	out, err := xml.MarshalIndent(inv, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}

// toZatcaParty prefers the Arabic registration name (BT-27, ZATCA mandatory).
func toZatcaParty(p normalized.Party) party {
	out := toParty(p)
	if p.NameAr != "" {
		out.LegalName = p.NameAr // RegistrationName in Arabic
	}
	return out
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
