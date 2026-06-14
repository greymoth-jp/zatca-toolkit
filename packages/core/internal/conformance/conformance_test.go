// Package conformance runs the engine against the OFFICIAL EN16931 example invoices to get
// an objective conformance number — the one metric that proves "are our rules actually
// right" vs self-tests. The official CEF examples are EUPL-licensed, so they are NEVER
// vendored: this test fetches them at runtime to a temp buffer and never writes them into
// the repo (no redistribution → LEGAL_RISK safe). It is env-gated (CONFORMANCE=1) so the
// normal `go test ./...` and offline CI skip it.
//
//	CONFORMANCE=1 go test ./internal/conformance/ -v
package conformance

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/convert"
	"github.com/greymoth-jp/zatca-toolkit/core/internal/validate"
)

const base = "https://raw.githubusercontent.com/ConnectingEurope/eInvoicing-EN16931/master/ubl/examples/"

// Official EN16931 UBL examples (EUPL — fetched at runtime only, not committed).
var examples = []string{
	"ubl-tc434-example1.xml", "ubl-tc434-example2.xml", "ubl-tc434-example3.xml",
	"ubl-tc434-example4.xml", "ubl-tc434-example5.xml", "ubl-tc434-example6.xml",
	"ubl-tc434-example7.xml", "ubl-tc434-example8.xml", "ubl-tc434-example9.xml",
	"ubl-tc434-example10.xml", "ubl-tc434-creditnote1.xml",
	"guide-example1.xml", "guide-example2.xml", "guide-example3.xml",
	"sample-discount-price.xml",
}

func ruleIDs(r validate.Report) string {
	var s []string
	for _, e := range r.Errors {
		s = append(s, e.RuleID)
	}
	return strings.Join(s, ",")
}

// TestEN16931OfficialConformance: parse + validate every official example. The hard bar is
// 100% PARSE coverage (we must be able to read every real EN16931 invoice). en16931 validity
// is logged: a known-valid example we mark invalid is a false-positive to investigate.
func TestEN16931OfficialConformance(t *testing.T) {
	if os.Getenv("CONFORMANCE") != "1" {
		t.Skip("set CONFORMANCE=1 to run (network; fetches EUPL fixtures at runtime, never vendored)")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	var fetched, parsed, valid int
	for _, name := range examples {
		resp, err := client.Get(base + name)
		if err != nil {
			t.Logf("FETCH FAIL %s: %v", name, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || len(body) == 0 {
			t.Logf("FETCH %s: status %d len %d", name, resp.StatusCode, len(body))
			continue
		}
		fetched++
		doc, err := convert.ParseUBL(body)
		if err != nil {
			t.Errorf("PARSE FAIL %s: %v", name, err)
			continue
		}
		parsed++
		rep := validate.Validate(doc, validate.ProfileEN16931)
		if rep.Valid {
			valid++
		} else {
			t.Logf("INFO %s: en16931 reported invalid -> %s", name, ruleIDs(rep))
		}
	}
	t.Logf("CONFORMANCE(EN16931 official): fetched %d/%d, parsed %d/%d, en16931-valid %d/%d",
		fetched, len(examples), parsed, len(examples), valid, len(examples))
	// Hard bar 1: every fetched official invoice must PARSE (read coverage).
	if fetched > 0 && parsed < fetched {
		t.Errorf("parse coverage below 100%%: parsed %d of %d fetched", parsed, fetched)
	}
	// Hard bar 2 (the FP gate / moat guard): no fetched official EN16931 invoice may be reported
	// INVALID by our rules — a false-positive means a rule is wrong or over-broad. This runs in CI
	// (CONFORMANCE=1), so any future rule that false-positives on a real invoice turns the build
	// red automatically. Network/fetch failures leave `fetched` low and do NOT trip this (only
	// genuine FPs among the invoices we actually fetched do), keeping CI robust to a flaky fetch.
	if fetched > 0 && valid < fetched {
		t.Errorf("conformance false-positives: %d of %d fetched official invoices reported invalid (see INFO logs above) — a rule is over-broad", fetched-valid, fetched)
	}
}
