// Command zatca-onboard runs the ZATCA onboarding chain against the PUBLIC developer-portal
// sandbox (https://gw-fatoora.zatca.gov.sa/e-invoicing/developer-portal). The sandbox needs no
// VAT registration and accepts the dummy OTP "12345" — so this proves the toolkit can drive a
// real Compliance CSID handshake end to end. It is sandbox-only by design (the CSIDs returned are
// test/non-production); switching to simulation/production needs real ZATCA credentials.
//
// Usage: zatca-onboard [-otp 12345] [-vat 399999999900003]
//
// NOT tax advice. Sandbox results are not legally valid invoices.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/adapters/ksa"
)

const sandboxBase = "https://gw-fatoora.zatca.gov.sa/e-invoicing/developer-portal"

func main() {
	otp := flag.String("otp", "12345", "OTP (sandbox accepts the dummy value 12345)")
	vat := flag.String("vat", "399999999900003", "15-digit VAT (3...3)")
	flag.Parse()

	fmt.Println("=== WN1a ZATCA sandbox onboarding (developer-portal) ===")
	fmt.Println("base:", sandboxBase)

	// 1. Generate a sandbox CSR (TSTZATCA-Code-Signing template).
	params := ksa.CSRParams{
		CountryCode:       "SA",
		Organization:      "WN1a Test Org",
		OrganizationUnit:  "Riyadh Branch",
		CommonName:        "WN1a-EGS-1",
		SerialNumber:      "1-WN1a|2-0.1.0|3-11111111-1111-1111-1111-111111111111",
		VAT:               *vat,
		InvoiceType:       "1100",
		RegisteredAddress: "Riyadh, SA",
		BusinessCategory:  "Software",
		CertTemplateName:  "TSTZATCA-Code-Signing",
	}
	csr, err := ksa.GenerateCSR(params)
	if err != nil {
		fmt.Println("STEP 1 CSR generation FAILED:", err)
		os.Exit(1)
	}
	csrB64 := base64.StdEncoding.EncodeToString(csr.CSRDER)
	fmt.Printf("STEP 1 OK: CSR generated (%d DER bytes), template TSTZATCA-Code-Signing\n", len(csr.CSRDER))

	// 2. POST /compliance with the OTP header -> Compliance CSID.
	reqBody, _ := json.Marshal(map[string]string{"csr": csrB64})
	req, err := http.NewRequest("POST", sandboxBase+"/compliance", bytes.NewReader(reqBody))
	if err != nil {
		fmt.Println("STEP 2 build request FAILED:", err)
		os.Exit(1)
	}
	req.Header.Set("OTP", *otp)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Version", "V2")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("STEP 2 HTTP error:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)

	fmt.Printf("STEP 2 /compliance -> HTTP %d\n", resp.StatusCode)
	// Pretty-print the JSON envelope if possible.
	var pretty bytes.Buffer
	if json.Indent(&pretty, rb, "", "  ") == nil {
		fmt.Println(pretty.String())
	} else {
		fmt.Println(string(rb))
	}

	if resp.StatusCode == 200 {
		var csid ksa.ComplianceCSIDResponse
		if json.Unmarshal(rb, &csid) == nil && csid.BinarySecurityToken != "" {
			fmt.Println("RESULT: COMPLIANCE CSID OBTAINED — CSR accepted by ZATCA sandbox.")
			fmt.Println("  requestID:", csid.RequestID)
			fmt.Println("  token len:", len(csid.BinarySecurityToken), " secret len:", len(csid.Secret))
		} else {
			fmt.Println("RESULT: HTTP 200 but no binarySecurityToken parsed — inspect body above.")
		}
	} else {
		fmt.Println("RESULT: NOT a 200 — see status/body (likely CSR-format or OTP issue).")
	}
}
