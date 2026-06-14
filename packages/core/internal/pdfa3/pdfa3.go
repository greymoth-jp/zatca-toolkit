// VENDORED from tools/pdfa3 (a separate, stdlib-only module) so the core WASM engine can
// expose Factur-X / PDF/A-3 generation to the @zatca/sdk without a cross-module dependency
// or polluting core/go.mod. Keep this copy in sync with tools/pdfa3/pdfa3.go.
//
// Package pdfa3 provides opt-in PDF/A-3b generation for the ZATCA toolkit.
//
// It embeds a ZATCA UBL 2.1 invoice XML into a minimal PDF/A-3b document with:
//   - AFRelationship /Data (ISO 32000-2 §14.13 for associated-file semantics)
//   - PDF/A-3b XMP metadata (pdfaid:part=3, pdfaid:conformance=B)
//   - EmbeddedFile filespec with /AF array in the Catalog
//   - sRGB OutputIntent (required for PDF/A)
//
// EXPERIMENTAL: The PDF structure is constructed from first principles using raw
// PDF object syntax, adapted from audrenbdb/facturx (MIT). It produces files that
// structurally conform to PDF/A-3b requirements, but full conformance has NOT been
// verified with veraPDF. Use the veraPDF open-source validator before relying on
// this output in a ZATCA Phase 2 submission.
//
// Library: zero external dependencies (stdlib only).
// License of this file: Apache-2.0 (same as the wn1a-zatca-toolkit core).
//
// References:
//   - audrenbdb/facturx (MIT) — PDF/A-3 object structure approach
//   - ISO 19005-3 (PDF/A-3)
//   - ISO 32000-2 (PDF 2.0) §14.13 AFRelationship
//   - ZATCA Phase 2 E-Invoicing Detailed Guidelines
package pdfa3

import (
	"bytes"
	"fmt"
	"strings"
)

// EmbedRequest holds the inputs for PDF/A-3b generation.
type EmbedRequest struct {
	// XMLContent is the raw UBL 2.1 invoice XML bytes.
	XMLContent []byte

	// XMLFilename is the attachment filename (e.g. "invoice.xml").
	// If empty, defaults to "invoice.xml".
	XMLFilename string

	// DocumentID is used to generate the PDF file ID (any stable string, e.g. invoice number).
	DocumentID string
}

// EmbedResult holds the generated PDF bytes and metadata.
type EmbedResult struct {
	// PDF contains the PDF/A-3b document bytes.
	PDF []byte

	// AttachmentFilename is the filename used for the embedded XML attachment.
	AttachmentFilename string

	// ConformanceLevel describes the target conformance (informational).
	ConformanceLevel string
}

// EmbedXMLIntoPDFA3 generates a minimal PDF/A-3b document with the supplied XML
// embedded as an associated file (AFRelationship /Data).
//
// The output PDF contains a single blank A4 page and the XML attachment in the
// /EmbeddedFiles name tree, with /AF in the Catalog pointing to the file spec.
//
// EXPERIMENTAL: structural PDF/A-3b layout only; verify with veraPDF before
// production ZATCA submission.
func EmbedXMLIntoPDFA3(req EmbedRequest) (*EmbedResult, error) {
	if len(req.XMLContent) == 0 {
		return nil, fmt.Errorf("pdfa3: XMLContent must not be empty")
	}

	filename := req.XMLFilename
	if filename == "" {
		filename = "invoice.xml"
	}

	docID := req.DocumentID
	if docID == "" {
		docID = filename
	}

	pdf := buildPDFA3(req.XMLContent, filename, docID)

	return &EmbedResult{
		PDF:                pdf,
		AttachmentFilename: filename,
		ConformanceLevel:   "PDF/A-3b (ISO 19005-3) — EXPERIMENTAL",
	}, nil
}

