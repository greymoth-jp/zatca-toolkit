// engine.mjs — loads the Go WebAssembly engine and exposes validate/generate to JS.
// The SAME deterministic Go rules run here (Node) and in the browser (audit app), so a
// pass/fail in the free auditor is exactly what the SDK and CI see. No server round-trip;
// the invoice never leaves the caller (LEGAL_RISK: invoice personal data stays client-side).

import { readFile } from 'node:fs/promises';

let enginePromise = null;

// b64ToBytes decodes a base64 string to a Uint8Array. Portable across Node (Buffer) and
// the browser (atob), so the engine API is identical in both.
function b64ToBytes(b64) {
  if (typeof Buffer !== 'undefined') return Uint8Array.from(Buffer.from(b64, 'base64'));
  const bin = atob(b64);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

/**
 * loadEngine instantiates the WASM engine once and returns a stable API.
 * Subsequent calls return the same instance.
 */
export function loadEngine() {
  if (!enginePromise) {
    enginePromise = instantiate();
  }
  return enginePromise;
}

async function instantiate() {
  // wasm_exec.js is Go's JS glue; importing it sets globalThis.Go.
  await import(new URL('../wasm/wasm_exec.js', import.meta.url));
  const Go = globalThis.Go;
  if (typeof Go !== 'function') {
    throw new Error('zatca/sdk: wasm_exec.js did not define globalThis.Go');
  }
  const go = new Go();
  const wasmBytes = await readFile(new URL('../wasm/engine.wasm', import.meta.url));
  const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject);

  // The Go main() sets the exported functions then parks on select{}; it signals us via
  // __zatcaReady before parking, so we do NOT await go.run (it never resolves by design).
  const ready = new Promise((resolve) => {
    globalThis.__zatcaReady = resolve;
  });
  go.run(instance);
  await ready;

  const call = (fnName, ...args) => {
    const fn = globalThis[fnName];
    if (typeof fn !== 'function') {
      throw new Error(`zatca/sdk: engine function ${fnName} not available`);
    }
    return JSON.parse(fn(...args));
  };

  return {
    /** Validate a UBL/ZATCA XML string. profile: 'zatca-ksa' (default) | 'en16931' | 'peppol-bis'. */
    validateXML: (xml, profile = 'zatca-ksa') => call('zatcaValidateXML', String(xml), profile),
    /** Validate an already-normalized invoice object. profile defaults to 'zatca-ksa'. */
    validateDoc: (doc, profile = 'zatca-ksa') => call('zatcaValidateDoc', JSON.stringify(doc), profile),
    /** Render a normalized invoice object to UBL 2.1 XML. */
    generateUBL: (doc) => call('zatcaGenerateUBL', JSON.stringify(doc)),
    /** Render a normalized invoice object to UN/CEFACT CII (EN16931) XML. */
    generateCII: (doc) => call('zatcaGenerateCII', JSON.stringify(doc)),
    /** Render the invoice to a Factur-X PDF/A-3 document (the CII embedded as factur-x.xml).
     * Returns { ok, pdf } where pdf is base64, plus pdfBytes (Uint8Array) for convenience.
     * The document targets PDF/A-3b structure but is NOT veraPDF-certified (run tools/pdfa3/verify.sh). */
    generateFacturX: (doc) => {
      const r = call('zatcaGenerateFacturX', JSON.stringify(doc));
      if (r && r.ok && typeof r.pdf === 'string') r.pdfBytes = b64ToBytes(r.pdf);
      return r;
    },
    /** Verify a ZATCA QR (Base64 TLV). opts: {signed=true, simplified=false}. */
    validateQR: (qr, { signed = true, simplified = false } = {}) => call('zatcaValidateQR', String(qr), signed, simplified),
    /** Verify the ZATCA structural elements (UUID/ICV/PIH/QR/signature) of a submitted invoice XML. */
    validateStructure: (xml) => call('zatcaValidateStructure', String(xml)),
    /** Engine version string. */
    version: () => call('zatcaVersion'),
  };
}
