# ZATCA Toolkit

> **Clear every invoice. Never stop the cash.**
> Open-source ZATCA (Saudi Fatoora Phase 2) e-invoicing toolkit — generate, validate,
> and sign compliant invoices in a few lines, on your own infrastructure.

`نوضّح كل فاتورة. لا نوقف التحصيل.`

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](./LICENSE)

An invoice that is **not cleared** is **not a valid tax invoice** — your buyer cannot
deduct the VAT, and you do not get paid. This toolkit is the open library that turns a
normalized invoice into a ZATCA-conformant, signed UBL document with a valid QR — so
clearance does not stand between you and the cash.

It is a **library that runs on your infrastructure**, not a hosted service: your invoice
data never leaves your side, and you stay clear of the accreditation and data-residency
obligations that come with operating clearance yourself.

---

## What is in the box

| Package | What it does | Status |
|---|---|---|
| `packages/core` (Go) | ZATCA-UBL 2.1 generation, EN16931 + Peppol + ZATCA validation, **secp256k1** CSID signing, ICV/PIH anti-tamper chain, QR (TLV) | engine green; two-way official conformance **15/15** |
| `packages/sdk` (`@zatca/sdk`, TS) | `invoice → validate → generate → sign → clear` client; UBL + CII + Factur-X; engine via WASM for browser + Node | built; `node --test` **24/24** green |
| `apps/audit` | Free client-side invoice auditor: paste XML → pass/fail + rule list (ar/en) + shareable result | built (browser-verified) |
| `apps/web` | Marketing LP + developer docs (ar/en RTL, "Clearance Statement" identity) | built |
| `tools/pdfa3` | PDF/A-3 + Factur-X generation (embed UBL/CII into a PDF); exposed via the SDK | structural PDF/A-3 — **not yet veraPDF-certified** |
| `tools/conformance-adjunct` | **Opt-in**: runs the **official** EN16931 (211) + Peppol BIS (~130) Schematron locally via Saxon-HE | built (opt-in, needs Java) |

## Quickstart (engine)

```go
package main

import (
    "fmt"
    "github.com/greymoth-jp/zatca-toolkit/core/internal/adapters/ksa"
)

func main() {
    signer, _ := ksa.NewSecp256k1Signer()                 // ZATCA curve (KMS-backed in prod)
    res, _ := ksa.ProcessStandard(ksa.Input{
        Doc: myNormalizedInvoice, UUID: "…", IssueTime: "10:30:00",
    }, signer, ksa.NewMockClient())                       // swap for your onboarded client
    fmt.Println(res.Status, res.QR)                       // cleared, <base64 QR>
}
```

### Quickstart (JS / TS — `@zatca/sdk`)

The same engine runs in Node and the browser via WebAssembly. This snippet is **CI-tested**
(`packages/sdk/examples/quickstart.mjs`), so it never goes stale:

```js
import { Zatca } from "@zatca/sdk";

const z = new Zatca({ mode: "sandbox" });
const { ubl } = await z.generate(myNormalizedInvoice);      // normalized → UBL 2.1
const r = await z.validateXML(ubl, "zatca-ksa");            // deterministic ZATCA/EN16931 check
console.log(r.report.valid ? "would clear" : r.report.errors); // each error has rule_id + ar/en + fix
```

## Why trust it (verified, not claimed)

- **Two tiers of validation**: (1) a **zero-dependency, browser-capable** engine with a curated,
  tested rule set (runs in Node and the browser via WASM, no server, no Java); and (2) an
  **opt-in adjunct** ([`tools/conformance-adjunct`](./tools/conformance-adjunct/)) that runs the
  **official** EN16931 (211 rules) and Peppol BIS (~130 rules) Schematron locally via Saxon-HE —
  the same authoritative rules accredited validators use, on your own machine. See
  [`_docs/SAXON_ADJUNCT.md`](./_docs/SAXON_ADJUNCT.md).
- **Official-fixture conformance**: the zero-dep engine is validated against the official EN16931
  example invoices — **15/15 parse and validate with zero false-positives**, and it **rejects**
  broken invoices with the right rule (two-way conformance suite, `packages/core/internal/conformance`).
- **Errors tell you the fix**: every finding carries `rule_id` + English/Arabic message +
  how-to-fix — not a cryptic code.
- **100% client-side**: the audit/SDK validate in-process (WASM); your invoice is never uploaded.
- **Green CI** on every push (engine `go test`, SDK `node --test`, WASM build, `zatca-check`) —
  **including the official-fixture conformance run**, so a rule that ever false-positives on a real
  EN16931 invoice turns the build red automatically.
- **Rule freshness, in the open**: [`RULES.md`](./RULES.md) is the honest registry of every
  business rule the engine enforces today (by profile layer) plus a changelog — rules are
  executable, tested code, not a PDF that drifts.
- These are deliberately scoped claims; see [`_docs/SELFKILL_SCORECARD.md`](./_docs/SELFKILL_SCORECARD.md)
  for an honest, competitor-by-competitor scorecard (including where we do **not** compete).

## Jurisdictions

- **Saudi Arabia (ZATCA / Fatoora Phase 2)** — primary. Standard (clearance) and simplified
  (reporting) document flows, ICV/PIH chain, QR tags 1–8 (tag 9 stamp is credential-gated).
- **UAE (PINT AE / Peppol)**, **France (Factur-X / PDP)**, and others reuse the EN16931 core;
  transmission is delegated to certified partners (see below).

## Compliance, scope, and responsibility

**This is not tax advice and not a compliance guarantee.** This toolkit prepares and signs
documents; it does **not** operate hosted clearance and is **not** a certified platform.
Live clearance/reporting requires your onboarded EGS + ZATCA Production CSID (or a certified
partner); Peppol/PDP transmission requires an accredited service provider. Final compliance,
certified integration, data residency, and retention are the **customer's responsibility**.
See [`NOTICE`](./NOTICE).

**Honest status of what is proven.** Generation (UBL / CII / Factur-X), validation (the curated
rule set, two-way conformance 15/15), the secp256k1 / XAdES signing structure, credential-free CSR
generation, and the QR / ICV / PIH chain are implemented and tested. What is **not yet proven**:
a real end-to-end clearance against ZATCA (the onboarding handshake needs a genuine Fatoora-portal
OTP — the `cmd/zatca-onboard` tool is ready to run the moment one is available), byte-exact
agreement with the official ZATCA SDK hashes (needs the SDK fixtures), and veraPDF-certified
PDF/A-3b. We say "structurally correct" / "passes our rules", never "certified" or "guaranteed to
clear". This honesty is deliberate — see [`_docs/STATUS.md`](./_docs/STATUS.md).

The toolkit deliberately avoids AGPL/EUPL/GPL/SSPL dependencies — only Apache-2.0 / MIT /
BSD / ISC / MPL-2.0 components are used.

## Why open source

Compliance rules change. A toolkit you can read, fork, and run yourself — with the rule set
as executable, tested code that fails in CI when it drifts — is more trustworthy than a black
box. The code is the product, and the moat is how fast the rules stay current.

## License

[Apache-2.0](./LICENSE).