// buildPDFA3 constructs the PDF/A-3b document bytes.
//
// Object layout (1-indexed):
//
//	1  Catalog   — /Type /Catalog, /Pages, /OutputIntents, /Names EmbeddedFiles, /AF
//	2  Info      — /Producer only (kept in sync with XMP pdf:Producer; no dates, to avoid an
//	             Info/XMP date mismatch that PDF/A validators flag)
//	3  Pages     — /Type /Pages, /Kids, /Count 1
//	4  XMPMeta   — /Type /Metadata /Subtype /XML  (PDF/A-3b + Factur-X extension)
//	5  OutputIntent — sRGB GTS_PDFA1
//	6  ICCProfile — minimal sRGB ICC stub (required; no external file)
//	7  FileSpec  — /AFRelationship /Data, /EF
//	8  Page      — /Type /Page, blank A4
//	9  EmbeddedFile — /Type /EmbeddedFile /Subtype /text#2Fxml
func buildPDFA3(xmlBytes []byte, filename, docID string) []byte {
	b := &pdfBuilder{}

	xmlLen := len(xmlBytes)
	xmpData := buildXMPMetadata(docID, filename)
	xmpLen := len(xmpData)
	iccData := minimalSRGBICC()
	iccHex := asciiHexEncode(iccData)

	// Escape filename for PDF string literal
	escapedName := pdfEscapeString(filename)

	// Obj 1: Catalog
	b.add(fmt.Sprintf(
		`<< /Type /Catalog /Pages 3 0 R /Metadata 4 0 R /OutputIntents [5 0 R] `+
			`/Names << /EmbeddedFiles << /Names [(%s) 7 0 R] >> >> /AF [7 0 R] >>`,
		escapedName,
	), nil)

	// Obj 2: Info
	b.add(`<< /Producer (zatca-toolkit/tools/pdfa3) >>`, nil)

	// Obj 3: Pages
	b.add(`<< /Type /Pages /Kids [8 0 R] /Count 1 >>`, nil)

	// Obj 4: XMP Metadata
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

	// Obj 7: FileSpec with AFRelationship /Data
	b.add(fmt.Sprintf(
		`<< /Type /Filespec /F (%s) /UF (%s) /Desc (ZATCA UBL 2.1 Invoice XML) `+
			`/AFRelationship /Data /EF << /F 9 0 R /UF 9 0 R >> >>`,
		escapedName, escapedName,
	), nil)

	// Obj 8: Page (blank A4: 595.28 x 841.89 pt)
	b.add(
		`<< /Type /Page /Parent 3 0 R /MediaBox [0 0 595 842] `+
			`/Contents 10 0 R /Resources << >> >>`,
		nil,
	)

	// Obj 9: EmbeddedFile stream (the XML)
	b.add(fmt.Sprintf(
		`<< /Type /EmbeddedFile /Subtype /text#2Fxml /Length %d /Params << /Size %d >> >>`,
		xmlLen, xmlLen,
	), xmlBytes)

	// Obj 10: Page content stream (empty — blank page)
	emptyStream := []byte(" ")
	b.add(fmt.Sprintf(`<< /Length %d >>`, len(emptyStream)), emptyStream)

	return b.build(docID)
}

// pdfBuilder accumulates PDF objects and serialises the file.
type pdfBuilder struct {
	objects []pdfObject
}

type pdfObject struct {
	dict   string
	stream []byte
}

func (b *pdfBuilder) add(dict string, stream []byte) {
	b.objects = append(b.objects, pdfObject{dict: dict, stream: stream})
}

func (b *pdfBuilder) build(docID string) []byte {
	var buf bytes.Buffer
	offsets := make([]int, len(b.objects))

	buf.WriteString("%PDF-1.7\n")
	buf.Write([]byte("%\xE2\xE3\xCF\xD3\n")) // binary marker

	for i, obj := range b.objects {
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n", i+1)
		buf.WriteString(obj.dict)
		if obj.stream != nil {
			buf.WriteString("\nstream\n")
			buf.Write(obj.stream)
			buf.WriteString("\nendstream")
		}
		buf.WriteString("\nendobj\n")
	}

	xrefOffset := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(b.objects)+1)
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}

	idHex := simpleHexID(docID)
	fmt.Fprintf(&buf,
		"trailer\n<< /Size %d /Root 1 0 R /Info 2 0 R /ID [<%s> <%s>] >>\nstartxref\n%d\n%%%%EOF\n",
		len(b.objects)+1, idHex, idHex, xrefOffset,
	)

	return buf.Bytes()
}

