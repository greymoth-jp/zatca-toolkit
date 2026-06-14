// Command zatca-check validates ZATCA/UBL invoice files and exits non-zero if any would not
// clear. Intended for CI: gate a repo of invoice fixtures, or a customer's pipeline, on the
// same deterministic rules the SDK/audit use.
//
//	zatca-check [--structural] file1.xml file2.xml ...
//
// --structural also verifies submitted-invoice elements (UUID/ICV/PIH/QR/signature).
// This is NOT tax advice; it reports against published rule sets only.
package main

import (
	"fmt"
	"os"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/check"
)

func main() {
	structural := false
	var files []string
	for _, a := range os.Args[1:] {
		switch a {
		case "--structural":
			structural = true
		case "-h", "--help":
			fmt.Fprintln(os.Stderr, "usage: zatca-check [--structural] <invoice.xml> ...")
			os.Exit(0)
		default:
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "usage: zatca-check [--structural] <invoice.xml> ...")
		os.Exit(2)
	}

	exit := 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Printf("ERROR  %s: %v\n", f, err)
			exit = 1
			continue
		}
		res, err := check.CheckXML(data, structural)
		if err != nil {
			fmt.Printf("ERROR  %s: %v\n", f, err)
			exit = 1
			continue
		}
		if res.Fatal {
			fmt.Printf("REJECTED  %s  (%d findings)\n", f, len(res.Findings))
			for _, fnd := range res.Findings {
				fmt.Printf("    [%s] %s %s\n", fnd.Severity, fnd.RuleID, fnd.MessageEN)
				if fnd.FixEN != "" {
					fmt.Printf("        fix: %s\n", fnd.FixEN)
				}
			}
			exit = 1
		} else if len(res.Findings) > 0 {
			fmt.Printf("CLEARED (with warnings)  %s  (%d)\n", f, len(res.Findings))
		} else {
			fmt.Printf("CLEARED  %s\n", f)
		}
	}
	os.Exit(exit)
}
