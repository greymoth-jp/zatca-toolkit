// Command sample generates sample PDF/A-3b files for use with the veraPDF
// verification harness.
//
// It writes two files:
//
//   - sample-zatca.pdf   — ZATCA UBL 2.1 invoice, AFRelationship /Data
//   - sample-facturx.pdf — Factur-X EN 16931 CII invoice, AFRelationship /Alternative
//
// Usage:
//
//	sample [-xml invoice.xml] [-out-dir /tmp]
//
// Flags:
//
//	-xml      optional path to a UBL 2.1 XML file; built-in fixture used when omitted
//	-out-dir  output directory (default: current directory)
//
// This command is opt-in (developer-run only). It is NOT wired into CI.
// Run tools/pdfa3/verify.sh to validate the generated files with veraPDF.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/greymoth-jp/zatca-toolkit/tools/pdfa3"
)

// fixtureUBLXML is a minimal ZATCA UBL 2.1 invoice used when -xml is omitted.
const fixtureUBLXML = `<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
         xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
         xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
  <cbc:UBLVersionID>2.1</cbc:UBLVersionID>
  <cbc:ID>SAMPLE-ZATCA-001</cbc:ID>
  <cbc:IssueDate>2026-06-14</cbc:IssueDate>
  <cbc:InvoiceTypeCode name="0100000">388</cbc:InvoiceTypeCode>
  <cac:AccountingSupplierParty>
    <cac:Party>
      <cac:PartyName><cbc:Name>Sample Supplier Co.</cbc:Name></cac:PartyName>
    </cac:Party>
  </cac:AccountingSupplierParty>
  <cac:AccountingCustomerParty>
    <cac:Party>
      <cac:PartyName><cbc:Name>Sample Customer Ltd.</cbc:Name></cac:PartyName>
    </cac:Party>
  </cac:AccountingCustomerParty>
  <cac:LegalMonetaryTotal>
    <cbc:TaxInclusiveAmount currencyID="SAR">1150.00</cbc:TaxInclusiveAmount>
  </cac:LegalMonetaryTotal>
</Invoice>`

// fixtureCIIXML is a minimal Factur-X EN 16931 CII invoice fixture.
const fixtureCIIXML = `<?xml version="1.0" encoding="UTF-8"?>
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
    <ram:ID>SAMPLE-FX-001</ram:ID>
    <ram:TypeCode>380</ram:TypeCode>
    <ram:IssueDateTime>
      <udt:DateTimeString format="102">20260614</udt:DateTimeString>
    </ram:IssueDateTime>
  </rsm:ExchangedDocument>
  <rsm:SupplyChainTradeTransaction>
    <ram:ApplicableHeaderTradeAgreement>
      <ram:SellerTradeParty>
        <ram:Name>Sample Supplier Co.</ram:Name>
      </ram:SellerTradeParty>
      <ram:BuyerTradeParty>
        <ram:Name>Sample Customer Ltd.</ram:Name>
      </ram:BuyerTradeParty>
    </ram:ApplicableHeaderTradeAgreement>
    <ram:ApplicableHeaderTradeSettlement>
      <ram:InvoiceCurrencyCode>EUR</ram:InvoiceCurrencyCode>
      <ram:SpecifiedTradeSettlementHeaderMonetarySummation>
        <ram:TaxInclusiveAmount>1000.00</ram:TaxInclusiveAmount>
      </ram:SpecifiedTradeSettlementHeaderMonetarySummation>
    </ram:ApplicableHeaderTradeSettlement>
  </rsm:SupplyChainTradeTransaction>
</rsm:CrossIndustryInvoice>`

func main() {
	xmlPath := flag.String("xml", "", "path to UBL 2.1 XML file (optional)")
	outDir := flag.String("out-dir", ".", "directory for output PDFs")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "sample: mkdir %s: %v\n", *outDir, err)
		os.Exit(1)
	}

	var ublXML []byte
	if *xmlPath != "" {
		data, err := os.ReadFile(*xmlPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sample: read xml: %v\n", err)
			os.Exit(1)
		}
		ublXML = data
		fmt.Printf("using provided XML: %s\n", *xmlPath)
	} else {
		ublXML = []byte(fixtureUBLXML)
		fmt.Println("using built-in ZATCA UBL fixture")
	}

	zatcaOut := filepath.Join(*outDir, "sample-zatca.pdf")
	if err := writeZATCA(ublXML, zatcaOut); err != nil {
		fmt.Fprintf(os.Stderr, "sample: %v\n", err)
		os.Exit(1)
	}

	fxOut := filepath.Join(*outDir, "sample-facturx.pdf")
	if err := writeFacturX([]byte(fixtureCIIXML), fxOut); err != nil {
		fmt.Fprintf(os.Stderr, "sample: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("  %s  (AFRelationship=/Data)\n", zatcaOut)
	fmt.Printf("  %s  (AFRelationship=/Alternative)\n", fxOut)
	fmt.Println()
	fmt.Println("run verify.sh to validate with veraPDF")
}

func writeZATCA(xmlBytes []byte, out string) error {
	res, err := pdfa3.EmbedXMLIntoPDFA3(pdfa3.EmbedRequest{
		XMLContent:  xmlBytes,
		XMLFilename: "invoice.xml",
		DocumentID:  "SAMPLE-ZATCA-001",
	})
	if err != nil {
		return fmt.Errorf("EmbedXMLIntoPDFA3: %w", err)
	}
	if err := os.WriteFile(out, res.PDF, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("wrote %s (%d bytes) — %s\n", out, len(res.PDF), res.ConformanceLevel)
	return nil
}

func writeFacturX(ciiXML []byte, out string) error {
	res, err := pdfa3.EmbedCIIFacturX(ciiXML, "EN 16931")
	if err != nil {
		return fmt.Errorf("EmbedCIIFacturX: %w", err)
	}
	if err := os.WriteFile(out, res.PDF, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	fmt.Printf("wrote %s (%d bytes) — %s\n", out, len(res.PDF), res.ConformanceLevel)
	return nil
}
