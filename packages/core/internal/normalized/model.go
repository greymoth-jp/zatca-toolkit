// Package normalized is the internal canonical invoice model (EN16931 BT/BG terms).
// Every jurisdiction adapter consumes this model; nothing upstream of normalization
// is allowed to leak into validation/conversion. See 00-core-platform/03_spec/glossary.md.
package normalized

// Doc is the normalized invoice (EN16931). Field comments cite the Business Term (BT)
// or Business Group (BG) so the mapping back to UBL/CII is auditable.
type Doc struct {
	// BG-2 process control
	ProfileID       string `json:"profile_id"`       // BT-23 business process / Peppol ProfileID
	CustomizationID string `json:"customization_id"` // BT-24 specification identifier (CIUS)

	ID          string `json:"id"`            // BT-1 invoice number
	IssueDate   string `json:"issue_date"`    // BT-2 issue date (YYYY-MM-DD)
	IssueTime   string `json:"issue_time"`    // KSA-DT-02 cbc:IssueTime (HH:MM:SS) — ZATCA mandatory
	DueDate     string `json:"due_date"`      // BT-9 payment due date

	// BillingReference (BG-3 / BT-25): the preceding invoice a credit/debit note corrects.
	BillingRefID   string `json:"billing_ref_id"`   // BT-25 preceding invoice number
	BillingRefDate string `json:"billing_ref_date"` // BT-26 preceding invoice issue date

	DeliveryDate     string `json:"delivery_date"`      // BG-13 / BT-72 actual delivery date
	PaymentMeansCode string `json:"payment_means_code"` // BG-16 / BT-81 payment means type code (UNCL4461)
	PayeeIBAN        string `json:"payee_iban"`         // BT-84 payment account identifier (IBAN)
	PaymentTerms     string `json:"payment_terms"`      // BT-20 payment terms (free text)

	// InvoicePeriod (BG-14 / BT-73,BT-74): the period the invoice covers (e.g. a service month),
	// distinct from IssueDate (a point in time). Optional; nil when the invoice has no period.
	InvoicePeriod *InvoicePeriod `json:"invoice_period,omitempty"`

	BuyerReference string `json:"buyer_reference"` // BT-10 buyer reference (e.g. cost centre / PO holder)
	OrderReference string `json:"order_reference"` // BT-13 purchase order reference
	SalesOrderRef  string `json:"sales_order_ref"` // BT-14 sales order reference
	TypeCode    string `json:"type_code"`     // BT-3 invoice type code (380 invoice, 381 credit note...)
	Currency    string `json:"currency"`      // BT-5 document currency (ISO 4217)
	TaxCurrency string `json:"tax_currency"`  // BT-6 VAT accounting currency
	Note        string `json:"note"`          // BT-22 invoice note

	// Simplified marks a KSA B2C "simplified" tax invoice (reported within 24h) as opposed
	// to a standard B2B/B2G invoice (cleared before sharing). It relaxes buyer requirements
	// and requires the QR tag-9 stamp. Encoded in ZATCA UBL as InvoiceTypeCode @name "02…".
	Simplified bool `json:"simplified"`


	Seller Party `json:"seller"` // BG-4 seller
	Buyer  Party `json:"buyer"`  // BG-7 buyer

	Lines            []Line            `json:"lines"`
	TaxBreakdown     []TaxSubtotal     `json:"tax_breakdown"`     // BG-23 VAT breakdown
	AllowanceCharges []AllowanceCharge `json:"allowance_charges"` // BG-20 allowances / BG-21 charges (document level)
	Totals           Totals            `json:"totals"`            // BG-22 document totals
}

// InvoicePeriod is BG-14 (the period the invoice covers, e.g. service months). Distinct from
// IssueDate (BT-2, when the invoice was created). Dates are ISO 8601 (YYYY-MM-DD).
type InvoicePeriod struct {
	StartDate string `json:"start_date"` // BT-73 invoicing period start date
	EndDate   string `json:"end_date"`   // BT-74 invoicing period end date
}

