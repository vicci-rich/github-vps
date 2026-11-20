// Package wasm is a build-only stub for github.com/DSiSc/wasm.
//
// The upstream wagon fork (workspace/wasm) does not compile with modern Go:
// exec/internal/compile/backend_amd64.go dereferences obj.Prog.Pcond, a field
// that no longer exists in the assembler API that golang-asm v0.15.1 exposes.
// This demo chain executes EVM contracts only (Justitia CRUD, source.sol), so
// the wasm path is not used: every candidate is reported as non-wasm and the
// validator routes it to the EVM. Executing real wasm contracts would need the
// upstream fork repaired against the current Go toolchain.
package wasm

// IsValidWasmCode always reports false under the stub.
func IsValidWasmCode(code []byte) bool { return false }
