## Austrian ebInterface 6.1 Invoice Microservice – Product Requirements

**Goal**: Provide a lightweight HTTP microservice that converts a simple JSON invoice payload into a legally aligned Austrian ebInterface 6.1 XML invoice.

### Functional Scope

- **Input**: `POST /generate` with JSON body containing:
  - **InvoiceNumber**, **InvoiceDate (YYYY-MM-DD)**.
  - **Biller**: `Name`, `Address`, `VAT-ID`, optional `Email`.
  - **InvoiceRecipient**: `Name`, `Address`, `VAT-ID`, `OrderReference` (mandatory for B2G), optional `Email`.
  - **Details**: array of line items (`Quantity`, `Description`, `UnitPrice`, `TaxRate`).
  - **PaymentDetails**: `IBAN`, `BIC`.
- **Output**: ebInterface 6.1 XML string with namespace `xmlns="http://www.ebinterface.at/schema/6p1/"`.

### Non-Functional Requirements

- **Performance**: Single-process Go binary, minimal allocations, suitable for container deployment.
- **Security**: All requests must include `X-API-KEY`; value is configured via `API_KEY` env var (default `secret-key` for local dev).
- **Validation**:
  - Mandatory fields for Austrian B2G: Biller and Recipient VAT-ID, Recipient OrderReference.
  - At least one line item, positive quantities, non-negative prices, valid date.
- **Money Handling**:
  - Monetary amounts represented as integer cents internally to avoid float rounding errors.
  - Input accepts decimal prices; they are converted to cents via rounding.

### Usage Example (curl)

```bash
curl -X POST http://localhost:8080/generate \
  -H "Content-Type: application/json" \
  -H "X-API-KEY: secret-key" \
  -d '{
    "InvoiceNumber": "INV-2025-0001",
    "InvoiceDate": "2025-01-15",
    "Biller": {
      "Name": "Sample Biller GmbH",
      "Address": "Hauptstrasse 1, 1010 Wien",
      "VAT-ID": "ATU12345678",
      "Email": "office@biller.at"
    },
    "InvoiceRecipient": {
      "Name": "Bundesministerium Beispiel",
      "Address": "Regierungsplatz 1, 1010 Wien",
      "VAT-ID": "ATU87654321",
      "OrderReference": "B2G-ORDER-1234",
      "Email": "rechnung@b2g.gv.at"
    },
    "Details": [
      {
        "Quantity": 2,
        "Description": "Consulting hours",
        "UnitPrice": 120.5,
        "TaxRate": 20
      }
    ],
    "PaymentDetails": {
      "IBAN": "AT611904300234573201",
      "BIC": "BSPAATWW"
    }
  }'
```


