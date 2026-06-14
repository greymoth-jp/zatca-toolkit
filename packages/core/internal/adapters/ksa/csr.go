package ksa

import (
	"crypto/sha256"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

// CSRParams holds all inputs needed to generate a ZATCA-compliant PKCS#10 CSR.
type CSRParams struct {
	CountryCode      string // must be "SA"
	Organization     string // taxpayer legal name
	OrganizationUnit string // branch/location name
	CommonName       string // EGS device identifier

	SerialNumber      string // "1-<name>|2-<version>|3-<UUID>"
	VAT               string // 15-digit, 3XXXXXXXXXXXXX3
	InvoiceType       string // 4-digit binary e.g. "1100"
	RegisteredAddress string // free text
	BusinessCategory  string // free text

	// CertTemplateName is the ZATCA certificate-template name, which is ENVIRONMENT-SPECIFIC and a
	// frequent cause of a sandbox "Invalid CSR" rejection if wrong:
	//   - sandbox / developer-portal: "TSTZATCA-Code-Signing"
	//   - simulation:                 "PREZATCA-Code-Signing"
	//   - production:                 "ZATCA-Code-Signing"
	// Empty defaults to the production template.
	CertTemplateName string
}

// CSRResult is the output of GenerateCSR.
type CSRResult struct {
	CSRPEM          []byte // PEM-encoded PKCS#10 CSR
	CSRDER          []byte // DER-encoded PKCS#10 CSR
	PrivateKeyBytes []byte // secp256k1 scalar, 32 bytes — store securely
	PublicKeyBytes  []byte // uncompressed SEC1, 65 bytes
}

// OIDs used throughout CSR construction.
var (
	oidCountry            = asn1.ObjectIdentifier{2, 5, 4, 6}
	oidOrganization       = asn1.ObjectIdentifier{2, 5, 4, 10}
	oidOrganizationalUnit = asn1.ObjectIdentifier{2, 5, 4, 11}
	oidCommonName         = asn1.ObjectIdentifier{2, 5, 4, 3}
	oidSerialNumber       = asn1.ObjectIdentifier{2, 5, 4, 5}
	oidUID                = asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 1}
	oidTitle              = asn1.ObjectIdentifier{2, 5, 4, 12}
	oidDescription        = asn1.ObjectIdentifier{2, 5, 4, 13}
	oidBusinessCategory   = asn1.ObjectIdentifier{2, 5, 4, 15}

	oidECPublicKey = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}
	oidSecp256k1   = asn1.ObjectIdentifier{1, 3, 132, 0, 10}

	oidECDSAWithSHA256 = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2}

	oidBasicConstraints        = asn1.ObjectIdentifier{2, 5, 29, 19}
	oidKeyUsage                = asn1.ObjectIdentifier{2, 5, 29, 15}
	oidSubjectAltName          = asn1.ObjectIdentifier{2, 5, 29, 17}
	oidCertificateTemplateName = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 20, 2}
	oidExtensionRequest        = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 14}
)

var (
	vatRE         = regexp.MustCompile(`^3\d{13}3$`)
	invoiceTypeRE = regexp.MustCompile(`^[01]{4}$`)
)

// GenerateCSR produces a ZATCA-compliant PKCS#10 CSR with a fresh secp256k1 key pair.
// No credentials are required; this is a pure local operation.
func GenerateCSR(params CSRParams) (*CSRResult, error) {
	if err := validateCSRParams(params); err != nil {
		return nil, err
	}
	priv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("zatca csr: key generation: %w", err)
	}
	return buildCSR(params, priv)
}

// GenerateCSRFromKey builds a CSR using a caller-supplied private key (deterministic tests).
func GenerateCSRFromKey(params CSRParams, priv *secp256k1.PrivateKey) (*CSRResult, error) {
	if priv == nil {
		return nil, errors.New("zatca csr: nil private key")
	}
	if err := validateCSRParams(params); err != nil {
		return nil, err
	}
	return buildCSR(params, priv)
}

func validateCSRParams(p CSRParams) error {
	if p.CountryCode != "SA" {
		return fmt.Errorf("zatca csr: country code must be SA, got %q", p.CountryCode)
	}
	if !vatRE.MatchString(p.VAT) {
		return fmt.Errorf("zatca csr: invalid VAT %q (must be 15 digits, 3...3)", p.VAT)
	}
	if !invoiceTypeRE.MatchString(p.InvoiceType) {
		return fmt.Errorf("zatca csr: invalid invoice type %q (must be 4-digit 0/1 string)", p.InvoiceType)
	}
	return nil
}

