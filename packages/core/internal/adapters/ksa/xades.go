package ksa

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

// xades.go — XAdES enveloped signature (SK-1, P0a-3). ZATCA's cryptographic stamp is an
// XAdES (ETSI EN 319 132-1) ds:Signature embedded in ext:UBLExtensions. The placeholder
// engine left UBLExtensions empty; this builds the real signature structure:
//
//   ds:Signature
//     ds:SignedInfo
//       CanonicalizationMethod (exclusive C14N)   <- see C14N NOTE below
//       SignatureMethod        (ecdsa-sha256)
//       Reference URI=""        -> DigestValue = canonical invoice hash (doc, sig/QR excluded)
//       Reference URI=#xadesSignedProperties -> DigestValue = hash(SignedProperties)
//     ds:SignatureValue        = secp256k1 sign( SHA-256( c14n(SignedInfo) ) )  [raw r||s]
//     ds:KeyInfo/X509Data/X509Certificate = CSID cert (creds-gated; dev placeholder)
//     ds:Object/QualifyingProperties/SignedProperties (SigningTime + SigningCertificate)
//
// C14N NOTE (why exclusive for the inner refs): SignedInfo / SignedProperties are signed as
// detached elements at build time but verified while nested deep in the invoice. Inclusive
// C14N would pull every inherited namespace (default Invoice ns, cbc, cac, ext, sig…) into
// the nested form and NOT into the detached form, breaking verification. Exclusive C14N
// emits only visibly-used namespaces, so build-bytes == verify-bytes. The document-level
// Reference uses the C14N 1.1 invoice hash (CanonicalInvoiceHash), canonicalized from the
// root where no inherited-namespace problem exists.
//
// CONFORMANCE (honest): this is a complete, internally self-verifying XAdES. Byte-exact
// agreement with ZATCA (exact CanonicalizationMethod algorithm, Transforms, cert/issuer
// fields) is pending official fixtures + a real CSID cert (creds-gated). STATUS §P0a-4.

const (
	nsDS    = "http://www.w3.org/2000/09/xmldsig#"
	nsXades = "http://uri.etsi.org/01903/v1.3.2#"
	nsSig   = "urn:oasis:names:specification:ubl:schema:xsd:CommonSignatureComponents-2"
	nsSAC   = "urn:oasis:names:specification:ubl:schema:xsd:SignatureAggregateComponents-2"

	algSHA256      = "http://www.w3.org/2001/04/xmlenc#sha256"
	algECDSASHA256 = "http://www.w3.org/2001/04/xmldsig-more#ecdsa-sha256"
	algC14N11      = "http://www.w3.org/2006/12/xml-c14n11"
	algExcC14N     = "http://www.w3.org/2001/10/xml-exc-c14n#"
	algEnveloped   = "http://www.w3.org/2000/09/xmldsig#enveloped-signature"

	signaturePropsID = "xadesSignedProperties"
)

// DevCertPlaceholder is a clearly-fake base64 certificate used so the structure is complete
// in dev/test. PRODUCTION MUST replace it with the ZATCA-issued CSID certificate (creds-
// gated; STATUS BLOCKED). It is NOT a real certificate and signs nothing.
const DevCertPlaceholder = "REVWLUNTSUQtUExBQ0VIT0xERVItTk9ULUEtUkVBTC1aQVRDQS1DRVJUSUZJQ0FURQ=="

// XAdESParams carries the signing-time metadata. Issuer/Serial/Cert are credential-gated;
// dev values are clearly marked.
type XAdESParams struct {
	SigningTime string // ISO8601, MUST match the invoice timestamp
	CertBase64  string // base64 DER of the CSID cert (DevCertPlaceholder in dev)
	IssuerName  string // X509 issuer DN (from cert in prod)
	Serial      string // X509 serial (from cert in prod)
}

// excCanonicalizer returns the exclusive-C14N canonicalizer used for the inner references.
func excCanonicalizer() dsig.Canonicalizer {
	return dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
}

