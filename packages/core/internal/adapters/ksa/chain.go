// Package ksa implements the KSA ZATCA (Fatoora Phase 2) jurisdiction adapter pieces.
// This file: the ICV (Invoice Counter Value) + PIH (Previous Invoice Hash) anti-tamper
// chain (FR-Z02, FR-Z07) — the core of the "fail-closed, no-tamper" guarantee.
//
// Chain semantics (ZATCA): every invoice carries a sequential ICV and a PIH equal to the
// hash of the immediately prior document. The hash is base64(SHA-256(canonical invoice
// XML)). Per ZATCA, rejected/failed documents STILL advance the chain — the next PIH
// references the prior document's hash regardless of clearance result (see STATUS diff #2).
package ksa

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// GenesisPIH is the PIH of the very first invoice. ZATCA defines it as the hash of the
// literal "0". NOTE: exact genesis value must be reconfirmed against the current ZATCA
// technical spec before production (STATUS Open Questions) — kept as a single constant so
// it is trivial to correct.
var GenesisPIH = Hash([]byte("0"))

// HashRaw returns the raw SHA-256 digest (used as the signing input for the CSID signer).
func HashRaw(xml []byte) [32]byte { return sha256.Sum256(xml) }

// Hash returns base64(SHA-256(xml)) — the ZATCA invoice hash used for the PIH chain and
// QR tag 6. The exact XML canonicalization ZATCA requires (C14N over specific nodes) is a
// generation-time concern handled where the signed XML is produced; this hashes the bytes
// it is given.
func Hash(xml []byte) string {
	sum := HashRaw(xml)
	return base64.StdEncoding.EncodeToString(sum[:])
}

// ChainEntry is the persisted tail of the chain for a (legal entity) counter.
type ChainEntry struct {
	ICV  int64  // counter value of the last issued document
	Hash string // base64(SHA-256(xml)) of the last issued document
}

// NextICV returns the counter value for the next document. ICV is 1-based.
func NextICV(prev *ChainEntry) int64 {
	if prev == nil {
		return 1
	}
	return prev.ICV + 1
}

// NextPIH returns the PIH to embed in the next document.
func NextPIH(prev *ChainEntry) string {
	if prev == nil {
		return GenesisPIH
	}
	return prev.Hash
}

var (
	ErrICVGap      = errors.New("icv_pih_chain_broken: ICV is not sequential")
	ErrPIHMismatch = errors.New("icv_pih_chain_broken: PIH does not match previous document hash")
)

// ValidateLink verifies that a document with the given (icv, pih) correctly follows prev.
// Returns a typed error (mapped to error-codes.md `icv_pih_chain_broken`, HTTP 409) on a
// gap or a broken hash link. This is what makes tampering / out-of-order issuance fail closed.
func ValidateLink(icv int64, pih string, prev *ChainEntry) error {
	expectedICV := NextICV(prev)
	if icv != expectedICV {
		return fmt.Errorf("%w: got ICV %d, expected %d", ErrICVGap, icv, expectedICV)
	}
	if pih != NextPIH(prev) {
		return ErrPIHMismatch
	}
	return nil
}

// Append produces the new chain tail after a document with the given xml is issued.
func Append(prev *ChainEntry, xml []byte) ChainEntry {
	return ChainEntry{ICV: NextICV(prev), Hash: Hash(xml)}
}
