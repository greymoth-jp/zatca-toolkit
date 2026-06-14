package ksa

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
)

// CSID signing (FR-Z03 / Z-T1). ZATCA's cryptographic stamp is an ECDSA signature
// (XAdES envelope) produced with the CSID key. This file defines the curve-agnostic
// Signer interface + a LocalSigner for tests/dev.
//
// PRODUCTION REQUIREMENTS (STATUS Open Questions — not assumed here):
//  - ZATCA curve is secp256k1 (CONFIRM vs current spec). LocalSigner uses P-256 from the
//    stdlib so the pipeline runs end-to-end without an external dep; swap to a secp256k1 +
//    KMS-backed Signer for production (the key MUST never leave the HSM/KMS).
//  - The full XAdES (ETSI EN 319 132-1) envelope embedding into ext:UBLExtensions is a
//    follow-up; here we produce the raw signature + public key that feed QR tags 7/8.

// Signer abstracts the CSID signing key. The production impl is KMS/HSM-backed: Sign sends
// a signing request and the private key is never exported.
type Signer interface {
	// Sign returns the ECDSA signature over digest plus the public key bytes (QR tags 7/8).
	Sign(digest []byte) (signature []byte, publicKey []byte, err error)
	// CurveName identifies the curve so callers can assert ZATCA conformance.
	CurveName() string
}

// LocalSigner is an in-process ECDSA signer for tests/dev ONLY.
type LocalSigner struct {
	priv  *ecdsa.PrivateKey
	curve string
}

// NewLocalSigner generates a fresh P-256 key. NOTE: placeholder curve (see file header).
func NewLocalSigner() (*LocalSigner, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &LocalSigner{priv: priv, curve: "P-256(placeholder; ZATCA=secp256k1)"}, nil
}

func (s *LocalSigner) CurveName() string { return s.curve }

// Sign produces an ASN.1 DER ECDSA signature over the digest and the PKIX-encoded public key.
func (s *LocalSigner) Sign(digest []byte) ([]byte, []byte, error) {
	if len(digest) == 0 {
		return nil, nil, errors.New("ksa sign: empty digest")
	}
	sig, err := ecdsa.SignASN1(rand.Reader, s.priv, digest)
	if err != nil {
		return nil, nil, err
	}
	pub, err := x509.MarshalPKIXPublicKey(&s.priv.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	return sig, pub, nil
}

// VerifyLocal verifies a LocalSigner signature (test helper).
func (s *LocalSigner) VerifyLocal(digest, sig []byte) bool {
	return ecdsa.VerifyASN1(&s.priv.PublicKey, digest, sig)
}