// canonSHA256 exclusive-canonicalizes el and returns (base64 digest, raw digest).
func canonSHA256(el *etree.Element) (string, []byte, error) {
	b, err := excCanonicalizer().Canonicalize(el)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(b)
	return base64.StdEncoding.EncodeToString(sum[:]), sum[:], nil
}

func buildSignedProperties(p XAdESParams, certDER []byte) *etree.Element {
	sp := etree.NewElement("xades:SignedProperties")
	sp.CreateAttr("xmlns:xades", nsXades)
	sp.CreateAttr("xmlns:ds", nsDS)
	sp.CreateAttr("Id", signaturePropsID)
	ssp := sp.CreateElement("xades:SignedSignatureProperties")
	ssp.CreateElement("xades:SigningTime").SetText(p.SigningTime)
	cert := ssp.CreateElement("xades:SigningCertificate").CreateElement("xades:Cert")
	cd := cert.CreateElement("xades:CertDigest")
	cd.CreateElement("ds:DigestMethod").CreateAttr("Algorithm", algSHA256)
	certHash := sha256.Sum256(certDER)
	cd.CreateElement("ds:DigestValue").SetText(base64.StdEncoding.EncodeToString(certHash[:]))
	is := cert.CreateElement("xades:IssuerSerial")
	is.CreateElement("ds:X509IssuerName").SetText(p.IssuerName)
	is.CreateElement("ds:X509SerialNumber").SetText(p.Serial)
	return sp
}

func buildSignedInfo(docDigestB64, spDigestB64 string) *etree.Element {
	si := etree.NewElement("ds:SignedInfo")
	si.CreateAttr("xmlns:ds", nsDS)
	si.CreateElement("ds:CanonicalizationMethod").CreateAttr("Algorithm", algExcC14N)
	si.CreateElement("ds:SignatureMethod").CreateAttr("Algorithm", algECDSASHA256)

	r1 := si.CreateElement("ds:Reference")
	r1.CreateAttr("URI", "")
	tr := r1.CreateElement("ds:Transforms")
	tr.CreateElement("ds:Transform").CreateAttr("Algorithm", algEnveloped)
	tr.CreateElement("ds:Transform").CreateAttr("Algorithm", algC14N11)
	r1.CreateElement("ds:DigestMethod").CreateAttr("Algorithm", algSHA256)
	r1.CreateElement("ds:DigestValue").SetText(docDigestB64)

	r2 := si.CreateElement("ds:Reference")
	r2.CreateAttr("URI", "#"+signaturePropsID)
	r2.CreateAttr("Type", "http://uri.etsi.org/01903#SignedProperties")
	r2.CreateElement("ds:DigestMethod").CreateAttr("Algorithm", algSHA256)
	r2.CreateElement("ds:DigestValue").SetText(spDigestB64)
	return si
}

