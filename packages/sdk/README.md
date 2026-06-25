# `@zatca/sdk`

> Open-source ZATCA (Saudi Fatoora Phase 2) e-invoicing SDK — **validate, generate,
> and sign** invoices in the browser or Node via WebAssembly.

The same deterministic Go rule engine that powers the free auditor runs here as WASM,
in-process: a pass/fail in your code is exactly what an accredited validator would see,
and **your invoice never leaves the caller** — no server round-trip.

This is **not tax advice and not a compliance guarantee.** The SDK prepares and validates
documents; it does **not** operate hosted clearance and is **not** a certified platform.
See the repository [`NOTICE`](../../NOTICE) and [`README`](../../README.md).

## Install

**Not yet published to npm.** Install it directly from the repo — the WASM engine is committed,
so `import { Zatca } from "@zatca/sdk"` resolves with no build step:

```bash
git clone https://github.com/greymoth-jp/zatca-toolkit
npm install ./zatca-toolkit/packages/sdk    # adds @zatca/sdk to your project
```

Requires Node >= 20 (this package is ESM, `"type": "module"`).

> **npm publish is coming soon** — the install will become `npm install @zatca/sdk` and the
> import is unchanged.

### Rebuilding the engine (optional)

The committed `wasm/engine.wasm` is what you import. To rebuild it from the Go core (needs Go):

```bash
cd packages/sdk
./build-wasm.sh        # regenerates wasm/engine.wasm + wasm/wasm_exec.js
```

## Quickstart

This snippet is CI-tested (`examples/quickstart.mjs`), so it never goes stale:

```js
import { Zatca } from "@zatca/sdk";

const z = new Zatca({ mode: "sandbox" });
const { ubl } = await z.generate(myNormalizedInvoice);      // normalized → UBL 2.1
const r = await z.validateXML(ubl, "zatca-ksa");            // deterministic ZATCA/EN16931 check
console.log(r.report.valid ? "would clear" : r.report.errors); // each error: rule_id + ar/en + severity
```

## Public API

```js
import { loadEngine, Zatca, signWebhook, verifyWebhook } from "@zatca/sdk";
```

- **`loadEngine()`** → `Promise<engine>` — the WASM engine, loaded once and cached. Methods:
  `validateXML(xml, profile?)`, `validateDoc(doc, profile?)`, `generateUBL(doc)`,
  `generateCII(doc)`, `validateQR(qr, opts?)`, `validateStructure(xml)`, `version()`.
  Profiles: `"zatca-ksa" | "en16931" | "peppol-bis"`.
- **`new Zatca({ mode, apiKey?, clearer? })`** → high-level client: `generate(doc)`,
  `validateXML(xml, profile?)`, `validate(doc, profile?)`, `clear(signedXml, { idempotencyKey })`.
  The default `clear()` is a clearly-labeled, **idempotent mock** (never claims a real
  clearance); pass your own `clearer` bound to an onboarded EGS / certified partner in production.
- **`signWebhook(body, secret)` / `verifyWebhook(body, sig, secret)`** — HMAC-SHA256
  helpers (timing-safe) for your own clearance callbacks.

TypeScript definitions ship in [`index.d.ts`](./index.d.ts).

## Examples

Runnable, well-commented scripts live in [`examples/`](./examples/) — see
[`examples/README.md`](./examples/README.md) for the index and how to run them.
Build the WASM engine first (above), then from `packages/sdk`:

```bash
node examples/01-generate-and-validate.mjs   # build → generate UBL → validate (verdict + findings)
node examples/02-credit-note-and-cii.mjs     # credit note (381) w/ BillingReference → UBL + UN/CEFACT CII
node examples/03-audit-inbound.mjs           # audit a third-party XML; structural + QR checks w/ fix hints
```

The minimal [`quickstart.mjs`](./examples/quickstart.mjs) and the full
[`api-tour.mjs`](./examples/api-tour.mjs) are also executed by `node --test` in CI, so
every documented snippet is guaranteed to run.

## License

[Apache-2.0](../../LICENSE).
