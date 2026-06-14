// client.mjs — the Zatca client. Generation + validation are REAL (WASM engine). Clearance
// is a credential-gated boundary: this SDK does NOT operate hosted clearance (that requires
// an onboarded EGS + ZATCA Production CSID, or a certified partner — see LEGAL_RISK). The
// default clearer is an honest MOCK that never claims a real clearance, and clearance is
// idempotent (same idempotencyKey => same result, so retries can never double-clear).

import { loadEngine } from './engine.mjs';

/**
 * mockClearer is the default, clearly-labeled stub. It returns a deterministic fake result
 * and marks it as a mock. Replace it with a real clearer bound to your onboarded credentials
 * or a certified partner. It NEVER claims a live clearance.
 */
function mockClearer() {
  let counter = 0;
  return async function clear(_signedXml, { idempotencyKey } = {}) {
    counter += 1;
    return {
      status: 'cleared',
      uuid: idempotencyKey || `MOCK-${counter}`,
      mock: true,
      note: 'MOCK clearance — not a real ZATCA clearance. Live clearance is credential-gated (EGS onboarding + Production CSID) or delegated to a certified partner. See NOTICE / LEGAL_RISK.',
    };
  };
}

export class Zatca {
  /**
   * @param {object} [opts]
   * @param {string} [opts.apiKey]
   * @param {'sandbox'|'production'} [opts.mode]
   * @param {(signedXml: string, o?: {idempotencyKey?: string}) => Promise<object>} [opts.clearer]
   */
  constructor(opts = {}) {
    this.apiKey = opts.apiKey || null;
    this.mode = opts.mode || 'sandbox';
    this._clearer = opts.clearer || mockClearer();
    this._enginePromise = loadEngine();
    this._clearCache = new Map(); // idempotencyKey -> result (no double-clear)
  }

  async _engine() {
    return this._enginePromise;
  }

  /** Validate a UBL/ZATCA XML string. Defaults to the ZATCA-KSA profile (this is a
   * KSA-first toolkit); pass 'en16931' or 'peppol-bis' to validate against those layers. */
  async validateXML(xml, profile = 'zatca-ksa') {
    return (await this._engine()).validateXML(xml, profile);
  }

  /** Validate a normalized invoice object. Defaults to the ZATCA-KSA profile. */
  async validate(doc, profile = 'zatca-ksa') {
    return (await this._engine()).validateDoc(doc, profile);
  }

  /** Render a normalized invoice to UBL 2.1 XML. */
  async generate(doc) {
    return (await this._engine()).generateUBL(doc);
  }

  /** Render the invoice to a Factur-X PDF/A-3 document (CII embedded as factur-x.xml).
   * Returns { ok, pdf (base64), pdfBytes }. PDF/A-3 structure; not veraPDF-certified. */
  async generateFacturX(doc) {
    return (await this._engine()).generateFacturX(doc);
  }

  /**
   * Clear a signed invoice. Idempotent on idempotencyKey: a repeat returns the first result
   * without re-invoking the clearer, so a retry cannot produce a second clearance.
   */
  async clear(signedXml, { idempotencyKey } = {}) {
    if (idempotencyKey && this._clearCache.has(idempotencyKey)) {
      return { ...this._clearCache.get(idempotencyKey), idempotentReplay: true };
    }
    const result = await this._clearer(signedXml, { idempotencyKey });
    if (idempotencyKey) this._clearCache.set(idempotencyKey, result);
    return result;
  }
}
