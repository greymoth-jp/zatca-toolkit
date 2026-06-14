// quickstart.mjs — the canonical few-lines example. This exact flow is run by
// test/quickstart.test.mjs in CI, so the documented snippet can never silently rot
// (a common failing of competitor SDK docs). In your app, import from '@zatca/sdk'.

import { Zatca } from '../src/index.mjs'; // published: import { Zatca } from '@zatca/sdk';

// A KSA standard tax invoice (normalized EN16931 model).
export const sampleInvoice = {
  profile_id: 'reporting:1.0',
  id: 'INV-2026-00042',
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

// Generate ZATCA-UBL, then check it would clear — all in-process, the invoice never leaves you.
export async function quickstart(doc = sampleInvoice) {
  const z = new Zatca({ mode: 'sandbox' });
  const ubl = (await z.generate(doc)).ubl;            // normalized -> UBL 2.1 XML
  const report = await z.validateXML(ubl, 'zatca-ksa'); // deterministic ZATCA/EN16931 check
  return { ubl, valid: report.report.valid, findings: report.report.errors };
}

// `node examples/quickstart.mjs`
if (import.meta.url === `file://${process.argv[1]}` || process.argv[1]?.endsWith('quickstart.mjs')) {
  const r = await quickstart();
  console.log(r.valid ? 'CLEARED — main rules pass' : 'NOT CLEARED', '·', r.findings.length, 'findings');
}
