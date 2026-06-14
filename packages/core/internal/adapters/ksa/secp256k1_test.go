package ksa

import (
	"crypto/sha256"
	"testing"
)

func TestSecp256k1SignVerifyRoundTrip(t *testing.T) {
	s, err := NewSecp256k1Signer()
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	if s.CurveName() != "secp256k1" {
		t.Fatalf("curve = %q, want secp256k1 (ZATCA conformance)", s.CurveName())
	}
	digest := sha256.Sum256([]byte("zatca-conformance-test"))
	sig, pub, err := s.Sign(digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	// XMLDSig/ZATCA require raw r||s (64 bytes), NOT DER.
	if len(sig) != 64 {
		t.Fatalf("signature len = %d, want 64 (raw r||s)", len(sig))
	}
	// SEC1 uncompressed point: 0x04 || X(32) || Y(32).
	if len(pub) != 65 || pub[0] != 0x04 {
		t.Fatalf("public key len/prefix = %d/0x%02x, want 65/0x04", len(pub), pub[0])
	}
	if !s.Verify(digest[:], sig) {
		t.Fatal("verify failed on valid signature")
	}
	// Tamper one bit → must fail.
	bad := make([]byte, len(sig))
	copy(bad, sig)
	bad[0] ^= 0x01
	if s.Verify(digest[:], bad) {
		t.Fatal("verify accepted a tampered signature")
	}
}

func TestSecp256k1Deterministic(t *testing.T) {
	// RFC6979 determinism: same key + same digest → identical signature.
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	s1, err := NewSecp256k1SignerFromBytes(key)
	if err != nil {
		t.Fatalf("from bytes: %v", err)
	}
	s2, _ := NewSecp256k1SignerFromBytes(key)
	digest := sha256.Sum256([]byte("determinism"))
	sig1, _, _ := s1.Sign(digest[:])
	sig2, _, _ := s2.Sign(digest[:])
	if string(sig1) != string(sig2) {
		t.Fatal("signatures differ: secp256k1 signing is not RFC6979-deterministic")
	}
}

// TestProcessStandardOnSecp256k1 proves the full KSA pipeline runs on the real curve.
func TestProcessStandardOnSecp256k1(t *testing.T) {
	signer, err := NewSecp256k1Signer()
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	client := &MockClient{Responses: []ClearResult{{Outcome: OutcomeCleared, UUID: "Z-SECP-1"}}}
	res, err := ProcessStandard(Input{Doc: ksaDoc(), UUID: "Z-SECP-1", IssueTime: "10:30:00"}, signer, client)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if res.Status != "cleared" {
		t.Fatalf("status = %q, want cleared", res.Status)
	}
	if res.CurveNote != "secp256k1" {
		t.Fatalf("curve note = %q, want secp256k1", res.CurveNote)
	}
	// QR tag 7 (signature) must be the 64-byte raw form once decoded.
	tags, err := DecodeQR(res.QR)
	if err != nil {
		t.Fatalf("decode qr: %v", err)
	}
	var tag7 []byte
	for _, tg := range tags {
		if tg.Tag == 7 {
			tag7 = tg.Value
		}
	}
	if len(tag7) != 64 {
		t.Fatalf("QR tag 7 len = %d, want 64 (raw r||s)", len(tag7))
	}
}
