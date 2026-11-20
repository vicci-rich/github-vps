package exec

import (
	"errors"
	"math/big"

	"github.com/DSiSc/craft/types"
	"github.com/DSiSc/repository"
)

// ErrUnsupported is returned by every stub entry point.
var ErrUnsupported = errors.New("wasm execution is not supported in this build")

// WasmChainContext is an opaque placeholder matching the upstream type.
type WasmChainContext struct{}

// NewWasmChainContext matches the upstream constructor signature.
func NewWasmChainContext(tx *types.Transaction, header *types.Header,
	chain *repository.Repository, author types.Address) *WasmChainContext {
	return &WasmChainContext{}
}

// VM is an opaque placeholder matching the upstream type.
type VM struct{}

// NewVM matches the upstream constructor signature.
func NewVM(chainContext *WasmChainContext, state *repository.Repository) *VM {
	return &VM{}
}

// Create always fails: the stub never deploys wasm contracts.
func (self *VM) Create(caller types.Address, code []byte, gas uint64, value *big.Int,
) (ret []byte, contractAddr types.Address, leftOverGas uint64, err error) {
	return nil, types.Address{}, gas, ErrUnsupported
}

// Call always fails: the stub never executes wasm contracts.
func (self *VM) Call(caller, addr types.Address, input []byte, gas uint64, value *big.Int,
) (ret []byte, leftOverGas uint64, err error) {
	return nil, gas, ErrUnsupported
}
