// Copy curl command to clipboard
function copyCurlCommand() {
    const curlCommand = `curl -X POST https://api.at-invoice.at/generate \\
  -H "X-API-KEY: at_live_..." \\
  -H "Content-Type: application/json" \\
  -d '{
    "invoice_number": "RE-2026-001",
    "invoice_date": "2026-01-07",
    "biller": {
      "name": "Musterfirma GmbH",
      "vat_id": "ATU13585627",
      "address": {
        "street": "Hauptstraße 1",
        "zip": "1010",
        "city": "Wien"
      },
      "email": "billing@musterfirma.at"
    },
    "recipient": {
      "name": "Kunde AG",
      "vat_id": "ATU87654321",
      "order_reference": "1234567890",
      "address": {
        "street": "Teststraße 5",
        "zip": "4020",
        "city": "Linz"
      }
    },
    "items": [{
      "description": "Beratungsleistung",
      "quantity": 10,
      "unit_price_cents": 12000,
      "tax_rate": 20
    }],
    "payment": {
      "iban": "AT123400000000005678",
      "bic": "BKAUATWW"
    }
  }'`;

    navigator.clipboard.writeText(curlCommand).then(() => {
        const btn = document.getElementById('copyBtn');
        const text = document.getElementById('copyText');
        if (btn && text) {
            btn.classList.add('bg-green-600');
            text.textContent = 'Kopiert!';
            
            setTimeout(() => {
                btn.classList.remove('bg-green-600');
                text.textContent = 'Kopieren';
            }, 2000);
        }
    }).catch(err => {
        console.error('Failed to copy:', err);
        alert('Kopieren fehlgeschlagen. Bitte manuell kopieren.');
    });
}

