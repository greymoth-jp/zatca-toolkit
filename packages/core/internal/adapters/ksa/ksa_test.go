package ksa

import (
	"bytes"
	"errors"
	"testing"
)

// FR-Z02: PIH chain — each invoice's PIH is the prior invoice's hash; first uses genesis.
func TestChainLinkAndHash(t *testing.T) {
	// genesis
	if got := NextPIH(nil); got != GenesisPIH {
		t.Fatalf("first PIH must be genesis")
	}
	if NextICV(nil) != 1 {
		t.Fatalf("first ICV must be 1")
	}

	xml1 := []byte("<Invoice>1</Invoice>")
	e1 := Append(nil, xml1)
	if e1.ICV != 1 || e1.Hash != Hash(xml1) {
		t.Fatalf("append1 wrong: %+v", e1)
	}

	// second doc must reference e1.Hash and ICV 2
	if NextPIH(&e1) != e1.Hash {
		t.Fatal("PIH of doc2 must equal hash of doc1")
	}
	if err := ValidateLink(2, e1.Hash, &e1); err != nil {
		t.Fatalf("valid link rejected: %v", err)
	}
}

// FR-Z07 / TC-S4: a broken PIH or ICV gap is rejected (icv_pih_chain_broken).
func TestChainBreakDetected(t *testing.T) {
	prev := ChainEntry{ICV: 5, Hash: "PREVHASH"}

	// wrong PIH
	if err := ValidateLink(6, "TAMPERED", &prev); !errors.Is(err, ErrPIHMismatch) {
		t.Fatalf("expected ErrPIHMismatch, got %v", err)
	}
	// ICV gap
	if err := ValidateLink(8, "PREVHASH", &prev); !errors.Is(err, ErrICVGap) {
		t.Fatalf("expected ErrICVGap, got %v", err)
	}
	// correct
	if err := ValidateLink(6, "PREVHASH", &prev); err != nil {
		t.Fatalf("correct link rejected: %v", err)
	}
}

func TestHashDeterministicAndBase64(t *testing.T) {
	h1 := Hash([]byte("abc"))
	h2 := Hash([]byte("abc"))
	if h1 != h2 {
		t.Fatal("hash must be deterministic")
	}
	// SHA-256 base64 is 44 chars (32 bytes -> 44 base64 incl padding)
	if len(h1) != 44 {
		t.Errorf("base64 SHA-256 len = %d, want 44", len(h1))
	}
}

// FR-Z04: QR TLV round-trips and carries tags 1-6 from data; 7/8 when signed; 9 simplified-only.
func TestQRTLVRoundTrip(t *testing.T) {
	f := QRFields{
		SellerName:   "Acme",
		VATNumber:    "300000000000003",
		Timestamp:    "2026-06-14T10:00:00Z",
		InvoiceTotal: "115.00",
		VATTotal:     "15.00",
		InvoiceHash:  Hash([]byte("<Invoice/>")),
		Signature:    []byte{0x01, 0x02, 0x03},
		PublicKey:    []byte{0x0a, 0x0b},
		Stamp:        []byte{0xff},
	}

	// standard: tags 1-8, NO tag 9
	std, err := EncodeQR(f, false)
	if err != nil {
		t.Fatalf("EncodeQR std: %v", err)
	}
	tags, err := DecodeQR(std)
	if err != nil {
		t.Fatalf("DecodeQR: %v", err)
	}
	got := map[byte][]byte{}
	for _, tg := range tags {
		got[tg.Tag] = tg.Value
	}
	if len(tags) != 8 {
		t.Fatalf("standard QR must have 8 tags, got %d", len(tags))
	}
	if string(got[1]) != "Acme" || string(got[2]) != "300000000000003" {
		t.Errorf("tag1/2 wrong: %q %q", got[1], got[2])
	}
	if string(got[4]) != "115.00" || string(got[5]) != "15.00" {
		t.Errorf("tag4/5 wrong")
	}
	if !bytes.Equal(got[7], []byte{1, 2, 3}) || !bytes.Equal(got[8], []byte{0x0a, 0x0b}) {
		t.Errorf("tag7/8 signature/pubkey not preserved")
	}
	if _, ok := got[9]; ok {
		t.Error("standard QR must NOT contain tag 9 (stamp)")
	}

	// simplified: includes tag 9
	simp, _ := EncodeQR(f, true)
	stags, _ := DecodeQR(simp)
	has9 := false
	for _, tg := range stags {
		if tg.Tag == 9 {
			has9 = true
		}
	}
	if !has9 {
		t.Error("simplified QR must contain tag 9 (stamp)")
	}
}
