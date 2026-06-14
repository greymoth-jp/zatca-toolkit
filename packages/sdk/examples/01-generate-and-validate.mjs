// 01-generate-and-validate.mjs — the core developer loop: build a normalized invoice,
// render it to ZATCA-UBL 2.1, then run the SAME deterministic rules an accredited
// validator runs — locally, in-process. The invoice never leaves this machine.
//
//   Run from packages/sdk:  node examples/01-generate-and-validate.mjs
//   Prereq: build the WASM engine once with  ./build-wasm.sh
//
// In your own app you would import from the published package instead:
//   import { Zatca } from '@zatca/sdk';
import { Zatca } from '../src/index.mjs';

// A KSA STANDARD tax invoice in the normalized EN16931 model (BT/BG field names).
// Numbers are pre-computed so the run is fully deterministic (no Date.now / random).
const invoice = {
  profile_id: 'reporting:1.0',
  id: 'INV-2026-00042',
  issue_date: '2026-06-14',   // BT-2  (YYYY-MM-DD)
  issue_time: '10:30:00',     // KSA-DT-02 — ZATCA mandatory
  type_code: '388',           // BT-3  388 = standard tax invoice
  currency: 'SAR',            // BT-5
  tax_currency: 'SAR',        // BT-6
  seller: {
    name: 'Acme Trading LLC',
    name_ar: 'شركة أكمي للتجارة', // Arabic name — ZATCA mandatory for the seller
    vat_id: '300000000000003',     // BT-31
    country_code: 'SA',
  },
  buyer: {
    name: 'Beta Retail Co',
    vat_id: '311111111111113',     // BT-48
    country_code: 'SA',
  },
  lines: [
    {
      id: '1', quantity: 2, unit_code: 'PCE', item_name: 'Widget',
      net_price: 50, net_amount: 100,            // BT-146 / BT-131
      vat_category: 'S', vat_rate: 15,           // BT-151 / BT-152 (standard 15%)
    },
  ],
  tax_breakdown: [
    { category: 'S', rate: 15, taxable_amount: 100, tax_amount: 15 }, // BG-23
  ],
  totals: {
    line_extension_amount: 100, // BT-106
    tax_exclusive_amount: 100,  // BT-109
    tax_amount: 15,             // BT-110
    tax_inclusive_amount: 115,  // BT-112
    payable_amount: 115,        // BT-115
  },
};

function printFindings(errors) {
  if (!errors || errors.length === 0) {
    console.log('  (no findings)');
    return;
  }
  for (const e of errors) {
    // Validation-report findings carry rule_id + path + bilingual messages + severity.
    console.log(`  [${e.severity.toUpperCase()}] ${e.rule_id}  @ ${e.path}`);
    console.log(`      en: ${e.message_en}`);
    console.log(`      ar: ${e.message_ar}`);
  }
}

async function main() {
  const z = new Zatca({ mode: 'sandbox' });

  console.log('=== 1. Generate ZATCA-UBL 2.1 from the normalized model ===');
  const gen = await z.generate(invoice);          // -> { ok, ubl }
  if (!gen.ok) throw new Error(`generation failed: ${gen.error}`);
  console.log('Generated UBL (first 3 lines):');
  console.log(gen.ubl.split('\n').slice(0, 3).map((l) => '  ' + l).join('\n'));
  console.log(`  ... ${gen.ubl.length} chars total`);

  console.log('\n=== 2. Validate the generated XML with the ZATCA-KSA ruleset ===');
  // profile: 'zatca-ksa' | 'en16931' | 'peppol-bis'
  const report = await z.validateXML(gen.ubl, 'zatca-ksa'); // -> { ok, report:{ valid, errors } }
  if (!report.ok) throw new Error(`hard parse error: ${report.error}`);
  console.log(`Verdict: ${report.report.valid ? 'VALID — main rules pass' : 'INVALID'}`);
  console.log('Findings:');
  printFindings(report.report.errors);

  console.log('\n=== 3. Same check, straight from the object (no XML in hand) ===');
  const docReport = await z.validate(invoice, 'zatca-ksa'); // -> { ok, report:{ valid, errors } }
  console.log(`Verdict: ${docReport.report.valid ? 'VALID' : 'INVALID'}  · ${docReport.report.errors.length} findings`);

  const verdict = report.report.valid && docReport.report.valid;
  console.log(`\nSUMMARY: standard invoice ${invoice.id} — ${verdict ? 'VALID (would clear main rules)' : 'INVALID'}.`);
}

main().catch((err) => { console.error('ERROR:', err.message); process.exitCode = 1; });
