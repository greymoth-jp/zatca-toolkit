// Command pdfa3 embeds a ZATCA UBL 2.1 XML invoice into a minimal PDF/A-3b document.
//
// Usage:
//
//	pdfa3 -xml invoice.xml [-out invoice.pdf] [-id INV-001]
//
// Flags:
//
//	-xml   path to the UBL 2.1 XML invoice file (required)
//	-out   output PDF path (default: <xml basename>.pdf)
//	-id    document identifier used in the PDF file ID (default: output filename)
//
// EXPERIMENTAL: see package documentation for conformance caveats.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/greymoth-jp/zatca-toolkit/tools/pdfa3"
)

func main() {
	xmlPath := flag.String("xml", "", "path to UBL 2.1 XML invoice file (required)")
	outPath := flag.String("out", "", "output PDF path (default: <xml>.pdf)")
	docID := flag.String("id", "", "document identifier for PDF file ID (default: output filename)")
	flag.Parse()

	if *xmlPath == "" {
		fmt.Fprintln(os.Stderr, "pdfa3: -xml flag is required")
		flag.Usage()
		os.Exit(1)
	}

	xmlBytes, err := os.ReadFile(*xmlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pdfa3: read xml: %v\n", err)
		os.Exit(1)
	}

	out := *outPath
	if out == "" {
		base := filepath.Base(*xmlPath)
		ext := filepath.Ext(base)
		out = strings.TrimSuffix(base, ext) + ".pdf"
	}

	id := *docID
	if id == "" {
		id = filepath.Base(out)
	}

	res, err := pdfa3.EmbedXMLIntoPDFA3(pdfa3.EmbedRequest{
		XMLContent:  xmlBytes,
		XMLFilename: filepath.Base(*xmlPath),
		DocumentID:  id,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "pdfa3: generate: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(out, res.PDF, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "pdfa3: write pdf: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("wrote %s (%d bytes) — conformance: %s\n", out, len(res.PDF), res.ConformanceLevel)
}
