package ksa

import (
	"bytes"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/pem"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

var canonicalParams = CSRParams{
	CountryCode:       "SA",
	Organization:      "Test Corp",
	OrganizationUnit:  "Riyadh Branch",
	CommonName:        "EA123456789",
	SerialNumber:      "1-TST|2-v1|3-ed22f1d8-e6a2-1118-9b58-d9a8f11e445f",
	VAT:               "310122393500003",
	InvoiceType:       "1100",
	RegisteredAddress: "Riyadh, Saudi Arabia",
	BusinessCategory:  "Information Technology",
}

func TestGenerateCSR_OutputShape(t *testing.T) {
	res, err := GenerateCSR(canonicalParams)
	if err != nil {
		t.Fatalf("GenerateCSR: %v", err)
	}
	if len(res.CSRDER) == 0 {
		t.Fatal("CSRDER is empty")
	}
	if len(res.CSRPEM) == 0 {
		t.Fatal("CSRPEM is empty")
	}
	if len(res.PrivateKeyBytes) != 32 {
		t.Fatalf("PrivateKeyBytes len = %d, want 32", len(res.PrivateKeyBytes))
	}
	if len(res.PublicKeyBytes) != 65 || res.PublicKeyBytes[0] != 0x04 {
		t.Fatalf("PublicKeyBytes len/prefix = %d/0x%02x, want 65/0x04", len(res.PublicKeyBytes), res.PublicKeyBytes[0])
	}

	block, _ := pem.Decode(res.CSRPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		t.Fatal("PEM block missing or wrong type")
	}
	if !bytes.Equal(block.Bytes, res.CSRDER) {
		t.Fatal("PEM block bytes do not match CSRDER")
	}
}

func TestGenerateCSR_SignatureVerifies(t *testing.T) {
	res, err := GenerateCSR(canonicalParams)
	if err != nil {
		t.Fatalf("GenerateCSR: %v", err)
	}

	var outerRaw struct {
		CertReqInfoRaw asn1.RawValue
		SigAlgRaw      asn1.RawValue
		Sig            asn1.BitString
	}
	rest, err := asn1.Unmarshal(res.CSRDER, &outerRaw)
	if err != nil {
		t.Fatalf("unmarshal outer CSR: %v", err)
	}
	if len(rest) != 0 {
		t.Fatalf("trailing bytes after CSR: %d", len(rest))
	}

	certReqInfoDER := outerRaw.CertReqInfoRaw.FullBytes

	rawSig := outerRaw.Sig.Bytes
	if len(rawSig) != 64 {
		t.Fatalf("signature bytes len = %d, want 64", len(rawSig))
	}

	pub, err := secp256k1.ParsePubKey(res.PublicKeyBytes)
	if err != nil {
		t.Fatalf("parse pubkey: %v", err)
	}
	hash := sha256.Sum256(certReqInfoDER)
	if !VerifySecp256k1(pub.SerializeUncompressed(), hash[:], rawSig) {
		t.Fatal("CSR signature verification failed")
	}

	tampered := make([]byte, len(rawSig))
	copy(tampered, rawSig)
	tampered[0] ^= 0xFF
	if VerifySecp256k1(pub.SerializeUncompressed(), hash[:], tampered) {
		t.Fatal("tampered signature should not verify")
	}
}

func TestGenerateCSR_SubjectDNFields(t *testing.T) {
	res, err := GenerateCSR(canonicalParams)
	if err != nil {
		t.Fatalf("GenerateCSR: %v", err)
	}

	fields := extractSubjectFields(t, res.CSRDER)

	if fields[oidCountry.String()] != "SA" {
		t.Errorf("C = %q, want SA", fields[oidCountry.String()])
	}
	if fields[oidOrganization.String()] != canonicalParams.Organization {
		t.Errorf("O = %q, want %q", fields[oidOrganization.String()], canonicalParams.Organization)
	}
	if fields[oidOrganizationalUnit.String()] != canonicalParams.OrganizationUnit {
		t.Errorf("OU = %q, want %q", fields[oidOrganizationalUnit.String()], canonicalParams.OrganizationUnit)
	}
	if fields[oidCommonName.String()] != canonicalParams.CommonName {
		t.Errorf("CN = %q, want %q", fields[oidCommonName.String()], canonicalParams.CommonName)
	}
}

func TestGenerateCSR_ExtensionsPresent(t *testing.T) {
	res, err := GenerateCSR(canonicalParams)
	if err != nil {
		t.Fatalf("GenerateCSR: %v", err)
	}

	exts := extractExtensions(t, res.CSRDER)

	if _, ok := exts[oidBasicConstraints.String()]; !ok {
		t.Error("basicConstraints extension missing")
	}
	if _, ok := exts[oidKeyUsage.String()]; !ok {
		t.Error("keyUsage extension missing")
	}
	if _, ok := exts[oidCertificateTemplateName.String()]; !ok {
		t.Error("certificateTemplateName extension missing")
	}
	if _, ok := exts[oidSubjectAltName.String()]; !ok {
		t.Error("subjectAltName extension missing")
	}

	if !exts[oidKeyUsage.String()].critical {
		t.Error("keyUsage must be critical")
	}
}

func TestGenerateCSR_CertTemplateNameValue(t *testing.T) {
	res, err := GenerateCSR(canonicalParams)
	if err != nil {
		t.Fatalf("GenerateCSR: %v", err)
	}

	exts := extractExtensions(t, res.CSRDER)
	ct, ok := exts[oidCertificateTemplateName.String()]
	if !ok {
		t.Fatal("certificateTemplateName missing")
	}

	var ps asn1.RawValue
	if _, err := asn1.Unmarshal(ct.value, &ps); err != nil {
		t.Fatalf("unmarshal cert template value: %v", err)
	}
	if string(ps.Bytes) != "ZATCA-Code-Signing" {
		t.Errorf("cert template value = %q, want ZATCA-Code-Signing", string(ps.Bytes))
	}
}

func TestGenerateCSR_InputValidation(t *testing.T) {
	cases := []struct {
		name   string
		params CSRParams
	}{
		{
			name:   "wrong country",
			params: with(canonicalParams, func(p *CSRParams) { p.CountryCode = "US" }),
		},
		{
			name:   "invalid VAT short",
			params: with(canonicalParams, func(p *CSRParams) { p.VAT = "123" }),
		},
		{
			name:   "invalid VAT no 3 prefix",
			params: with(canonicalParams, func(p *CSRParams) { p.VAT = "410122393500003" }),
		},
		{
			name:   "invalid VAT no 3 suffix",
			params: with(canonicalParams, func(p *CSRParams) { p.VAT = "310122393500009" }),
		},
		{
			name:   "invalid invoice type",
			params: with(canonicalParams, func(p *CSRParams) { p.InvoiceType = "abc" }),
		},
		{
			name:   "invoice type too short",
			params: with(canonicalParams, func(p *CSRParams) { p.InvoiceType = "110" }),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := GenerateCSR(tc.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestGenerateCSRFromKey_Deterministic(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	priv := secp256k1.PrivKeyFromBytes(key)

	res1, err := GenerateCSRFromKey(canonicalParams, priv)
	if err != nil {
		t.Fatalf("first GenerateCSRFromKey: %v", err)
	}
	res2, err := GenerateCSRFromKey(canonicalParams, priv)
	if err != nil {
		t.Fatalf("second GenerateCSRFromKey: %v", err)
	}

	if !bytes.Equal(res1.CSRDER, res2.CSRDER) {
		t.Fatal("CSR output is not deterministic for the same key and params")
	}
}

// ---- helpers ----

func with(p CSRParams, fn func(*CSRParams)) CSRParams {
	fn(&p)
	return p
}

type parsedExtension struct {
	value    []byte
	critical bool
}

// extractSubjectFields returns a map of OID string → UTF-8 string value from the Subject DN.
func extractSubjectFields(t *testing.T, csrDER []byte) map[string]string {
	t.Helper()
	var outer struct {
		CertReqInfo asn1.RawValue
		SigAlg      asn1.RawValue
		Sig         asn1.BitString
	}
	if _, err := asn1.Unmarshal(csrDER, &outer); err != nil {
		t.Fatalf("extract subject: unmarshal outer: %v", err)
	}

	var cri struct {
		Version  int
		Subject  asn1.RawValue
		SPKI     asn1.RawValue
		Attrs    asn1.RawValue `asn1:"tag:0,optional"`
	}
	if _, err := asn1.Unmarshal(outer.CertReqInfo.FullBytes, &cri); err != nil {
		t.Fatalf("extract subject: unmarshal CRI: %v", err)
	}

	return parseRDNSequence(t, cri.Subject.FullBytes)
}

// parseRDNSequence extracts OID → value from a DER Name.
func parseRDNSequence(t *testing.T, nameDER []byte) map[string]string {
	t.Helper()
	result := make(map[string]string)
	var rdns []asn1.RawValue
	rest := nameDER
	var outer asn1.RawValue
	if _, err := asn1.Unmarshal(rest, &outer); err != nil {
		t.Fatalf("parseRDNSequence outer: %v", err)
	}
	rest = outer.Bytes
	for len(rest) > 0 {
		var rdn asn1.RawValue
		var err error
		rest, err = asn1.Unmarshal(rest, &rdn)
		if err != nil {
			t.Fatalf("parseRDNSequence rdn: %v", err)
		}
		rdns = append(rdns, rdn)
		setBytes := rdn.Bytes
		for len(setBytes) > 0 {
			var atv asn1.RawValue
			setBytes, err = asn1.Unmarshal(setBytes, &atv)
			if err != nil {
				t.Fatalf("parseRDNSequence atv: %v", err)
			}
			var oid asn1.ObjectIdentifier
			valRest, err := asn1.Unmarshal(atv.Bytes, &oid)
			if err != nil {
				t.Fatalf("parseRDNSequence oid: %v", err)
			}
			var val asn1.RawValue
			if _, err := asn1.Unmarshal(valRest, &val); err != nil {
				t.Fatalf("parseRDNSequence val: %v", err)
			}
			result[oid.String()] = string(val.Bytes)
		}
	}
	return result
}

// extractExtensions parses extensions from the PKCS#9 extensionRequest attribute in the CSR.
func extractExtensions(t *testing.T, csrDER []byte) map[string]parsedExtension {
	t.Helper()
	var outer struct {
		CertReqInfo asn1.RawValue
		SigAlg      asn1.RawValue
		Sig         asn1.BitString
	}
	if _, err := asn1.Unmarshal(csrDER, &outer); err != nil {
		t.Fatalf("extractExtensions: unmarshal outer: %v", err)
	}

	var cri struct {
		Version int
		Subject asn1.RawValue
		SPKI    asn1.RawValue
		Attrs   asn1.RawValue `asn1:"tag:0,optional"`
	}
	if _, err := asn1.Unmarshal(outer.CertReqInfo.FullBytes, &cri); err != nil {
		t.Fatalf("extractExtensions: unmarshal CRI: %v", err)
	}

	attrBytes := cri.Attrs.Bytes
	result := make(map[string]parsedExtension)

	for len(attrBytes) > 0 {
		var attr asn1.RawValue
		var err error
		attrBytes, err = asn1.Unmarshal(attrBytes, &attr)
		if err != nil {
			t.Fatalf("extractExtensions: unmarshal attr: %v", err)
		}
		var attrOID asn1.ObjectIdentifier
		attrValRest, err := asn1.Unmarshal(attr.Bytes, &attrOID)
		if err != nil {
			t.Fatalf("extractExtensions: unmarshal attr OID: %v", err)
		}
		if !attrOID.Equal(oidExtensionRequest) {
			continue
		}
		var attrValSet asn1.RawValue
		if _, err := asn1.Unmarshal(attrValRest, &attrValSet); err != nil {
			t.Fatalf("extractExtensions: unmarshal attrVal SET: %v", err)
		}
		var extsSeq asn1.RawValue
		if _, err := asn1.Unmarshal(attrValSet.Bytes, &extsSeq); err != nil {
			t.Fatalf("extractExtensions: unmarshal exts SEQUENCE: %v", err)
		}
		extBytes := extsSeq.Bytes
		for len(extBytes) > 0 {
			var extRaw asn1.RawValue
			extBytes, err = asn1.Unmarshal(extBytes, &extRaw)
			if err != nil {
				t.Fatalf("extractExtensions: unmarshal ext: %v", err)
			}
			var extOID asn1.ObjectIdentifier
			extRest, err := asn1.Unmarshal(extRaw.Bytes, &extOID)
			if err != nil {
				t.Fatalf("extractExtensions: ext OID: %v", err)
			}

			critical := false
			var firstVal asn1.RawValue
			extRest2, err := asn1.Unmarshal(extRest, &firstVal)
			if err != nil {
				t.Fatalf("extractExtensions: ext firstVal: %v", err)
			}
			var extnValue []byte
			if firstVal.Tag == asn1.TagBoolean {
				if len(firstVal.Bytes) > 0 && firstVal.Bytes[0] != 0 {
					critical = true
				}
				var octetStr []byte
				if _, err := asn1.Unmarshal(extRest2, &octetStr); err != nil {
					t.Fatalf("extractExtensions: ext octet after bool: %v", err)
				}
				extnValue = octetStr
			} else {
				extnValue = firstVal.Bytes
			}
			result[extOID.String()] = parsedExtension{value: extnValue, critical: critical}
		}
	}
	return result
}
