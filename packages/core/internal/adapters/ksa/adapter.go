package ksa

import (
	"encoding/base64"
	"strconv"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/convert"
	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
)

// End-to-end KSA adapter wiring: generate ZATCA-UBL → hash → CSID-sign → QR → clear/report.
// This realizes the JurisdictionAdapter contract (generate/sign/clear) for KSA. The
// XAdES envelope embedding is still a placeholder (Z-T1 follow-up); everything else is real.
//
// Idempotency / no-double-clear: callers MUST carry the same UUID + ICV across retries; the
// adapter is deterministic for a given (doc, uuid, prev), and the resilience core
// (apps/api) treats a cleared result as terminal. Per ZATCA, a rejected document still
// advances the PIH chain (STATUS diff #2), so NewChain advances on any terminal outcome.

type Input struct {
	Doc       *normalized.Doc
	UUID      string      // document UUID (assigned once; reused on retry)
	IssueTime string      // HH:MM:SS
	Prev      *ChainEntry // tail of the PIH chain (nil for first invoice)
}

type Result struct {
	Status      string // cleared | cleared_with_warnings | rejected | reported | retrying
	UUID        string
	QR          string // base64 TLV
	SignedXML   []byte // ZATCA-UBL incl. QR (XAdES envelope = placeholder)
	InvoiceHash string // base64 SHA-256 (PIH chain + QR tag 6)
	NewChain    *ChainEntry // advanced chain tail (nil when retrying)
	Warnings    []string
	Errors      []string
	Retryable   bool // true when authority unavailable → caller backs off (FR-Z08)
	CurveNote   string
}

// ProcessStandard runs a B2B/B2G clearance (FR-Z05). Simplified reporting uses the same
// pipeline via ProcessSimplified (different authority call + tag 9).
func ProcessStandard(in Input, signer Signer, client ZatcaClient) (Result, error) {
	return process(in, signer, client, false)
}

// ProcessSimplified runs B2C reporting (FR-Z06): immediate issue + QR (incl. tag 9), report within 24h.
func ProcessSimplified(in Input, signer Signer, client ZatcaClient) (Result, error) {
	return process(in, signer, client, true)
}

func process(in Input, signer Signer, client ZatcaClient, simplified bool) (Result, error) {
	icv := NextICV(in.Prev)
	pih := NextPIH(in.Prev)

	// 1. Generate the (unsigned) ZATCA-UBL with ICV/PIH refs.
	base, err := convert.ToZatcaUBL(in.Doc, convert.ZatcaUBLOpts{
		UUID: in.UUID, ICV: icv, PIH: pih, IssueTime: in.IssueTime,
	})
	if err != nil {
		return Result{}, err
	}

	// 2. ZATCA invoice hash: SHA-256 over the CANONICALIZED UBL (signature/QR excluded),
	//    not the raw bytes (P0a-2 / SK-1). This digest is QR tag 6, the PIH chain link, and
	//    the CSID signing input.
	digestArr, err := CanonicalInvoiceHash(base)
	if err != nil {
		return Result{}, err
	}
	digest := digestArr[:]
	invoiceHash := base64.StdEncoding.EncodeToString(digest)
	sig, pub, err := signer.Sign(digest)
	if err != nil {
		return Result{}, err
	}

	// 3. Build the QR (tags 1-8, +9 for simplified once the ZATCA stamp is available).
	qr, err := EncodeQR(QRFields{
		SellerName:   sellerName(in.Doc),
		VATNumber:    in.Doc.Seller.VATID,
		Timestamp:    in.Doc.IssueDate + "T" + def(in.IssueTime, "00:00:00") + "Z",
		InvoiceTotal: money(in.Doc.Totals.TaxInclusiveAmount),
		VATTotal:     money(in.Doc.Totals.TaxAmount),
		InvoiceHash:  invoiceHash,
		Signature:    sig,
		PublicKey:    pub,
	}, simplified)
	if err != nil {
		return Result{}, err
	}

	// 4. Re-emit the UBL with the QR embedded (this is the document submitted/issued).
	signedXML, err := convert.ToZatcaUBL(in.Doc, convert.ZatcaUBLOpts{
		UUID: in.UUID, ICV: icv, PIH: pih, IssueTime: in.IssueTime, QR: qr,
	})
	if err != nil {
		return Result{}, err
	}

	// 4.5 Embed the XAdES enveloped signature into UBLExtensions (P0a-3). The signature's
	// document Reference digest equals invoiceHash (QR + UBLExtensions excluded from the
	// hash), so adding the QR before signing does not invalidate the signature.
	signedXML, err = BuildSignedUBL(signedXML, signer, XAdESParams{
		SigningTime: in.Doc.IssueDate + "T" + def(in.IssueTime, "00:00:00") + "Z",
	})
	if err != nil {
		return Result{}, err
	}

	// 5. Submit to ZATCA (clearance for standard, reporting for simplified).
	var cr ClearResult
	if simplified {
		cr, err = client.Report(signedXML, in.UUID, invoiceHash)
	} else {
		cr, err = client.Clear(signedXML, in.UUID, invoiceHash)
	}
	if err != nil {
		return Result{}, err
	}

	res := Result{
		UUID: firstNonEmpty(cr.UUID, in.UUID), QR: qr, SignedXML: signedXML,
		InvoiceHash: invoiceHash, Warnings: cr.Warnings, Errors: cr.Errors, CurveNote: signer.CurveName(),
	}

	switch cr.Outcome {
	case OutcomeUnavailable:
		// transient — do not finalize; caller retries with the SAME uuid/icv (no chain advance).
		res.Status = "retrying"
		res.Retryable = true
		return res, nil
	case OutcomeCleared:
		res.Status = "cleared"
	case OutcomeClearedWithWarn:
		res.Status = "cleared_with_warnings"
	case OutcomeReported:
		res.Status = "reported"
	case OutcomeRejected:
		res.Status = "rejected"
	default:
		res.Status = string(cr.Outcome)
	}
	// Terminal (incl. rejected): the chain advances — the next PIH references this hash.
	newTail := ChainEntry{ICV: icv, Hash: invoiceHash}
	res.NewChain = &newTail
	return res, nil
}

func sellerName(d *normalized.Doc) string {
	if d.Seller.NameAr != "" {
		return d.Seller.NameAr
	}
	return d.Seller.Name
}

func money(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

func def(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
