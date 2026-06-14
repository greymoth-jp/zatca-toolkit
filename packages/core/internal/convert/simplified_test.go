package convert

import (
	"strings"
	"testing"
)

// The simplified flag must round-trip via the InvoiceTypeCode @name (0100000 / 0200000).
func TestSimplifiedRoundTrip(t *testing.T) {
	d := parseSampleDoc()
	d.Simplified = true
	out, err := ToZatcaUBL(d, ZatcaUBLOpts{UUID: "U", ICV: 1, PIH: "YWJj", IssueTime: "10:30:00"})
	if err != nil {
		t.Fatalf("ToZatcaUBL: %v", err)
	}
	if !strings.Contains(string(out), `name="0200000"`) {
		t.Fatalf("simplified invoice should emit InvoiceTypeCode @name 0200000:\n%s", out)
	}
	got, err := ParseUBL(out)
	if err != nil {
		t.Fatalf("ParseUBL: %v", err)
	}
	if !got.Simplified {
		t.Fatal("parsed doc should be marked Simplified")
	}
	if got.TypeCode != "388" {
		t.Fatalf("type code value should still be 388, got %q", got.TypeCode)
	}
}

func TestStandardTypeCodeName(t *testing.T) {
	out, _ := ToUBL(parseSampleDoc()) // Simplified=false
	if !strings.Contains(string(out), `name="0100000"`) {
		t.Fatalf("standard invoice should emit InvoiceTypeCode @name 0100000:\n%s", out)
	}
	got, _ := ParseUBL(out)
	if got.Simplified {
		t.Fatal("standard invoice must not be marked Simplified")
	}
}