// buildXMPMetadata returns the XMP packet declaring PDF/A-3b conformance.
func buildXMPMetadata(docID, filename string) []byte {
	xmp := fmt.Sprintf(`<?xpacket begin="" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""
        xmlns:dc="http://purl.org/dc/elements/1.1/">
      <dc:title>
        <rdf:Alt><rdf:li xml:lang="x-default">%s</rdf:li></rdf:Alt>
      </dc:title>
      <dc:format>application/pdf</dc:format>
    </rdf:Description>
    <rdf:Description rdf:about=""
        xmlns:pdf="http://ns.adobe.com/pdf/1.3/">
      <pdf:Producer>zatca-toolkit/tools/pdfa3</pdf:Producer>
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
            <pdfaSchema:schema>ZATCA UBL 2.1 Invoice Extension Schema</pdfaSchema:schema>
            <pdfaSchema:namespaceURI>urn:oasis:names:specification:ubl:schema:xsd:Invoice-2</pdfaSchema:namespaceURI>
            <pdfaSchema:prefix>ubl</pdfaSchema:prefix>
            <pdfaSchema:property>
              <rdf:Seq>
                <rdf:li rdf:parseType="Resource">
                  <pdfaProperty:name>DocumentFileName</pdfaProperty:name>
                  <pdfaProperty:valueType>Text</pdfaProperty:valueType>
                  <pdfaProperty:category>external</pdfaProperty:category>
                  <pdfaProperty:description>Name of the embedded UBL XML invoice file</pdfaProperty:description>
                </rdf:li>
                <rdf:li rdf:parseType="Resource">
                  <pdfaProperty:name>DocumentType</pdfaProperty:name>
                  <pdfaProperty:valueType>Text</pdfaProperty:valueType>
                  <pdfaProperty:category>external</pdfaProperty:category>
                  <pdfaProperty:description>Document type (INVOICE)</pdfaProperty:description>
                </rdf:li>
              </rdf:Seq>
            </pdfaSchema:property>
          </rdf:li>
        </rdf:Bag>
      </pdfaExtension:schemas>
    </rdf:Description>
    <rdf:Description rdf:about=""
        xmlns:ubl="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2">
      <ubl:DocumentFileName>%s</ubl:DocumentFileName>
      <ubl:DocumentType>INVOICE</ubl:DocumentType>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
%s<?xpacket end="w"?>`,
		xmlEscape(docID),
		xmlEscape(filename),
		xmpPadding(),
	)
	return []byte(xmp)
}

