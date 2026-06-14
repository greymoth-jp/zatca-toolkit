package ksa

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

func decodeB64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	return b
}

func ksaDoc() *normalized.Doc {
	return &normalized.Doc{
		ID: "INV-0001", IssueDate: "2026-06-14", TypeCode: "388", Currency: "SAR",
		Seller: normalized.Party{Name: "Acme Trading LLC", NameAr: "شركة أكمي للتجارة", VATID: "300000000000003"},
		Buyer:  normalized.Party{Name: "Beta Retail Co", VATID: "311111111111113"},
		Lines:  []normalized.Line{{ID: "1", Quantity: 2, ItemName: "Widget", NetPrice: 50, NetAmount: 100, VATCategory: "S", VATRate: 15}},
		TaxBreakdown: []normalized.TaxSubtotal{{Category: "S", Rate: 15, TaxableAmount: 100, TaxAmount: 15}},
		Totals: normalized.Totals{LineExtensionAmount: 100, TaxExclusiveAmount: 100, TaxAmount: 15, TaxInclusiveAmount: 115, PayableAmount: 115},
	}
}

func mustSigner(t *testing.T) *LocalSigner {
	s, err := NewLocalSigner()
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	return s
}

// End-to-end: generate ZATCA-UBL → sign → QR → clear (mock CLEARED). FR-Z01/Z03/Z04/Z05.
func TestProcessStandardClears(t *testing.T) {
	signer := mustSigner(t)
	client := &MockClient{Responses: []ClearResult{{Outcome: OutcomeCleared, UUID: "Z-UUID-1"}}}

	res, err := ProcessStandard(Input{Doc: ksaDoc(), UUID: "Z-UUID-1", IssueTime: "10:30:00"}, signer, client)
	if err != nil {
		t.Fatalf("ProcessStandard: %v", err)
	}
	if res.Status != "cleared" {
		t.Fatalf("status = %q, want cleared", res.Status)
	}
	if res.QR == "" || res.InvoiceHash == "" {
		t.Fatal("expected QR + invoice hash")
	}
	if res.NewChain == nil || res.NewChain.ICV != 1 || res.NewChain.Hash != res.InvoiceHash {
		t.Fatalf("chain not advanced correctly: %+v", res.NewChain)
	}

	// ZATCA-UBL content (Z-T0): ICV/PIH refs, UUID, Arabic seller, UBLExtensions, QR embedded.
	xml := string(res.SignedXML)
	for _, want := range []string{
		"cbc:UUID>Z-UUID-1", "cbc:ID>ICV", "cbc:ID>PIH", "cbc:ID>QR",
		"شركة أكمي للتجارة", // Arabic seller name (BT-27)
		"ext:UBLExtensions", "reporting:1.0",
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("ZATCA-UBL missing %q", want)
		}
	}

	// QR signature (tag 7) must verify against the signed digest (tag 6 = invoice hash).
	tags, err := DecodeQR(res.QR)
	if err != nil {
		t.Fatalf("DecodeQR: %v", err)
	}
	tagMap := map[byte][]byte{}
	for _, tg := range tags {
		tagMap[tg.Tag] = tg.Value
	}
	if len(tagMap[7]) == 0 || len(tagMap[8]) == 0 {
		t.Fatal("QR must carry signature (7) + public key (8)")
	}
	// The signature in tag 7 must verify against the invoice digest (tag 6).
	if !signer.VerifyLocal(decodeB64(t, string(tagMap[6])), tagMap[7]) {
		t.Fatal("QR signature (tag 7) does not verify against invoice hash (tag 6)")
	}
}

// FR-Z08 / TC-S3: 503 → retrying (no chain advance) → cleared on retry, same UUID. No double-clear.
func TestProcessRetryThenClear(t *testing.T) {
	signer := mustSigner(t)
	client := &MockClient{Responses: []ClearResult{{Outcome: OutcomeUnavailable, HTTPStatus: 503}}}

	first, _ := ProcessStandard(Input{Doc: ksaDoc(), UUID: "U-9", IssueTime: "10:00:00"}, signer, client)
	if first.Status != "retrying" || !first.Retryable {
		t.Fatalf("expected retrying, got %q", first.Status)
	}
	if first.NewChain != nil {
		t.Fatal("chain must NOT advance on transient failure")
	}

	// retry with the SAME uuid + same prev (nil) → same ICV → cleared.
	client2 := &MockClient{Responses: []ClearResult{{Outcome: OutcomeCleared, UUID: "U-9"}}}
	retry, _ := ProcessStandard(Input{Doc: ksaDoc(), UUID: "U-9", IssueTime: "10:00:00"}, signer, client2)
	if retry.Status != "cleared" || retry.UUID != "U-9" {
		t.Fatalf("retry not cleared on same uuid: %+v", retry)
	}
	if retry.NewChain.ICV != 1 {
		t.Fatalf("retry ICV must stay 1, got %d", retry.NewChain.ICV)
	}
}

// Per ZATCA (STATUS diff #2): a REJECTED document still advances the PIH chain.
func TestRejectedStillAdvancesChain(t *testing.T) {
	signer := mustSigner(t)
	client := &MockClient{Responses: []ClearResult{{Outcome: OutcomeRejected, Errors: []string{"BR-KSA-..."}}}}
	prev := &ChainEntry{ICV: 4, Hash: "PREV"}

	res, _ := ProcessStandard(Input{Doc: ksaDoc(), UUID: "U-5", IssueTime: "10:00:00", Prev: prev}, signer, client)
	if res.Status != "rejected" {
		t.Fatalf("status = %q, want rejected", res.Status)
	}
	if res.NewChain == nil || res.NewChain.ICV != 5 {
		t.Fatalf("rejected doc must advance chain to ICV 5, got %+v", res.NewChain)
	}
}

// Simplified reporting goes through Report() and produces a valid QR.
func TestProcessSimplifiedReports(t *testing.T) {
	signer := mustSigner(t)
	client := &MockClient{Responses: []ClearResult{{Outcome: OutcomeReported, UUID: "R-1"}}}
	res, err := ProcessSimplified(Input{Doc: ksaDoc(), UUID: "R-1", IssueTime: "09:00:00"}, signer, client)
	if err != nil || res.Status != "reported" {
		t.Fatalf("simplified: status=%q err=%v", res.Status, err)
	}
	if res.QR == "" {
		t.Fatal("simplified must carry a QR")
	}
}

// The HTTP client must refuse without sandbox credentials (no fabricated live calls).
func TestHTTPClientRefusesWithoutCreds(t *testing.T) {
	c := &HTTPClient{}
	if _, err := c.Clear([]byte("<x/>"), "u", "h"); err != ErrSandboxCredentialsRequired {
		t.Fatalf("expected ErrSandboxCredentialsRequired, got %v", err)
	}
}
