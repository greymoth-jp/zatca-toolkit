import { test } from 'node:test';
import assert from 'node:assert/strict';
import { verifyWebhook, signWebhook } from '../src/index.mjs';

const secret = 'whsec_test_123';
const body = JSON.stringify({ event: 'invoice.cleared', uuid: 'U-1' });

test('valid signature verifies', () => {
  const sig = signWebhook(body, secret);
  assert.equal(verifyWebhook(body, sig, secret), true);
});

test('signature without the hmac-sha256= prefix still verifies', () => {
  const sig = signWebhook(body, secret).replace('hmac-sha256=', '');
  assert.equal(verifyWebhook(body, sig, secret), true);
});

test('wrong secret fails', () => {
  const sig = signWebhook(body, secret);
  assert.equal(verifyWebhook(body, sig, 'whsec_wrong'), false);
});

test('tampered body fails', () => {
  const sig = signWebhook(body, secret);
  assert.equal(verifyWebhook(body + ' ', sig, secret), false);
});

test('missing signature or secret fails closed', () => {
  assert.equal(verifyWebhook(body, '', secret), false);
  assert.equal(verifyWebhook(body, signWebhook(body, secret), ''), false);
});