// xmpPadding returns the whitespace padding an XMP packet with end="w" (writable)
// should carry before the closing PI (ISO 16684-1 / XMP spec): ~2 KB of spaces, a
// newline every 80 columns. A near-zero-padding packet declaring end="w" is what a
// strict PDF/A validator flags.
func xmpPadding() string {
	var sb strings.Builder
	sb.WriteByte('\n')
	for i := 0; i < 24; i++ { // 24 * ~81 bytes ~= 1.9 KB
		sb.WriteString(strings.Repeat(" ", 80))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// minimalSRGBICC returns a structurally complete, self-authored sRGB ICC v2 profile
// (device class 'mntr', colour space RGB, PCS XYZ) carrying every tag an ICC / PDF-A
// validator requires of a matrix/TRC display profile: the profile description (desc),
// media white point (wtpt), the red/green/blue colorants (rXYZ/gXYZ/bXYZ), the three
// tone-reproduction curves (rTRC/gTRC/bTRC) and the copyright (cprt).
//
// It is built from first principles — no external profile file, no licence
// encumbrance — so it can be embedded directly in the PDF/A-3 OutputIntent. The
// colorants are the standard sRGB primaries chromatically adapted to the D50 PCS
// white; the tone curves use the sRGB ~2.2 gamma (a valid single-value curveType).
//
// NOTE: this replaces the earlier 128-byte zero-tag header (which was an invalid ICC
// profile). The structure is verified by a round-trip parse in the test suite, but a
// full PDF/A-3b conformance run still requires veraPDF (see verify.sh).
func minimalSRGBICC() []byte {
	be32 := func(v uint32) []byte { return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)} }
	be16 := func(v uint16) []byte { return []byte{byte(v >> 8), byte(v)} }
	s15 := func(f float64) []byte { return be32(uint32(int32(f*65536.0 + 0.5))) } // s15Fixed16Number

	xyzType := func(x, y, z float64) []byte {
		b := append([]byte("XYZ "), 0, 0, 0, 0)
		b = append(b, s15(x)...)
		b = append(b, s15(y)...)
		b = append(b, s15(z)...)
		return b
	}
	curveGamma := func(gamma float64) []byte {
		b := append([]byte("curv"), 0, 0, 0, 0)
		b = append(b, be32(1)...)                       // count 1 => gamma curve
		b = append(b, be16(uint16(gamma*256.0+0.5))...) // u8Fixed8Number
		return b
	}
	textType := func(s string) []byte {
		b := append([]byte("text"), 0, 0, 0, 0)
		return append(append(b, []byte(s)...), 0) // null-terminated ASCII
	}
	descType := func(s string) []byte { // textDescriptionType (ICC v2)
		ascii := append([]byte(s), 0)
		b := append([]byte("desc"), 0, 0, 0, 0)
		b = append(b, be32(uint32(len(ascii)))...) // ASCII count incl. null
		b = append(b, ascii...)
		b = append(b, be32(0)...)          // unicode language code
		b = append(b, be32(0)...)          // unicode count (0 => no unicode body)
		b = append(b, be16(0)...)          // scriptcode code
		b = append(b, 0)                   // scriptcode count
		b = append(b, make([]byte, 67)...) // fixed 67-byte Macintosh description
		return b
	}

	type tag struct {
		sig  string
		data []byte
	}
	tags := []tag{
		{"desc", descType("sRGB IEC61966-2.1")},
		{"wtpt", xyzType(0.9642, 1.0, 0.8249)}, // D50 PCS illuminant
		{"rXYZ", xyzType(0.43607, 0.22249, 0.01392)},
		{"gXYZ", xyzType(0.38515, 0.71687, 0.09708)},
		{"bXYZ", xyzType(0.14307, 0.06061, 0.71410)},
		{"rTRC", curveGamma(2.2)},
		{"gTRC", curveGamma(2.2)},
		{"bTRC", curveGamma(2.2)},
		{"cprt", textType("CC0 1.0 Universal - sRGB profile generated by wn1a-zatca-toolkit")},
	}

	// Layout: 128-byte header, tag table (4-byte count + 12 bytes per tag), then the
	// 4-byte-aligned tag data.
	dataStart := 128 + 4 + 12*len(tags)
	if r := dataStart % 4; r != 0 {
		dataStart += 4 - r
	}

	type entry struct {
		sig            string
		offset, length uint32
	}
	var data bytes.Buffer
	entries := make([]entry, 0, len(tags))
	off := dataStart
	for _, t := range tags {
		entries = append(entries, entry{t.sig, uint32(off), uint32(len(t.data))})
		data.Write(t.data)
		off += len(t.data)
		for off%4 != 0 { // align the next tag to a 4-byte boundary
			data.WriteByte(0)
			off++
		}
	}

	out := make([]byte, dataStart+data.Len())

	// Header (all unspecified fields stay zero, which is valid).
	copy(out[0:], be32(uint32(len(out))))         // profile size
	copy(out[8:], []byte{0x02, 0x10, 0x00, 0x00}) // version 2.1.0
	copy(out[12:], []byte("mntr"))                // device class
	copy(out[16:], []byte("RGB "))                // colour space
	copy(out[20:], []byte("XYZ "))                // PCS
	copy(out[36:], []byte("acsp"))                // profile file signature
	copy(out[64:], be32(0))                       // rendering intent: perceptual
	copy(out[68:], s15(0.9642))                   // PCS illuminant (D50) X
	copy(out[72:], s15(1.0))                      // Y
	copy(out[76:], s15(0.8249))                   // Z

	// Tag table.
	p := 128
	copy(out[p:], be32(uint32(len(tags))))
	p += 4
	for _, e := range entries {
		copy(out[p:], []byte(e.sig))
		copy(out[p+4:], be32(e.offset))
		copy(out[p+8:], be32(e.length))
		p += 12
	}

	// Tag data.
	copy(out[dataStart:], data.Bytes())
	return out
}

// asciiHexEncode encodes bytes as ASCIIHex for PDF stream filter.
func asciiHexEncode(data []byte) []byte {
	var sb strings.Builder
	sb.Grow(len(data)*2 + 1)
	for _, b := range data {
		fmt.Fprintf(&sb, "%02X", b)
	}
	sb.WriteByte('>')
	return []byte(sb.String())
}

// pdfEscapeString escapes a string for use in a PDF literal string (parentheses).
func pdfEscapeString(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, c := range s {
		switch c {
		case '(':
			sb.WriteString(`\(`)
		case ')':
			sb.WriteString(`\)`)
		case '\\':
			sb.WriteString(`\\`)
		default:
			sb.WriteRune(c)
		}
	}
	return sb.String()
}

// xmlEscape escapes a string for use in XML content / attribute values.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// simpleHexID generates a 16-byte (32 hex char) PDF file ID from a string.
func simpleHexID(input string) string {
	var h [16]byte
	for i, c := range []byte(input) {
		h[i%16] = (h[i%16]+c)*33 + byte(i)*7
	}
	for i := range h {
		h[i] = (h[i] + h[(i+7)%16]) * 31
	}
	var sb strings.Builder
	for _, b := range h {
		fmt.Fprintf(&sb, "%02X", b)
	}
	return sb.String()
}
