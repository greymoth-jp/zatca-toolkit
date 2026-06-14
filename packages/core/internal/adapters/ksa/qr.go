package ksa

import (
	"encoding/base64"
	"errors"
)

// QR (TLV/Base64) generation per ZATCA (FR-Z04). The QR is a TLV byte string —
// [tag:1][len:1][value:len] per field — then Base64-encoded.
//
// Tags (verified vs ZATCA QR guide, 2026-06; see STATUS):
//  1 seller name, 2 VAT number, 3 timestamp (ISO8601), 4 invoice total (incl VAT),
//  5 VAT total, 6 invoice hash (base64 SHA-256), 7 ECDSA signature, 8 ECDSA public key,
//  9 stamp signature — **simplified invoices only** (NOT standard; STATUS diff #1).
//
// Tags 1–6 derive from invoice data; 7/8/9 are supplied by the CSID signer (Z-T1, KMS).
// All ZATCA QR values fit in a single length byte (≤255), incl. P-256/secp256k1 keys/sigs.

type QRFields struct {
	SellerName   string // tag 1
	VATNumber    string // tag 2
	Timestamp    string // tag 3 (ISO8601, MUST equal the invoice timestamp — clock-skew safe)
	InvoiceTotal string // tag 4 (TaxInclusiveAmount)
	VATTotal     string // tag 5 (total VAT)
	InvoiceHash  string // tag 6 (base64 SHA-256 of signed XML)
	Signature    []byte // tag 7 (from signer)
	PublicKey    []byte // tag 8 (from signer)
	Stamp        []byte // tag 9 (simplified only, from ZATCA CA via signer)
}

// EncodeQR builds the TLV and returns its Base64 string. `simplified` controls whether
// tag 9 (the ZATCA stamp signature) is included.
func EncodeQR(f QRFields, simplified bool) (string, error) {
	var tlv []byte
	put := func(tag byte, val []byte) error {
		if len(val) > 255 {
			return errors.New("ksa qr: tag value exceeds single-byte length")
		}
		tlv = append(tlv, tag, byte(len(val)))
		tlv = append(tlv, val...)
		return nil
	}

	// Tags 1–6 (always present).
	for i, v := range []string{f.SellerName, f.VATNumber, f.Timestamp, f.InvoiceTotal, f.VATTotal, f.InvoiceHash} {
		if err := put(byte(i+1), []byte(v)); err != nil {
			return "", err
		}
	}
	// Tags 7–8 (signature, public key) — present once signed.
	if len(f.Signature) > 0 {
		if err := put(7, f.Signature); err != nil {
			return "", err
		}
	}
	if len(f.PublicKey) > 0 {
		if err := put(8, f.PublicKey); err != nil {
			return "", err
		}
	}
	// Tag 9 — simplified only.
	if simplified && len(f.Stamp) > 0 {
		if err := put(9, f.Stamp); err != nil {
			return "", err
		}
	}

	return base64.StdEncoding.EncodeToString(tlv), nil
}

// TLVTag is one decoded TLV element (used for validation/round-trip tests, and by the
// inbound/validation path to assert a QR's tag set).
type TLVTag struct {
	Tag   byte
	Value []byte
}

// DecodeQR parses a Base64 ZATCA QR back into its TLV tags (validator/round-trip use).
func DecodeQR(b64 string) ([]TLVTag, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	var tags []TLVTag
	for i := 0; i < len(raw); {
		if i+2 > len(raw) {
			return nil, errors.New("ksa qr: truncated TLV header")
		}
		tag := raw[i]
		length := int(raw[i+1])
		i += 2
		if i+length > len(raw) {
			return nil, errors.New("ksa qr: TLV value overruns buffer")
		}
		tags = append(tags, TLVTag{Tag: tag, Value: raw[i : i+length]})
		i += length
	}
	return tags, nil
}
