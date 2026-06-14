// api-tour.mjs — every public @zatca/sdk call, in one runnable file. test/api-tour.test.mjs
// executes this in CI, so EVERY snippet reproduced in the docs site is guaranteed to run and
// return the documented shape. Competitor SDK docs drift from the code; ours cannot. In your
// app: import { Zatca, loadEngine, signWebhook, verifyWebhook } from '@zatca/sdk';

import { Zatca, loadEngine, signWebhook, verifyWebhook } from '../src/index.mjs';

// A KSA standard tax invoice (normalized EN16931 model). Same shape the engine round-trips.
export const sampleInvoice = {
  profile_id: 'reporting:1.0',
  id: 'INV-2026-00042',
  issue_date: '2026-06-14',
  issue_time: '10:30:00',
  type_code: '388', // 388 = tax invoice (standard); 381 = credit note
  currency: 'SAR',
  tax_currency: 'SAR',
  seller: { name: 'Acme Trading LLC', name_ar: 'شركة أكمي للتجارة', vat_id: '300000000000003', country_code: 'SA' },
  buyer: { name: 'Beta Retail Co', vat_id: '311111111111113', country_code: 'SA' },
  lines: [{ id: '1', quantity: 2, unit_code: 'PCE', item_name: 'Widget', net_price: 50, net_amount: 100, vat_category: 'S', vat_rate: 15 }],
  tax_breakdown: [{ category: 'S', rate: 15, taxable_amount: 100, tax_amount: 15 }],
  totals: { line_extension_amount: 100, tax_exclusive_amount: 100, tax_amount: 15, tax_inclusive_amount: 115, payable_amount: 115 },
};

// 1. Engine version — proves the WASM engine loaded (same engine in browser + Node).
export async function tourVersion() {
  const engine = await loadEngine();
  return engine.version(); // -> { ok: true, version: 'zatca-toolkit-engine ...' }
}

// 2. Generate ZATCA-UBL 2.1 from the normalized model.
export async function tourGenerate(doc = sampleInvoice) {
  const z = new Zatca({ mode: 'sandbox' });
  const gen = await z.generate(doc); // -> { ok: true, ubl: '<Invoice ...>' }
  return gen;
}

// 3. Validate the generated XML. profile: 'zatca-ksa' | 'en16931' | 'peppol-bis'.
export async function tourValidateXML(doc = sampleInvoice) {
  const z = new Zatca({ mode: 'sandbox' });
  const { ubl } = await z.generate(doc);
  const r = await z.validateXML(ubl, 'zatca-ksa'); // -> { ok, report: { valid, errors:[...] } }
  return r;
}

// 4. Validate the normalized object directly (no XML in hand yet).
export async function tourValidateDoc(doc = sampleInvoice) {
  const z = new Zatca({ mode: 'sandbox' });
  return z.validate(doc, 'en16931'); // -> { ok, report: { valid, errors:[...] } }
}

// 5. Structural check (UUID/ICV/PIH/QR/signature) of a submitted invoice. Plain UBL is
//    intentionally missing these, so this returns findings — exactly what to expect pre-signing.
export async function tourValidateStructure(doc = sampleInvoice) {
  const engine = await loadEngine();
  const { ubl } = engine.generateUBL(doc);
  return engine.validateStructure(ubl); // -> { ok:false, findings:[{ rule_id:'BR-KSA-ST-UUID', ... }] }
}

// 6. Verify a ZATCA QR (Base64 TLV). A malformed QR returns a finding WITH an Arabic+English fix.
export async function tourValidateQR(qr = 'not-a-real-qr') {
  const engine = await loadEngine();
  return engine.validateQR(qr, { signed: true, simplified: false });
  // -> { ok:false, findings:[{ rule_id:'BR-KSA-QR-01', message_en, message_ar, fix_en, fix_ar }] }
}

// 7. Clearance is credential-gated. The default clearer is a labeled MOCK (never a real
//    clearance) and is idempotent: a repeated idempotencyKey replays, never double-clears.
export async function tourClear(doc = sampleInvoice) {
  const z = new Zatca({ mode: 'sandbox' }); // pass { clearer } bound to your EGS/partner in prod
  const { ubl } = await z.generate(doc);
  const first = await z.clear(ubl, { idempotencyKey: 'inv-42' });   // { status:'cleared', mock:true, ... }
  const replay = await z.clear(ubl, { idempotencyKey: 'inv-42' });  // { ..., idempotentReplay:true }
  return { first, replay };
}

// 8. Webhook helpers — HMAC sign/verify for your own clearance callbacks.
export function tourWebhook() {
  const body = JSON.stringify({ uuid: 'inv-42', status: 'cleared' });
  const sig = signWebhook(body, 'whsec_demo');         // -> 'hmac-sha256=...'
  const ok = verifyWebhook(body, sig, 'whsec_demo');   // -> true
  return { sig, ok };
}

export async function runAll() {
  return {
    version: await tourVersion(),
    generate: await tourGenerate(),
    validateXML: await tourValidateXML(),
    validateDoc: await tourValidateDoc(),
    structure: await tourValidateStructure(),
    qr: await tourValidateQR(),
    clear: await tourClear(),
    webhook: tourWebhook(),
  };
}

// `node examples/api-tour.mjs`
if (process.argv[1]?.endsWith('api-tour.mjs')) {
  const r = await runAll();
  console.log('version :', r.version.version);
  console.log('generate:', r.generate.ok, r.generate.ubl.slice(0, 24) + '...');
  console.log('validate:', r.validateXML.report.valid ? 'valid' : 'invalid', '·', r.validateXML.report.errors.length, 'findings');
  console.log('structure findings:', r.structure.findings.map((f) => f.rule_id).join(', '));
  console.log('qr finding:', r.qr.findings[0]?.rule_id);
  console.log('clear   :', r.clear.first.status, '· replay=', r.clear.replay.idempotentReplay === true);
  console.log('webhook :', r.webhook.ok ? 'verified' : 'FAILED');
}
