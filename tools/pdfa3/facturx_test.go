package pdfa3_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/greymoth-jp/zatca-toolkit/tools/pdfa3"
)

const minimalCIIXML = `<?xml version="1.0" encoding="UTF-8"?>
<rsm:CrossIndustryInvoice
    xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
    xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100"
    xmlns:udt="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100">
  <rsm:ExchangedDocumentContext>
    <ram:GuidelineSpecifiedDocumentContextParameter>
      <ram:ID>urn:cen.eu:en16931:2017</ram:ID>
    </ram:GuidelineSpecifiedDocumentContextParameter>
  </rsm:ExchangedDocumentContext>
  <rsm:ExchangedDocument>
    <ram:ID>CII-TEST-001</ram:ID>
    <ram:TypeCode>380</ram:TypeCode>
    <ram:IssueDateTime>
      <udt:DateTimeString format="102">20260614</udt:DateTimeString>
    </ram:IssueDateTime>
  </rsm:ExchangedDocument>
</rsm:CrossIndustryInvoice>`

func TestEmbedCIIFacturX_produces_pdf(t *testing.T) {
	res, err := pdfa3.EmbedCIIFacturX([]byte(minimalCIIXML), "EN 16931")
	if err != nil {
		t.Fatalf("EmbedCIIFacturX: %v", err)
	}
	if len(res.PDF) == 0 {
		t.Fatal("PDF output is empty")
	}
	if !bytes.HasPrefix(res.PDF, []byte("%PDF-1.7")) {
		t.Errorf("PDF does not start with %%PDF-1.7; got %q", res.PDF[:min(20, len(res.PDF))])
	}
}

func TestEmbedCIIFacturX_attachment_name_is_facturx(t *testing.T) {
	res, err := pdfa3.EmbedCIIFacturX([]byte(minimalCIIXML), "EN 16931")
	if err != nil {
		t.Fatalf("EmbedCIIFacturX: %v", err)
	}
	if res.AttachmentFilename != "factur-x.xml" {
		t.Errorf("AttachmentFilename = %q; want %q", res.AttachmentFilename, "factur-x.xml")
	}
	if !bytes.Contains(res.PDF, []byte("factur-x.xml")) {
		t.Error("filename 'factur-x.xml' not found in PDF output")
	}
}

func TestEmbedCIIFacturX_afrelationship_alternative(t *testing.T) {
	res, err := pdfa3.EmbedCIIFacturX([]byte(minimalCIIXML), "EN 16931")
	if err != nil {
		t.Fatalf("EmbedCIIFacturX: %v", err)
	}
	pdfStr := string(res.PDF)
	if !strings.Contains(pdfStr, "/AFRelationship /Alternative") {
		t.Error("/AFRelationship /Alternative not found; required for Factur-X")
	}
	if strings.Contains(pdfStr, "/AFRelationship /Data") {
		t.Error("/AFRelationship /Data must NOT appear in Factur-X output")
	}
}

func TestEmbedCIIFacturX_xmp_facturx_extension(t *testing.T) {
	res, err := pdfa3.EmbedCIIFacturX([]byte(minimalCIIXML), "EN 16931")
	if err != nil {
		t.Fatalf("EmbedCIIFacturX: %v", err)
	}
	pdfStr := string(res.PDF)
	checks := []struct {
		name  string
		token string
	}{
		{"Factur-X namespace URI", `urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#`},
		{"fx:DocumentType INVOICE", `<fx:DocumentType>INVOICE</fx:DocumentType>`},
		{"fx:Version 1.0", `<fx:Version>1.0</fx:Version>`},
		{"fx:ConformanceLevel EN 16931", `<fx:ConformanceLevel>EN 16931</fx:ConformanceLevel>`},
		{"fx:DocumentFileName factur-x.xml", `<fx:DocumentFileName>factur-x.xml</fx:DocumentFileName>`},
		{"pdfaid:part 3", `<pdfaid:part>3</pdfaid:part>`},
		{"pdfaid:conformance B", `<pdfaid:conformance>B</pdfaid:conformance>`},
		{"sRGB OutputIntent", `/GTS_PDFA1`},
	}
	for _, c := range checks {
		if !strings.Contains(pdfStr, c.token) {
			t.Errorf("missing %s: token %q not found in PDF", c.name, c.token)
		}
	}
}

func TestEmbedCIIFacturX_cii_xml_embedded(t *testing.T) {
	xmlBytes := []byte(minimalCIIXML)
	res, err := pdfa3.EmbedCIIFacturX(xmlBytes, "BASIC")
	if err != nil {
		t.Fatalf("EmbedCIIFacturX: %v", err)
	}
	if !bytes.Contains(res.PDF, xmlBytes) {
		t.Error("CII XML bytes not found verbatim in PDF output")
	}
}

func TestEmbedCIIFacturX_empty_level_defaults_en16931(t *testing.T) {
	res, err := pdfa3.EmbedCIIFacturX([]byte(minimalCIIXML), "")
	if err != nil {
		t.Fatalf("EmbedCIIFacturX: %v", err)
	}
	if !strings.Contains(string(res.PDF), `<fx:ConformanceLevel>EN 16931</fx:ConformanceLevel>`) {
		t.Error("empty level should default to EN 16931 in XMP")
	}
	if !strings.Contains(res.ConformanceLevel, "EN 16931") {
		t.Errorf("ConformanceLevel = %q; want to contain 'EN 16931'", res.ConformanceLevel)
	}
}

func TestEmbedCIIFacturX_empty_xml_error(t *testing.T) {
	_, err := pdfa3.EmbedCIIFacturX(nil, "EN 16931")
	if err == nil {
		t.Error("expected error for nil ciiXML, got nil")
	}
}

func TestEmbedCIIFacturX_conformance_level_in_result(t *testing.T) {
	res, err := pdfa3.EmbedCIIFacturX([]byte(minimalCIIXML), "EXTENDED")
	if err != nil {
		t.Fatalf("EmbedCIIFacturX: %v", err)
	}
	if !strings.Contains(res.ConformanceLevel, "EXTENDED") {
		t.Errorf("ConformanceLevel = %q; want to contain 'EXTENDED'", res.ConformanceLevel)
	}
	if !strings.Contains(strings.ToUpper(res.ConformanceLevel), "EXPERIMENTAL") {
		t.Errorf("ConformanceLevel = %q; missing EXPERIMENTAL marker", res.ConformanceLevel)
	}
}
