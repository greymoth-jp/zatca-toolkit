package convert

import (
	"errors"
	"strconv"
	"strings"

	"github.com/beevik/etree"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

// parse.go — UBL XML → normalized.Doc (FR-A01 inbound / audit wedge). The free auditor
// (apps/audit) lets anyone paste a ZATCA/EN16931 UBL invoice; to validate it we must first
// recover the normalized model. Go's encoding/xml cannot reliably unmarshal prefixed
// element names, so we navigate the cbc:/cac: tree with etree (same lib as canonical.go).
//
// This parser is intentionally lenient: it extracts the EN16931 core terms the validator
// needs and tolerates the ZATCA additions (UUID/ICV/PIH/QR/UBLExtensions) without choking.
// Missing fields become zero values, which the validator then reports as rule failures —
// that IS the audit's job, so the parser must not error on an incomplete invoice.

// ParseUBL parses a UBL 2.1 / ZATCA-UBL invoice into the normalized model.
func ParseUBL(xmlBytes []byte) (*normalized.Doc, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(xmlBytes); err != nil {
		return nil, err
	}
	root := doc.Root()
	// Accept both the UBL Invoice and CreditNote document syntaxes (ZATCA 388 invoice /
	// 381 credit note / 383 debit note). They differ in root, type-code element, and line
	// element name.
	if root == nil || (root.Tag != "Invoice" && root.Tag != "CreditNote") {
		return nil, errors.New("convert parse: root element is not a UBL Invoice or CreditNote")
	}
	typeCodeTag := "cbc:InvoiceTypeCode"
	lineTag, qtyTag := "cac:InvoiceLine", "cbc:InvoicedQuantity"
	if root.Tag == "CreditNote" {
		typeCodeTag = "cbc:CreditNoteTypeCode"
		lineTag, qtyTag = "cac:CreditNoteLine", "cbc:CreditedQuantity"
	}

	d := &normalized.Doc{
		ProfileID:       stext(root, "cbc:ProfileID"),
		CustomizationID: stext(root, "cbc:CustomizationID"),
		ID:              stext(root, "cbc:ID"),
		IssueDate:       stext(root, "cbc:IssueDate"),
		IssueTime:       stext(root, "cbc:IssueTime"),
		DueDate:         stext(root, "cbc:DueDate"),
		BillingRefID:    stext(root, "cac:BillingReference/cac:InvoiceDocumentReference/cbc:ID"),
		BillingRefDate:  stext(root, "cac:BillingReference/cac:InvoiceDocumentReference/cbc:IssueDate"),
		DeliveryDate:     stext(root, "cac:Delivery/cbc:ActualDeliveryDate"),
		PaymentMeansCode: stext(root, "cac:PaymentMeans/cbc:PaymentMeansCode"),
		PayeeIBAN:        stext(root, "cac:PaymentMeans/cac:PayeeFinancialAccount/cbc:ID"),
		PaymentTerms:     stext(root, "cac:PaymentTerms/cbc:Note"),
		BuyerReference:   stext(root, "cbc:BuyerReference"),
		OrderReference:   stext(root, "cac:OrderReference/cbc:ID"),
		SalesOrderRef:    stext(root, "cac:OrderReference/cbc:SalesOrderID"),
		TypeCode:        stext(root, typeCodeTag),
		Currency:        stext(root, "cbc:DocumentCurrencyCode"),
		TaxCurrency:     stext(root, "cbc:TaxCurrencyCode"),
		Note:            stext(root, "cbc:Note"),
	}

	// Simplified (B2C) is signalled by the type-code @name starting with "02".
	if tc := root.FindElement(typeCodeTag); tc != nil {
		if name := tc.SelectAttrValue("name", ""); len(name) >= 2 && name[:2] == "02" {
			d.Simplified = true
		}
	}

	// InvoicePeriod (BG-14): present only when cac:InvoicePeriod carries a start or end date.
	if ip := root.FindElement("cac:InvoicePeriod"); ip != nil {
		start, end := stext(ip, "cbc:StartDate"), stext(ip, "cbc:EndDate")
		if start != "" || end != "" {
			d.InvoicePeriod = &normalized.InvoicePeriod{StartDate: start, EndDate: end}
		}
	}

	if sp := root.FindElement("cac:AccountingSupplierParty/cac:Party"); sp != nil {
		d.Seller = parseParty(sp)
	}
	if cp := root.FindElement("cac:AccountingCustomerParty/cac:Party"); cp != nil {
		d.Buyer = parseParty(cp)
	}

	// Tax breakdown + document-level VAT amount. ZATCA may emit more than one TaxTotal;
	// take subtotals wherever they appear and the document TaxAmount from the one carrying it.
	for _, tt := range root.FindElements("cac:TaxTotal") {
		if amt := tt.FindElement("cbc:TaxAmount"); amt != nil && d.Totals.TaxAmount == 0 {
			d.Totals.TaxAmount = pf(amt.Text())
		}
		for _, sub := range tt.FindElements("cac:TaxSubtotal") {
			d.TaxBreakdown = append(d.TaxBreakdown, normalized.TaxSubtotal{
				Category:            stext(sub, "cac:TaxCategory/cbc:ID"),
				Rate:                ftext(sub, "cac:TaxCategory/cbc:Percent"),
				TaxableAmount:       ftext(sub, "cbc:TaxableAmount"),
				TaxAmount:           ftext(sub, "cbc:TaxAmount"),
				ExemptionReasonCode: stext(sub, "cac:TaxCategory/cbc:TaxExemptionReasonCode"),
				ExemptionReason:     stext(sub, "cac:TaxCategory/cbc:TaxExemptionReason"),
			})
		}
	}

	if m := root.FindElement("cac:LegalMonetaryTotal"); m != nil {
		d.Totals.LineExtensionAmount = ftext(m, "cbc:LineExtensionAmount")
		d.Totals.TaxExclusiveAmount = ftext(m, "cbc:TaxExclusiveAmount")
		d.Totals.TaxInclusiveAmount = ftext(m, "cbc:TaxInclusiveAmount")
		d.Totals.AllowanceTotal = ftext(m, "cbc:AllowanceTotalAmount")
		d.Totals.ChargeTotal = ftext(m, "cbc:ChargeTotalAmount")
		d.Totals.PrepaidAmount = ftext(m, "cbc:PrepaidAmount")
		d.Totals.PayableAmount = ftext(m, "cbc:PayableAmount")
	}

	// Document-level allowances (BG-20) and charges (BG-21) are direct children of the root
	// (line-level ones live under the line element, so they are not matched here).
	for _, ac := range root.FindElements("cac:AllowanceCharge") {
		d.AllowanceCharges = append(d.AllowanceCharges, normalized.AllowanceCharge{
			Charge:      stext(ac, "cbc:ChargeIndicator") == "true",
			Amount:      ftext(ac, "cbc:Amount"),
			BaseAmount:  ftext(ac, "cbc:BaseAmount"),
			Percent:     ftext(ac, "cbc:MultiplierFactorNumeric"),
			Reason:      stext(ac, "cbc:AllowanceChargeReason"),
			ReasonCode:  stext(ac, "cbc:AllowanceChargeReasonCode"),
			VATCategory: stext(ac, "cac:TaxCategory/cbc:ID"),
			VATRate:     ftext(ac, "cac:TaxCategory/cbc:Percent"),
		})
	}

	for _, ln := range root.FindElements(lineTag) {
		line := normalized.Line{
			ID:        stext(ln, "cbc:ID"),
			NetAmount: ftext(ln, "cbc:LineExtensionAmount"),
			ItemName:     stext(ln, "cac:Item/cbc:Name"),
			NetPrice:     ftext(ln, "cac:Price/cbc:PriceAmount"),
			BaseQuantity: ftext(ln, "cac:Price/cbc:BaseQuantity"),
		}
		if q := ln.FindElement(qtyTag); q != nil {
			line.Quantity = pf(q.Text())
			line.UnitCode = q.SelectAttrValue("unitCode", "")
		}
		if cat := ln.FindElement("cac:Item/cac:ClassifiedTaxCategory"); cat != nil {
			line.VATCategory = stext(cat, "cbc:ID")
			line.VATRate = ftext(cat, "cbc:Percent")
		}
		// Line-level allowances (BG-27) / charges (BG-28): direct children of the line element.
		for _, ac := range ln.FindElements("cac:AllowanceCharge") {
			line.AllowanceCharges = append(line.AllowanceCharges, normalized.AllowanceCharge{
				Charge:      stext(ac, "cbc:ChargeIndicator") == "true",
				Amount:      ftext(ac, "cbc:Amount"),
				BaseAmount:  ftext(ac, "cbc:BaseAmount"),
				Percent:     ftext(ac, "cbc:MultiplierFactorNumeric"),
				Reason:      stext(ac, "cbc:AllowanceChargeReason"),
				ReasonCode:  stext(ac, "cbc:AllowanceChargeReasonCode"),
			})
		}
		// Line invoicing period (BG-26): present only when cac:InvoicePeriod carries a date.
		if lp := ln.FindElement("cac:InvoicePeriod"); lp != nil {
			s, e := stext(lp, "cbc:StartDate"), stext(lp, "cbc:EndDate")
			if s != "" || e != "" {
				line.Period = &normalized.InvoicePeriod{StartDate: s, EndDate: e}
			}
		}
		d.Lines = append(d.Lines, line)
	}

	return d, nil
}

func parseParty(p *etree.Element) normalized.Party {
	party := normalized.Party{
		Name:        stext(p, "cac:PartyName/cbc:Name"),
		NameAr:      stext(p, "cac:PartyLegalEntity/cbc:RegistrationName"),
		VATID:       stext(p, "cac:PartyTaxScheme/cbc:CompanyID"),
		EndpointID:  stext(p, "cbc:EndpointID"),
		CountryCode: stext(p, "cac:PostalAddress/cac:Country/cbc:IdentificationCode"),
		Street:      stext(p, "cac:PostalAddress/cbc:StreetName"),
		City:        stext(p, "cac:PostalAddress/cbc:CityName"),
		PostalZone:  stext(p, "cac:PostalAddress/cbc:PostalZone"),
	}
	if party.Name == "" {
		party.Name = party.NameAr // fall back when only the legal name is present
	}
	// CompanyID carries a non-VAT party identifier: prefer the legal registration
	// (BT-30/BT-47 cac:PartyLegalEntity/cbc:CompanyID), then fall back to the generic party
	// identifier (BT-29/BT-46 cac:PartyIdentification/cbc:ID). This lets BR-CO-26 recognise a
	// seller that is identified without a VAT id (e.g. official example ubl-tc434-example7).
	party.CompanyID = stext(p, "cac:PartyLegalEntity/cbc:CompanyID")
	if party.CompanyID == "" {
		party.CompanyID = stext(p, "cac:PartyIdentification/cbc:ID")
	}
	return party
}

// stext returns the trimmed text of the element at path, or "" if absent.
func stext(el *etree.Element, path string) string {
	if e := el.FindElement(path); e != nil {
		return strings.TrimSpace(e.Text())
	}
	return ""
}

// ftext returns the float value of the element at path, or 0 if absent/unparseable.
func ftext(el *etree.Element, path string) float64 {
	if e := el.FindElement(path); e != nil {
		return pf(e.Text())
	}
	return 0
}

func pf(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}
