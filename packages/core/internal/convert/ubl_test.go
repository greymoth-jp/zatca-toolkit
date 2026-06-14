package convert

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

func sampleDoc() *normalized.Doc {
	return &normalized.Doc{
		ProfileID:       "urn:fdc:peppol.eu:2017:poacc:billing:01:1.0",
		CustomizationID: "urn:cen.eu:en16931:2017",
		ID:              "INV-0001",
		IssueDate:       "2026-06-14",
		TypeCode:        "380",
		Currency:        "SAR",
		Seller:          normalized.Party{Name: "Acme Trading LLC", VATID: "300000000000003", CountryCode: "SA", EndpointID: "300000000000003"},
		Buyer:           normalized.Party{Name: "Beta Retail Co", VATID: "311111111111113", CountryCode: "SA", EndpointID: "311111111111113"},
		Lines:           []normalized.Line{{ID: "1", Quantity: 2, UnitCode: "PCE", ItemName: "Widget", NetPrice: 50, NetAmount: 100}},
		TaxBreakdown:    []normalized.TaxSubtotal{{Category: "S", Rate: 15, TaxableAmount: 100, TaxAmount: 15}},
		Totals:          normalized.Totals{LineExtensionAmount: 100, TaxExclusiveAmount: 100, TaxAmount: 15, TaxInclusiveAmount: 115, PayableAmount: 115},
	}
}

// FR-C01: normalized -> UBL 2.1. Output must be well-formed XML carrying the key
// EN16931 terms (ID, currency, totals, line).
func TestToUBLWellFormed(t *testing.T) {
	out, err := ToUBL(sampleDoc())
	if err != nil {
		t.Fatalf("ToUBL error: %v", err)
	}
	// Must parse back as well-formed XML.
	var probe struct{ XMLName xml.Name }
	if err := xml.Unmarshal(out, &probe); err != nil {
		t.Fatalf("output is not well-formed XML: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"<Invoice", "cbc:ID>INV-0001", "DocumentCurrencyCode>SAR",
		"PayableAmount currencyID=\"SAR\">115", "cac:InvoiceLine", "Widget",
		"urn:oasis:names:specification:ubl:schema:xsd:Invoice-2",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("UBL output missing %q\n---\n%s", want, s)
		}
	}
	if !bytes.HasPrefix(out, []byte("<?xml")) {
		t.Error("UBL output must start with XML declaration")
	}
}

func TestToUBLDeterministic(t *testing.T) {
	a, _ := ToUBL(sampleDoc())
	b, _ := ToUBL(sampleDoc())
	if !bytes.Equal(a, b) {
		t.Fatal("UBL generation must be deterministic")
	}
}

// VAT exemption reason (BT-120/121) must be emitted in the tax category and round-trip.
func TestToUBLExemptionReason(t *testing.T) {
	d := sampleDoc()
	d.TaxBreakdown = []normalized.TaxSubtotal{
		{Category: "E", Rate: 0, TaxableAmount: 100, TaxAmount: 0, ExemptionReasonCode: "VATEX-SA-29", ExemptionReason: "Financial services"},
	}
	out, err := ToUBL(d)
	if err != nil {
		t.Fatalf("ToUBL error: %v", err)
	}
	s := string(out)
	for _, want := range []string{"cbc:TaxExemptionReasonCode>VATEX-SA-29", "cbc:TaxExemptionReason>Financial services"} {
		if !strings.Contains(s, want) {
			t.Errorf("exemption reason UBL missing %q\n---\n%s", want, s)
		}
	}
	pd, err := ParseUBL(out)
	if err != nil {
		t.Fatalf("ParseUBL error: %v", err)
	}
	if len(pd.TaxBreakdown) != 1 || pd.TaxBreakdown[0].ExemptionReasonCode != "VATEX-SA-29" ||
		pd.TaxBreakdown[0].ExemptionReason != "Financial services" {
		t.Errorf("exemption reason not round-tripped: %+v", pd.TaxBreakdown)
	}
}

