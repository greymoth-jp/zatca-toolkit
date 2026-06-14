package pdfa3

import (
	"fmt"
)

// EmbedCIIFacturX generates a Factur-X PDF/A-3b document with the supplied
// UN/CEFACT CII XML embedded under the conventional attachment name "factur-x.xml".
//
// Differences from EmbedXMLIntoPDFA3 (ZATCA UBL path):
//   - Attachment filename is always "factur-x.xml" (Factur-X §3.1 convention).
//   - AFRelationship is /Alternative (EN 16931 / Factur-X mandate), not /Data.
//   - XMP metadata carries the Factur-X extension schema
//     (urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#) with
//     DocumentType=INVOICE, Version=1.0, and the supplied ConformanceLevel.
//
// level is the Factur-X profile name, e.g. "EN 16931", "MINIMUM", "BASIC WL",
// "BASIC", "EXTENDED". It is written verbatim into the XMP ConformanceLevel field.
// If level is empty it defaults to "EN 16931".
//
// EXPERIMENTAL: structural PDF/A-3b layout only; verify with veraPDF before
// production use.
func EmbedCIIFacturX(ciiXML []byte, level string) (*EmbedResult, error) {
	if len(ciiXML) == 0 {
		return nil, fmt.Errorf("pdfa3: ciiXML must not be empty")
	}
	if level == "" {
		level = "EN 16931"
	}

	const filename = "factur-x.xml"
	pdf := buildFacturXPDFA3(ciiXML, filename, level)

	return &EmbedResult{
		PDF:                pdf,
		AttachmentFilename: filename,
		ConformanceLevel:   "PDF/A-3b (ISO 19005-3) Factur-X " + level + " — EXPERIMENTAL",
	}, nil
}

// buildFacturXPDFA3 constructs a Factur-X PDF/A-3b document.
//
// Object layout mirrors buildPDFA3 (same object numbering):
//
//	1  Catalog
//	2  Info
//	3  Pages
//	4  XMPMeta  — Factur-X extension schema (urn:factur-x...)
//	5  OutputIntent
//	6  ICCProfile
//	7  FileSpec  — /AFRelationship /Alternative
//	8  Page
//	9  EmbeddedFile (the CII XML)
//	10 Page content stream (blank)
func buildFacturXPDFA3(xmlBytes []byte, filename, level string) []byte {
	b := &pdfBuilder{}

	xmlLen := len(xmlBytes)
	xmpData := buildFacturXXMP(filename, level)
	xmpLen := len(xmpData)
	iccData := minimalSRGBICC()
	iccHex := asciiHexEncode(iccData)

	escapedName := pdfEscapeString(filename)

	// Obj 1: Catalog
	b.add(fmt.Sprintf(
		`<< /Type /Catalog /Pages 3 0 R /Metadata 4 0 R /OutputIntents [5 0 R] `+
			`/Names << /EmbeddedFiles << /Names [(%s) 7 0 R] >> >> /AF [7 0 R] >>`,
		escapedName,
	), nil)

	// Obj 2: Info
	b.add(`<< /Producer (zatca-toolkit/tools/pdfa3 factur-x) >>`, nil)

	// Obj 3: Pages
	b.add(`<< /Type /Pages /Kids [8 0 R] /Count 1 >>`, nil)

	// Obj 4: XMP Metadata (Factur-X extension)
	b.add(fmt.Sprintf(`<< /Type /Metadata /Subtype /XML /Length %d >>`, xmpLen), xmpData)

	// Obj 5: OutputIntent (sRGB GTS_PDFA1)
	b.add(
		`<< /Type /OutputIntent /S /GTS_PDFA1 `+
			`/OutputConditionIdentifier (sRGB IEC61966-2.1) `+
			`/RegistryName (http://www.color.org) `+
			`/Info (sRGB IEC61966-2.1) /DestOutputProfile 6 0 R >>`,
		nil,
	)

	// Obj 6: ICC Profile (minimal sRGB stub, ASCIIHex encoded)
	b.add(fmt.Sprintf(
		`<< /N 3 /Length %d /Filter /ASCIIHexDecode >>`,
		len(iccHex),
	), iccHex)

	// Obj 7: FileSpec with AFRelationship /Alternative (Factur-X mandate)
	b.add(fmt.Sprintf(
		`<< /Type /Filespec /F (%s) /UF (%s) /Desc (Factur-X CII Invoice XML) `+
			`/AFRelationship /Alternative /EF << /F 9 0 R /UF 9 0 R >> >>`,
		escapedName, escapedName,
	), nil)

	// Obj 8: Page (blank A4)
	b.add(
		`<< /Type /Page /Parent 3 0 R /MediaBox [0 0 595 842] `+
			`/Contents 10 0 R /Resources << >> >>`,
		nil,
	)

	// Obj 9: EmbeddedFile stream (the CII XML)
	b.add(fmt.Sprintf(
		`<< /Type /EmbeddedFile /Subtype /text#2Fxml /Length %d /Params << /Size %d >> >>`,
		xmlLen, xmlLen,
	), xmlBytes)

	// Obj 10: Page content stream (blank)
	emptyStream := []byte(" ")
	b.add(fmt.Sprintf(`<< /Length %d >>`, len(emptyStream)), emptyStream)

	return b.build(filename)
}

