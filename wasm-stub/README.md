# wasm-stub

Build-only stand-in for `github.com/DSiSc/wasm`.

Upstream (workspace/wasm) cannot compile with the Go toolchain in this
Codespace: `exec/internal/compile/backend_amd64.go` uses `obj.Prog.Pcond`,
removed from the assembler API that `golang-asm v0.15.1` exposes. The demo
chain executes EVM contracts (Justitia CRUD, `source.sol`), so this module
reports every code blob as non-wasm and the validator routes it to the EVM.

Wired in `justitia-1/go.mod`:

```
replace github.com/DSiSc/wasm => ../wasm-stub
```

Executing real wasm contracts would require repairing the upstream wagon fork
against the current Go assembler API; that is out of scope here and nothing in
the CRUD/merge path depends on it.
