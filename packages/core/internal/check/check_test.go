package check

import (
	"os"
	"path/filepath"
	"testing"
)

func sample(t *testing.T, name string) []byte {
	t.Helper()
	p := filepath.Join("..", "..", "..", "..", "apps", "audit", "samples", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return b
}

func TestCheckGoodSamplePasses(t *testing.T) {
	r, err := CheckXML(sample(t, "good.xml"), false)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if r.Fatal {
		t.Fatalf("good sample must not be fatal; findings=%+v", r.Findings)
	}
}

func TestCheckBadSampleIsFatal(t *testing.T) {
	r, err := CheckXML(sample(t, "bad.xml"), false)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !r.Fatal {
		t.Fatal("bad sample must be fatal")
	}
	if len(r.Findings) == 0 {
		t.Fatal("bad sample must report findings")
	}
}

func TestCheckStructuralOnPlainUBLIsFatal(t *testing.T) {
	// good.xml is a plain UBL (no UUID/ICV/PIH/QR/signature) -> structural check is fatal.
	r, err := CheckXML(sample(t, "good.xml"), true)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !r.Fatal {
		t.Fatal("structural check on a plain UBL should be fatal (missing ZATCA elements)")
	}
}

func TestCheckRejectsGarbage(t *testing.T) {
	if _, err := CheckXML([]byte("<Order/>"), false); err == nil {
		t.Fatal("non-Invoice XML should error")
	}
}
