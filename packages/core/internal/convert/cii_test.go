package convert

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

// FR-C02: normalized -> CII (UN/CEFACT). Output must be well-formed and carry the key
// EN16931 terms in the CII binding.
func TestToCIIWellFormed(t *testing.T) {
	out, err := ToCII(sampleDoc())
	if err != nil {
		t.Fatalf("ToCII error: %v", err)
	}
	var probe struct{ XMLName xml.Name }
	if err := xml.Unmarshal(out, &probe); err != nil {
		t.Fatalf("CII output is not well-formed XML: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"rsm:CrossIndustryInvoice",
		"urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100",
		"ram:ID>INV-0001",
		"urn:cen.eu:en16931:2017",
		`format="102">20260614`, // BT-2 issue date in CII 102 format
		"ram:GrandTotalAmount>115",
		`ram:TaxTotalAmount currencyID="SAR">15`,
		"Widget",
		`schemeID="VA">300000000000003`, // seller VAT
	} {
		if !strings.Contains(s, want) {
			t.Errorf("CII output missing %q\n---\n%s", want, s)
		}
	}
	if !bytes.HasPrefix(out, []byte("<?xml")) {
		t.Error("CII output must start with XML declaration")
	}
}

func TestToCIIDeterministic(t *testing.T) {
	a, _ := ToCII(sampleDoc())
	b, _ := ToCII(sampleDoc())
	if !bytes.Equal(a, b) {
		t.Fatal("CII generation must be deterministic")
	}
}

func TestDateToCII(t *testing.T) {
	if got := dateToCII("2026-06-14"); got != "20260614" {
		t.Errorf("dateToCII = %q, want 20260614", got)
	}
}
