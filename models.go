package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"time"
)

// -------- JSON input models (aligned with tests/golden_test.json) --------

type InvoiceJSON struct {
	InvoiceNumber string         `json:"invoice_number"`
	InvoiceDate   string         `json:"invoice_date"` // ISO-8601 (YYYY-MM-DD)
	Biller        BillerJSON     `json:"biller"`
	Recipient     RecipientJSON  `json:"recipient"`
	Items         []LineItemJSON `json:"items"`
	Payment       PaymentDetails `json:"payment"`
}

type BillerJSON struct {
	Name        string       `json:"name"`
	VATID       string       `json:"vat_id"`
	BillerID    string       `json:"biller_id"`
	Email       string       `json:"email,omitempty"`
	ContactName string       `json:"contact_name,omitempty"`
	Address     AddressJSON  `json:"address"`
}

type RecipientJSON struct {
	Name        string      `json:"name"`
	VATID       string      `json:"vat_id"`
	OrderID     string      `json:"order_id"`
	Email       string      `json:"email,omitempty"`
	ContactName string      `json:"contact_name,omitempty"`
	Address     AddressJSON `json:"address"`
}

type AddressJSON struct {
	Street string `json:"street"`
	ZIP    string `json:"zip"`
	City   string `json:"city"`
}

type LineItemJSON struct {
	Description     string  `json:"description"`
	Quantity        int64   `json:"quantity"`
	UnitPriceCents  int64   `json:"unit_price_cents"`
	TaxRate         float64 `json:"tax_rate"`
}

type PaymentDetails struct {
	IBAN string `json:"iban"`
	BIC  string `json:"bic"`
}

// -------- ebInterface 6.1 XML models (simplified) --------

// EbTax represents the top-level tax element (required after Details).
// Contains a list of TaxItem elements with full tax information.
type EbTax struct {
	TaxItems []EbTaxItemSummary `xml:"TaxItem"` // TaxItem elements with TaxableAmount, TaxPercent, TaxAmount
}

// EbInterfaceInvoice represents a minimal ebInterface 6.1 invoice.
// Field order here defines the element order in the generated XML.
// Correct order based on official ebInterface 6.1 example:
// InvoiceNumber, InvoiceDate, Biller, InvoiceRecipient, Details, Tax, TotalGrossAmount, PayableAmount, PaymentMethod
// Note: There is NO InvoiceSummary element in ebInterface 6.1 - tax summary is in Tax element
type EbInterfaceInvoice struct {
	XMLName          xml.Name            `xml:"http://www.ebinterface.at/schema/6p1/ Invoice"`
	GeneratingSystem string              `xml:"GeneratingSystem,attr"`
	DocumentType     string              `xml:"DocumentType,attr"`
	InvoiceCurrency  string              `xml:"InvoiceCurrency,attr"`
	Language         string              `xml:"Language,attr"`
	InvoiceNumber    string              `xml:"InvoiceNumber"`
	InvoiceDate      string              `xml:"InvoiceDate"`
	Delivery         *EbDelivery         `xml:"Delivery,omitempty"` // Optional delivery information
	Biller           EbBiller            `xml:"Biller"`
	InvoiceRecipient EbRecipient         `xml:"InvoiceRecipient"`
	Details          EbDetails           `xml:"Details"`
	Tax              EbTax               `xml:"Tax"`                    // REQUIRED after Details - contains tax summary
	TotalGrossAmount string              `xml:"TotalGrossAmount"`       // Direct child of Invoice
	PayableAmount    string              `xml:"PayableAmount"`          // Direct child of Invoice
	PaymentMethod EbPaymentMethod `xml:"PaymentMethod"` // PaymentMethod (not PaymentInstructions)
	// Note: Extensions removed - not in official ebInterface 6.1 example
	// Optional elements after PaymentMethod: PaymentConditions, Comment, Extension (singular)
}

// EbAddress models the structured postal address required by ebInterface.
// Name must be the FIRST child of Address.
// Element order: Name, Street, Town, ZIP, Country
type EbAddress struct {
	Name    string    `xml:"Name"`
	Street  string    `xml:"Street"`
	Town    string    `xml:"Town"`
	ZIP     string    `xml:"ZIP"`
	Country EbCountry `xml:"Country"`
}