// Line-level allowances/charges must be emitted inside the line and round-trip.
func TestToUBLLineAllowance(t *testing.T) {
	d := sampleDoc()
	d.Lines[0].AllowanceCharges = []normalized.AllowanceCharge{
		{Amount: 5, Reason: "Line discount", ReasonCode: "95"},
	}
	out, err := ToUBL(d)
	if err != nil {
		t.Fatalf("ToUBL error: %v", err)
	}
	s := string(out)
	// The allowance must be inside the line, before cac:Item.
	if !strings.Contains(s, "cac:InvoiceLine") || !strings.Contains(s, "AllowanceChargeReason>Line discount") {
		t.Errorf("line allowance not emitted\n---\n%s", s)
	}
	pd, err := ParseUBL(out)
	if err != nil {
		t.Fatalf("ParseUBL error: %v", err)
	}
	if len(pd.Lines) != 1 || len(pd.Lines[0].AllowanceCharges) != 1 ||
		pd.Lines[0].AllowanceCharges[0].Amount != 5 || pd.Lines[0].AllowanceCharges[0].Charge {
		t.Errorf("line allowance not round-tripped: %+v", pd.Lines)
	}
}

// Delivery date, payment means and payment terms must be emitted in UBL and round-trip.
func TestToUBLPaymentAndDelivery(t *testing.T) {
	d := sampleDoc()
	d.DeliveryDate = "2026-06-13"
	d.PaymentMeansCode = "30" // credit transfer
	d.PayeeIBAN = "SA0380000000608010167519"
	d.PaymentTerms = "Net 30 days"
	d.BuyerReference = "BR-77"
	d.OrderReference = "PO-2026-9"
	out, err := ToUBL(d)
	if err != nil {
		t.Fatalf("ToUBL error: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"cac:Delivery", "cbc:ActualDeliveryDate>2026-06-13", "cac:PaymentMeans",
		"cbc:PaymentMeansCode>30", "cac:PayeeFinancialAccount", "cac:PaymentTerms", "cbc:Note>Net 30 days",
		"cbc:BuyerReference>BR-77", "cac:OrderReference", "cbc:ID>PO-2026-9",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("payment/delivery UBL missing %q\n---\n%s", want, s)
		}
	}
	pd, err := ParseUBL(out)
	if err != nil {
		t.Fatalf("ParseUBL error: %v", err)
	}
	if pd.DeliveryDate != "2026-06-13" || pd.PaymentMeansCode != "30" ||
		pd.PayeeIBAN != "SA0380000000608010167519" || pd.PaymentTerms != "Net 30 days" {
		t.Errorf("payment/delivery not round-tripped: %+v", pd)
	}
	if pd.BuyerReference != "BR-77" || pd.OrderReference != "PO-2026-9" {
		t.Errorf("buyer/order reference not round-tripped: buyerRef=%q orderRef=%q", pd.BuyerReference, pd.OrderReference)
	}
}

// Invoice period (BG-14) must be emitted as cac:InvoicePeriod and round-trip.
func TestToUBLInvoicePeriod(t *testing.T) {
	d := sampleDoc()
	d.InvoicePeriod = &normalized.InvoicePeriod{StartDate: "2026-05-01", EndDate: "2026-05-31"}
	out, err := ToUBL(d)
	if err != nil {
		t.Fatalf("ToUBL error: %v", err)
	}
	s := string(out)
	for _, want := range []string{"cac:InvoicePeriod", "cbc:StartDate>2026-05-01", "cbc:EndDate>2026-05-31"} {
		if !strings.Contains(s, want) {
			t.Errorf("invoice period UBL missing %q\n---\n%s", want, s)
		}
	}
	pd, err := ParseUBL(out)
	if err != nil {
		t.Fatalf("ParseUBL error: %v", err)
	}
	if pd.InvoicePeriod == nil || pd.InvoicePeriod.StartDate != "2026-05-01" || pd.InvoicePeriod.EndDate != "2026-05-31" {
		t.Errorf("invoice period not round-tripped: %+v", pd.InvoicePeriod)
	}
	// No period -> no element, nil after parse (FP-safe).
	out2, _ := ToUBL(sampleDoc())
	if strings.Contains(string(out2), "cac:InvoicePeriod") {
		t.Error("invoice without a period must not emit cac:InvoicePeriod")
	}
	pd2, _ := ParseUBL(out2)
	if pd2.InvoicePeriod != nil {
		t.Errorf("invoice with no period must parse to nil InvoicePeriod, got %+v", pd2.InvoicePeriod)
	}
}

