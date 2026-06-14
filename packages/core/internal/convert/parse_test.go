package convert

import (
	"testing"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

func parseSampleDoc() *normalized.Doc {
	return &normalized.Doc{
		ID: "INV-PARSE-1", IssueDate: "2026-06-14", TypeCode: "388", Currency: "SAR",
		Seller: normalized.Party{Name: "Acme Trading LLC", NameAr: "شركة أكمي", VATID: "300000000000003", CountryCode: "SA"},
		Buyer:  normalized.Party{Name: "Beta Retail Co", VATID: "311111111111113", CountryCode: "SA"},
		Lines: []normalized.Line{
			{ID: "1", Quantity: 2, UnitCode: "PCE", ItemName: "Widget", NetPrice: 50, NetAmount: 100},
			{ID: "2", Quantity: 1, UnitCode: "PCE", ItemName: "Gadget", NetPrice: 40, NetAmount: 40},
		},
		TaxBreakdown: []normalized.TaxSubtotal{{Category: "S", Rate: 15, TaxableAmount: 140, TaxAmount: 21}},
		Totals:       normalized.Totals{LineExtensionAmount: 140, TaxExclusiveAmount: 140, TaxAmount: 21, TaxInclusiveAmount: 161, PayableAmount: 161},
	}
}

func TestParseUBLRoundTrip(t *testing.T) {
	src := parseSampleDoc()
	xmlBytes, err := ToUBL(src)
	if err != nil {
		t.Fatalf("ToUBL: %v", err)
	}
	got, err := ParseUBL(xmlBytes)
	if err != nil {
		t.Fatalf("ParseUBL: %v", err)
	}

	if got.ID != src.ID || got.Currency != src.Currency || got.TypeCode != src.TypeCode {
		t.Fatalf("header mismatch: %+v", got)
	}
	if got.Seller.VATID != src.Seller.VATID {
		t.Fatalf("seller VATID = %q, want %q", got.Seller.VATID, src.Seller.VATID)
	}
	if got.Buyer.VATID != src.Buyer.VATID {
		t.Fatalf("buyer VATID = %q, want %q", got.Buyer.VATID, src.Buyer.VATID)
	}
	if len(got.Lines) != 2 || got.Lines[0].NetAmount != 100 || got.Lines[1].ItemName != "Gadget" {
		t.Fatalf("lines mismatch: %+v", got.Lines)
	}
	if len(got.TaxBreakdown) != 1 || got.TaxBreakdown[0].TaxAmount != 21 || got.TaxBreakdown[0].Rate != 15 {
		t.Fatalf("tax breakdown mismatch: %+v", got.TaxBreakdown)
	}
	if got.Totals.TaxInclusiveAmount != 161 || got.Totals.TaxAmount != 21 {
		t.Fatalf("totals mismatch: %+v", got.Totals)
	}
}

// The parser must also tolerate ZATCA-UBL (extra UUID/ICV/PIH/UBLExtensions) without error.
func TestParseZatcaUBL(t *testing.T) {
	src := parseSampleDoc()
	xmlBytes, err := ToZatcaUBL(src, ZatcaUBLOpts{UUID: "U-1", ICV: 7, PIH: "abc==", IssueTime: "10:30:00"})
	if err != nil {
		t.Fatalf("ToZatcaUBL: %v", err)
	}
	got, err := ParseUBL(xmlBytes)
	if err != nil {
		t.Fatalf("ParseUBL(zatca): %v", err)
	}
	if got.Seller.VATID != src.Seller.VATID {
		t.Fatalf("zatca seller VATID = %q, want %q", got.Seller.VATID, src.Seller.VATID)
	}
	if got.Totals.TaxInclusiveAmount != 161 {
		t.Fatalf("zatca total = %v, want 161", got.Totals.TaxInclusiveAmount)
	}
	if len(got.Lines) != 2 {
		t.Fatalf("zatca lines = %d, want 2", len(got.Lines))
	}
}

func TestParseUBLRejectsNonInvoice(t *testing.T) {
	if _, err := ParseUBL([]byte(`<?xml version="1.0"?><Order/>`)); err == nil {
		t.Fatal("expected error for non-Invoice root")
	}
}
