package pdfa3_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/greymoth-jp/zatca-toolkit/tools/pdfa3"
)

// minimalUBLXML is a truncated-but-valid UBL 2.1 invoice XML fragment.
const minimalUBLXML = `<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
         xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
         xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
  <cbc:UBLVersionID>2.1</cbc:UBLVersionID>
  <cbc:ID>INV-TEST-001</cbc:ID>
  <cbc:IssueDate>2026-06-14</cbc:IssueDate>
  <cbc:InvoiceTypeCode name="0100000">388</cbc:InvoiceTypeCode>
  <cac:AccountingSupplierParty>
    <cac:Party>
      <cac:PartyName><cbc:Name>Test Supplier</cbc:Name></cac:PartyName>
    </cac:Party>
  </cac:AccountingSupplierParty>
  <cac:AccountingCustomerParty>
    <cac:Party>
      <cac:PartyName><cbc:Name>Test Customer</cbc:Name></cac:PartyName>
    </cac:Party>
  </cac:AccountingCustomerParty>
  <cac:LegalMonetaryTotal>
    <cbc:TaxInclusiveAmount currencyID="SAR">115.00</cbc:TaxInclusiveAmount>
  </cac:LegalMonetaryTotal>
</Invoice>`

func TestEmbedXMLIntoPDFA3_produces_valid_pdf(t *testing.T) {
	res, err := pdfa3.EmbedXMLIntoPDFA3(pdfa3.EmbedRequest{
		XMLContent:  []byte(minimalUBLXML),
		XMLFilename: "invoice.xml",
		DocumentID:  "INV-TEST-001",
	})
	if err != nil {
		t.Fatalf("EmbedXMLIntoPDFA3: %v", err)
	}

	if len(res.PDF) == 0 {
		t.Fatal("PDF output is empty")
	}

	if !bytes.HasPrefix(res.PDF, []byte("%PDF-1.7")) {
		t.Errorf("PDF does not start with %%PDF-1.7 header; got %q", res.PDF[:min(20, len(res.PDF))])
	}

	if !bytes.HasSuffix(bytes.TrimRight(res.PDF, "\n"), []byte("%%EOF")) {
		// Allow trailing newline after %%EOF
		tail := bytes.TrimRight(res.PDF, "\n\r")
		if !bytes.HasSuffix(tail, []byte("%%EOF")) {
			t.Errorf("PDF does not end with %%%%EOF")
		}
	}
}

func TestEmbedXMLIntoPDFA3_xml_present_in_output(t *testing.T) {
	xmlBytes := []byte(minimalUBLXML)
	res, err := pdfa3.EmbedXMLIntoPDFA3(pdfa3.EmbedRequest{
		XMLContent:  xmlBytes,
		XMLFilename: "zatca-invoice.xml",
		DocumentID:  "INV-TEST-002",
	})
	if err != nil {
		t.Fatalf("EmbedXMLIntoPDFA3: %v", err)
	}

	// The XML bytes must be present verbatim in the PDF stream.
	if !bytes.Contains(res.PDF, xmlBytes) {
		t.Error("embedded XML bytes not found in PDF output")
	}
}

func TestEmbedXMLIntoPDFA3_afrelationship_data_present(t *testing.T) {
	res, err := pdfa3.EmbedXMLIntoPDFA3(pdfa3.EmbedRequest{
		XMLContent:  []byte(minimalUBLXML),
		XMLFilename: "invoice.xml",
		DocumentID:  "INV-TEST-003",
	})
	if err != nil {
		t.Fatalf("EmbedXMLIntoPDFA3: %v", err)
	}

	pdfStr := string(res.PDF)

	if !strings.Contains(pdfStr, "/AFRelationship /Data") {
		t.Error("/AFRelationship /Data not found in PDF; required for PDF/A-3 associated-file compliance")
	}
}

func TestEmbedXMLIntoPDFA3_pdfa3_xmp_metadata_present(t *testing.T) {
	res, err := pdfa3.EmbedXMLIntoPDFA3(pdfa3.EmbedRequest{
		XMLContent:  []byte(minimalUBLXML),
		XMLFilename: "invoice.xml",
		DocumentID:  "INV-TEST-004",
	})
	if err != nil {
		t.Fatalf("EmbedXMLIntoPDFA3: %v", err)
	}

	pdfStr := string(res.PDF)

	checks := []struct {
		name  string
		token string
	}{
		{"XMP packet begin", `<?xpacket begin=`},
		{"pdfaid:part=3", `<pdfaid:part>3</pdfaid:part>`},
		{"pdfaid:conformance=B", `<pdfaid:conformance>B</pdfaid:conformance>`},
		{"sRGB OutputIntent", `/GTS_PDFA1`},
		{"EmbeddedFiles catalog", `/EmbeddedFiles`},
		{"/AF array in catalog", `/AF [7 0 R]`},
	}

	for _, c := range checks {
		if !strings.Contains(pdfStr, c.token) {
			t.Errorf("missing %s: token %q not found in PDF", c.name, c.token)
		}
	}
}

func TestEmbedXMLIntoPDFA3_filename_in_filespec(t *testing.T) {
	const customName = "zatca_inv_001.xml"
	res, err := pdfa3.EmbedXMLIntoPDFA3(pdfa3.EmbedRequest{
		XMLContent:  []byte(minimalUBLXML),
		XMLFilename: customName,
		DocumentID:  "INV-TEST-005",
	})
	if err != nil {
		t.Fatalf("EmbedXMLIntoPDFA3: %v", err)
	}

	if res.AttachmentFilename != customName {
		t.Errorf("AttachmentFilename = %q; want %q", res.AttachmentFilename, customName)
	}

	if !bytes.Contains(res.PDF, []byte(customName)) {
		t.Errorf("filename %q not found in PDF output", customName)
	}
}

func TestEmbedXMLIntoPDFA3_default_filename(t *testing.T) {
	res, err := pdfa3.EmbedXMLIntoPDFA3(pdfa3.EmbedRequest{
		XMLContent: []byte(minimalUBLXML),
	})
	if err != nil {
		t.Fatalf("EmbedXMLIntoPDFA3: %v", err)
	}
	if res.AttachmentFilename != "invoice.xml" {
		t.Errorf("default AttachmentFilename = %q; want %q", res.AttachmentFilename, "invoice.xml")
	}
}

func TestEmbedXMLIntoPDFA3_empty_xml_returns_error(t *testing.T) {
	_, err := pdfa3.EmbedXMLIntoPDFA3(pdfa3.EmbedRequest{
		XMLContent: nil,
	})
	if err == nil {
		t.Error("expected error for empty XMLContent, got nil")
	}
}

func TestEmbedXMLIntoPDFA3_conformance_level_informational(t *testing.T) {
	res, err := pdfa3.EmbedXMLIntoPDFA3(pdfa3.EmbedRequest{
		XMLContent: []byte(minimalUBLXML),
	})
	if err != nil {
		t.Fatalf("EmbedXMLIntoPDFA3: %v", err)
	}
	if !strings.Contains(res.ConformanceLevel, "PDF/A-3b") {
		t.Errorf("ConformanceLevel = %q; expected to contain 'PDF/A-3b'", res.ConformanceLevel)
	}
	if !strings.Contains(strings.ToUpper(res.ConformanceLevel), "EXPERIMENTAL") {
		t.Errorf("ConformanceLevel = %q; expected EXPERIMENTAL marker", res.ConformanceLevel)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
