package ksa

import (
	"strings"
	"testing"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/convert"
)

func unsignedUBL(t *testing.T) []byte {
	t.Helper()
	out, err := convert.ToZatcaUBL(ksaDoc(), convert.ZatcaUBLOpts{
		UUID: "U-XADES-1", ICV: 1, PIH: GenesisPIH, IssueTime: "10:30:00",
	})
	if err != nil {
		t.Fatalf("ToZatcaUBL: %v", err)
	}
	return out
}

func TestXAdESBuildAndSelfVerify(t *testing.T) {
	signer, err := NewSecp256k1Signer()
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	signed, err := BuildSignedUBL(unsignedUBL(t), signer, XAdESParams{
		SigningTime: "2026-06-14T10:30:00Z", IssuerName: "CN=ZATCA-Dev-Placeholder", Serial: "1",
	})
	if err != nil {
		t.Fatalf("BuildSignedUBL: %v", err)
	}

	s := string(signed)
	for _, want := range []string{"ds:Signature", "ds:SignedInfo", "ds:SignatureValue", "xades:SignedProperties", "ds:X509Certificate"} {
		if !strings.Contains(s, want) {
			t.Fatalf("signed UBL missing %q", want)
		}
	}

	ok, err := VerifySignedUBL(signed, signer.PublicKeySEC1())
	if err != nil {
		t.Fatalf("VerifySignedUBL error: %v", err)
	}
	if !ok {
		t.Fatal("self-verify failed on a freshly built XAdES signature")
	}
}

func TestXAdESDetectsTamperedContent(t *testing.T) {
	signer, _ := NewSecp256k1Signer()
	signed, err := BuildSignedUBL(unsignedUBL(t), signer, XAdESParams{SigningTime: "2026-06-14T10:30:00Z", Serial: "1"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// Tamper invoice content AFTER signing → document Reference digest must no longer match.
	tampered := strings.Replace(string(signed), "Acme Trading LLC", "Evil Corp", 1)
	if tampered == string(signed) {
		t.Skip("seller name not present to tamper")
	}
	ok, err := VerifySignedUBL([]byte(tampered), signer.PublicKeySEC1())
	if err != nil {
		t.Fatalf("verify tampered: %v", err)
	}
	if ok {
		t.Fatal("verification accepted a tampered invoice")
	}
}

func TestXAdESRejectsWrongKey(t *testing.T) {
	signer, _ := NewSecp256k1Signer()
	other, _ := NewSecp256k1Signer()
	signed, err := BuildSignedUBL(unsignedUBL(t), signer, XAdESParams{SigningTime: "2026-06-14T10:30:00Z", Serial: "1"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	ok, _ := VerifySignedUBL(signed, other.PublicKeySEC1())
	if ok {
		t.Fatal("verification accepted the wrong public key")
	}
}

// End-to-end: the full ProcessStandard pipeline (generate → canonical hash → secp256k1
// XAdES → QR → clear) must produce a SignedXML that verifies. Ties P0a-1/2/3 together.
func TestProcessStandardProducesVerifiableXAdES(t *testing.T) {
	signer, err := NewSecp256k1Signer()
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	client := &MockClient{Responses: []ClearResult{{Outcome: OutcomeCleared, UUID: "Z-E2E-1"}}}
	res, err := ProcessStandard(Input{Doc: ksaDoc(), UUID: "Z-E2E-1", IssueTime: "10:30:00"}, signer, client)
	if err != nil {
		t.Fatalf("ProcessStandard: %v", err)
	}
	ok, err := VerifySignedUBL(res.SignedXML, signer.PublicKeySEC1())
	if err != nil {
		t.Fatalf("VerifySignedUBL: %v", err)
	}
	if !ok {
		t.Fatal("pipeline-produced SignedXML did not verify")
	}
}

// The document Reference digest inside the XAdES MUST equal the canonical invoice hash,
// i.e. the same value carried by QR tag 6 and the PIH chain.
func TestXAdESDocDigestEqualsInvoiceHash(t *testing.T) {
	signer, _ := NewSecp256k1Signer()
	ubl := unsignedUBL(t)
	invoiceHash, err := CanonicalInvoiceHashB64(ubl)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	signed, err := BuildSignedUBL(ubl, signer, XAdESParams{SigningTime: "2026-06-14T10:30:00Z", Serial: "1"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(string(signed), invoiceHash) {
		t.Fatalf("XAdES document Reference digest does not carry the invoice hash %q", invoiceHash)
	}
}
