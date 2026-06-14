//go:build !(js && wasm)

// This stub exists so the cmd/wasm package builds on the host (keeping `go test ./...` and
// `go vet ./...` green). The real engine bridge is in main_wasm.go and only compiles for
// GOOS=js GOARCH=wasm. Build the WASM with: GOOS=js GOARCH=wasm go build -o engine.wasm ./cmd/wasm
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "cmd/wasm is a WebAssembly target; build with GOOS=js GOARCH=wasm go build -o engine.wasm ./cmd/wasm")
	os.Exit(1)
}
