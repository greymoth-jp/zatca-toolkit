package ksa

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// ErrBlockedNeedsCredentials is returned by APIClient methods when the client has no
// credentials configured. This is an honest blocker: no fabricated responses, no real
// network calls. See ZATCA_SANDBOX_PLAN.md §4 for how to obtain credentials.
var ErrBlockedNeedsCredentials = errors.New("BLOCKED: requires onboarded EGS + ZATCA Production CSID")

// ── Onboarding flow structs ──────────────────────────────────────────────────

// ComplianceCSIDRequest is the body for POST /compliance (Phase B of onboarding).
// CSR is the base64-encoded DER Certificate Signing Request generated locally.
type ComplianceCSIDRequest struct {
	CSR string `json:"csr"`
}

// ComplianceCSIDResponse is the successful response from the compliance endpoint.
// BinarySecurityToken + Secret are used as Basic auth credentials for subsequent calls.
type ComplianceCSIDResponse struct {
	BinarySecurityToken string `json:"binarySecurityToken"`
	Secret              string `json:"secret"`
	RequestID           string `json:"requestID"`
}

// ProductionCSIDRequest is the body for POST /production/csids (Phase D of onboarding).
type ProductionCSIDRequest struct {
	ComplianceRequestID string `json:"compliance_request_id"`
}

// ProductionCSIDResponse is the successful response from the production-CSID endpoint.
// These credentials are valid for 1 year and gate all clearance/reporting calls.
type ProductionCSIDResponse struct {
	BinarySecurityToken string `json:"binarySecurityToken"`
	Secret              string `json:"secret"`
	RequestID           string `json:"requestID"`
}

// ── Document flow structs ────────────────────────────────────────────────────

// InvoicePayload is the shared body for clearance and reporting submissions.
type InvoicePayload struct {
	InvoiceHash string `json:"invoiceHash"` // base64 SHA-256 of the canonical XML
	UUID        string `json:"uuid"`        // UUID-4, assigned once at invoice creation
	Invoice     string `json:"invoice"`     // base64-encoded signed XML
}

// ClearanceAPIResponse is the synchronous response from the clearance endpoint (standard 388).
type ClearanceAPIResponse struct {
	ReportingStatus  string   `json:"reportingStatus"`
	ClearanceStatus  string   `json:"clearanceStatus"`
	ValidationStatus string   `json:"validationStatus"`
	CertificateID    string   `json:"certificateId"`
	InvoiceNumber    string   `json:"invoiceNumber"`
	ClearedInvoice   string   `json:"clearedInvoice"` // base64 XML with ZATCA stamp (tag 9)
	Error            *string  `json:"error"`
	WarningMessages  []string `json:"warningMessages"`
	ErrorMessages    []string `json:"errorMessages"`
}

// ReportingAPIResponse is the asynchronous response from the reporting endpoint (simplified 383).
type ReportingAPIResponse struct {
	ReportingStatus string   `json:"reportingStatus"`
	StatusCode      string   `json:"statusCode"`
	Timestamp       string   `json:"timestamp"`
	ErrorMessages   []string `json:"errorMessages,omitempty"`
}

// ── APIClient ────────────────────────────────────────────────────────────────

// APIClient is the Fatoora HTTP API client for onboarding and document flows.
// BaseURL, Username, and Secret MUST all be non-empty for any method to execute;
// otherwise ErrBlockedNeedsCredentials is returned without a network call.
//
// Inject a custom HTTPClient (e.g. wrapping an httptest.Server) for tests.
type APIClient struct {
	BaseURL    string       // e.g. "https://gw-fatoora.zatca.gov.sa/e-invoicing/simulation"
	Username   string       // binarySecurityToken from ComplianceCSIDResponse or ProductionCSIDResponse
	Secret     string       // secret from the same response
	HTTPClient *http.Client // nil → http.DefaultClient; inject for tests
}

func (c *APIClient) ready() bool {
	return c.BaseURL != "" && c.Username != "" && c.Secret != ""
}

func (c *APIClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *APIClient) basicAuth() string {
	raw := c.Username + ":" + c.Secret
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

func (c *APIClient) do(method, path string, body, dst any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return fmt.Errorf("zatca api: encode body: %w", err)
		}
	}

	req, err := http.NewRequest(method, c.BaseURL+path, &buf)
	if err != nil {
		return fmt.Errorf("zatca api: build request: %w", err)
	}
	req.Header.Set("Authorization", c.basicAuth())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Version", "V2")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("zatca api: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("zatca api: status %d", resp.StatusCode)
	}
	if dst != nil {
		if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
			return fmt.Errorf("zatca api: decode response: %w", err)
		}
	}
	return nil
}

// RequestComplianceCSID submits a CSR to the compliance onboarding endpoint.
// CREDENTIAL-GATED: returns ErrBlockedNeedsCredentials when Username/Secret are empty.
// The compliance endpoint uses a temporary OTP-derived credential (see §4.1 of the plan).
func (c *APIClient) RequestComplianceCSID(req ComplianceCSIDRequest) (ComplianceCSIDResponse, error) {
	if !c.ready() {
		return ComplianceCSIDResponse{}, ErrBlockedNeedsCredentials
	}
	var resp ComplianceCSIDResponse
	if err := c.do("POST", "/compliance", req, &resp); err != nil {
		return ComplianceCSIDResponse{}, err
	}
	return resp, nil
}

// RequestProductionCSID exchanges a passing compliance request ID for a Production CSID.
// CREDENTIAL-GATED: returns ErrBlockedNeedsCredentials when credentials are absent.
func (c *APIClient) RequestProductionCSID(req ProductionCSIDRequest) (ProductionCSIDResponse, error) {
	if !c.ready() {
		return ProductionCSIDResponse{}, ErrBlockedNeedsCredentials
	}
	var resp ProductionCSIDResponse
	if err := c.do("POST", "/production/csids", req, &resp); err != nil {
		return ProductionCSIDResponse{}, err
	}
	return resp, nil
}

// ClearInvoice submits a signed standard invoice (type 388) to the clearance endpoint.
// The call is synchronous; ZATCA returns the stamped XML in ClearedInvoice.
// CREDENTIAL-GATED: returns ErrBlockedNeedsCredentials when credentials are absent.
func (c *APIClient) ClearInvoice(payload InvoicePayload) (ClearanceAPIResponse, error) {
	if !c.ready() {
		return ClearanceAPIResponse{}, ErrBlockedNeedsCredentials
	}
	var resp ClearanceAPIResponse
	if err := c.do("POST", "/invoices/clearance/single", payload, &resp); err != nil {
		return ClearanceAPIResponse{}, err
	}
	return resp, nil
}

// ReportInvoice submits a signed simplified invoice (type 383) to the reporting endpoint.
// The call is asynchronous (24-hour SLA); ZATCA responds with 202 Accepted.
// CREDENTIAL-GATED: returns ErrBlockedNeedsCredentials when credentials are absent.
func (c *APIClient) ReportInvoice(payload InvoicePayload) (ReportingAPIResponse, error) {
	if !c.ready() {
		return ReportingAPIResponse{}, ErrBlockedNeedsCredentials
	}
	var resp ReportingAPIResponse
	if err := c.do("POST", "/invoices/reporting/single", payload, &resp); err != nil {
		return ReportingAPIResponse{}, err
	}
	return resp, nil
}
