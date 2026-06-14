package pdfa3

import (
	"bytes"
	"strings"
	"testing"
)

// The embedded sRGB ICC profile must be a structurally valid ICC v2 profile (not the old
// 128-byte zero-tag stub): correct declared size, an 'acsp' signature, mntr/RGB/XYZ in the
// header, and a tag table whose entries are in bounds and carry the tags a matrix/TRC
// display profile needs. This is a structural round-trip — NOT a veraPDF conformance proof.
func TestMinimalSRGBICCStructure(t *testing.T) {
	p := minimalSRGBICC()
	if len(p) <= 128 {
		t.Fatalf("ICC profile must be a full profile with a tag table, got %d bytes", len(p))
	}
	be32 := func(o int) int { return int(p[o])<<24 | int(p[o+1])<<16 | int(p[o+2])<<8 | int(p[o+3]) }
	if size := be32(0); size != len(p) {
		t.Errorf("declared profile size %d != actual %d", size, len(p))
	}
	if string(p[36:40]) != "acsp" {
		t.Errorf("missing 'acsp' signature, got %q", p[36:40])
	}
	if string(p[12:16]) != "mntr" || string(p[16:20]) != "RGB " || string(p[20:24]) != "XYZ " {
		t.Errorf("header class/space/pcs wrong: %q/%q/%q", p[12:16], p[16:20], p[20:24])
	}
	n := be32(128)
	if n != 9 {
		t.Fatalf("tag count = %d, want 9", n)
	}
	seen := map[string]bool{}
	for i := 0; i < n; i++ {
		base := 132 + i*12
		sig := string(p[base : base+4])
		off, ln := be32(base+4), be32(base+8)
		if off < 132 || off+ln > len(p) {
			t.Errorf("tag %q out of bounds: off=%d len=%d total=%d", sig, off, ln, len(p))
			continue
		}
		switch typ := string(p[off : off+4]); typ {
		case "desc", "XYZ ", "curv", "text":
		default:
			t.Errorf("tag %q has unexpected type signature %q", sig, typ)
		}
		seen[sig] = true
	}
	for _, want := range []string{"desc", "wtpt", "rXYZ", "gXYZ", "bXYZ", "rTRC", "gTRC", "bTRC", "cprt"} {
		if !seen[want] {
			t.Errorf("required ICC tag %q missing", want)
		}
	}
}

// XMP packets declare end="w" (writable), which requires real whitespace padding before the
// closing PI. Both XMP builders must emit only-whitespace padding of >=100 bytes there.
func TestXMPHasWritablePadding(t *testing.T) {
	if pad := xmpPadding(); len(pad) < 100 || len(strings.TrimSpace(pad)) != 0 {
		t.Fatalf("xmpPadding must be >=100 bytes of whitespace, got %d bytes (non-ws %d)", len(pad), len(strings.TrimSpace(pad)))
	}
	cases := map[string][]byte{
		"ubl":      buildXMPMetadata("DOC-1", "invoice.xml"),
		"factur-x": buildFacturXXMP("factur-x.xml", "EN 16931"),
	}
	for name, xmp := range cases {
		i := bytes.Index(xmp, []byte("</x:xmpmeta>"))
		j := bytes.Index(xmp, []byte("<?xpacket end="))
		if i < 0 || j < 0 || j <= i {
			t.Fatalf("%s: xmp packet structure unexpected", name)
		}
		gap := xmp[i+len("</x:xmpmeta>") : j]
		if len(bytes.TrimSpace(gap)) != 0 {
			t.Errorf("%s: expected only whitespace before end PI, got %q", name, bytes.TrimSpace(gap))
		}
		if len(gap) < 100 {
			t.Errorf("%s: insufficient XMP padding before end PI: %d bytes", name, len(gap))
		}
	}
}