// AllowanceCharge is a document-level allowance (BG-20) or charge (BG-21). The same shape covers
// both; Charge distinguishes them. Amounts are positive; Charge selects add vs subtract.
type AllowanceCharge struct {
	Charge      bool    `json:"charge"`       // false = allowance (BG-20), true = charge (BG-21)
	Amount      float64 `json:"amount"`       // BT-92 (allowance) / BT-99 (charge)
	BaseAmount  float64 `json:"base_amount"`  // BT-93 / BT-100 (optional)
	Percent     float64 `json:"percent"`      // BT-94 / BT-101 (optional)
	Reason      string  `json:"reason"`       // BT-97 / BT-104
	ReasonCode  string  `json:"reason_code"`  // BT-98 / BT-105
	VATCategory string  `json:"vat_category"` // BT-95 / BT-102
	VATRate     float64 `json:"vat_rate"`     // BT-96 / BT-103
}

// Party is a seller or buyer (BG-4 / BG-7).
type Party struct {
	Name        string `json:"name"`         // BT-27 / BT-44 registration name
	NameAr      string `json:"name_ar"`      // Arabic name (ZATCA mandatory for seller)
	VATID       string `json:"vat_id"`       // BT-31 / BT-48 VAT identifier
	CompanyID   string `json:"company_id"`   // legal registration id (e.g. KSA CR)
	EndpointID  string `json:"endpoint_id"`  // BT-34 / BT-49 electronic address (Peppol)
	EndpointScheme string `json:"endpoint_scheme"` // electronic address scheme id
	CountryCode string `json:"country_code"` // BT-40 / BT-55 ISO 3166 alpha-2
	Street      string `json:"street"`       // BT-35 / BT-50
	City        string `json:"city"`         // BT-37 / BT-52
	PostalZone  string `json:"postal_zone"`  // BT-38 / BT-53
}

// Line is an invoice line (BG-25).
type Line struct {
	ID        string  `json:"id"`         // BT-126 line identifier
	Quantity  float64 `json:"quantity"`   // BT-129 invoiced quantity
	UnitCode  string  `json:"unit_code"`  // BT-130 unit of measure
	ItemName  string  `json:"item_name"`  // BT-153 item name
	NetPrice  float64 `json:"net_price"`  // BT-146 item net price
	BaseQuantity float64 `json:"base_quantity"` // BT-149 price base quantity (price is per this many units; default 1)
	NetAmount float64 `json:"net_amount"` // BT-131 line net amount
	VATCategory string `json:"vat_category"` // BT-151 VAT category code (S/Z/E/O...)
	VATRate     float64 `json:"vat_rate"`     // BT-152 VAT rate (%)
	AllowanceCharges []AllowanceCharge `json:"allowance_charges"` // BG-27 line allowances / BG-28 line charges
	// Period (BG-26 / BT-134,BT-135): the period this line covers (e.g. a sub-service span),
	// distinct from the document invoice period (BG-14). Optional; nil when absent.
	Period *InvoicePeriod `json:"period,omitempty"`
}

// TaxSubtotal is one VAT breakdown group (BG-23).
type TaxSubtotal struct {
	Category            string  `json:"category"`              // BT-118 VAT category code
	Rate                float64 `json:"rate"`                  // BT-119 VAT rate
	TaxableAmount       float64 `json:"taxable_amount"`        // BT-116 taxable amount
	TaxAmount           float64 `json:"tax_amount"`            // BT-117 VAT category tax amount
	ExemptionReasonCode string  `json:"exemption_reason_code"` // BT-121 VAT exemption reason code
	ExemptionReason     string  `json:"exemption_reason"`      // BT-120 VAT exemption reason text
}

// Totals is the document monetary summary (BG-22).
type Totals struct {
	LineExtensionAmount float64 `json:"line_extension_amount"` // BT-106 sum of line net amounts
	TaxExclusiveAmount  float64 `json:"tax_exclusive_amount"`  // BT-109 total without VAT
	TaxAmount           float64 `json:"tax_amount"`            // BT-110 total VAT amount
	TaxInclusiveAmount  float64 `json:"tax_inclusive_amount"`  // BT-112 total with VAT
	AllowanceTotal      float64 `json:"allowance_total"`       // BT-107 sum of allowances
	ChargeTotal         float64 `json:"charge_total"`          // BT-108 sum of charges
	PrepaidAmount       float64 `json:"prepaid_amount"`        // BT-113 prepaid amount
	PayableAmount       float64 `json:"payable_amount"`        // BT-115 amount due for payment
}
