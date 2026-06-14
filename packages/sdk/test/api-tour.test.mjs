// api-tour.test.mjs — runs every snippet from examples/api-tour.mjs (the same code the docs
// site shows) and asserts the documented return shapes. If a documented snippet rots, CI fails.

import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  tourVersion, tourGenerate, tourValidateXML, tourValidateDoc,
  tourValidateStructure, tourValidateQR, tourClear, tourWebhook,
} from '../examples/api-tour.mjs';

test('docs: version() returns the engine version', async () => {
  const v = await tourVersion();
  assert.equal(v.ok, true);
  assert.match(v.version, /zatca-toolkit-engine/);
});

test('docs: generate() returns ok + UBL Invoice XML', async () => {
  const gen = await tourGenerate();
  assert.equal(gen.ok, true);
  assert.match(gen.ubl, /<Invoice/);
});

test('docs: validateXML() returns a report with an errors array', async () => {
  const r = await tourValidateXML();
  assert.ok(r.report, 'expected report');
  assert.equal(typeof r.report.valid, 'boolean');
  assert.ok(Array.isArray(r.report.errors));
  assert.ok(!r.error, `unexpected hard error: ${r.error}`);
});

test('docs: validateDoc() validates the normalized object directly', async () => {
  const r = await tourValidateDoc();
  assert.ok(r.report);
  assert.ok(Array.isArray(r.report.errors));
});

test('docs: validateStructure() flags missing ZATCA structural elements', async () => {
  const r = await tourValidateStructure();
  assert.equal(r.ok, false);
  const ids = (r.findings || []).map((f) => f.rule_id);
  for (const want of ['BR-KSA-ST-UUID', 'BR-KSA-ST-ICV', 'BR-KSA-ST-PIH', 'BR-KSA-ST-SIG']) {
    assert.ok(ids.includes(want), `expected ${want}`);
  }
});

test('docs: validateQR() returns BR-KSA-QR-01 with EN+AR fix', async () => {
  const r = await tourValidateQR();
  assert.equal(r.ok, false);
  const f = (r.findings || []).find((x) => x.rule_id === 'BR-KSA-QR-01');
  assert.ok(f, 'expected BR-KSA-QR-01');
  assert.ok(f.fix_en && f.fix_ar, 'QR finding must carry EN+AR fix');
});

test('docs: clear() is a labeled mock and idempotent (no double-clear)', async () => {
  const { first, replay } = await tourClear();
  assert.equal(first.mock, true, 'default clearer must be a labeled mock');
  assert.equal(first.status, 'cleared');
  assert.equal(replay.idempotentReplay, true, 'same key must replay, never double-clear');
  assert.equal(first.uuid, replay.uuid);
});

test('docs: webhook sign/verify round-trips', () => {
  const { sig, ok } = tourWebhook();
  assert.match(sig, /^hmac-sha256=/);
  assert.equal(ok, true);
});
