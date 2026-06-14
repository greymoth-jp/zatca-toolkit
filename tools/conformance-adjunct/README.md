# Conformance adjunct (opt-in, official EN16931 Schematron)

**This is not part of the core library, the `@zatca/sdk`, or the WASM/browser engine.** It is an
optional, opt-in tool for users who want **certified-grade parity** by running the *official*
EN16931 rules (~200+) instead of the curated subset the zero-dependency engine ships.

It drives **Saxon-HE** over the **official preprocessed EN16931 Schematron XSLT** and reads the
resulting SVRL (pass/fail per rule).

## Why it is separate / opt-in
- It requires a **Java runtime** (JRE 8+). The core engine deliberately has **zero** runtime
  dependencies and runs in the browser; this adjunct does not.
- It downloads two artifacts **at runtime** into a cache and never vendors them:
  - **Saxon-HE 10.9** — Mozilla Public License 2.0 (permissive, policy-compatible).
  - **Official EN16931 UBL validation XSLT** — **EUPL-1.2**. We do **not** commit or redistribute
    it; it is fetched at runtime, exactly like the conformance fixtures.

## Usage
```sh
# EN16931 (default)
node tools/conformance-adjunct/run.mjs path/to/invoice.xml
# Peppol BIS (compiles the official Schematron to XSLT on first run, then caches it)
node tools/conformance-adjunct/run.mjs path/to/invoice.xml --profile peppol
# custom cache location:
ZATCA_CONFORMANCE_CACHE=/var/cache/zatca node tools/conformance-adjunct/run.mjs invoice.xml
```
Output is JSON: `{ profile, firedRules, failedAsserts, findings:[{ flag, location, message }] }`.
Exit codes: `0` no failed assertions · `1` failed assertions · `2` BLOCKED (no Java) · `3` error.

## Verified
- **EN16931**: all **15 official example invoices pass** (0 failed-asserts each) — agrees with the
  in-core two-way conformance suite; **211 rules/doc**. A mutated invoice (corrupted payable
  total) is rejected with the official **[BR-CO-16]**.
- **Peppol BIS**: the official `base-example.xml` **passes** (0 failed-asserts, 39 rules fired);
  a non-Peppol invoice fails on genuine Peppol rules (ProfileID format, buyer reference,
  electronic addresses, line-net calc, NL-R-003); **~130 rules/doc**.
- vs the curated zero-dependency core (~34 EN16931 + 8 Peppol rules).

## Scope / legal
**Not tax advice and not a compliance guarantee.** This runs published rule sets; live
clearance/reporting still requires your onboarded EGS + ZATCA credentials or a certified partner.
See [`_docs/SAXON_ADJUNCT.md`](../../_docs/SAXON_ADJUNCT.md) for architecture and the phased plan.
