# Clearance Check — free ZATCA invoice auditor

A fully client-side audit tool: paste a ZATCA / UBL 2.1 invoice, get an instant pass/fail
against EN16931 + ZATCA rules. The invoice **never leaves the browser** — validation runs in
the Go WASM engine locally (also the data-residency posture: no upload, no server).

- **Fear → relief**: a clear ✕ NOT CLEARED / ✓ CLEARED verdict, rule-by-rule findings
  (rule_id + path + EN/AR explanation), and a shareable result image.
- **Arabic-first + RTL**, English toggle.
- **Not tax advice**, not a compliance guarantee (see footer / `NOTICE`).

Pure static site — no framework, no build step beyond staging the engine.

## Run locally

```bash
bash build.sh        # copies engine.wasm + wasm_exec.js from packages/sdk/wasm into engine/
node serve.mjs       # http://localhost:8799   (any static server works; .wasm needs application/wasm)
```

## Deploy

Static host (Vercel/Netlify/Cloudflare Pages/S3). Build command: `bash build.sh`, output dir:
this folder. Ensure `.wasm` is served as `application/wasm`.

## Files

- `index.html` / `styles.css` / `app.js` — the app ("The Clearance Statement": security-document
  brutalism on bone paper, Arabic-first, girih seal, ink CLEARED/REJECTED stamp).
- `seal.svg` — the girih (8-fold khatim) official seal motif.
- `samples/{good,bad}.xml` — demo invoices, generated from the real engine
  (`GEN_SAMPLES=1 go test ./internal/convert/ -run TestGenerateAuditSamples`).
- `engine/` — staged WASM (gitignored; produced by `build.sh`).
