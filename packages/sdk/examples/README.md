# `@zatca/sdk` — runnable examples

Each file is a self-contained ES module that imports the SDK from `../src/index.mjs`
(in your own app you would `import … from "@zatca/sdk"`). They use only the Node
standard library and the SDK — no extra dependencies — and are deterministic
(no `Date.now` / random), so the output is the same on every run.

## Prerequisite — build the WASM engine once

The SDK runs the Go rule engine as WebAssembly. Build it before running any example:

```bash
cd packages/sdk
./build-wasm.sh        # produces wasm/engine.wasm + wasm/wasm_exec.js
```

Then run any example from the `packages/sdk` directory:

```bash
node examples/01-generate-and-validate.mjs
node examples/02-credit-note-and-cii.mjs
node examples/03-audit-inbound.mjs
```

## What each example shows

| File | Shows |
|---|---|
| [`01-generate-and-validate.mjs`](./01-generate-and-validate.mjs) | Build a standard tax invoice (type 388) in the normalized model, render **ZATCA-UBL 2.1**, then validate both the XML and the object with the `zatca-ksa` ruleset. Prints the verdict and any findings (`rule_id` + EN/AR messages + severity). |
| [`02-credit-note-and-cii.mjs`](./02-credit-note-and-cii.mjs) | Build a **credit note (type 381)** that references its prior invoice (`billing_ref_id` / `billing_ref_date` → BillingReference) and carries an `issue_time`; show that **BR-KSA-CN-REF** fires when the reference is missing; then render **UN/CEFACT CII (EN16931)** from the same model. |
| [`03-audit-inbound.mjs`](./03-audit-inbound.mjs) | The **auditor's view**: validate a third-party UBL string locally and group findings by severity (`fatal` / `warning`). Demonstrates `validateStructure` (UUID/ICV/PIH/QR/signature) and `validateQR` (Base64 TLV) — the ZATCA structural/QR gates — each finding carrying a bilingual fix hint (`fix_en` / `fix_ar`). |

There are also two minimal references next to these:
[`quickstart.mjs`](./quickstart.mjs) (the shortest flow) and
[`api-tour.mjs`](./api-tour.mjs) (every public call in one file). Both are executed by
`node --test` in CI, so the snippets can never silently drift from the code.

## API used (all from `@zatca/sdk`)

- `loadEngine()` → `{ validateXML, validateDoc, generateUBL, generateCII, validateQR, validateStructure, version }`
- `new Zatca({ mode })` → `generate`, `validateXML`, `validate`, `clear` (idempotent **mock** clearer by default)
- `signWebhook`, `verifyWebhook` (HMAC-SHA256 helpers, not used by these three examples)

## Not tax advice / not a compliance guarantee

These examples prepare and validate documents to help you evaluate the SDK. They are
**not tax advice and not a compliance guarantee.** The SDK does **not** operate hosted
clearance and is **not** a certified platform: live clearance/reporting requires your
onboarded EGS + ZATCA Production CSID (or a certified partner). The default `clear()` is
a clearly-labeled **mock** that never claims a real clearance. See the repository
[`NOTICE`](../../../NOTICE) and [`README`](../../../README.md).
