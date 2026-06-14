package ksa

import (
	"encoding/asn1"
	"errors"
	"math/big"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

// secp256k1.go — ZATCA-conformant CSID signing (SK-1, the #1 deep-self-kill item).
//
// WHY THIS EXISTS: the placeholder LocalSigner uses P-256 from the stdlib so the pipeline
// could run end-to-end, but ZATCA's cryptographic stamp is an ECDSA signature over the
// **secp256k1** curve (the same curve as the EGS/CSID certificate). A P-256 signature can
// never clear. This signer produces real secp256k1 signatures.
//
// SIGNATURE ENCODING (conformance detail the placeholder got wrong): ZATCA's XAdES
// SignatureValue and QR tag 7 carry the ECDSA signature in the **XMLDSig raw form** —
// r||s, each left-padded to 32 bytes (64 bytes total) — NOT ASN.1 DER. The placeholder
// emitted DER via ecdsa.SignASN1, which ZATCA rejects. We serialize to DER internally
// (the library's native form) then re-encode to fixed-width raw r||s.
//
// LOW-S: decred's ecdsa.Sign is RFC6979-deterministic and already emits canonical low-S
// signatures, which XMLDSig/ZATCA require (high-S is non-canonical and rejected).
//
// KEY CUSTODY: NewSecp256k1Signer generates an in-process key for tests/dev. Production
// MUST back this with the KMS/HSM-held CSID private key (key never exported) and supply
// the public key from the issued CSID certificate — see KMSSigner stub + STATUS BLOCKED
// (Production CSID is creds-gated).

const curveSecp256k1 = "secp256k1"

// scalarLen is the byte width of a secp256k1 field/scalar element (256 bits).
const scalarLen = 32

// Secp256k1Signer is an in-process secp256k1 ECDSA signer for tests/dev.
type Secp256k1Signer struct {
	priv *secp256k1.PrivateKey
}

// NewSecp256k1Signer generates a fresh secp256k1 key (dev/test only).
func NewSecp256k1Signer() (*Secp256k1Signer, error) {
	priv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	return &Secp256k1Signer{priv: priv}, nil
}

// NewSecp256k1SignerFromBytes loads a 32-byte secp256k1 scalar (for deterministic fixtures).
func NewSecp256k1SignerFromBytes(d []byte) (*Secp256k1Signer, error) {
	if len(d) != scalarLen {
		return nil, errors.New("ksa secp256k1: private key must be 32 bytes")
	}
	priv := secp256k1.PrivKeyFromBytes(d)
	return &Secp256k1Signer{priv: priv}, nil
}

func (s *Secp256k1Signer) CurveName() string { return curveSecp256k1 }

// Sign returns the XMLDSig raw r||s signature (64 bytes) over digest plus the SEC1
// uncompressed public key (65 bytes: 0x04||X||Y). These feed QR tags 7/8 and the XAdES
// SignatureValue / KeyInfo. (Production substitutes the CSID cert's public key for tag 8.)
func (s *Secp256k1Signer) Sign(digest []byte) (signature []byte, publicKey []byte, err error) {
	if len(digest) == 0 {
		return nil, nil, errors.New("ksa secp256k1: empty digest")
	}
	sig := ecdsa.Sign(s.priv, digest)
	raw, err := derToRawSig(sig.Serialize())
	if err != nil {
		return nil, nil, err
	}
	return raw, s.priv.PubKey().SerializeUncompressed(), nil
}

// Verify checks an XMLDSig raw r||s signature against this signer's public key (test helper).
func (s *Secp256k1Signer) Verify(digest, raw []byte) bool {
	sig, err := rawSigToDecred(raw)
	if err != nil {
		return false
	}
	return sig.Verify(digest, s.priv.PubKey())
}

// PublicKeySEC1 returns the uncompressed SEC1 public key point (0x04||X||Y).
func (s *Secp256k1Signer) PublicKeySEC1() []byte { return s.priv.PubKey().SerializeUncompressed() }

// VerifySecp256k1 verifies an XMLDSig raw r||s signature over digest against a SEC1
// public key (uncompressed 0x04||X||Y, or compressed). Used by the XAdES verifier, which
// recovers the key from ds:KeyInfo rather than holding the private key.
func VerifySecp256k1(pubSEC1, digest, raw []byte) bool {
	pub, err := secp256k1.ParsePubKey(pubSEC1)
	if err != nil {
		return false
	}
	sig, err := rawSigToDecred(raw)
	if err != nil {
		return false
	}
	return sig.Verify(digest, pub)
}

// derToRawSig converts an ASN.1 DER ECDSA signature to the XMLDSig fixed-width r||s form.
func derToRawSig(der []byte) ([]byte, error) {
	var parsed struct{ R, S *big.Int }
	if _, err := asn1.Unmarshal(der, &parsed); err != nil {
		return nil, err
	}
	if parsed.R == nil || parsed.S == nil {
		return nil, errors.New("ksa secp256k1: malformed DER signature")
	}
	out := make([]byte, 2*scalarLen)
	parsed.R.FillBytes(out[:scalarLen])
	parsed.S.FillBytes(out[scalarLen:])
	return out, nil
}

// rawSigToDecred parses a fixed-width r||s signature back into a decred ECDSA signature.
func rawSigToDecred(raw []byte) (*ecdsa.Signature, error) {
	if len(raw) != 2*scalarLen {
		return nil, errors.New("ksa secp256k1: raw signature must be 64 bytes")
	}
	var r, s secp256k1.ModNScalar
	if r.SetByteSlice(raw[:scalarLen]) {
		return nil, errors.New("ksa secp256k1: r overflows curve order")
	}
	if s.SetByteSlice(raw[scalarLen:]) {
		return nil, errors.New("ksa secp256k1: s overflows curve order")
	}
	return ecdsa.NewSignature(&r, &s), nil
}
