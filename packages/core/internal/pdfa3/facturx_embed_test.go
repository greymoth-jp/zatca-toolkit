package pdfa3

import (
	"bytes"
	"testing"
)

// EmbedCIIFacturX must produce a PDF that begins with %PDF-, ends with %%EOF, embeds the
// supplied CII XML verbatim, and carries the Factur-X /Alternative AFRelationship. This is a
// structural check of the WASM-exposed path (zatcaGenerateFacturX) — NOT a veraPDF proof.
func TestEmbedCIIFacturXStructure(t *testing.T) {
	cii := []byte(`<?xml version="1.0" encoding="UTF-8"?>` +
		`<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100">x</rsm:CrossIndustryInvoice>`)
	res, err := EmbedCIIFacturX(cii, "EN 16931")
	if err != nil {
		t.Fatalf("EmbedCIIFacturX: %v", err)
	}
	if !bytes.HasPrefix(res.PDF, []byte("%PDF-")) {
		t.Errorf("PDF must start with %%PDF-, got %q", res.PDF[:min(8, len(res.PDF))])
	}
	if !bytes.Contains(res.PDF, []byte("%%EOF")) {
		t.Error("PDF must contain the trailing EOF marker")
	}
	if !bytes.Contains(res.PDF, []byte("CrossIndustryInvoice")) {
		t.Error("embedded CII XML not found in the PDF bytes")
	}
	if !bytes.Contains(res.PDF, []byte("/Alternative")) {
		t.Error("Factur-X AFRelationship /Alternative not found")
	}
	if res.AttachmentFilename != "factur-x.xml" {
		t.Errorf("attachment filename = %q, want factur-x.xml", res.AttachmentFilename)
	}
}

// An empty CII payload must be rejected (mirrors the tools/pdfa3 behaviour).
func TestEmbedCIIFacturXEmptyError(t *testing.T) {
	if _, err := EmbedCIIFacturX(nil, "EN 16931"); err == nil {
		t.Error("expected an error for empty CII content")
	}
}
