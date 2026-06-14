import { test } from 'node:test';
import assert from 'node:assert/strict';
import { quickstart, sampleInvoice } from '../examples/quickstart.mjs';
import { Zatca } from '../src/index.mjs';

// Guarantees the documented quickstart actually works end to end (generate -> validate),
// so the README snippet is never stale.
test('quickstart: the documented sample invoice clears', async () => {
  const r = await quickstart();
  assert.match(r.ubl, /<Invoice/);
  assert.equal(r.valid, true, `sample should clear; findings: ${JSON.stringify(r.findings)}`);
});

test('quickstart: a broken invoice does not clear', async () => {
  const broken = structuredClone(sampleInvoice);
  broken.totals.tax_inclusive_amount = 9999; // break BR-CO-15
  const r = await quickstart(broken);
  assert.equal(r.valid, false);
  assert.ok(r.findings.some((f) => f.rule_id === 'BR-CO-15'));
});

// Regression for the KSA-first default: Zatca.validateXML/validate must default to the
// 'zatca-ksa' profile, not 'peppol-bis'. This is discriminating, not tautological — the
// sample carries no Peppol electronic addresses, so it PASSES zatca-ksa but FAILS peppol-bis.
// If the default ever regresses to peppol, the first assertion goes red.
test('Zatca client defaults to the zatca-ksa profile (KSA-first)', async () => {
  const z = new Zatca({ mode: 'sandbox' });
  const { ubl } = await z.generate(sampleInvoice);

  const def = await z.validateXML(ubl); // no profile -> must run ZATCA-KSA
  assert.equal(def.report.valid, true,
    `default profile must be zatca-ksa; got findings: ${JSON.stringify(def.report.errors)}`);

  const peppol = await z.validateXML(ubl, 'peppol-bis'); // sanity: same XML fails Peppol
  assert.equal(peppol.report.valid, false,
    'sanity check: the sample lacks Peppol endpoints and must fail peppol-bis');

  // validate(doc) (object form) must default to zatca-ksa too.
  const defDoc = await z.validate(sampleInvoice);
  assert.equal(defDoc.report.valid, true, 'validate(doc) must also default to zatca-ksa');
});