// EbCountry contains the ISO country code and the localized country name.
type EbCountry struct {
	CountryCode string `xml:"CountryCode,attr"`
	Name        string `xml:",chardata"`
}

// EbContact wraps contact information.
// Name must come before Email in ebInterface 6.1.
type EbContact struct {
	Name  string `xml:"Name"`
	Email string `xml:"Email,omitempty"`
}

// EbDelivery represents delivery information (optional but recommended for B2G).
type EbDelivery struct {
	Date    string    `xml:"Date"`    // Delivery date (same as invoice date if not specified)
	Address EbAddress `xml:"Address"` // Delivery address (usually same as biller address)
	Contact EbContact `xml:"Contact,omitempty"`
}

// EbBiller follows strict element order: VATID, Address, Contact, InvoiceRecipientsBillerID.
type EbBiller struct {
	VATID                     string    `xml:"VATIdentificationNumber"`
	Address                   EbAddress `xml:"Address"`
	Contact                   EbContact `xml:"Contact"`
	InvoiceRecipientsBillerID string    `xml:"InvoiceRecipientsBillerID,omitempty"`
}

// EbRecipient follows strict element order: VATID, OrderReference, Address, Contact.
type EbRecipient struct {
	VATID          string          `xml:"VATIdentificationNumber"`
	OrderReference EbOrderReference `xml:"OrderReference"`
	Address        EbAddress       `xml:"Address"`
	Contact        EbContact       `xml:"Contact"`
}

// EbOrderReference wraps the Austrian B2G order number in an OrderID element.
type EbOrderReference struct {
	OrderID string `xml:"OrderID"`
}

type EbDetails struct {
	ItemList EbItemList `xml:"ItemList"`
}

type EbItemList struct {
	Items []EbItem `xml:"ListLineItem"`
}

// EbQuantity wraps the quantity value and its mandatory unit attribute.
type EbQuantity struct {
	Unit  string  `xml:"Unit,attr"`
	Value float64 `xml:",chardata"`
}

// EbOrderReferenceItem represents order reference for a line item.
type EbOrderReferenceItem struct {
	OrderID            string `xml:"OrderID"`            // Order ID from recipient
	OrderPositionNumber string `xml:"OrderPositionNumber"` // Position number in order (required if OrderID is present)
}

// EbItem represents a single line item in the invoice.
// Element order: Description, Quantity, UnitPrice, InvoiceRecipientsOrderReference (optional), TaxItem, LineItemAmount
type EbItem struct {
	Description                    string                 `xml:"Description"`
	Quantity                       EbQuantity             `xml:"Quantity"`
	UnitPrice                      string                 `xml:"UnitPrice"`                      // Decimal string (e.g., "120.00")
	InvoiceRecipientsOrderReference *EbOrderReferenceItem `xml:"InvoiceRecipientsOrderReference,omitempty"`
	TaxItem                        EbTaxItem              `xml:"TaxItem"`
	LineItemAmount                 string                 `xml:"LineItemAmount"`                 // Decimal string (e.g., "1200.00") - MUST come after TaxItem
}

// EbTaxPercent represents the tax rate with category code as an attribute.
type EbTaxPercent struct {
	TaxCategoryCode string  `xml:"TaxCategoryCode,attr"` // e.g., S
	Value            float64 `xml:",chardata"`            // e.g., 20
}

// EbTaxItem represents tax information for a line item (inside Details/ListLineItem).
// Element order: TaxableAmount, TaxPercent
// Note: In Details, TaxItem does NOT have TaxAmount - only TaxableAmount and TaxPercent
type EbTaxItem struct {
	TaxableAmount string       `xml:"TaxableAmount"` // Decimal string (e.g., "1200.00") - net amount for the line
	TaxPercent    EbTaxPercent `xml:"TaxPercent"`    // Tax rate with category code attribute (e.g., <TaxPercent TaxCategoryCode="S">20</TaxPercent>)
}