// BuildSignedUBL embeds a full XAdES enveloped signature into the UBL's UBLExtensions and
// returns the signed document. The document Reference digest equals the canonical invoice
// hash (signature/QR excluded), so it matches QR tag 6 / the PIH chain.
func BuildSignedUBL(ublXML []byte, signer Signer, p XAdESParams) ([]byte, error) {
	if p.CertBase64 == "" {
		p.CertBase64 = DevCertPlaceholder
	}
	certDER, err := base64.StdEncoding.DecodeString(p.CertBase64)
	if err != nil {
		return nil, errors.New("ksa xades: CertBase64 is not valid base64")
	}

	docDigestB64, err := CanonicalInvoiceHashB64(ublXML)
	if err != nil {
		return nil, err
	}

	sp := buildSignedProperties(p, certDER)
	spDigestB64, _, err := canonSHA256(sp)
	if err != nil {
		return nil, err
	}

	si := buildSignedInfo(docDigestB64, spDigestB64)
	_, siHash, err := canonSHA256(si)
	if err != nil {
		return nil, err
	}
	rawSig, _, err := signer.Sign(siHash)
	if err != nil {
		return nil, err
	}

	// Assemble ds:Signature.
	sigEl := etree.NewElement("ds:Signature")
	sigEl.CreateAttr("xmlns:ds", nsDS)
	sigEl.CreateAttr("Id", "signature")
	sigEl.AddChild(si.Copy())
	sigEl.CreateElement("ds:SignatureValue").SetText(base64.StdEncoding.EncodeToString(rawSig))
	x509 := sigEl.CreateElement("ds:KeyInfo").CreateElement("ds:X509Data")
	x509.CreateElement("ds:X509Certificate").SetText(p.CertBase64)
	qp := sigEl.CreateElement("ds:Object").CreateElement("xades:QualifyingProperties")
	qp.CreateAttr("xmlns:xades", nsXades)
	qp.CreateAttr("Target", "signature")
	qp.AddChild(sp.Copy())

	// Inject into ext:UBLExtensions/ext:UBLExtension/ext:ExtensionContent.
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(ublXML); err != nil {
		return nil, err
	}
	root := doc.Root()
	if root == nil {
		return nil, errors.New("ksa xades: document has no root")
	}
	ec := root.FindElement("ext:UBLExtensions/ext:UBLExtension/ext:ExtensionContent")
	if ec == nil {
		return nil, errors.New("ksa xades: ext:ExtensionContent not found (expected signature placeholder)")
	}
	for _, child := range ec.ChildElements() {
		ec.RemoveChild(child)
	}
	uds := ec.CreateElement("sig:UBLDocumentSignatures")
	uds.CreateAttr("xmlns:sig", nsSig)
	uds.CreateAttr("xmlns:sac", nsSAC)
	uds.CreateElement("sac:SignatureInformation").AddChild(sigEl)

	doc.WriteSettings = etree.WriteSettings{}
	return doc.WriteToBytes()
}

// VerifySignedUBL checks a signed UBL end-to-end against the SEC1 public key (in production
// recovered from ds:KeyInfo/X509Certificate): (1) SignatureValue verifies over c14n(SignedInfo),
// (2) the document Reference digest matches the recomputed canonical invoice hash, (3) the
// SignedProperties Reference digest matches the recomputed SignedProperties hash.
func VerifySignedUBL(signedXML []byte, pubSEC1 []byte) (bool, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(signedXML); err != nil {
		return false, err
	}
	root := doc.Root()
	if root == nil {
		return false, errors.New("ksa xades verify: no root")
	}
	sigEl := root.FindElement("//ds:Signature")
	if sigEl == nil {
		return false, errors.New("ksa xades verify: ds:Signature not found")
	}

	si := sigEl.FindElement("ds:SignedInfo")
	svEl := sigEl.FindElement("ds:SignatureValue")
	if si == nil || svEl == nil {
		return false, errors.New("ksa xades verify: SignedInfo/SignatureValue missing")
	}
	_, siHash, err := canonSHA256(si)
	if err != nil {
		return false, err
	}
	rawSig, err := base64.StdEncoding.DecodeString(svEl.Text())
	if err != nil {
		return false, errors.New("ksa xades verify: SignatureValue not base64")
	}
	if !VerifySecp256k1(pubSEC1, siHash, rawSig) {
		return false, nil
	}

	// (2) document Reference digest.
	docRef := si.FindElement("ds:Reference[@URI='']/ds:DigestValue")
	if docRef == nil {
		return false, errors.New("ksa xades verify: document Reference DigestValue missing")
	}
	recomputedDoc, err := CanonicalInvoiceHashB64(signedXML)
	if err != nil {
		return false, err
	}
	if docRef.Text() != recomputedDoc {
		return false, nil
	}

	// (3) SignedProperties Reference digest.
	sp := sigEl.FindElement("ds:Object/xades:QualifyingProperties/xades:SignedProperties")
	if sp == nil {
		return false, errors.New("ksa xades verify: SignedProperties missing")
	}
	spDigestRecomp, _, err := canonSHA256(sp)
	if err != nil {
		return false, err
	}
	spRef := si.FindElement("ds:Reference[@URI='#" + signaturePropsID + "']/ds:DigestValue")
	if spRef == nil {
		return false, errors.New("ksa xades verify: SignedProperties Reference DigestValue missing")
	}
	return spDigestRecomp == spRef.Text(), nil
}
