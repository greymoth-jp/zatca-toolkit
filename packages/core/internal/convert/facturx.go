package convert

import (
	"fmt"
	"strings"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

// FR-C03 — Factur-X (a PDF/A-3 carrying an embedded EN16931 CII XML named
// `factur-x.xml`, associated via AFRelationship=Alternative).
//
// This module produces the DETERMINISTIC parts: the embedded CII payload and the
// Factur-X XMP metadata packet (the RDF that declares the factur-x extension schema +
// PDF/A-3 identification). These are real, required artifacts and are unit-tested.
//
// The final binary assembly (writing a PDF/A-3 with the OutputIntent/ICC profile,
// embedded-file dictionary, and font embedding such that veraPDF PASSES) is a separate
// step — see FacturXPackage + STATUS.md. Strict PDF/A-3 plumbing is "buy" (node-zugferd,
// MIT) rather than hand-rolled, since veraPDF conformance is unforgiving.

// FacturXProfile is the Factur-X conformance level (drives fx:ConformanceLevel + the
// CII GuidelineSpecifiedDocumentContextParameter).
type FacturXProfile string

const (
	ProfileEN16931FX FacturXProfile = "EN 16931" // == COMFORT
	ProfileBASIC     FacturXProfile = "BASIC"
)

const facturXFileName = "factur-x.xml"

// FacturXPackage is everything needed to assemble the PDF/A-3, independent of the PDF
// writer used. A PDF/A-3 writer embeds Payload as an associated file and sets XMP.
type FacturXPackage struct {
	FileName        string // must be "factur-x.xml"
	MimeType        string // "application/xml"
	AFRelationship  string // "Alternative"
	Payload         []byte // the EN16931 CII XML
	XMP             []byte // the XMP metadata packet to set on the PDF/A-3 catalog
	ConformanceLevel FacturXProfile
}

// BuildFacturX produces the deterministic Factur-X package from the normalized doc.
func BuildFacturX(d *normalized.Doc, profile FacturXProfile) (*FacturXPackage, error) {
	cii, err := ToCII(d)
	if err != nil {
		return nil, fmt.Errorf("factur-x: CII generation: %w", err)
	}
	return &FacturXPackage{
		FileName:         facturXFileName,
		MimeType:         "application/xml",
		AFRelationship:   "Alternative",
		Payload:          cii,
		XMP:              facturXMP(d, profile),
		ConformanceLevel: profile,
	}, nil
}

// facturXMP builds the XMP metadata packet declaring PDF/A-3 + the factur-x extension
// schema. Deterministic (no timestamps) so output is byte-stable for snapshot tests.
func facturXMP(d *normalized.Doc, profile FacturXProfile) []byte {
	title := xmlEscape("Invoice " + d.ID)
	var b strings.Builder
	b.WriteString(`<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>` + "\n")
	b.WriteString(`<x:xmpmeta xmlns:x="adobe:ns:meta/">` + "\n")
	b.WriteString(`  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">` + "\n")
	// PDF/A identification — part 3, conformance B.
	b.WriteString(`    <rdf:Description rdf:about="" xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/">` + "\n")
	b.WriteString(`      <pdfaid:part>3</pdfaid:part>` + "\n")
	b.WriteString(`      <pdfaid:conformance>B</pdfaid:conformance>` + "\n")
	b.WriteString(`    </rdf:Description>` + "\n")
	// Dublin Core title.
	b.WriteString(`    <rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/">` + "\n")
	b.WriteString(`      <dc:title><rdf:Alt><rdf:li xml:lang="x-default">` + title + `</rdf:li></rdf:Alt></dc:title>` + "\n")
	b.WriteString(`    </rdf:Description>` + "\n")
	// Factur-X extension schema (the part that makes it a Factur-X document).
	b.WriteString(`    <rdf:Description rdf:about="" xmlns:fx="urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#">` + "\n")
	b.WriteString(`      <fx:DocumentType>INVOICE</fx:DocumentType>` + "\n")
	b.WriteString(`      <fx:DocumentFileName>` + facturXFileName + `</fx:DocumentFileName>` + "\n")
	b.WriteString(`      <fx:Version>1.0</fx:Version>` + "\n")
	b.WriteString(`      <fx:ConformanceLevel>` + string(profile) + `</fx:ConformanceLevel>` + "\n")
	b.WriteString(`    </rdf:Description>` + "\n")
	b.WriteString(`  </rdf:RDF>` + "\n")
	b.WriteString(`</x:xmpmeta>` + "\n")
	b.WriteString(`<?xpacket end="w"?>`)
	return []byte(b.String())
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
