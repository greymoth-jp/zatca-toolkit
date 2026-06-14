#!/usr/bin/env bash
# Stage the WASM engine for the static audit app. The audit is a pure static site (no build
# step, no framework); it only needs engine.wasm + wasm_exec.js next to index.html. We copy
# them from packages/sdk/wasm (the single source of truth) rather than committing a second
# 3.8MB copy — engine/ is gitignored. Run before serving/deploying:  bash build.sh
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
src="$here/../../packages/sdk/wasm"
mkdir -p "$here/engine"
cp "$src/engine.wasm" "$here/engine/engine.wasm"
cp "$src/wasm_exec.js" "$here/engine/wasm_exec.js"
echo "staged engine -> $here/engine/"