func buildCSR(params CSRParams, priv *secp256k1.PrivateKey) (*CSRResult, error) {
	privBytes := priv.Serialize()
	pubBytes := priv.PubKey().SerializeUncompressed()

	subjectDER, err := encodeSubjectDN(params)
	if err != nil {
		return nil, fmt.Errorf("zatca csr: subject DN: %w", err)
	}

	spkiDER, err := encodeSecp256k1SPKI(pubBytes)
	if err != nil {
		return nil, fmt.Errorf("zatca csr: spki: %w", err)
	}

	attrDER, err := encodeExtensionsAttribute(params)
	if err != nil {
		return nil, fmt.Errorf("zatca csr: extensions: %w", err)
	}

	certReqInfoDER, err := encodeCertificationRequestInfo(subjectDER, spkiDER, attrDER)
	if err != nil {
		return nil, fmt.Errorf("zatca csr: certReqInfo: %w", err)
	}

	hash := sha256.Sum256(certReqInfoDER)
	sig := ecdsa.Sign(priv, hash[:])
	rawSig, err := derToRawSig(sig.Serialize())
	if err != nil {
		return nil, fmt.Errorf("zatca csr: sig convert: %w", err)
	}

	csrDER, err := encodeCertificationRequest(certReqInfoDER, rawSig)
	if err != nil {
		return nil, fmt.Errorf("zatca csr: marshal csr: %w", err)
	}

	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	return &CSRResult{
		CSRPEM:          csrPEM,
		CSRDER:          csrDER,
		PrivateKeyBytes: privBytes,
		PublicKeyBytes:  pubBytes,
	}, nil
}

// encodeSubjectDN returns a DER-encoded RDNSequence for the Subject field.
// Order: C → OU → O → CN per spec §1.1.
func encodeSubjectDN(p CSRParams) ([]byte, error) {
	fields := []struct {
		oid asn1.ObjectIdentifier
		val string
	}{
		{oidCountry, p.CountryCode},
		{oidOrganizationalUnit, p.OrganizationUnit},
		{oidOrganization, p.Organization},
		{oidCommonName, p.CommonName},
	}
	return encodeRDNSequence(fields)
}

func encodeRDNSequence(fields []struct {
	oid asn1.ObjectIdentifier
	val string
}) ([]byte, error) {
	// RDNSequence ::= SEQUENCE OF RelativeDistinguishedName
	// RelativeDistinguishedName ::= SET OF AttributeTypeAndValue
	// AttributeTypeAndValue ::= SEQUENCE { type OID, value ANY }
	var rdnSeqBytes []byte
	for _, f := range fields {
		atv, err := encodeAttributeTypeAndValue(f.oid, f.val)
		if err != nil {
			return nil, err
		}
		// Wrap single ATV in a SET (one-element RDN).
		rdnBytes, err := asn1.Marshal(asn1.RawValue{
			Class:      asn1.ClassUniversal,
			Tag:        asn1.TagSet,
			IsCompound: true,
			Bytes:      atv,
		})
		if err != nil {
			return nil, err
		}
		rdnSeqBytes = append(rdnSeqBytes, rdnBytes...)
	}
	// Wrap the SET-of-RDNs in a SEQUENCE.
	return asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassUniversal,
		Tag:        asn1.TagSequence,
		IsCompound: true,
		Bytes:      rdnSeqBytes,
	})
}

func encodeAttributeTypeAndValue(oid asn1.ObjectIdentifier, value string) ([]byte, error) {
	oidDER, err := asn1.Marshal(oid)
	if err != nil {
		return nil, err
	}
	valDER, err := asn1.Marshal(asn1.RawValue{
		Class: asn1.ClassUniversal,
		Tag:   asn1.TagUTF8String,
		Bytes: []byte(value),
	})
	if err != nil {
		return nil, err
	}
	inner := append(oidDER, valDER...)
	return asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassUniversal,
		Tag:        asn1.TagSequence,
		IsCompound: true,
		Bytes:      inner,
	})
}

