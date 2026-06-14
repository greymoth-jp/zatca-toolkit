package ksa

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

// canonical.go — ZATCA invoice hash (SK-1, P0a-2). The earlier HashRaw SHA-256'd the raw
// UBL bytes, which is NOT what ZATCA hashes: the invoice hash is SHA-256 over the
// **canonicalized** XML with the signature-related and QR nodes removed. Two documents that
// differ only in pretty-print whitespace must hash identically; the QR/signature (which are
// derived FROM the hash) must be excluded so the hash is well-defined before they exist.
//
// ALGORITHM (per ZATCA signing spec; flavor pending official-fixture confirmation):
//  1. Remove from the invoice, for hashing purposes:
//       - ext:UBLExtensions                         (carries the XAdES signature)
//       - cac:Signature                             (UBL signature reference, if present)
//       - cac:AdditionalDocumentReference[ID='QR']  (the QR, derived from this hash)
//  2. Canonicalize the result (Canonical XML 1.1 — W3C C14N via goxmldsig, Apache-2.0).
//  3. SHA-256 the canonical bytes.
//
// CONFORMANCE NOTE (honest): this is the correct algorithm and uses a real W3C C14N
// implementation, but byte-exact agreement with ZATCA requires running the official SDK
// fixtures (STATUS §疑った前提 #2). Until then we assert structure + determinism, not
// fixture identity.

// hashExcludedSpaceTags are the (prefix, localName) pairs removed before hashing.
var hashExcludedSpaceTags = [][2]string{
	{"ext", "UBLExtensions"},
	{"cac", "Signature"},
}

// CanonicalInvoiceHash computes the ZATCA invoice hash digest over the canonicalized UBL.
func CanonicalInvoiceHash(ublXML []byte) ([32]byte, error) {
	var zero [32]byte
	out, err := canonicalForHash(ublXML)
	if err != nil {
		return zero, err
	}
	return sha256.Sum256(out), nil
}

// canonicalForHash returns the canonical bytes that the invoice hash is taken over
// (signature/QR nodes stripped, then C14N 1.1). Exposed unexported for tests.
func canonicalForHash(ublXML []byte) ([]byte, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(ublXML); err != nil {
		return nil, err
	}
	root := doc.Root()
	if root == nil {
		return nil, errors.New("ksa canonical: document has no root element")
	}
	// Drop insignificant (pretty-print) whitespace between elements first, so the hash is
	// formatting-independent and removing a node leaves no orphan indentation text node.
	compactWhitespace(root)
	stripForHash(root)

	canon := dsig.MakeC14N11Canonicalizer()
	return canon.Canonicalize(root)
}

// compactWhitespace removes whitespace-only text nodes from elements that contain child
// elements (i.e., indentation between tags), while preserving real text in leaf elements.
func compactWhitespace(el *etree.Element) {
	if len(el.ChildElements()) > 0 {
		tokens := make([]etree.Token, len(el.Child))
		copy(tokens, el.Child)
		for _, tok := range tokens {
			if cd, ok := tok.(*etree.CharData); ok && strings.TrimSpace(cd.Data) == "" {
				el.RemoveChild(cd)
			}
		}
	}
	for _, c := range el.ChildElements() {
		compactWhitespace(c)
	}
}

// CanonicalInvoiceHashB64 returns the base64 invoice hash (QR tag 6 / PIH chain link).
func CanonicalInvoiceHashB64(ublXML []byte) (string, error) {
	d, err := CanonicalInvoiceHash(ublXML)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(d[:]), nil
}

// stripForHash removes the signature/QR nodes that ZATCA excludes from the hashed content.
func stripForHash(root *etree.Element) {
	for _, child := range root.ChildElements() {
		if isExcludedSpaceTag(child) || isQRReference(child) {
			root.RemoveChild(child)
		}
	}
}

func isExcludedSpaceTag(el *etree.Element) bool {
	for _, pair := range hashExcludedSpaceTags {
		if el.Space == pair[0] && el.Tag == pair[1] {
			return true
		}
	}
	return false
}

// isQRReference reports whether el is cac:AdditionalDocumentReference with cbc:ID == "QR".
func isQRReference(el *etree.Element) bool {
	if el.Space != "cac" || el.Tag != "AdditionalDocumentReference" {
		return false
	}
	id := el.SelectElement("cbc:ID")
	return id != nil && id.Text() == "QR"
}
