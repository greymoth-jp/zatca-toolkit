// 03-audit-inbound.mjs — the AUDITOR's view. You received an invoice XML from a third
// party and want to know, locally, whether it would clear — without uploading it
// anywhere. The same engine that GENERATES also parses + validates arbitrary UBL.
//
// It also shows the two ZATCA-specific gates that a plain (unsigned) invoice fails:
//   - validateStructure : UUID / ICV / PIH / QR / XAdES signature presence
//   - validateQR        : the Base64-TLV QR a cleared invoice must embed
// Both return findings WITH bilingual fix hints (fix_en / fix_ar).
//
//   Run from packages/sdk:  node examples/03-audit-inbound.mjs
//   Prereq: build the WASM engine once with  ./build-wasm.sh
//
// In your own app:  import { loadEngine } from '@zatca/sdk';
import { loadEngine } from '../src/index.mjs';

// A normalized invoice we'll render to XML to stand in for an inbound third-party file.
const sample = {
  profile_id: 'reporting:1.0',
  id: 'INV-INBOUND-9001',
  issue_date: '2026-06-14',
  issue_time: '10:30:00',
  type_code: '388',
  currency: 'SAR',
  tax_currency: 'SAR',
  seller: { name: 'Acme Trading LLC', name_ar: 'شركة أكمي للتجارة', vat_id: '300000000000003', country_code: 'SA' },
  buyer: { name: 'Beta Retail Co', vat_id: '311111111111113', country_code: 'SA' },
  lines: [{ id: '1', quantity: 2, unit_code: 'PCE', item_name: 'Widget', net_price: 50, net_amount: 100, vat_category: 'S', vat_rate: 15 }],
  tax_breakdown: [{ category: 'S', rate: 15, taxable_amount: 100, tax_amount: 15 }],
  totals: { line_extension_amount: 100, tax_exclusive_amount: 100, tax_amount: 15, tax_inclusive_amount: 115, payable_amount: 115 },
};

function groupBySeverity(findings = []) {
  const groups = { fatal: [], warning: [] };
  for (const f of findings) (groups[f.severity] ??= []).push(f);
  return groups;
}

function printGrouped(findings) {
  const groups = groupBySeverity(findings);
  for (const sev of ['fatal', 'warning']) {
    const list = groups[sev] ?? [];
    console.log(`  ${sev.toUpperCase()} (${list.length}):`);
    for (const f of list) {
      console.log(`    - ${f.rule_id}  @ ${f.path}`);
      console.log(`        ${f.message_en}`);
      if (f.fix_en) console.log(`        FIX: ${f.fix_en}`);
    }
  }
}

async function main() {
  const engine = await loadEngine();

  // Produce an inbound XML string to audit. In practice this is bytes you received
  // (e.g. fs.readFileSync('partner-invoice.xml','utf8')); here we generate one.
  const inboundXml = engine.generateUBL(sample).ubl;

  console.log('=== 1. Audit the inbound XML against the ZATCA-KSA ruleset ===');
  const report = engine.validateXML(inboundXml, 'zatca-ksa'); // -> { ok, report:{ valid, errors } }
  if (!report.ok) {
    // A non-invoice / malformed file fails closed with a hard parse error.
    console.log(`  HARD ERROR (could not parse): ${report.error}`);
  } else {
    console.log(`  Verdict: ${report.report.valid ? 'VALID' : 'INVALID'}`);
    console.log('  Findings grouped by severity:');
    printGrouped(report.report.errors);
  }

  console.log('\n=== 2. Malformed input fails closed (not a UBL invoice) ===');
  const bad = engine.validateXML('<not-an-invoice/>');
  console.log(`  ok=${bad.ok}  error="${bad.error}"`);

  console.log('\n=== 3. ZATCA STRUCTURAL check (UUID / ICV / PIH / QR / signature) ===');
  // A plain unsigned invoice is intentionally missing these — exactly what to expect
  // pre-signing. Each finding ships an actionable bilingual fix hint.
  const structure = engine.validateStructure(inboundXml); // -> { ok, findings:[...] }
  console.log(`  Structurally ready to submit: ${structure.ok ? 'yes' : 'no'}`);
  printGrouped(structure.findings);

  console.log('\n=== 4. ZATCA QR check (Base64 TLV) ===');
  const qr = engine.validateQR('not-a-real-qr-!!!', { signed: true, simplified: false }); // -> { ok, findings:[...] }
  console.log(`  QR valid: ${qr.ok ? 'yes' : 'no'}`);
  printGrouped(qr.findings);

  const totalIssues =
    (report.ok ? report.report.errors.length : 1) +
    (structure.findings?.length ?? 0) +
    (qr.findings?.length ?? 0);
  console.log(`\nSUMMARY: audited ${sample.id} — content ${report.ok && report.report.valid ? 'VALID' : 'has issues'}; ` +
    `${structure.findings?.length ?? 0} structural + ${qr.findings?.length ?? 0} QR findings (${totalIssues} total to resolve before clearance).`);
}

main().catch((err) => { console.error('ERROR:', err.message); process.exitCode = 1; });