// encodeSecp256k1SPKI returns a DER SubjectPublicKeyInfo for a secp256k1 public key.
func encodeSecp256k1SPKI(pub65 []byte) ([]byte, error) {
	if len(pub65) != 65 || pub65[0] != 0x04 {
		return nil, errors.New("zatca csr: public key must be 65-byte uncompressed SEC1")
	}
	// AlgorithmIdentifier ::= SEQUENCE { algorithm OID, parameters OID }
	algIDBytes, err := func() ([]byte, error) {
		oidECPubDER, err := asn1.Marshal(oidECPublicKey)
		if err != nil {
			return nil, err
		}
		paramDER, err := asn1.Marshal(oidSecp256k1)
		if err != nil {
			return nil, err
		}
		inner := append(oidECPubDER, paramDER...)
		return asn1.Marshal(asn1.RawValue{
			Class:      asn1.ClassUniversal,
			Tag:        asn1.TagSequence,
			IsCompound: true,
			Bytes:      inner,
		})
	}()
	if err != nil {
		return nil, err
	}

	bitStrDER, err := asn1.Marshal(asn1.BitString{Bytes: pub65, BitLength: len(pub65) * 8})
	if err != nil {
		return nil, err
	}

	inner := append(algIDBytes, bitStrDER...)
	return asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassUniversal,
		Tag:        asn1.TagSequence,
		IsCompound: true,
		Bytes:      inner,
	})
}

// x509Extension mirrors the RFC 5280 Extension structure.
type x509Extension struct {
	ExtnID    asn1.ObjectIdentifier
	Critical  bool   `asn1:"optional"`
	ExtnValue []byte // encoded as OCTET STRING by asn1 for []byte
}

// encodeExtensionsAttribute encodes all ZATCA extensions as a PKCS#9 extensionRequest Attribute.
func encodeExtensionsAttribute(p CSRParams) ([]byte, error) {
	exts, err := buildZATCAExtensions(p)
	if err != nil {
		return nil, err
	}

	// Encode []x509Extension as SEQUENCE OF Extension.
	var extsSeqBytes []byte
	for _, ext := range exts {
		extDER, err := asn1.Marshal(ext)
		if err != nil {
			return nil, fmt.Errorf("marshal extension %v: %w", ext.ExtnID, err)
		}
		extsSeqBytes = append(extsSeqBytes, extDER...)
	}
	extsSeqDER, err := asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassUniversal,
		Tag:        asn1.TagSequence,
		IsCompound: true,
		Bytes:      extsSeqBytes,
	})
	if err != nil {
		return nil, err
	}

	// attrValue ::= SET OF AttributeValue; for extensionRequest, AttributeValue = Extensions (SEQUENCE OF).
	attrValueDER, err := asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassUniversal,
		Tag:        asn1.TagSet,
		IsCompound: true,
		Bytes:      extsSeqDER,
	})
	if err != nil {
		return nil, err
	}

	// Attribute ::= SEQUENCE { attrType OID, attrValues SET OF }
	attrTypeOIDDER, err := asn1.Marshal(oidExtensionRequest)
	if err != nil {
		return nil, err
	}
	attrInner := append(attrTypeOIDDER, attrValueDER...)
	return asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassUniversal,
		Tag:        asn1.TagSequence,
		IsCompound: true,
		Bytes:      attrInner,
	})
}

// buildZATCAExtensions returns the four ZATCA-required extensions.
func buildZATCAExtensions(p CSRParams) ([]x509Extension, error) {
	bc, err := buildBasicConstraints()
	if err != nil {
		return nil, err
	}
	ku, err := buildKeyUsage()
	if err != nil {
		return nil, err
	}
	ct, err := buildCertTemplateName(p.CertTemplateName)
	if err != nil {
		return nil, err
	}
	san, err := buildSubjectAltName(p)
	if err != nil {
		return nil, err
	}
	return []x509Extension{bc, ku, ct, san}, nil
}

// buildBasicConstraints: OID 2.5.29.19, CA:FALSE, not critical.
func buildBasicConstraints() (x509Extension, error) {
	// basicConstraints ::= SEQUENCE { cA BOOLEAN DEFAULT FALSE }; empty SEQUENCE = CA:FALSE.
	val, err := asn1.Marshal(struct{ CA bool `asn1:"optional"` }{})
	if err != nil {
		return x509Extension{}, err
	}
	return x509Extension{ExtnID: oidBasicConstraints, Critical: false, ExtnValue: val}, nil
}

// buildKeyUsage: bits 0=digitalSignature, 1=nonRepudiation, 2=keyEncipherment, critical.
func buildKeyUsage() (x509Extension, error) {
	// Named bit string: bit 0 = MSB of first content byte.
	// Bits 0,1,2 set → 0b111xxxxx → 0xE0; unused bits = 5.
	bs := asn1.BitString{Bytes: []byte{0xE0}, BitLength: 3}
	val, err := asn1.Marshal(bs)
	if err != nil {
		return x509Extension{}, err
	}
	return x509Extension{ExtnID: oidKeyUsage, Critical: true, ExtnValue: val}, nil
}

