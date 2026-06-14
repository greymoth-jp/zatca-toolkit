package convert

import "testing"

func structIDs(t *testing.T, xmlBytes []byte) map[string]bool {
	t.Helper()
	errs, err := ZatcaStructuralIssues(xmlBytes)
	if err != nil {
		t.Fatalf("ZatcaStructuralIssues: %v", err)
	}
	m := map[string]bool{}
	for _, e := range errs {
		m[e.RuleID] = true
	}
	return m
}

// A plain EN16931 UBL has none of the ZATCA structural elements -> all flagged.
func TestStructuralPlainUBLMissesEverything(t *testing.T) {
	out, _ := ToUBL(parseSampleDoc())
	ids := structIDs(t, out)
	for _, want := range []string{"BR-KSA-ST-UUID", "BR-KSA-ST-ICV", "BR-KSA-ST-PIH", "BR-KSA-ST-QR", "BR-KSA-ST-SIG"} {
		if !ids[want] {
			t.Fatalf("plain UBL should be missing %s", want)
		}
	}
}

// A ZATCA UBL (UUID/ICV/PIH/QR present) clears those, but still lacks the XAdES signature
// until it is signed -> only BR-KSA-ST-SIG remains.
func TestStructuralZatcaUBLOnlyMissesSignature(t *testing.T) {
	out, err := ToZatcaUBL(parseSampleDoc(), ZatcaUBLOpts{UUID: "U-1", ICV: 1, PIH: "YWJj", IssueTime: "10:30:00", QR: "cXItZGF0YQ=="})
	if err != nil {
		t.Fatalf("ToZatcaUBL: %v", err)
	}
	ids := structIDs(t, out)
	for _, gone := range []string{"BR-KSA-ST-UUID", "BR-KSA-ST-ICV", "BR-KSA-ST-PIH", "BR-KSA-ST-QR"} {
		if ids[gone] {
			t.Fatalf("ZATCA UBL should NOT flag %s (it is present)", gone)
		}
	}
	if !ids["BR-KSA-ST-SIG"] {
		t.Fatal("unsigned ZATCA UBL should still flag the missing signature (BR-KSA-ST-SIG)")
	}
}

func TestStructuralRejectsNonInvoice(t *testing.T) {
	if _, err := ZatcaStructuralIssues([]byte(`<?xml version="1.0"?><Order/>`)); err == nil {
		t.Fatal("expected error for non-Invoice root")
	}
}
