import { test } from 'node:test';
import assert from 'node:assert/strict';
import { loadEngine, Zatca } from '../src/index.mjs';

const sampleDoc = {
  id: 'INV-SDK-1',
  issue_date: '2026-06-14',
  type_code: '388',
  currency: 'SAR',
  seller: { name: 'Acme Trading LLC', name_ar: 'شركة أكمي', vat_id: '300000000000003', country_code: 'SA' },
  buyer: { name: 'Beta Retail Co', vat_id: '311111111111113', country_code: 'SA' },
  lines: [{ id: '1', quantity: 2, unit_code: 'PCE', item_name: 'Widget', net_price: 50, net_amount: 100 }],
  tax_breakdown: [{ category: 'S', rate: 15, taxable_amount: 100, tax_amount: 15 }],
  totals: { line_extension_amount: 100, tax_exclusive_amount: 100, tax_amount: 15, tax_inclusive_amount: 115, payable_amount: 115 },
};

test('engine loads and reports a version', async () => {
  const engine = await loadEngine();
  const v = engine.version();
  assert.equal(v.ok, true);
  assert.match(v.version, /zatca-toolkit-engine/);
});

test('generate UBL then validate it through WASM (round-trip)', async () => {
  const engine = await loadEngine();
  const gen = engine.generateUBL(sampleDoc);
  assert.equal(gen.ok, true);
  assert.match(gen.ubl, /<Invoice/);

  const result = engine.validateXML(gen.ubl, 'en16931');
  assert.ok(result.report, 'expected a validation report');
  assert.ok(Array.isArray(result.report.errors));
  // No PARSE error — the engine successfully parsed its own output.
  assert.ok(!result.error, `unexpected hard error: ${result.error}`);
});

test('generate CII (UN/CEFACT) from the sample doc', async () => {
  const engine = await loadEngine();
  const r = engine.generateCII(sampleDoc);
  assert.equal(r.ok, true);
  assert.match(r.cii, /rsm:CrossIndustryInvoice/);
  assert.match(r.cii, /urn:un:unece:uncefact:data:standard:CrossIndustryInvoice/);
});

test('generate Factur-X PDF/A-3 (CII embedded) — engine + client', async () => {
  const engine = await loadEngine();
  const r = engine.generateFacturX(sampleDoc);
  assert.equal(r.ok, true, `expected ok; error: ${r.error}`);
  assert.equal(typeof r.pdf, 'string', 'pdf must be a base64 string');
  assert.ok(r.pdfBytes instanceof Uint8Array, 'pdfBytes must be a Uint8Array');
  // A real PDF: starts with %PDF-, ends with the EOF marker.
  const head = Buffer.from(r.pdfBytes.slice(0, 5)).toString('latin1');
  assert.equal(head, '%PDF-', `PDF must start with %PDF-, got ${head}`);
  const all = Buffer.from(r.pdfBytes).toString('latin1');
  assert.ok(all.includes('%%EOF'), 'PDF must contain the EOF marker');
  // The CII XML is embedded uncompressed, so its root element is present in the bytes.
  assert.ok(all.includes('CrossIndustryInvoice'), 'embedded CII not found in the PDF');

  // The high-level client exposes the same capability.
  const z = new Zatca({ mode: 'sandbox' });
  const cr = await z.generateFacturX(sampleDoc);
  assert.equal(cr.ok, true);
  assert.ok(cr.pdfBytes instanceof Uint8Array && cr.pdfBytes.length > 1000);
});

test('malformed XML fails closed with an error', async () => {
  const engine = await loadEngine();
  const result = engine.validateXML('<not-an-invoice/>');
  assert.equal(result.ok, false);
  assert.ok(result.error, 'expected a parse error message');
});

test('validateStructure flags a plain UBL missing ZATCA structural elements', async () => {
  const engine = await loadEngine();
  const gen = engine.generateUBL(sampleDoc); // plain UBL: no UUID/ICV/PIH/QR/signature
  const r = engine.validateStructure(gen.ubl);
  assert.equal(r.ok, false);
  const flagged = (r.findings || []).map((f) => f.rule_id);
  for (const want of ['BR-KSA-ST-UUID', 'BR-KSA-ST-ICV', 'BR-KSA-ST-PIH', 'BR-KSA-ST-SIG']) {
    assert.ok(flagged.includes(want), `expected ${want}`);
  }
});

test('validateQR rejects a non-Base64 QR with a fix hint', async () => {
  const engine = await loadEngine();
  const r = engine.validateQR('not-a-real-qr-!!!');
  assert.equal(r.ok, false);
  assert.ok(Array.isArray(r.findings) && r.findings.length > 0);
  const f = r.findings.find((x) => x.rule_id === 'BR-KSA-QR-01');
  assert.ok(f, 'expected BR-KSA-QR-01');
  assert.ok(f.fix_en && f.fix_ar, 'QR finding must carry EN+AR fix guidance');
});

test('Zatca client validate + idempotent mock clear (no double-clear)', async () => {
  const z = new Zatca({ apiKey: 'test' });
  const gen = await z.generate(sampleDoc);
  assert.equal(gen.ok, true);

  const a = await z.clear(gen.ubl, { idempotencyKey: 'key-1' });
  assert.equal(a.mock, true, 'default clearer must be a labeled mock');
  assert.equal(a.status, 'cleared');

  const b = await z.clear(gen.ubl, { idempotencyKey: 'key-1' });
  assert.equal(b.idempotentReplay, true, 'same key must replay, never double-clear');
  assert.equal(a.uuid, b.uuid);
});
