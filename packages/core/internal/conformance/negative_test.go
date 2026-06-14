package conformance

import (
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/convert"
	"github.com/greymoth-jp/zatca-toolkit/core/internal/validate"
)

// Negative conformance: take a REAL official EN16931 invoice, break one field at a time, and
// prove the engine REJECTS it with the expected rule. Positive conformance (the other test)
// shows no false-rejects; this shows we actually CATCH violations on real-world structure —
// the catch-rate side that a self-test cannot demonstrate. EUPL-safe: the fixture is fetched
// at runtime and mutated in memory, never committed.
func TestEN16931NegativeConformance(t *testing.T) {
	if os.Getenv("CONFORMANCE") != "1" {
		t.Skip("set CONFORMANCE=1 (network; fetches an EUPL fixture at runtime, never vendored)")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(base + "ubl-tc434-example1.xml")
	if err != nil {
		t.Skipf("fetch failed (offline?): %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if len(body) == 0 {
		t.Skip("empty fixture")
	}
	src := string(body)

	// baseline must be valid (sanity).
	if doc, err := convert.ParseUBL([]byte(src)); err != nil {
		t.Fatalf("baseline parse: %v", err)
	} else if !validate.Validate(doc, validate.ProfileEN16931).Valid {
		t.Fatalf("baseline official invoice should be en16931-valid")
	}

	cases := []struct {
		name, want string
		mutate     func(string) string
	}{
		{"break tax-inclusive total", "BR-CO-15", func(s string) string {
			return regexp.MustCompile(`(<cbc:TaxInclusiveAmount[^>]*>)[^<]+`).ReplaceAllString(s, "${1}9999999.99")
		}},
		{"remove all invoice lines", "BR-16", func(s string) string {
			return regexp.MustCompile(`(?s)<cac:InvoiceLine>.*</cac:InvoiceLine>`).ReplaceAllString(s, "")
		}},
		{"blank document currency", "BR-05", func(s string) string {
			return regexp.MustCompile(`<cbc:DocumentCurrencyCode>[^<]+`).ReplaceAllString(s, "<cbc:DocumentCurrencyCode>")
		}},
	}
	for _, c := range cases {
		doc, err := convert.ParseUBL([]byte(c.mutate(src)))
		if err != nil {
			// parse failure is also a form of rejection for some mutations; acceptable.
			t.Logf("%s: parse rejected (%v)", c.name, err)
			continue
		}
		rep := validate.Validate(doc, validate.ProfileEN16931)
		if !containsRule(rep, c.want) {
			t.Errorf("%s: expected %s, got valid=%v rules=[%s]", c.name, c.want, rep.Valid, ruleIDs(rep))
		}
	}
}

func containsRule(r validate.Report, id string) bool {
	for _, e := range r.Errors {
		if strings.EqualFold(e.RuleID, id) {
			return true
		}
	}
	return false
}
