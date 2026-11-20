package wasm

import "testing"

// The stub must never report arbitrary bytes as a wasm module, otherwise the
// validator would route an EVM contract to the disabled wasm path.
func TestIsValidWasmCodeAlwaysFalse(t *testing.T) {
	cases := [][]byte{
		nil,
		{},
		[]byte("hello"),
		{0x00, 0x61, 0x73, 0x6d}, // the wasm magic, still refused by the stub
	}
	for _, c := range cases {
		if IsValidWasmCode(c) {
			t.Fatalf("IsValidWasmCode(%x) = true, want false", c)
		}
	}
}
