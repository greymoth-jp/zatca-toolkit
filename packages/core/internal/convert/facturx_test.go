package convert

import (
	"bytes"
	"strings"
	"testing"
)

// FR-C03 (deterministic parts): the Factur-X package must carry the EN16931 CII as
// `factur-x.xml` (AFRelationship Alternative) and a PDF/A-3 XMP packet declaring the
// factur-x extension schema + pdfaid part 3.
func TestBuildFacturXPackage(t *testing.T) {
	pkg, err := BuildFacturX(sampleDoc(), ProfileEN16931FX)
	if err != nil {
		t.Fatalf("BuildFacturX error: %v", err)
	}
	if pkg.FileName != "factur-x.xml" {
		t.Errorf("embedded file must be factur-x.xml, got %q", pkg.FileName)
	}
	if pkg.AFRelationship != "Alternative" {
		t.Errorf("AFRelationship = %q, want Alternative", pkg.AFRelationship)
	}
	if pkg.MimeType != "application/xml" {
		t.Errorf("MimeType = %q", pkg.MimeType)
	}
	// payload is the EN16931 CII
	if !bytes.Contains(pkg.Payload, []byte("rsm:CrossIndustryInvoice")) {
		t.Error("payload must be CII XML")
	}

	xmp := string(pkg.XMP)
	for _, want := range []string{
		"<pdfaid:part>3</pdfaid:part>",
		"<pdfaid:conformance>B</pdfaid:conformance>",
		"urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#",
		"<fx:DocumentType>INVOICE</fx:DocumentType>",
		"<fx:DocumentFileName>factur-x.xml</fx:DocumentFileName>",
		"<fx:ConformanceLevel>EN 16931</fx:ConformanceLevel>",
		"<dc:title>",
		`<?xpacket begin`,
	} {
		if !strings.Contains(xmp, want) {
			t.Errorf("XMP missing %q\n---\n%s", want, xmp)
		}
	}
}

func TestFacturXDeterministic(t *testing.T) {
	a, _ := BuildFacturX(sampleDoc(), ProfileEN16931FX)
	b, _ := BuildFacturX(sampleDoc(), ProfileEN16931FX)
	if !bytes.Equal(a.XMP, b.XMP) || !bytes.Equal(a.Payload, b.Payload) {
		t.Fatal("Factur-X package must be deterministic")
	}
}

func TestFacturXProfileReflected(t *testing.T) {
	pkg, _ := BuildFacturX(sampleDoc(), ProfileBASIC)
	if !strings.Contains(string(pkg.XMP), "<fx:ConformanceLevel>BASIC</fx:ConformanceLevel>") {
		t.Error("conformance level must reflect the requested profile")
	}
}
