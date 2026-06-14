package ksa

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func goodQR(t *testing.T, simplified bool) string {
	t.Helper()
	h := sha256.Sum256([]byte("invoice"))
	f := QRFields{
		SellerName:   "Acme Trading LLC",
		VATNumber:    "300000000000003",
		Timestamp:    "2026-06-14T10:30:00Z",
		InvoiceTotal: "115.00",
		VATTotal:     "15.00",
		InvoiceHash:  base64.StdEncoding.EncodeToString(h[:]),
		Signature:    make([]byte, 64),
		PublicKey:    make([]byte, 33),
	}
	if simplified {
		f.Stamp = make([]byte, 64)
	}
	qr, err := EncodeQR(f, simplified)
	if err != nil {
		t.Fatalf("EncodeQR: %v", err)
	}
	return qr
}

func qrIDs(qr string, signed, simplified bool) map[string]bool {
	m := map[string]bool{}
	for _, e := range ValidateQR(qr, signed, simplified) {
		m[e.RuleID] = true
	}
	return m
}

func TestValidateQRGoodPasses(t *testing.T) {
	if len(ValidateQR(goodQR(t, false), true, false)) != 0 {
		t.Fatalf("a well-formed signed QR should have no findings: %v", ValidateQR(goodQR(t, false), true, false))
	}
}

func TestValidateQRRejectsGarbage(t *testing.T) {
	if !qrIDs("not-base64-!!!", true, false)["BR-KSA-QR-01"] {
		t.Fatal("garbage QR should fail BR-KSA-QR-01")
	}
}

func TestValidateQRMissingSignatureTags(t *testing.T) {
	// build a QR without signature/key (unsigned), then validate as signed -> missing tags
	h := sha256.Sum256([]byte("x"))
	qr, _ := EncodeQR(QRFields{
		SellerName: "A", VATNumber: "300000000000003", Timestamp: "2026-06-14T10:30:00Z",
		InvoiceTotal: "115.00", VATTotal: "15.00", InvoiceHash: base64.StdEncoding.EncodeToString(h[:]),
	}, false)
	if !qrIDs(qr, true, false)["BR-KSA-QR-02"] {
		t.Fatal("signed QR missing tags 7/8 should fail BR-KSA-QR-02")
	}
}

func TestValidateQRBadVAT(t *testing.T) {
	h := sha256.Sum256([]byte("x"))
	qr, _ := EncodeQR(QRFields{
		SellerName: "A", VATNumber: "12345", Timestamp: "2026-06-14T10:30:00Z",
		InvoiceTotal: "115.00", VATTotal: "15.00", InvoiceHash: base64.StdEncoding.EncodeToString(h[:]),
		Signature: make([]byte, 64), PublicKey: make([]byte, 33),
	}, false)
	if !qrIDs(qr, true, false)["BR-KSA-QR-VAT"] {
		t.Fatal("bad VAT in QR should fail BR-KSA-QR-VAT")
	}
}

func TestValidateQRTag9OnlySimplified(t *testing.T) {
	// goodQR(simplified=true) carries tag 9; validating as standard must flag it
	if !qrIDs(goodQR(t, true), true, false)["BR-KSA-QR-09"] {
		t.Fatal("tag 9 on a standard invoice should fail BR-KSA-QR-09")
	}
	// and validating the same as simplified must NOT flag it
	if qrIDs(goodQR(t, true), true, true)["BR-KSA-QR-09"] {
		t.Fatal("tag 9 on a simplified invoice must be allowed")
	}
}
