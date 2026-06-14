# zatca-check — CI Action

Fail your build if an invoice would not clear ZATCA. Deterministic, offline, runs on the
same engine as the SDK and the free audit. **Not tax advice.**

```yaml
- uses: greymoth-jp/zatca-toolkit/ci-action@einvoice-platform
  with:
    files: 'invoices/*.xml'        # space-separated files or globs
    structural: 'false'            # true also checks UUID/ICV/PIH/QR/signature (submitted invoices)
```

Exit code is non-zero (build fails) if any file has a fatal finding. Each finding prints its
rule id, message, and a fix hint. Backed by `packages/core/cmd/zatca-check` (also runnable
directly: `go run ./cmd/zatca-check [--structural] file.xml ...`).
