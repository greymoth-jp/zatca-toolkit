// webhook.mjs — verify inbound webhook signatures (api.md: X-Signature: hmac-sha256=...).
// Timing-safe comparison; no third-party dependency (node:crypto only).

import { createHmac, timingSafeEqual } from 'node:crypto';

const PREFIX = 'hmac-sha256=';

/**
 * verifyWebhook checks an HMAC-SHA256 signature over the raw request body.
 * @param {string|Buffer} rawBody  the exact bytes received (do not re-serialize JSON)
 * @param {string} signatureHeader the X-Signature header value (with or without prefix)
 * @param {string} secret          the endpoint signing secret
 * @returns {boolean}
 */
export function verifyWebhook(rawBody, signatureHeader, secret) {
  if (!signatureHeader || !secret) return false;
  const provided = signatureHeader.startsWith(PREFIX)
    ? signatureHeader.slice(PREFIX.length)
    : signatureHeader;

  const expected = createHmac('sha256', secret).update(rawBody).digest('hex');

  const a = Buffer.from(provided, 'hex');
  const b = Buffer.from(expected, 'hex');
  if (a.length !== b.length || a.length === 0) return false;
  return timingSafeEqual(a, b);
}

/** sign produces the X-Signature header value for a body (test/util/sender helper). */
export function signWebhook(rawBody, secret) {
  return PREFIX + createHmac('sha256', secret).update(rawBody).digest('hex');
}
