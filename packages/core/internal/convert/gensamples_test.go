package convert

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

// TestGenerateAuditSamples (re)generates apps/audit/samples/{good,bad}.xml from the real
// engine so the demo invoices always match what the engine validates. It is a generator,
// not an assertion: it only runs when GEN_SAMPLES=1, so normal `go test ./...` skips it.
//   GEN_SAMPLES=1 go test ./internal/convert/ -run TestGenerateAuditSamples
func TestGenerateAuditSamples(t *testing.T) {
	if os.Getenv("GEN_SAMPLES") != "1" {
		t.Skip("set GEN_SAMPLES=1 to regenerate apps/audit/samples")
	}
	outDir := filepath.Join("..", "..", "..", "..", "apps", "audit", "samples")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	good := &normalized.Doc{
		ProfileID:       "urn:fdc:peppol.eu:2017:poacc:billing:01:1.0",
		CustomizationID: "urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0",
		ID:              "INV-2026-00042", IssueDate: "2026-06-14", TypeCode: "388", Currency: "SAR",
		Seller: normalized.Party{Name: "Acme Trading LLC", NameAr: "شركة أكمي للتجارة", VATID: "300000000000003", EndpointID: "300000000000003", CountryCode: "SA"},
		Buyer:  normalized.Party{Name: "Beta Retail Co", NameAr: "شركة بيتا للتجزئة", VATID: "311111111111113", EndpointID: "311111111111113", CountryCode: "SA"},
		Lines: []normalized.Line{
			{ID: "1", Quantity: 2, UnitCode: "PCE", ItemName: "Widget", NetPrice: 50, NetAmount: 100, VATCategory: "S", VATRate: 15},
			{ID: "2", Quantity: 1, UnitCode: "PCE", ItemName: "Gadget", NetPrice: 40, NetAmount: 40, VATCategory: "S", VATRate: 15},
		},
		TaxBreakdown: []normalized.TaxSubtotal{{Category: "S", Rate: 15, TaxableAmount: 140, TaxAmount: 21}},
		Totals:       normalized.Totals{LineExtensionAmount: 140, TaxExclusiveAmount: 140, TaxAmount: 21, TaxInclusiveAmount: 161, PayableAmount: 161},
	}

	// Broken on purpose: missing buyer name (BR-07) + buyer endpoint (PEPPOL-R010),
	// line sum mismatch (BR-CO-10/13), VAT total mismatch (BR-CO-15/16).
	bad := &normalized.Doc{
		ProfileID:       "urn:fdc:peppol.eu:2017:poacc:billing:01:1.0",
		CustomizationID: "urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0",
		ID:              "INV-2026-00043", IssueDate: "2026-06-14", TypeCode: "388", Currency: "SAR",
		Seller: normalized.Party{Name: "Acme Trading LLC", NameAr: "شركة أكمي للتجارة", VATID: "300000000000003", EndpointID: "300000000000003", CountryCode: "SA"},
		Buyer:  normalized.Party{VATID: "31111", CountryCode: "SA"},
		Lines: []normalized.Line{
			{ID: "1", Quantity: 2, UnitCode: "PCE", ItemName: "Widget", NetPrice: 50, NetAmount: 100},
			{ID: "2", Quantity: 1, UnitCode: "PCE", ItemName: "Gadget", NetPrice: 40, NetAmount: 40},
		},
		TaxBreakdown: []normalized.TaxSubtotal{{Category: "S", Rate: 15, TaxableAmount: 140, TaxAmount: 21}},
		Totals:       normalized.Totals{LineExtensionAmount: 50, TaxExclusiveAmount: 140, TaxAmount: 21, TaxInclusiveAmount: 999, PayableAmount: 161},
	}

	for name, d := range map[string]*normalized.Doc{"good": good, "bad": bad} {
		xmlBytes, err := ToUBL(d)
		if err != nil {
			t.Fatalf("ToUBL %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(outDir, name+".xml"), xmlBytes, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		t.Logf("wrote %s.xml (%d bytes)", name, len(xmlBytes))
	}
}
