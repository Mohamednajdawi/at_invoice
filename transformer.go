package main

import (
	"encoding/xml"
	"fmt"
	"math"
)

// formatCentsAsDecimal converts cents (int64) to a decimal string with 2 decimal places.
// Example: 12000 -> "120.00"
func formatCentsAsDecimal(cents int64) string {
	return fmt.Sprintf("%.2f", float64(cents)/100.0)
}

// TransformToEbInterface maps the JSON invoice into a minimal ebInterface 6.1 XML document.
func TransformToEbInterface(inv InvoiceJSON) ([]byte, error) {
	const currency = "EUR"

	items := make([]EbItem, 0, len(inv.Items))
	var totalNetCts int64
	var totalTaxCts int64
	taxBuckets := map[float64]struct {
		taxableCts int64
		taxCts     int64
	}{}

	for i, li := range inv.Items {
		lineNetCts := li.UnitPriceCents * li.Quantity
		taxRate := li.TaxRate
		taxCts := int64(math.Round(float64(lineNetCts) * taxRate / 100.0))

		totalNetCts += lineNetCts
		totalTaxCts += taxCts

		b := taxBuckets[taxRate]
		b.taxableCts += lineNetCts
		b.taxCts += taxCts
		taxBuckets[taxRate] = b

		item := EbItem{
			Description: li.Description,
			Quantity: EbQuantity{
				Unit:  "C62", // default to pieces; can be adjusted per item later
				Value: float64(li.Quantity),
			},
			UnitPrice: formatCentsAsDecimal(li.UnitPriceCents),
			InvoiceRecipientsOrderReference: &EbOrderReferenceItem{
				OrderID:            inv.Recipient.OrderID,
				OrderPositionNumber: fmt.Sprintf("%d", i+1), // Position number (1-based)
			},
			TaxItem: EbTaxItem{
				TaxableAmount: formatCentsAsDecimal(lineNetCts), // Net amount for the line (before tax)
				TaxPercent: EbTaxPercent{
					TaxCategoryCode: taxCategoryFromRate(taxRate),
					Value:           taxRate,
				},
				// Note: TaxItem in Details does NOT have TaxAmount
			},
			LineItemAmount: formatCentsAsDecimal(lineNetCts), // Line item NET amount (before tax) - MUST come after TaxItem
		}
		items = append(items, item)
	}

	summary := make([]EbTaxItemSummary, 0, len(taxBuckets))
	for rate, b := range taxBuckets {
		summary = append(summary, EbTaxItemSummary{
			TaxableAmount: formatCentsAsDecimal(b.taxableCts), // MUST be FIRST
			TaxPercent: EbTaxPercent{
				TaxCategoryCode: taxCategoryFromRate(rate),
				Value:           rate,
			}, // MUST be SECOND
			TaxAmount: formatCentsAsDecimal(b.taxCts), // MUST be THIRD
		})
	}

	totalGrossCts := totalNetCts + totalTaxCts

	doc := EbInterfaceInvoice{
		GeneratingSystem: "austrian-invoice-microservice",
		DocumentType:     "Invoice",
		InvoiceCurrency:  currency,
		Language:         "de",
		InvoiceNumber: inv.InvoiceNumber,
		InvoiceDate:   inv.InvoiceDate,
		Delivery: &EbDelivery{
			Date:    inv.InvoiceDate, // Use invoice date as delivery date
			Address: composeEbAddress(inv.Biller.Name, inv.Biller.Address),
			Contact: EbContact{
				Name:  getContactName(inv.Biller.ContactName, "Billing Department"),
				Email: inv.Biller.Email,
			},
		},
		Biller: EbBiller{
			VATID:   inv.Biller.VATID,
			Address: composeEbAddress(inv.Biller.Name, inv.Biller.Address),
			Contact: EbContact{
				Name:  getContactName(inv.Biller.ContactName, "Billing Department"),
				Email: inv.Biller.Email,
			},
			InvoiceRecipientsBillerID: inv.Biller.BillerID,
		},
		InvoiceRecipient: EbRecipient{
			VATID: inv.Recipient.VATID,
			OrderReference: EbOrderReference{
				OrderID: inv.Recipient.OrderID,
			},
			Address: composeEbAddress(inv.Recipient.Name, inv.Recipient.Address),
			Contact: EbContact{
				Name:  getContactName(inv.Recipient.ContactName, "Accounting"),
				Email: inv.Recipient.Email,
			},
		},
		Details: EbDetails{
			ItemList: EbItemList{
				Items: items,
			},
		},
		Tax: EbTax{
			TaxItems: summary, // Tax summary items with TaxableAmount, TaxPercent, TaxAmount
		},
		TotalGrossAmount: formatCentsAsDecimal(totalGrossCts), // Direct child of Invoice
		PayableAmount:    formatCentsAsDecimal(totalGrossCts), // Direct child of Invoice
		PaymentMethod: EbPaymentMethod{
			UniversalBankTransaction: EbUniversalBankTransaction{
				BeneficiaryAccount: EbBeneficiaryAccount{
					BIC:              inv.Payment.BIC,              // MUST be FIRST
					IBAN:             inv.Payment.IBAN,            // MUST be SECOND
					BankAccountOwner: inv.Biller.Name,            // MUST be THIRD - Use biller name as account owner
				},
			},
		},
		// Extensions removed - not part of official ebInterface 6.1 structure
	}

	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal ebInterface: %w", err)
	}
	return append([]byte(xml.Header), out...), nil
}

// taxCategoryFromRate maps Austrian VAT rates to ebInterface tax category codes.
func taxCategoryFromRate(rate float64) string {
	switch rate {
	case 0:
		return "Z"
	case 10, 13:
		return "S"
	case 20:
		return "S"
	default:
		return "S"
	}
}

// getContactName returns the provided contact name or a default value if empty.
func getContactName(provided, defaultName string) string {
	if provided != "" {
		return provided
	}
	return defaultName
}