// buildCertTemplateName: OID 1.3.6.1.4.1.311.20.2, PRINTABLESTRING with the environment template
// name. Defaults to the production template when name is empty.
func buildCertTemplateName(name string) (x509Extension, error) {
	if name == "" {
		name = "ZATCA-Code-Signing"
	}
	val, err := asn1.Marshal(asn1.RawValue{
		Class: asn1.ClassUniversal,
		Tag:   asn1.TagPrintableString,
		Bytes: []byte(name),
	})
	if err != nil {
		return x509Extension{}, err
	}
	return x509Extension{ExtnID: oidCertificateTemplateName, Critical: false, ExtnValue: val}, nil
}

// buildSubjectAltName encodes a dirName GeneralName with the ZATCA business attributes.
func buildSubjectAltName(p CSRParams) (x509Extension, error) {
	sanFields := []struct {
		oid asn1.ObjectIdentifier
		val string
	}{
		{oidSerialNumber, p.SerialNumber},
		{oidUID, p.VAT},
		{oidTitle, p.InvoiceType},
		{oidBusinessCategory, p.BusinessCategory},
	}
	if p.RegisteredAddress != "" {
		sanFields = append(sanFields, struct {
			oid asn1.ObjectIdentifier
			val string
		}{oidDescription, p.RegisteredAddress})
	}

	dirNameDER, err := encodeRDNSequence(sanFields)
	if err != nil {
		return x509Extension{}, fmt.Errorf("san rdns: %w", err)
	}

	// GeneralName ::= CHOICE { ..., directoryName [4] Name, ... }
	// directoryName is [4] EXPLICIT — the tag wraps the Name SEQUENCE.
	dirNameGeneral, err := asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassContextSpecific,
		Tag:        4,
		IsCompound: true,
		Bytes:      dirNameDER,
	})
	if err != nil {
		return x509Extension{}, fmt.Errorf("san dirName: %w", err)
	}

	// subjectAltName value = SEQUENCE OF GeneralName.
	val, err := asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassUniversal,
		Tag:        asn1.TagSequence,
		IsCompound: true,
		Bytes:      dirNameGeneral,
	})
	if err != nil {
		return x509Extension{}, fmt.Errorf("san sequence: %w", err)
	}

	return x509Extension{ExtnID: oidSubjectAltName, Critical: false, ExtnValue: val}, nil
}

// encodeCertificationRequestInfo encodes CertificationRequestInfo as SEQUENCE.
func encodeCertificationRequestInfo(subjectDER, spkiDER, attrDER []byte) ([]byte, error) {
	versionDER, err := asn1.Marshal(0)
	if err != nil {
		return nil, err
	}

	// attributes [0] IMPLICIT SET OF Attribute — wrap attrDER bytes in [0].
	attrsTagged, err := asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassContextSpecific,
		Tag:        0,
		IsCompound: true,
		Bytes:      attrDER,
	})
	if err != nil {
		return nil, err
	}

	var inner []byte
	inner = append(inner, versionDER...)
	inner = append(inner, subjectDER...)
	inner = append(inner, spkiDER...)
	inner = append(inner, attrsTagged...)

	return asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassUniversal,
		Tag:        asn1.TagSequence,
		IsCompound: true,
		Bytes:      inner,
	})
}

// encodeCertificationRequest assembles the outer PKCS#10 SEQUENCE.
func encodeCertificationRequest(certReqInfoDER, rawSig []byte) ([]byte, error) {
	// AlgorithmIdentifier for ecdsa-with-SHA256: parameters field ABSENT (RFC 5758 §3.2).
	algOIDDER, err := asn1.Marshal(oidECDSAWithSHA256)
	if err != nil {
		return nil, err
	}
	sigAlgDER, err := asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassUniversal,
		Tag:        asn1.TagSequence,
		IsCompound: true,
		Bytes:      algOIDDER,
	})
	if err != nil {
		return nil, err
	}

	sigDER, err := asn1.Marshal(asn1.BitString{Bytes: rawSig, BitLength: len(rawSig) * 8})
	if err != nil {
		return nil, err
	}

	var inner []byte
	inner = append(inner, certReqInfoDER...)
	inner = append(inner, sigAlgDER...)
	inner = append(inner, sigDER...)

	return asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassUniversal,
		Tag:        asn1.TagSequence,
		IsCompound: true,
		Bytes:      inner,
	})
}