// Document-level allowances/charges must be emitted as cac:AllowanceCharge and round-trip.
func TestToUBLAllowanceCharge(t *testing.T) {
	d := sampleDoc()
	d.AllowanceCharges = []normalized.AllowanceCharge{
		{Amount: 10, BaseAmount: 100, Percent: 10, Reason: "Volume discount", ReasonCode: "95", VATCategory: "S", VATRate: 15},
		{Charge: true, Amount: 5, Reason: "Freight", VATCategory: "S", VATRate: 15},
	}
	d.Totals.AllowanceTotal = 10
	d.Totals.ChargeTotal = 5
	out, err := ToUBL(d)
	if err != nil {
		t.Fatalf("ToUBL error: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"cac:AllowanceCharge", "cbc:ChargeIndicator>false", "cbc:ChargeIndicator>true",
		"AllowanceChargeReason>Volume discount", "cbc:MultiplierFactorNumeric>10", "cbc:BaseAmount",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("allowance/charge UBL missing %q\n---\n%s", want, s)
		}
	}
	pd, err := ParseUBL(out)
	if err != nil {
		t.Fatalf("ParseUBL error: %v", err)
	}
	if len(pd.AllowanceCharges) != 2 {
		t.Fatalf("round-trip allowance/charge count = %d, want 2: %+v", len(pd.AllowanceCharges), pd.AllowanceCharges)
	}
	if pd.AllowanceCharges[0].Charge || !pd.AllowanceCharges[1].Charge {
		t.Errorf("charge indicators wrong: %+v", pd.AllowanceCharges)
	}
	if pd.AllowanceCharges[0].Amount != 10 || pd.AllowanceCharges[0].Reason != "Volume discount" {
		t.Errorf("allowance not round-tripped: %+v", pd.AllowanceCharges[0])
	}
}

// LegalMonetaryTotal must emit the optional allowance/charge/prepaid totals (BT-107/108/113)
// when non-zero and round-trip them; a plain invoice must omit them entirely.
func TestToUBLMonetaryTotals(t *testing.T) {
	// Plain invoice: none of the optional totals are emitted.
	plain, _ := ToUBL(sampleDoc())
	for _, unwanted := range []string{"cbc:AllowanceTotalAmount", "cbc:ChargeTotalAmount", "cbc:PrepaidAmount"} {
		if strings.Contains(string(plain), unwanted) {
			t.Errorf("a plain invoice must not emit %q", unwanted)
		}
	}

	// With totals set, all three appear and round-trip.
	d := sampleDoc()
	d.Totals.AllowanceTotal = 10
	d.Totals.ChargeTotal = 5
	d.Totals.PrepaidAmount = 40
	d.Totals.TaxExclusiveAmount = 95 // 100 - 10 + 5
	d.Totals.TaxAmount = 14.25
	d.Totals.TaxInclusiveAmount = 109.25
	d.Totals.PayableAmount = 69.25 // 109.25 - 40
	d.TaxBreakdown = []normalized.TaxSubtotal{{Category: "S", Rate: 15, TaxableAmount: 95, TaxAmount: 14.25}}
	out, err := ToUBL(d)
	if err != nil {
		t.Fatalf("ToUBL error: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"cbc:AllowanceTotalAmount currencyID=\"SAR\">10",
		"cbc:ChargeTotalAmount currencyID=\"SAR\">5",
		"cbc:PrepaidAmount currencyID=\"SAR\">40",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("monetary totals UBL missing %q\n---\n%s", want, s)
		}
	}
	pd, err := ParseUBL(out)
	if err != nil {
		t.Fatalf("ParseUBL error: %v", err)
	}
	if pd.Totals.AllowanceTotal != 10 || pd.Totals.ChargeTotal != 5 || pd.Totals.PrepaidAmount != 40 {
		t.Errorf("monetary totals not round-tripped: %+v", pd.Totals)
	}
}

