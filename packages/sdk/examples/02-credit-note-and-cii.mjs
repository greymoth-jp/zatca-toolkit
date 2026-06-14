// 02-credit-note-and-cii.mjs — two things beyond a plain invoice:
//   (a) a CREDIT NOTE (BT-3 type 381) that references the invoice it corrects
//       (BillingReference BT-25/BT-26) plus an IssueTime (KSA-DT-02 mandatory), and
//   (b) UN/CEFACT CII (EN16931) output — the other syntax the engine can emit.
//
//   Run from packages/sdk:  node examples/02-credit-note-and-cii.mjs
//   Prereq: build the WASM engine once with  ./build-wasm.sh
//
// In your own app:  import { loadEngine } from '@zatca/sdk';
import { loadEngine } from '../src/index.mjs';

// A credit note correcting INV-2026-00042 (one of the two widgets is returned).
// The BillingReference is what makes this a *valid* credit note under ZATCA:
// rule BR-KSA-CN-REF requires a type 381/383 note to reference its prior invoice.
const creditNote = {
  profile_id: 'reporting:1.0',
  id: 'CN-2026-00007',
  issue_date: '2026-06-20',
  issue_time: '14:05:00',          // KSA-DT-02 — ZATCA mandatory
  type_code: '381',                // BT-3  381 = credit note
  currency: 'SAR',
  tax_currency: 'SAR',
  billing_ref_id: 'INV-2026-00042', // BT-25 preceding invoice number  -> BillingReference
  billing_ref_date: '2026-06-14',   // BT-26 preceding invoice issue date
  seller: { name: 'Acme Trading LLC', name_ar: 'شركة أكمي للتجارة', vat_id: '300000000000003', country_code: 'SA' },
  buyer: { name: 'Beta Retail Co', vat_id: '311111111111113', country_code: 'SA' },
  lines: [
    { id: '1', quantity: 1, unit_code: 'PCE', item_name: 'Widget (returned)', net_price: 50, net_amount: 50, vat_category: 'S', vat_rate: 15 },
  ],
  tax_breakdown: [{ category: 'S', rate: 15, taxable_amount: 50, tax_amount: 7.5 }],
  totals: { line_extension_amount: 50, tax_exclusive_amount: 50, tax_amount: 7.5, tax_inclusive_amount: 57.5, payable_amount: 57.5 },
};

function rootElement(xml) {
  return xml.match(/<(?:\w+:)?(Invoice|CreditNote)[\s>]/)?.[1] ?? '(unknown)';
}

async function main() {
  const engine = await loadEngine();

  console.log('=== 1. Generate the credit note as ZATCA-UBL 2.1 ===');
  const ubl = engine.generateUBL(creditNote);     // -> { ok, ubl }
  if (!ubl.ok) throw new Error(`UBL generation failed: ${ubl.error}`);
  console.log(`  root element : <${rootElement(ubl.ubl)}>   (UBL uses a distinct CreditNote root)`);
  console.log(`  carries BillingReference : ${/BillingReference/.test(ubl.ubl)}`);
  console.log(`  carries IssueTime        : ${/IssueTime/.test(ubl.ubl)}`);

  console.log('\n=== 2. Validate it (BR-KSA-CN-REF must be satisfied) ===');
  const report = engine.validateXML(ubl.ubl, 'zatca-ksa');
  console.log(`  Verdict: ${report.report.valid ? 'VALID' : 'INVALID'}  · ${report.report.errors.length} findings`);

  // Counter-example: drop the reference and watch the credit-note rule fire.
  const orphan = { ...creditNote, billing_ref_id: '', billing_ref_date: '' };
  const orphanReport = engine.validateDoc(orphan, 'zatca-ksa');
  console.log('  Without a BillingReference, the engine flags:',
    orphanReport.report.errors.map((e) => e.rule_id).join(', ') || '(nothing)');

  console.log('\n=== 3. Render UN/CEFACT CII (EN16931) from the same model ===');
  const cii = engine.generateCII(creditNote);     // -> { ok, cii }
  if (!cii.ok) throw new Error(`CII generation failed: ${cii.error}`);
  console.log(`  is CrossIndustryInvoice : ${/rsm:CrossIndustryInvoice/.test(cii.cii)}`);
  console.log(`  type code 381 present   : ${/<ram:TypeCode>381</.test(cii.cii)}`);
  console.log('  CII (first 2 lines):');
  console.log(cii.cii.split('\n').slice(0, 2).map((l) => '    ' + l).join('\n'));

  console.log(`\nSUMMARY: credit note ${creditNote.id} -> UBL <${rootElement(ubl.ubl)}> ${report.report.valid ? 'VALID' : 'INVALID'}; CII emitted (${cii.cii.length} chars).`);
}

main().catch((err) => { console.error('ERROR:', err.message); process.exitCode = 1; });