// buildFacturXXMP returns the XMP packet declaring PDF/A-3b conformance and the
// Factur-X extension schema (urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#).
func buildFacturXXMP(filename, level string) []byte {
	xmp := fmt.Sprintf(`<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""
        xmlns:dc="http://purl.org/dc/elements/1.1/">
      <dc:title>
        <rdf:Alt><rdf:li xml:lang="x-default">Factur-X Invoice</rdf:li></rdf:Alt>
      </dc:title>
      <dc:format>application/pdf</dc:format>
    </rdf:Description>
    <rdf:Description rdf:about=""
        xmlns:pdf="http://ns.adobe.com/pdf/1.3/">
      <pdf:Producer>zatca-toolkit/tools/pdfa3 factur-x</pdf:Producer>
    </rdf:Description>
    <rdf:Description rdf:about=""
        xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/">
      <pdfaid:part>3</pdfaid:part>
      <pdfaid:conformance>B</pdfaid:conformance>
    </rdf:Description>
    <rdf:Description rdf:about=""
        xmlns:pdfaExtension="http://www.aiim.org/pdfa/ns/extension/"
        xmlns:pdfaSchema="http://www.aiim.org/pdfa/ns/schema#"
        xmlns:pdfaProperty="http://www.aiim.org/pdfa/ns/property#">
      <pdfaExtension:schemas>
        <rdf:Bag>
          <rdf:li rdf:parseType="Resource">
            <pdfaSchema:schema>Factur-X PDFA Extension Schema</pdfaSchema:schema>
            <pdfaSchema:namespaceURI>urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#</pdfaSchema:namespaceURI>
            <pdfaSchema:prefix>fx</pdfaSchema:prefix>
            <pdfaSchema:property>
              <rdf:Seq>
                <rdf:li rdf:parseType="Resource">
                  <pdfaProperty:name>DocumentFileName</pdfaProperty:name>
                  <pdfaProperty:valueType>Text</pdfaProperty:valueType>
                  <pdfaProperty:category>external</pdfaProperty:category>
                  <pdfaProperty:description>The name of the embedded XML document</pdfaProperty:description>
                </rdf:li>
                <rdf:li rdf:parseType="Resource">
                  <pdfaProperty:name>DocumentType</pdfaProperty:name>
                  <pdfaProperty:valueType>Text</pdfaProperty:valueType>
                  <pdfaProperty:category>external</pdfaProperty:category>
                  <pdfaProperty:description>The type of the hybrid document in capital letters, e.g. INVOICE or ORDER</pdfaProperty:description>
                </rdf:li>
                <rdf:li rdf:parseType="Resource">
                  <pdfaProperty:name>Version</pdfaProperty:name>
                  <pdfaProperty:valueType>Text</pdfaProperty:valueType>
                  <pdfaProperty:category>external</pdfaProperty:category>
                  <pdfaProperty:description>The actual version of the standard applying to the embedded XML document</pdfaProperty:description>
                </rdf:li>
                <rdf:li rdf:parseType="Resource">
                  <pdfaProperty:name>ConformanceLevel</pdfaProperty:name>
                  <pdfaProperty:valueType>Text</pdfaProperty:valueType>
                  <pdfaProperty:category>external</pdfaProperty:category>
                  <pdfaProperty:description>The conformance level of the embedded XML document</pdfaProperty:description>
                </rdf:li>
              </rdf:Seq>
            </pdfaSchema:property>
          </rdf:li>
        </rdf:Bag>
      </pdfaExtension:schemas>
    </rdf:Description>
    <rdf:Description rdf:about=""
        xmlns:fx="urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#">
      <fx:DocumentFileName>%s</fx:DocumentFileName>
      <fx:DocumentType>INVOICE</fx:DocumentType>
      <fx:Version>1.0</fx:Version>
      <fx:ConformanceLevel>%s</fx:ConformanceLevel>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
%s<?xpacket end="w"?>`,
		xmlEscape(filename),
		xmlEscape(level),
		xmpPadding(),
	)
	return []byte(xmp)
}