// A credit note (type 381) must be emitted in the UBL CreditNote document syntax, and must
// round-trip back through ParseUBL to the same type and lines.
func TestToUBLCreditNote(t *testing.T) {
	d := sampleDoc()
	d.ID = "CN-0001"
	d.TypeCode = "381"
	d.IssueTime = "10:30:00"
	d.BillingRefID = "INV-0001"
	d.BillingRefDate = "2026-06-10"
	out, err := ToUBL(d)
	if err != nil {
		t.Fatalf("ToUBL credit note error: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"<CreditNote", "cbc:CreditNoteTypeCode", ">381<", "cac:CreditNoteLine",
		"cbc:CreditedQuantity", "urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2",
		"cbc:IssueTime>10:30:00", "cac:BillingReference", "cac:InvoiceDocumentReference", ">INV-0001<",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("credit note UBL missing %q\n---\n%s", want, s)
		}
	}
	if strings.Contains(s, "<Invoice") || strings.Contains(s, "cac:InvoiceLine") {
		t.Error("credit note must NOT use the Invoice document syntax")
	}
	// Round-trip: parse it back.
	pd, err := ParseUBL(out)
	if err != nil {
		t.Fatalf("ParseUBL(credit note) error: %v", err)
	}
	if pd.TypeCode != "381" {
		t.Errorf("round-trip type = %q, want 381", pd.TypeCode)
	}
	if pd.IssueTime != "10:30:00" || pd.BillingRefID != "INV-0001" || pd.BillingRefDate != "2026-06-10" {
		t.Errorf("round-trip issue time / billing ref wrong: time=%q ref=%q date=%q", pd.IssueTime, pd.BillingRefID, pd.BillingRefDate)
	}
	if len(pd.Lines) != 1 || pd.Lines[0].ItemName != "Widget" || pd.Lines[0].Quantity != 2 {
		t.Errorf("round-trip lines wrong: %+v", pd.Lines)
	}
}

// Invoice line period (BG-26) must be emitted inside the line and round-trip.
func TestToUBLLinePeriod(t *testing.T) {
	d := sampleDoc()
	d.Lines[0].Period = &normalized.InvoicePeriod{StartDate: "2026-05-01", EndDate: "2026-05-31"}
	out, err := ToUBL(d)
	if err != nil {
		t.Fatalf("ToUBL error: %v", err)
	}
	if !strings.Contains(string(out), "cbc:StartDate>2026-05-01") {
		t.Errorf("line period not emitted\n---\n%s", out)
	}
	pd, err := ParseUBL(out)
	if err != nil {
		t.Fatalf("ParseUBL error: %v", err)
	}
	if pd.Lines[0].Period == nil || pd.Lines[0].Period.StartDate != "2026-05-01" || pd.Lines[0].Period.EndDate != "2026-05-31" {
		t.Errorf("line period not round-tripped: %+v", pd.Lines[0].Period)
	}
	// No line period -> nil after parse.
	out2, _ := ToUBL(sampleDoc())
	pd2, _ := ParseUBL(out2)
	if len(pd2.Lines) > 0 && pd2.Lines[0].Period != nil {
		t.Errorf("line with no period must parse to nil, got %+v", pd2.Lines[0].Period)
	}
}
