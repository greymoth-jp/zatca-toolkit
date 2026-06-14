package convert

import (
	"encoding/xml"
	"strings"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

// FR-C02 — UN/CEFACT Cross Industry Invoice (CII) D16B. This is the EN16931 CII
// syntax binding and is also the XML embedded inside a Factur-X PDF/A-3 (FR-C03).
// We emit the EN16931 (COMFORT) profile subset.

const (
	nsRSM = "urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
	nsRAM = "urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100"
	nsUDT = "urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100"
	// EN16931 CII guideline id (Factur-X EN16931/COMFORT).
	ciiGuideline = "urn:cen.eu:en16931:2017"
)

type ciiInvoice struct {
	XMLName xml.Name `xml:"rsm:CrossIndustryInvoice"`
	XmlnsRSM string  `xml:"xmlns:rsm,attr"`
	XmlnsRAM string  `xml:"xmlns:ram,attr"`
	XmlnsUDT string  `xml:"xmlns:udt,attr"`

	Context struct {
		Guideline struct {
			ID string `xml:"ram:ID"`
		} `xml:"ram:GuidelineSpecifiedDocumentContextParameter"`
	} `xml:"rsm:ExchangedDocumentContext"`

	Document struct {
		ID       string `xml:"ram:ID"`
		TypeCode string `xml:"ram:TypeCode"`
		IssueDateTime struct {
			DateTimeString ciiDate `xml:"udt:DateTimeString"`
		} `xml:"ram:IssueDateTime"`
	} `xml:"rsm:ExchangedDocument"`

	Transaction ciiTransaction `xml:"rsm:SupplyChainTradeTransaction"`
}

type ciiDate struct {
	Format string `xml:"format,attr"`
	Value  string `xml:",chardata"`
}

type ciiTransaction struct {
	Lines     []ciiLine `xml:"ram:IncludedSupplyChainTradeLineItem"`
	Agreement struct {
		Seller ciiParty `xml:"ram:SellerTradeParty"`
		Buyer  ciiParty `xml:"ram:BuyerTradeParty"`
	} `xml:"ram:ApplicableHeaderTradeAgreement"`
	Delivery struct{} `xml:"ram:ApplicableHeaderTradeDelivery"`
	Settlement ciiSettlement `xml:"ram:ApplicableHeaderTradeSettlement"`
}

type ciiParty struct {
	Name        string `xml:"ram:Name"`
	TaxReg      *ciiTaxReg `xml:"ram:SpecifiedTaxRegistration,omitempty"`
}

type ciiTaxReg struct {
	ID ciiSchemeID `xml:"ram:ID"`
}

type ciiSchemeID struct {
	Scheme string `xml:"schemeID,attr"`
	Value  string `xml:",chardata"`
}

type ciiLine struct {
	DocLine struct {
		LineID string `xml:"ram:LineID"`
	} `xml:"ram:AssociatedDocumentLineDocument"`
	Product struct {
		Name string `xml:"ram:Name"`
	} `xml:"ram:SpecifiedTradeProduct"`
	LineAgreement struct {
		NetPrice struct {
			ChargeAmount float64 `xml:"ram:ChargeAmount"`
		} `xml:"ram:NetPriceProductTradePrice"`
	} `xml:"ram:SpecifiedLineTradeAgreement"`
	LineDelivery struct {
		BilledQuantity ciiQty `xml:"ram:BilledQuantity"`
	} `xml:"ram:SpecifiedLineTradeDelivery"`
	LineSettlement struct {
		Tax ciiLineTax `xml:"ram:ApplicableTradeTax"`
		Sum struct {
			LineTotal float64 `xml:"ram:LineTotalAmount"`
		} `xml:"ram:SpecifiedTradeSettlementLineMonetarySummation"`
	} `xml:"ram:SpecifiedLineTradeSettlement"`
}

type ciiQty struct {
	UnitCode string  `xml:"unitCode,attr,omitempty"`
	Value    float64 `xml:",chardata"`
}

type ciiLineTax struct {
	TypeCode     string  `xml:"ram:TypeCode"`
	CategoryCode string  `xml:"ram:CategoryCode"`
	RatePercent  float64 `xml:"ram:RateApplicablePercent"`
}

type ciiSettlement struct {
	Currency string       `xml:"ram:InvoiceCurrencyCode"`
	Taxes    []ciiHdrTax  `xml:"ram:ApplicableTradeTax"`
	Summation ciiSummation `xml:"ram:SpecifiedTradeSettlementHeaderMonetarySummation"`
}

type ciiHdrTax struct {
	CalculatedAmount float64 `xml:"ram:CalculatedAmount"`
	TypeCode         string  `xml:"ram:TypeCode"`
	BasisAmount      float64 `xml:"ram:BasisAmount"`
	CategoryCode     string  `xml:"ram:CategoryCode"`
	RatePercent      float64 `xml:"ram:RateApplicablePercent"`
}

type ciiSummation struct {
	LineTotal     float64    `xml:"ram:LineTotalAmount"`
	TaxBasisTotal float64    `xml:"ram:TaxBasisTotalAmount"`
	TaxTotal      ciiCurAmt  `xml:"ram:TaxTotalAmount"`
	GrandTotal    float64    `xml:"ram:GrandTotalAmount"`
	DuePayable    float64    `xml:"ram:DuePayableAmount"`
}

type ciiCurAmt struct {
	Currency string  `xml:"currencyID,attr"`
	Value    float64 `xml:",chardata"`
}

// ToCII renders the normalized Doc into EN16931 CII XML bytes (with declaration).
func ToCII(d *normalized.Doc) ([]byte, error) {
	inv := ciiInvoice{XmlnsRSM: nsRSM, XmlnsRAM: nsRAM, XmlnsUDT: nsUDT}
	inv.Context.Guideline.ID = ciiGuideline
	inv.Document.ID = d.ID
	inv.Document.TypeCode = d.TypeCode
	inv.Document.IssueDateTime.DateTimeString = ciiDate{Format: "102", Value: dateToCII(d.IssueDate)}

	for _, l := range d.Lines {
		var cl ciiLine
		cl.DocLine.LineID = l.ID
		cl.Product.Name = l.ItemName
		cl.LineAgreement.NetPrice.ChargeAmount = l.NetPrice
		cl.LineDelivery.BilledQuantity = ciiQty{UnitCode: l.UnitCode, Value: l.Quantity}
		cl.LineSettlement.Tax = ciiLineTax{TypeCode: "VAT", CategoryCode: l.VATCategory, RatePercent: l.VATRate}
		cl.LineSettlement.Sum.LineTotal = l.NetAmount
		inv.Transaction.Lines = append(inv.Transaction.Lines, cl)
	}

	inv.Transaction.Agreement.Seller = toCIIParty(d.Seller)
	inv.Transaction.Agreement.Buyer = toCIIParty(d.Buyer)

	s := &inv.Transaction.Settlement
	s.Currency = d.Currency
	for _, t := range d.TaxBreakdown {
		s.Taxes = append(s.Taxes, ciiHdrTax{
			CalculatedAmount: t.TaxAmount, TypeCode: "VAT", BasisAmount: t.TaxableAmount,
			CategoryCode: t.Category, RatePercent: t.Rate,
		})
	}
	s.Summation = ciiSummation{
		LineTotal:     d.Totals.LineExtensionAmount,
		TaxBasisTotal: d.Totals.TaxExclusiveAmount,
		TaxTotal:      ciiCurAmt{Currency: d.Currency, Value: d.Totals.TaxAmount},
		GrandTotal:    d.Totals.TaxInclusiveAmount,
		DuePayable:    d.Totals.PayableAmount,
	}

	out, err := xml.MarshalIndent(inv, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}

func toCIIParty(p normalized.Party) ciiParty {
	out := ciiParty{Name: p.Name}
	if p.VATID != "" {
		out.TaxReg = &ciiTaxReg{ID: ciiSchemeID{Scheme: "VA", Value: p.VATID}}
	}
	return out
}

// dateToCII converts YYYY-MM-DD to the CII "102" format YYYYMMDD.
func dateToCII(isoDate string) string {
	return strings.ReplaceAll(isoDate, "-", "")
}