// EbTaxItemSummary represents tax summary information (inside Tax element).
// Element order: TaxableAmount, TaxPercent, TaxAmount
// Note: In Tax element, TaxItem includes TaxAmount
type EbTaxItemSummary struct {
	TaxableAmount string       `xml:"TaxableAmount"` // Decimal string (e.g., "1200.00") - MUST be FIRST
	TaxPercent    EbTaxPercent `xml:"TaxPercent"`    // MUST be SECOND - Tax rate with category code attribute
	TaxAmount     string       `xml:"TaxAmount"`     // Decimal string (e.g., "240.00") - MUST be THIRD
}

// EbPaymentMethod represents payment method information (required after PayableAmount).
// Note: In ebInterface 6.1, this is called PaymentMethod, not PaymentInstructions
type EbPaymentMethod struct {
	Comment                  string                     `xml:"Comment,omitempty"`
	UniversalBankTransaction EbUniversalBankTransaction `xml:"UniversalBankTransaction"`
}

// EbUniversalBankTransaction wraps bank account details.
type EbUniversalBankTransaction struct {
	BeneficiaryAccount EbBeneficiaryAccount `xml:"BeneficiaryAccount"`
}

// EbBeneficiaryAccount contains bank account details.
// Element order: BIC, IBAN, BankAccountOwner (as per official example)
type EbBeneficiaryAccount struct {
	BIC              string `xml:"BIC"`              // Bank identifier code - MUST be FIRST
	IBAN             string `xml:"IBAN"`             // International bank account number - MUST be SECOND
	BankAccountOwner string `xml:"BankAccountOwner"` // Account owner name - MUST be THIRD
}

// EbSimpleExtensions allows carrying the original JSON invoice number for debugging.
type EbSimpleExtensions struct {
	OriginalInvoiceNumber string `xml:"OriginalInvoiceNumber,omitempty"`
}


// -------- Utilities --------

func decodeJSON(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func validateInvoice(inv InvoiceJSON) error {
	if inv.InvoiceNumber == "" {
		return fmt.Errorf("InvoiceNumber is required")
	}
	if inv.InvoiceDate == "" {
		return fmt.Errorf("InvoiceDate is required")
	}
	if _, err := time.Parse("2006-01-02", inv.InvoiceDate); err != nil {
		return fmt.Errorf("InvoiceDate must be YYYY-MM-DD")
	}
	if inv.Biller.Name == "" || inv.Biller.VATID == "" {
		return fmt.Errorf("Biller name and vat_id are required")
	}
	if inv.Biller.Address.Street == "" || inv.Biller.Address.ZIP == "" || inv.Biller.Address.City == "" {
		return fmt.Errorf("Biller address street, zip and city are required")
	}
	if inv.Recipient.Name == "" || inv.Recipient.VATID == "" {
		return fmt.Errorf("Recipient name and vat_id are required")
	}
	if inv.Recipient.Address.Street == "" || inv.Recipient.Address.ZIP == "" || inv.Recipient.Address.City == "" {
		return fmt.Errorf("Recipient address street, zip and city are required")
	}
	// B2G: OrderReference mandatory
	if inv.Recipient.OrderID == "" {
		return fmt.Errorf("recipient.order_id is required for B2G")
	}
	if len(inv.Items) == 0 {
		return fmt.Errorf("at least one line item is required")
	}
	for i, d := range inv.Items {
		if d.Quantity <= 0 {
			return fmt.Errorf("line %d: Quantity must be > 0", i+1)
		}
		if d.Description == "" {
			return fmt.Errorf("line %d: Description is required", i+1)
		}
		if d.UnitPriceCents < 0 {
			return fmt.Errorf("line %d: unit_price_cents must be >= 0", i+1)
		}
	}
	if inv.Payment.IBAN == "" || inv.Payment.BIC == "" {
		return fmt.Errorf("PaymentDetails IBAN and BIC are required")
	}
	return nil
}

func toCents(amount float64) int64 {
	return int64(math.Round(amount * 100))
}

// composeEbAddress maps the JSON address into the ebInterface structured form.
// Name must be the first field in the Address structure.
func composeEbAddress(name string, a AddressJSON) EbAddress {
	return EbAddress{
		Name:   name,
		Street: a.Street,
		ZIP:    a.ZIP,
		Town:   a.City,
		Country: EbCountry{
			CountryCode: "AT",
			Name:        "Österreich",
		},
	}
}

