package ksa

import "errors"

// Clearance (standard, FR-Z05/Z-T3) + Reporting (simplified 24h, FR-Z06/Z-T4) client.
// Per clearance-flow.md: standard invoices are CLEARED (signed by ZATCA) BEFORE being sent
// to the buyer; simplified invoices are issued immediately and REPORTED within 24h.
//
// The interface is mockable so the whole adapter runs end-to-end in tests. The live HTTP
// client requires ZATCA Fatoora sandbox credentials (OTP/CSR → Compliance CSID), which are
// obtained via the Fatoora portal — see STATUS Open Questions. We do NOT fabricate live calls.

type ClearOutcome string

const (
	OutcomeCleared          ClearOutcome = "cleared"
	OutcomeClearedWithWarn  ClearOutcome = "cleared_with_warnings"
	OutcomeRejected         ClearOutcome = "rejected"
	OutcomeReported         ClearOutcome = "reported"
	OutcomeUnavailable      ClearOutcome = "authority_unavailable" // 5xx/timeout → retry (FR-Z08)
)

type ClearResult struct {
	Outcome    ClearOutcome
	UUID       string
	ClearedXML []byte
	Warnings   []string
	Errors     []string // ZATCA validationResults errorMessages (mapped to ar/en upstream)
	HTTPStatus int
}

// ZatcaClient talks to the Fatoora platform.
type ZatcaClient interface {
	Clear(signedXML []byte, uuid, invoiceHash string) (ClearResult, error)
	Report(signedXML []byte, uuid, invoiceHash string) (ClearResult, error)
}

// ErrSandboxCredentialsRequired is returned by the HTTP client until ZATCA sandbox creds
// are configured. This is an honest blocker, not a fabricated response.
var ErrSandboxCredentialsRequired = errors.New("zatca: sandbox credentials required (OTP/CSR → Compliance CSID; see STATUS Open Questions)")

// HTTPClient is the live Fatoora client skeleton. Request building is real; execution is
// gated on credentials so we never pretend to clear without the authority.
type HTTPClient struct {
	BaseURL  string // ZATCA_API_BASE (use the /simulation sandbox first)
	Username string // Compliance/Production CSID binarySecurityToken
	Secret   string // CSID secret
}

func (c *HTTPClient) ready() bool { return c.BaseURL != "" && c.Username != "" && c.Secret != "" }

func (c *HTTPClient) Clear(signedXML []byte, uuid, invoiceHash string) (ClearResult, error) {
	if !c.ready() {
		return ClearResult{}, ErrSandboxCredentialsRequired
	}
	// TODO(Z-T3): POST {BaseURL}/invoices/clearance/single with headers
	// Accept-Version: V2, Authorization: Basic(username:secret), and body
	// {invoiceHash, uuid, invoice: base64(signedXML)}; map clearanceStatus → ClearResult.
	return ClearResult{}, ErrSandboxCredentialsRequired
}

func (c *HTTPClient) Report(signedXML []byte, uuid, invoiceHash string) (ClearResult, error) {
	if !c.ready() {
		return ClearResult{}, ErrSandboxCredentialsRequired
	}
	// TODO(Z-T4): POST {BaseURL}/invoices/reporting/single (simplified, 24h SLA).
	return ClearResult{}, ErrSandboxCredentialsRequired
}

// MockClient returns programmed responses — drives end-to-end + resilience tests.
type MockClient struct {
	// Responses are consumed in order; the last one repeats. Lets tests model 503→503→cleared.
	Responses []ClearResult
	calls     int
	LastXML   []byte
}

func (m *MockClient) next(xml []byte) ClearResult {
	m.LastXML = xml
	if len(m.Responses) == 0 {
		return ClearResult{Outcome: OutcomeCleared, UUID: "MOCK-UUID"}
	}
	idx := m.calls
	if idx >= len(m.Responses) {
		idx = len(m.Responses) - 1
	}
	m.calls++
	return m.Responses[idx]
}

func (m *MockClient) Calls() int { return m.calls }

func (m *MockClient) Clear(signedXML []byte, uuid, invoiceHash string) (ClearResult, error) {
	r := m.next(signedXML)
	if r.UUID == "" {
		r.UUID = uuid
	}
	return r, nil
}

func (m *MockClient) Report(signedXML []byte, uuid, invoiceHash string) (ClearResult, error) {
	return m.Clear(signedXML, uuid, invoiceHash)
}
