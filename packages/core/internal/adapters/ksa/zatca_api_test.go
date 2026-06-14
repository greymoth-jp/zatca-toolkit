package ksa

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── (a) Credential-gate tests ────────────────────────────────────────────────

func TestAPIClient_BlockedWithoutCredentials(t *testing.T) {
	cases := []struct {
		name string
		fn   func(*APIClient) error
	}{
		{"RequestComplianceCSID", func(c *APIClient) error {
			_, err := c.RequestComplianceCSID(ComplianceCSIDRequest{CSR: "dGVzdA=="})
			return err
		}},
		{"RequestProductionCSID", func(c *APIClient) error {
			_, err := c.RequestProductionCSID(ProductionCSIDRequest{ComplianceRequestID: "req-1"})
			return err
		}},
		{"ClearInvoice", func(c *APIClient) error {
			_, err := c.ClearInvoice(InvoicePayload{InvoiceHash: "h", UUID: "u", Invoice: "i"})
			return err
		}},
		{"ReportInvoice", func(c *APIClient) error {
			_, err := c.ReportInvoice(InvoicePayload{InvoiceHash: "h", UUID: "u", Invoice: "i"})
			return err
		}},
	}

	empties := []APIClient{
		{},
		{BaseURL: "http://example.com"},
		{Username: "tok", Secret: "sec"},
		{BaseURL: "http://example.com", Username: "tok"},
		{BaseURL: "http://example.com", Secret: "sec"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, c := range empties {
				c := c
				err := tc.fn(&c)
				if !errors.Is(err, ErrBlockedNeedsCredentials) {
					t.Errorf("partial creds %+v: want ErrBlockedNeedsCredentials, got %v", c, err)
				}
			}
		})
	}
}

// ── helpers for stub server tests ────────────────────────────────────────────

func clientFor(t *testing.T, srv *httptest.Server) *APIClient {
	t.Helper()
	return &APIClient{
		BaseURL:    srv.URL,
		Username:   "dummy-token",
		Secret:     "dummy-secret",
		HTTPClient: srv.Client(),
	}
}

func assertBasicAuth(t *testing.T, r *http.Request) {
	t.Helper()
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Basic ") {
		t.Fatalf("missing Basic auth header, got %q", auth)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
	if err != nil {
		t.Fatalf("auth header not valid base64: %v", err)
	}
	if string(decoded) != "dummy-token:dummy-secret" {
		t.Fatalf("auth = %q, want dummy-token:dummy-secret", decoded)
	}
}

func assertAcceptVersion(t *testing.T, r *http.Request) {
	t.Helper()
	if v := r.Header.Get("Accept-Version"); v != "V2" {
		t.Fatalf("Accept-Version = %q, want V2", v)
	}
}

func decodeBody(t *testing.T, r *http.Request, dst any) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if err := json.Unmarshal(body, dst); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
}

// ── (b) Stub transport tests ─────────────────────────────────────────────────

func TestAPIClient_RequestComplianceCSID_ShapesRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/compliance" {
			t.Errorf("path = %q, want /compliance", r.URL.Path)
		}
		assertBasicAuth(t, r)
		assertAcceptVersion(t, r)

		var body ComplianceCSIDRequest
		decodeBody(t, r, &body)
		if body.CSR != "dGVzdC1jc3I=" {
			t.Errorf("CSR = %q, want dGVzdC1jc3I=", body.CSR)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ComplianceCSIDResponse{
			BinarySecurityToken: "tok-abc",
			Secret:              "sec-xyz",
			RequestID:           "req-001",
		})
	}))
	defer srv.Close()

	c := clientFor(t, srv)
	resp, err := c.RequestComplianceCSID(ComplianceCSIDRequest{CSR: "dGVzdC1jc3I="})
	if err != nil {
		t.Fatalf("RequestComplianceCSID: %v", err)
	}
	if resp.BinarySecurityToken != "tok-abc" {
		t.Errorf("BinarySecurityToken = %q, want tok-abc", resp.BinarySecurityToken)
	}
	if resp.Secret != "sec-xyz" {
		t.Errorf("Secret = %q, want sec-xyz", resp.Secret)
	}
	if resp.RequestID != "req-001" {
		t.Errorf("RequestID = %q, want req-001", resp.RequestID)
	}
}

