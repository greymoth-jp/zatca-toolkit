//go:build js && wasm

// Command wasm exposes the deterministic engine (validate / parse / generate) to JavaScript
// via syscall/js, so the same Go rules run in the browser (free audit app, P1a) and in Node
// (the @zatca/sdk, P0b) with zero server round-trip — the invoice never leaves the client,
// which is also the data-residency posture (LEGAL_RISK: invoice personal data stays client-side).
//
// Each exported function takes string arguments and returns a JSON string with a stable
// envelope: {ok, error?, report?, ubl?}. Returning JSON (not live JS objects) keeps the
// boundary simple and identical across browser/Node.
package main

import (
	"encoding/base64"
	"encoding/json"
	"syscall/js"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/adapters/ksa"
	"github.com/greymoth-jp/zatca-toolkit/core/internal/convert"
	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
	"github.com/greymoth-jp/zatca-toolkit/core/internal/pdfa3"
	"github.com/greymoth-jp/zatca-toolkit/core/internal/validate"
)

func main() {
	js.Global().Set("zatcaValidateXML", js.FuncOf(validateXML))
	js.Global().Set("zatcaValidateDoc", js.FuncOf(validateDoc))
	js.Global().Set("zatcaValidateQR", js.FuncOf(validateQR))
	js.Global().Set("zatcaValidateStructure", js.FuncOf(validateStructure))
	js.Global().Set("zatcaGenerateUBL", js.FuncOf(generateUBL))
	js.Global().Set("zatcaGenerateCII", js.FuncOf(generateCII))
	js.Global().Set("zatcaGenerateFacturX", js.FuncOf(generateFacturX))
	js.Global().Set("zatcaGenerateCSR", js.FuncOf(generateCSR))
	js.Global().Set("zatcaVersion", js.FuncOf(version))
	// Signal readiness, then block forever so the exported funcs stay callable.
	if cb := js.Global().Get("__zatcaReady"); cb.Type() == js.TypeFunction {
		cb.Invoke()
	}
	select {}
}

type envelope struct {
	OK       bool                 `json:"ok"`
	Error    string               `json:"error,omitempty"`
	Report   *validate.Report     `json:"report,omitempty"`
	Findings []validate.RuleError `json:"findings,omitempty"`
	UBL        string             `json:"ubl,omitempty"`
	CII        string             `json:"cii,omitempty"`
	PDF        string             `json:"pdf,omitempty"` // base64-encoded PDF/A-3 (Factur-X) bytes
	CSR        string             `json:"csr,omitempty"` // base64-encoded PKCS#10 CSR (DER)
	PrivateKey string             `json:"privateKey,omitempty"` // base64 secp256k1 scalar (sandbox/test only)
	PublicKey  string             `json:"publicKey,omitempty"`  // base64 uncompressed SEC1
	Version    string             `json:"version,omitempty"`
}

func emit(e envelope) string {
	b, err := json.Marshal(e)
	if err != nil {
		return `{"ok":false,"error":"marshal failed"}`
	}
	return string(b)
}

func fail(msg string) string { return emit(envelope{OK: false, Error: msg}) }

func profileFrom(args []js.Value, idx int) validate.Profile {
	if len(args) > idx {
		switch args[idx].String() {
		case string(validate.ProfileEN16931):
			return validate.ProfileEN16931
		case string(validate.ProfileZATCA):
			return validate.ProfileZATCA
		}
	}
	return validate.ProfilePeppol
}

// zatcaValidateXML(xmlString, profile?) — parse a UBL/ZATCA invoice and validate it.
func validateXML(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return fail("missing xml argument")
	}
	doc, err := convert.ParseUBL([]byte(args[0].String()))
	if err != nil {
		return fail("parse: " + err.Error())
	}
	return emit(validated(validate.Validate(doc, profileFrom(args, 1))))
}

// validated wraps a Report into the envelope, ensuring Errors is always an array (never
// JSON null) so JS consumers can iterate without a null check.
func validated(rep validate.Report) envelope {
	if rep.Errors == nil {
		rep.Errors = []validate.RuleError{}
	}
	return envelope{OK: rep.Valid, Report: &rep}
}

// zatcaValidateDoc(normalizedJSON, profile?) — validate an already-normalized invoice.
func validateDoc(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return fail("missing doc argument")
	}
	var doc normalized.Doc
	if err := json.Unmarshal([]byte(args[0].String()), &doc); err != nil {
		return fail("parse doc: " + err.Error())
	}
	return emit(validated(validate.Validate(&doc, profileFrom(args, 1))))
}

