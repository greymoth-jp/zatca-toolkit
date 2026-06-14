// Package convert renders the normalized model into syntax bindings (UBL 2.1, CII,
// Factur-X). FR-C01 = UBL 2.1. Generation is deterministic; encoding/xml guarantees
// well-formed output. Schematron validity of the output is asserted by tests against
// the normalized model that produced it.
package convert

import (
	"encoding/xml"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

// ublInvoice mirrors the subset of the OASIS UBL 2.1 Invoice we emit. Namespaces use
// the standard UBL prefixes (cbc = CommonBasicComponents, cac = CommonAggregate).
type ublInvoice struct {
	XMLName         xml.Name `xml:"Invoice"`
	XmlnsInvoice    string   `xml:"xmlns,attr"`
	XmlnsCBC        string   `xml:"xmlns:cbc,attr"`
	XmlnsCAC        string   `xml:"xmlns:cac,attr"`
	CustomizationID string   `xml:"cbc:CustomizationID"`
	ProfileID       string   `xml:"cbc:ProfileID"`
	ID              string      `xml:"cbc:ID"`
	IssueDate       string      `xml:"cbc:IssueDate"`
	IssueTime       string      `xml:"cbc:IssueTime,omitempty"`
	InvoiceTypeCode ublTypeCode `xml:"cbc:InvoiceTypeCode"`
	Note            string      `xml:"cbc:Note,omitempty"`
	CurrencyCode    string   `xml:"cbc:DocumentCurrencyCode"`
	BuyerReference  string   `xml:"cbc:BuyerReference,omitempty"`
	InvoicePeriod   *ublInvoicePeriod `xml:"cac:InvoicePeriod,omitempty"`
	OrderRef        *ublOrderRef   `xml:"cac:OrderReference,omitempty"`
	BillingRef      *ublBillingRef `xml:"cac:BillingReference,omitempty"`
	Supplier        party    `xml:"cac:AccountingSupplierParty>cac:Party"`
	Customer        party    `xml:"cac:AccountingCustomerParty>cac:Party"`
	Delivery        *ublDelivery     `xml:"cac:Delivery,omitempty"`
	PaymentMeans    *ublPaymentMeans `xml:"cac:PaymentMeans,omitempty"`
	PaymentTerms    *ublPaymentTerms `xml:"cac:PaymentTerms,omitempty"`
	AllowanceCharges []ublAllowanceCharge `xml:"cac:AllowanceCharge"`
	TaxTotal        taxTotal `xml:"cac:TaxTotal"`
	Monetary        monetary `xml:"cac:LegalMonetaryTotal"`
	Lines           []ublLine `xml:"cac:InvoiceLine"`
}

type party struct {
	EndpointID  string `xml:"cbc:EndpointID,omitempty"`
	Name        string `xml:"cac:PartyName>cbc:Name"`
	Country     string `xml:"cac:PostalAddress>cac:Country>cbc:IdentificationCode,omitempty"`
	CompanyID   string `xml:"cac:PartyTaxScheme>cbc:CompanyID,omitempty"`
	TaxScheme   string `xml:"cac:PartyTaxScheme>cac:TaxScheme>cbc:ID,omitempty"`
	LegalName   string `xml:"cac:PartyLegalEntity>cbc:RegistrationName,omitempty"`
}

type taxTotal struct {
	TaxAmount amount      `xml:"cbc:TaxAmount"`
	Subtotals []taxSub    `xml:"cac:TaxSubtotal"`
}

type taxSub struct {
	Taxable  amount `xml:"cbc:TaxableAmount"`
	TaxAmt   amount `xml:"cbc:TaxAmount"`
	Category struct {
		ID                  string  `xml:"cbc:ID"`
		Percent             float64 `xml:"cbc:Percent"`
		ExemptionReasonCode string  `xml:"cbc:TaxExemptionReasonCode,omitempty"`
		ExemptionReason     string  `xml:"cbc:TaxExemptionReason,omitempty"`
		Scheme              string  `xml:"cac:TaxScheme>cbc:ID"`
	} `xml:"cac:TaxCategory"`
}

// monetary is cac:LegalMonetaryTotal (BG-22). Element order follows the OASIS UBL 2.1 schema:
// LineExtension, TaxExclusive, TaxInclusive, AllowanceTotal, ChargeTotal, Prepaid, Payable.
// The allowance/charge/prepaid totals are pointers so they are emitted only when non-zero
// (omitempty does not apply to non-pointer structs in encoding/xml).
type monetary struct {
	LineExtension  amount  `xml:"cbc:LineExtensionAmount"`
	TaxExclusive   amount  `xml:"cbc:TaxExclusiveAmount"`
	TaxInclusive   amount  `xml:"cbc:TaxInclusiveAmount"`
	AllowanceTotal *amount `xml:"cbc:AllowanceTotalAmount,omitempty"` // BT-107
	ChargeTotal    *amount `xml:"cbc:ChargeTotalAmount,omitempty"`    // BT-108
	Prepaid        *amount `xml:"cbc:PrepaidAmount,omitempty"`        // BT-113
	Payable        amount  `xml:"cbc:PayableAmount"`
}

// optAmount returns a pointer amount for an optional monetary total, or nil when the value is
// zero so the element is omitted rather than emitted as 0.
func optAmount(currency string, v float64) *amount {
	if v == 0 {
		return nil
	}
	return &amount{currency, v}
}

// legalMonetaryTotal builds the document totals (BG-22) including the optional
// allowance/charge/prepaid totals.
func legalMonetaryTotal(d *normalized.Doc) monetary {
	return monetary{
		LineExtension:  amount{d.Currency, d.Totals.LineExtensionAmount},
		TaxExclusive:   amount{d.Currency, d.Totals.TaxExclusiveAmount},
		TaxInclusive:   amount{d.Currency, d.Totals.TaxInclusiveAmount},
		AllowanceTotal: optAmount(d.Currency, d.Totals.AllowanceTotal),
		ChargeTotal:    optAmount(d.Currency, d.Totals.ChargeTotal),
		Prepaid:        optAmount(d.Currency, d.Totals.PrepaidAmount),
		Payable:        amount{d.Currency, d.Totals.PayableAmount},
	}
}

// ublTypeCode is cbc:InvoiceTypeCode with the ZATCA transaction-type @name (e.g. "0100000"
// standard, "0200000" simplified) plus the code value (388 invoice / 381 CN / 383 DN).
type ublTypeCode struct {
	Name  string `xml:"name,attr,omitempty"`
	Value string `xml:",chardata"`
}

// ublBillingRef is cac:BillingReference: the preceding invoice a credit/debit note corrects.
type ublBillingRef struct {
	ID        string `xml:"cac:InvoiceDocumentReference>cbc:ID"`
	IssueDate string `xml:"cac:InvoiceDocumentReference>cbc:IssueDate,omitempty"`
}

// billingRef builds the BillingReference when a preceding-invoice id is present.
func billingRef(d *normalized.Doc) *ublBillingRef {
	if d.BillingRefID == "" {
		return nil
	}
	return &ublBillingRef{ID: d.BillingRefID, IssueDate: d.BillingRefDate}
}

// ublOrderRef is cac:OrderReference: the purchase order (BT-13) + optional sales order (BT-14).
type ublOrderRef struct {
	ID           string `xml:"cbc:ID"`
	SalesOrderID string `xml:"cbc:SalesOrderID,omitempty"`
}

// orderRef builds the OrderReference when a purchase-order id (BT-13) is present.
func orderRef(d *normalized.Doc) *ublOrderRef {
	if d.OrderReference == "" {
		return nil
	}
	return &ublOrderRef{ID: d.OrderReference, SalesOrderID: d.SalesOrderRef}
}

// ublInvoicePeriod is cac:InvoicePeriod (BG-14): the period the invoice covers (BT-73/BT-74).
type ublInvoicePeriod struct {
	StartDate string `xml:"cbc:StartDate,omitempty"`
	EndDate   string `xml:"cbc:EndDate,omitempty"`
}

// invoicePeriod builds the InvoicePeriod when a start or end date is present.
func invoicePeriod(d *normalized.Doc) *ublInvoicePeriod {
	if d.InvoicePeriod == nil || (d.InvoicePeriod.StartDate == "" && d.InvoicePeriod.EndDate == "") {
		return nil
	}
	return &ublInvoicePeriod{StartDate: d.InvoicePeriod.StartDate, EndDate: d.InvoicePeriod.EndDate}
}

// linePeriod builds the line-level InvoicePeriod (BG-26) when a start or end date is present.
func linePeriod(p *normalized.InvoicePeriod) *ublInvoicePeriod {
	if p == nil || (p.StartDate == "" && p.EndDate == "") {
		return nil
	}
	return &ublInvoicePeriod{StartDate: p.StartDate, EndDate: p.EndDate}
}

// ublDelivery is cac:Delivery (BG-13): the actual delivery date.
type ublDelivery struct {
	ActualDeliveryDate string `xml:"cbc:ActualDeliveryDate"`
}

// ublPaymentMeans is cac:PaymentMeans (BG-16): payment means code + optional payee account.
type ublPaymentMeans struct {
	Code           string `xml:"cbc:PaymentMeansCode"`
	PayeeAccountID string `xml:"cac:PayeeFinancialAccount>cbc:ID,omitempty"`
}

// ublPaymentTerms is cac:PaymentTerms (BT-20): free-text payment terms.
type ublPaymentTerms struct {
	Note string `xml:"cbc:Note"`
}

func delivery(d *normalized.Doc) *ublDelivery {
	if d.DeliveryDate == "" {
		return nil
	}
	return &ublDelivery{ActualDeliveryDate: d.DeliveryDate}
}

func paymentMeans(d *normalized.Doc) *ublPaymentMeans {
	if d.PaymentMeansCode == "" {
		return nil // cbc:PaymentMeansCode is mandatory inside cac:PaymentMeans
	}
	return &ublPaymentMeans{Code: d.PaymentMeansCode, PayeeAccountID: d.PayeeIBAN}
}

func paymentTerms(d *normalized.Doc) *ublPaymentTerms {
	if d.PaymentTerms == "" {
		return nil
	}
	return &ublPaymentTerms{Note: d.PaymentTerms}
}

// typeCodeName returns the ZATCA InvoiceTypeCode @name: standard vs simplified transaction.
func typeCodeName(d *normalized.Doc) string {
	if d.Simplified {
		return "0200000"
	}
	return "0100000"
}

type ublClassifiedTaxCat struct {
	ID      string  `xml:"cbc:ID"`
	Percent float64 `xml:"cbc:Percent"`
	Scheme  string  `xml:"cac:TaxScheme>cbc:ID"`
}

// ublAllowanceCharge is a document-level cac:AllowanceCharge (BG-20 allowance / BG-21 charge).
type ublAllowanceCharge struct {
	ChargeIndicator bool                 `xml:"cbc:ChargeIndicator"`
	ReasonCode      string               `xml:"cbc:AllowanceChargeReasonCode,omitempty"`
	Reason          string               `xml:"cbc:AllowanceChargeReason,omitempty"`
	Percent         float64              `xml:"cbc:MultiplierFactorNumeric,omitempty"`
	Amount          amount               `xml:"cbc:Amount"`
	BaseAmount      *amount              `xml:"cbc:BaseAmount,omitempty"`
	TaxCategory     *ublClassifiedTaxCat `xml:"cac:TaxCategory,omitempty"`
}

// toAllowanceCharges maps the normalized document-level allowances/charges to UBL.
func toAllowanceCharges(acs []normalized.AllowanceCharge, currency string) []ublAllowanceCharge {
	var out []ublAllowanceCharge
	for _, a := range acs {
		u := ublAllowanceCharge{
			ChargeIndicator: a.Charge,
			ReasonCode:      a.ReasonCode,
			Reason:          a.Reason,
			Percent:         a.Percent,
			Amount:          amount{currency, a.Amount},
		}
		if a.BaseAmount != 0 {
			u.BaseAmount = &amount{currency, a.BaseAmount}
		}
		if a.VATCategory != "" {
			u.TaxCategory = &ublClassifiedTaxCat{ID: a.VATCategory, Percent: a.VATRate, Scheme: "VAT"}
		}
		out = append(out, u)
	}
	return out
}

type ublLine struct {
	ID         string `xml:"cbc:ID"`
	Quantity   qty    `xml:"cbc:InvoicedQuantity"`
	LineAmount amount `xml:"cbc:LineExtensionAmount"`
	Period     *ublInvoicePeriod    `xml:"cac:InvoicePeriod,omitempty"`
	AllowanceCharges []ublAllowanceCharge `xml:"cac:AllowanceCharge"`
	Item       struct {
		Name                  string               `xml:"cbc:Name"`
		ClassifiedTaxCategory *ublClassifiedTaxCat `xml:"cac:ClassifiedTaxCategory,omitempty"`
	} `xml:"cac:Item"`
	Price struct {
		Amount amount `xml:"cbc:PriceAmount"`
	} `xml:"cac:Price"`
}

// classifyLine builds the line-level VAT category (BT-151/BT-152) when present.
func classifyLine(l normalized.Line) *ublClassifiedTaxCat {
	if l.VATCategory == "" {
		return nil
	}
	return &ublClassifiedTaxCat{ID: l.VATCategory, Percent: l.VATRate, Scheme: "VAT"}
}

type amount struct {
	Currency string  `xml:"currencyID,attr"`
	Value    float64 `xml:",chardata"`
}

type qty struct {
	UnitCode string  `xml:"unitCode,attr,omitempty"`
	Value    float64 `xml:",chardata"`
}

const (
	nsInvoice    = "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
	nsCreditNote = "urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2"
	nsCBC        = "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
	nsCAC        = "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
)

// ublCreditNote mirrors ublInvoice but in the UBL CreditNote document syntax (BT-3 = 381).
type ublCreditNote struct {
	XMLName            xml.Name `xml:"CreditNote"`
	XmlnsCN            string   `xml:"xmlns,attr"`
	XmlnsCBC           string   `xml:"xmlns:cbc,attr"`
	XmlnsCAC           string   `xml:"xmlns:cac,attr"`
	CustomizationID    string   `xml:"cbc:CustomizationID"`
	ProfileID          string   `xml:"cbc:ProfileID"`
	ID                 string      `xml:"cbc:ID"`
	IssueDate          string      `xml:"cbc:IssueDate"`
	IssueTime          string      `xml:"cbc:IssueTime,omitempty"`
	CreditNoteTypeCode ublTypeCode `xml:"cbc:CreditNoteTypeCode"`
	Note               string   `xml:"cbc:Note,omitempty"`
	CurrencyCode       string   `xml:"cbc:DocumentCurrencyCode"`
	BuyerReference     string   `xml:"cbc:BuyerReference,omitempty"`
	InvoicePeriod      *ublInvoicePeriod `xml:"cac:InvoicePeriod,omitempty"`
	OrderRef           *ublOrderRef   `xml:"cac:OrderReference,omitempty"`
	BillingRef         *ublBillingRef `xml:"cac:BillingReference,omitempty"`
	Supplier           party    `xml:"cac:AccountingSupplierParty>cac:Party"`
	Customer           party    `xml:"cac:AccountingCustomerParty>cac:Party"`
	Delivery           *ublDelivery     `xml:"cac:Delivery,omitempty"`
	PaymentMeans       *ublPaymentMeans `xml:"cac:PaymentMeans,omitempty"`
	PaymentTerms       *ublPaymentTerms `xml:"cac:PaymentTerms,omitempty"`
	AllowanceCharges   []ublAllowanceCharge `xml:"cac:AllowanceCharge"`
	TaxTotal           taxTotal `xml:"cac:TaxTotal"`
	Monetary           monetary `xml:"cac:LegalMonetaryTotal"`
	Lines              []ublCreditNoteLine `xml:"cac:CreditNoteLine"`
}

// ublCreditNoteLine differs from ublLine only in the quantity element (cbc:CreditedQuantity).
type ublCreditNoteLine struct {
	ID         string `xml:"cbc:ID"`
	Quantity   qty    `xml:"cbc:CreditedQuantity"`
	LineAmount amount `xml:"cbc:LineExtensionAmount"`
	Period     *ublInvoicePeriod    `xml:"cac:InvoicePeriod,omitempty"`
	AllowanceCharges []ublAllowanceCharge `xml:"cac:AllowanceCharge"`
	Item       struct {
		Name                  string               `xml:"cbc:Name"`
		ClassifiedTaxCategory *ublClassifiedTaxCat `xml:"cac:ClassifiedTaxCategory,omitempty"`
	} `xml:"cac:Item"`
	Price struct {
		Amount amount `xml:"cbc:PriceAmount"`
	} `xml:"cac:Price"`
}

// toCreditNoteUBL renders a normalized credit note (type 381) in the UBL CreditNote syntax.
func toCreditNoteUBL(d *normalized.Doc) ([]byte, error) {
	cn := ublCreditNote{
		XmlnsCN:            nsCreditNote,
		XmlnsCBC:           nsCBC,
		XmlnsCAC:           nsCAC,
		CustomizationID:    d.CustomizationID,
		ProfileID:          d.ProfileID,
		ID:                 d.ID,
		IssueDate:          d.IssueDate,
		IssueTime:          d.IssueTime,
		CreditNoteTypeCode: ublTypeCode{Name: typeCodeName(d), Value: d.TypeCode},
		Note:               d.Note,
		CurrencyCode:       d.Currency,
		BuyerReference:     d.BuyerReference,
		InvoicePeriod:      invoicePeriod(d),
		OrderRef:           orderRef(d),
		BillingRef:         billingRef(d),
		Supplier:           toParty(d.Seller),
		Customer:           toParty(d.Buyer),
		Delivery:           delivery(d),
		PaymentMeans:       paymentMeans(d),
		PaymentTerms:       paymentTerms(d),
		AllowanceCharges:   toAllowanceCharges(d.AllowanceCharges, d.Currency),
		Monetary:           legalMonetaryTotal(d),
	}
	cn.TaxTotal.TaxAmount = amount{d.Currency, d.Totals.TaxAmount}
	for _, t := range d.TaxBreakdown {
		var s taxSub
		s.Taxable = amount{d.Currency, t.TaxableAmount}
		s.TaxAmt = amount{d.Currency, t.TaxAmount}
		s.Category.ID = t.Category
		s.Category.Percent = t.Rate
		s.Category.ExemptionReasonCode = t.ExemptionReasonCode
		s.Category.ExemptionReason = t.ExemptionReason
		s.Category.Scheme = "VAT"
		cn.TaxTotal.Subtotals = append(cn.TaxTotal.Subtotals, s)
	}
	for _, l := range d.Lines {
		var ln ublCreditNoteLine
		ln.ID = l.ID
		ln.Quantity = qty{l.UnitCode, l.Quantity}
		ln.LineAmount = amount{d.Currency, l.NetAmount}
		ln.Period = linePeriod(l.Period)
		ln.AllowanceCharges = toAllowanceCharges(l.AllowanceCharges, d.Currency)
		ln.Item.Name = l.ItemName
		ln.Item.ClassifiedTaxCategory = classifyLine(l)
		ln.Price.Amount = amount{d.Currency, l.NetPrice}
		cn.Lines = append(cn.Lines, ln)
	}
	out, err := xml.MarshalIndent(cn, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}

// ToUBL renders a normalized Doc into UBL 2.1 XML bytes (with declaration). A credit note
// (type 381) uses the UBL CreditNote document syntax (different root, CreditNoteTypeCode,
// CreditNoteLine, CreditedQuantity); everything else (388 invoice, 383 debit note) uses the
// Invoice document syntax with the appropriate type code.
func ToUBL(d *normalized.Doc) ([]byte, error) {
	if d.TypeCode == "381" {
		return toCreditNoteUBL(d)
	}
	inv := ublInvoice{
		XmlnsInvoice:    nsInvoice,
		XmlnsCBC:        nsCBC,
		XmlnsCAC:        nsCAC,
		CustomizationID: d.CustomizationID,
		ProfileID:       d.ProfileID,
		ID:              d.ID,
		IssueDate:       d.IssueDate,
		IssueTime:       d.IssueTime,
		InvoiceTypeCode: ublTypeCode{Name: typeCodeName(d), Value: d.TypeCode},
		Note:            d.Note,
		CurrencyCode:    d.Currency,
		BuyerReference:  d.BuyerReference,
		InvoicePeriod:   invoicePeriod(d),
		OrderRef:        orderRef(d),
		BillingRef:      billingRef(d),
		Supplier:        toParty(d.Seller),
		Customer:        toParty(d.Buyer),
		Delivery:        delivery(d),
		PaymentMeans:    paymentMeans(d),
		PaymentTerms:    paymentTerms(d),
		AllowanceCharges: toAllowanceCharges(d.AllowanceCharges, d.Currency),
		Monetary:         legalMonetaryTotal(d),
	}
	inv.TaxTotal.TaxAmount = amount{d.Currency, d.Totals.TaxAmount}
	for _, t := range d.TaxBreakdown {
		var s taxSub
		s.Taxable = amount{d.Currency, t.TaxableAmount}
		s.TaxAmt = amount{d.Currency, t.TaxAmount}
		s.Category.ID = t.Category
		s.Category.Percent = t.Rate
		s.Category.ExemptionReasonCode = t.ExemptionReasonCode
		s.Category.ExemptionReason = t.ExemptionReason
		s.Category.Scheme = "VAT"
		inv.TaxTotal.Subtotals = append(inv.TaxTotal.Subtotals, s)
	}
	for _, l := range d.Lines {
		var ln ublLine
		ln.ID = l.ID
		ln.Quantity = qty{l.UnitCode, l.Quantity}
		ln.LineAmount = amount{d.Currency, l.NetAmount}
		ln.Period = linePeriod(l.Period)
		ln.AllowanceCharges = toAllowanceCharges(l.AllowanceCharges, d.Currency)
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

func toParty(p normalized.Party) party {
	out := party{
		EndpointID: p.EndpointID,
		Name:       p.Name,
		Country:    p.CountryCode,
		LegalName:  p.Name,
	}
	if p.VATID != "" {
		out.CompanyID = p.VATID
		out.TaxScheme = "VAT"
	}
	return out
}