func TestAPIClient_RequestProductionCSID_ShapesRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/production/csids" {
			t.Errorf("path = %q, want /production/csids", r.URL.Path)
		}
		assertBasicAuth(t, r)
		assertAcceptVersion(t, r)

		var body ProductionCSIDRequest
		decodeBody(t, r, &body)
		if body.ComplianceRequestID != "req-001" {
			t.Errorf("ComplianceRequestID = %q, want req-001", body.ComplianceRequestID)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ProductionCSIDResponse{
			BinarySecurityToken: "ptok-abc",
			Secret:              "psec-xyz",
			RequestID:           "preq-001",
		})
	}))
	defer srv.Close()

	c := clientFor(t, srv)
	resp, err := c.RequestProductionCSID(ProductionCSIDRequest{ComplianceRequestID: "req-001"})
	if err != nil {
		t.Fatalf("RequestProductionCSID: %v", err)
	}
	if resp.BinarySecurityToken != "ptok-abc" {
		t.Errorf("BinarySecurityToken = %q, want ptok-abc", resp.BinarySecurityToken)
	}
}

func TestAPIClient_ClearInvoice_ShapesRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/invoices/clearance/single" {
			t.Errorf("path = %q, want /invoices/clearance/single", r.URL.Path)
		}
		assertBasicAuth(t, r)
		assertAcceptVersion(t, r)

		var body InvoicePayload
		decodeBody(t, r, &body)
		if body.InvoiceHash != "hash-abc" {
			t.Errorf("InvoiceHash = %q, want hash-abc", body.InvoiceHash)
		}
		if body.UUID != "uuid-388" {
			t.Errorf("UUID = %q, want uuid-388", body.UUID)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ClearanceAPIResponse{
			ReportingStatus:  "CLEARED",
			ClearanceStatus:  "CLEARED",
			ValidationStatus: "VALID",
			CertificateID:    "cert-001",
			ClearedInvoice:   base64.StdEncoding.EncodeToString([]byte("<invoice/>")),
			WarningMessages:  []string{},
			ErrorMessages:    []string{},
		})
	}))
	defer srv.Close()

	c := clientFor(t, srv)
	resp, err := c.ClearInvoice(InvoicePayload{
		InvoiceHash: "hash-abc",
		UUID:        "uuid-388",
		Invoice:     base64.StdEncoding.EncodeToString([]byte("<signed/>")),
	})
	if err != nil {
		t.Fatalf("ClearInvoice: %v", err)
	}
	if resp.ClearanceStatus != "CLEARED" {
		t.Errorf("ClearanceStatus = %q, want CLEARED", resp.ClearanceStatus)
	}
	if resp.CertificateID != "cert-001" {
		t.Errorf("CertificateID = %q, want cert-001", resp.CertificateID)
	}
}

func TestAPIClient_ReportInvoice_ShapesRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/invoices/reporting/single" {
			t.Errorf("path = %q, want /invoices/reporting/single", r.URL.Path)
		}
		assertBasicAuth(t, r)
		assertAcceptVersion(t, r)

		var body InvoicePayload
		decodeBody(t, r, &body)
		if body.UUID != "uuid-383" {
			t.Errorf("UUID = %q, want uuid-383", body.UUID)
		}

		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ReportingAPIResponse{
			ReportingStatus: "SUBMITTED",
			StatusCode:      "202",
			Timestamp:       "2026-06-14T10:00:00Z",
		})
	}))
	defer srv.Close()

	c := clientFor(t, srv)
	resp, err := c.ReportInvoice(InvoicePayload{
		InvoiceHash: "hash-383",
		UUID:        "uuid-383",
		Invoice:     base64.StdEncoding.EncodeToString([]byte("<simplified/>")),
	})
	if err != nil {
		t.Fatalf("ReportInvoice: %v", err)
	}
	if resp.ReportingStatus != "SUBMITTED" {
		t.Errorf("ReportingStatus = %q, want SUBMITTED", resp.ReportingStatus)
	}
	if resp.StatusCode != "202" {
		t.Errorf("StatusCode = %q, want 202", resp.StatusCode)
	}
}

func TestAPIClient_HTTPError_PropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := clientFor(t, srv)
	_, err := c.ClearInvoice(InvoicePayload{InvoiceHash: "h", UUID: "u", Invoice: "i"})
	if err == nil {
		t.Fatal("expected error on 400 response, got nil")
	}
}
