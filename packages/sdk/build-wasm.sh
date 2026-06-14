#!/usr/bin/env bash
# Rebuild the WebAssembly engine and refresh the matching wasm_exec.js glue.
# The committed engine.wasm lets `npm i @zatca/sdk` work without a Go toolchain; CI rebuilds
# it to guard against drift. wasm_exec.js MUST come from the SAME Go toolchain that built the
# wasm, or the runtime contract mismatches.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
core="$here/../core"

# Pin a toolchain that ships lib/wasm/wasm_exec.js (module-cache toolchains may be trimmed).
: "${GO_WASM_TOOLCHAIN:=go1.25.11}"

echo "Building engine.wasm with $GO_WASM_TOOLCHAIN ..."
( cd "$core" && GOTOOLCHAIN="$GO_WASM_TOOLCHAIN" GOOS=js GOARCH=wasm go build -o "$here/wasm/engine.wasm" ./cmd/wasm )

goroot="$(cd "$core" && GOTOOLCHAIN="$GO_WASM_TOOLCHAIN" go env GOROOT)"
for cand in "$goroot/lib/wasm/wasm_exec.js" "$goroot/misc/wasm/wasm_exec.js"; do
  if [ -f "$cand" ]; then cp "$cand" "$here/wasm/wasm_exec.js"; echo "wasm_exec.js <- $cand"; break; fi
done

echo "Done. Artifacts in $here/wasm/"