// zatcaGenerateUBL(normalizedJSON) — render a normalized invoice to UBL 2.1 XML.
func generateUBL(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return fail("missing doc argument")
	}
	var doc normalized.Doc
	if err := json.Unmarshal([]byte(args[0].String()), &doc); err != nil {
		return fail("parse doc: " + err.Error())
	}
	xmlBytes, err := convert.ToUBL(&doc)
	if err != nil {
		return fail("generate: " + err.Error())
	}
	return emit(envelope{OK: true, UBL: string(xmlBytes)})
}

// zatcaGenerateCII(normalizedJSON) — render a normalized invoice to UN/CEFACT CII (EN16931).
func generateCII(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return fail("missing doc argument")
	}
	var doc normalized.Doc
	if err := json.Unmarshal([]byte(args[0].String()), &doc); err != nil {
		return fail("parse doc: " + err.Error())
	}
	xmlBytes, err := convert.ToCII(&doc)
	if err != nil {
		return fail("generate: " + err.Error())
	}
	return emit(envelope{OK: true, CII: string(xmlBytes)})
}

// zatcaGenerateFacturX(normalizedJSON) — render the invoice to UN/CEFACT CII, then embed it in a
// Factur-X PDF/A-3 document. The PDF bytes are returned base64-encoded in envelope.pdf.
// NOTE: the document targets PDF/A-3b structure but is NOT veraPDF-verified here (see
// tools/pdfa3/verify.sh); the SDK/docs describe it as "PDF/A-3 structure", not certified PDF/A-3b.
func generateFacturX(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return fail("missing doc argument")
	}
	var doc normalized.Doc
	if err := json.Unmarshal([]byte(args[0].String()), &doc); err != nil {
		return fail("parse doc: " + err.Error())
	}
	cii, err := convert.ToCII(&doc)
	if err != nil {
		return fail("generate cii: " + err.Error())
	}
	res, err := pdfa3.EmbedCIIFacturX(cii, "EN 16931")
	if err != nil {
		return fail("embed factur-x: " + err.Error())
	}
	return emit(envelope{OK: true, PDF: base64.StdEncoding.EncodeToString(res.PDF)})
}

// zatcaGenerateCSR(paramsJSON) — generate a ZATCA PKCS#10 CSR + fresh secp256k1 key pair locally
// (no credentials). paramsJSON maps to ksa.CSRParams (CountryCode, Organization, OrganizationUnit,
// CommonName, SerialNumber, VAT, InvoiceType, RegisteredAddress, BusinessCategory, CertTemplateName).
// Returns base64 CSR (DER) + base64 private/public keys — the keys are for sandbox/test use.
func generateCSR(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return fail("missing params argument")
	}
	var p ksa.CSRParams
	if err := json.Unmarshal([]byte(args[0].String()), &p); err != nil {
		return fail("parse params: " + err.Error())
	}
	res, err := ksa.GenerateCSR(p)
	if err != nil {
		return fail("csr: " + err.Error())
	}
	return emit(envelope{
		OK:         true,
		CSR:        base64.StdEncoding.EncodeToString(res.CSRDER),
		PrivateKey: base64.StdEncoding.EncodeToString(res.PrivateKeyBytes),
		PublicKey:  base64.StdEncoding.EncodeToString(res.PublicKeyBytes),
	})
}

// zatcaValidateQR(qrBase64, signed?, simplified?) — verify a ZATCA QR (TLV) tag set.
func validateQR(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return fail("missing qr argument")
	}
	signed := len(args) < 2 || args[1].Bool() // default: treat as a signed (post-clearance) QR
	simplified := len(args) >= 3 && args[2].Bool()
	findings := ksa.ValidateQR(args[0].String(), signed, simplified)
	if findings == nil {
		findings = []validate.RuleError{}
	}
	return emit(envelope{OK: len(findings) == 0, Findings: findings})
}

// zatcaValidateStructure(xml) — verify the ZATCA structural elements (UUID/ICV/PIH/QR/signature)
// of a submitted/cleared invoice (distinct from the pre-submission semantic check).
func validateStructure(_ js.Value, args []js.Value) any {
	if len(args) < 1 {
		return fail("missing xml argument")
	}
	findings, err := convert.ZatcaStructuralIssues([]byte(args[0].String()))
	if err != nil {
		return fail("parse: " + err.Error())
	}
	if findings == nil {
		findings = []validate.RuleError{}
	}
	return emit(envelope{OK: len(findings) == 0, Findings: findings})
}

func version(_ js.Value, _ []js.Value) any {
	return emit(envelope{OK: true, Version: "zatca-toolkit-engine/0.1.0"})
}
