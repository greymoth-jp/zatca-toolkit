package ksa

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/convert"
)

func ublFor(t *testing.T, qr string) []byte {
	t.Helper()
	out, err := convert.ToZatcaUBL(ksaDoc(), convert.ZatcaUBLOpts{
		UUID: "U-CANON-1", ICV: 1, PIH: GenesisPIH, IssueTime: "10:30:00", QR: qr,
	})
	if err != nil {
		t.Fatalf("ToZatcaUBL: %v", err)
	}
	return out
}

// The invoice hash MUST exclude the QR AdditionalDocumentReference and the UBLExtensions
// (signature) — otherwise the hash could not be computed before those nodes exist. So a
// document with a QR and the same document without one must hash identically.
func TestCanonicalHashExcludesQRAndExtensions(t *testing.T) {
	hNoQR, err := CanonicalInvoiceHashB64(ublFor(t, ""))
	if err != nil {
		t.Fatalf("hash no-qr: %v", err)
	}
	hWithQR, err := CanonicalInvoiceHashB64(ublFor(t, "BASE64QRPLACEHOLDERVALUE=="))
	if err != nil {
		t.Fatalf("hash with-qr: %v", err)
	}
	if hNoQR != hWithQR {
		t.Fatalf("QR/UBLExtensions not excluded from invoice hash:\n no-qr=%s\n qr=%s", hNoQR, hWithQR)
	}

	// And the canonical bytes themselves must not contain the stripped nodes.
	canon, err := canonicalForHash(ublFor(t, "QRDATA=="))
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	for _, banned := range []string{"UBLExtensions", "QRDATA=="} {
		if bytes.Contains(canon, []byte(banned)) {
			t.Fatalf("canonical form still contains stripped node %q", banned)
		}
	}
}

func TestCanonicalHashDeterministicAndContentSensitive(t *testing.T) {
	a, _ := CanonicalInvoiceHashB64(ublFor(t, ""))
	b, _ := CanonicalInvoiceHashB64(ublFor(t, ""))
	if a != b {
		t.Fatal("canonical hash is not deterministic")
	}

	// A content change (different ID) must change the hash.
	doc := ksaDoc()
	doc.ID = "INV-DIFFERENT"
	changed, err := convert.ToZatcaUBL(doc, convert.ZatcaUBLOpts{UUID: "U-CANON-1", ICV: 1, PIH: GenesisPIH, IssueTime: "10:30:00"})
	if err != nil {
		t.Fatalf("ToZatcaUBL changed: %v", err)
	}
	c, _ := CanonicalInvoiceHashB64(changed)
	if a == c {
		t.Fatal("canonical hash did not change on content change")
	}
}

// Canonicalization must produce well-formed XML without an XML declaration (C14N drops it).
// Canonicalization must be invariant to insignificant XML formatting (indentation /
// whitespace between elements) — the property that makes the signed hash stable across
// serializers. Two formatting variants of the same invoice must hash identically.
func TestCanonicalHashFormattingInvariant(t *testing.T) {
	ubl := ublFor(t, "")
	pretty, err := CanonicalInvoiceHashB64(ubl)
	if err != nil {
		t.Fatalf("hash pretty: %v", err)
	}
	// Collapse inter-element whitespace/newlines (a different but equivalent serialization).
	collapsed := regexp.MustCompile(`>\s+<`).ReplaceAll(ubl, []byte("><"))
	cHash, err := CanonicalInvoiceHashB64(collapsed)
	if err != nil {
		t.Fatalf("hash collapsed: %v", err)
	}
	if pretty != cHash {
		t.Fatalf("canonical hash must be formatting-invariant:\n pretty=%s\n collapsed=%s", pretty, cHash)
	}
	// Add extra blank lines + leading indentation (still equivalent).
	reindented := regexp.MustCompile(`\n`).ReplaceAll(ubl, []byte("\n   \n"))
	rHash, _ := CanonicalInvoiceHashB64(reindented)
	if pretty != rHash {
		t.Fatalf("canonical hash must ignore extra whitespace:\n pretty=%s\n reindented=%s", pretty, rHash)
	}
}

func TestCanonicalFormShape(t *testing.T) {
	canon, err := canonicalForHash(ublFor(t, ""))
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(string(canon)), "<?xml") {
		t.Fatal("C14N output must not retain the XML declaration")
	}
	if !bytes.Contains(canon, []byte("Invoice")) {
		t.Fatal("canonical form lost the Invoice root")
	}
}
