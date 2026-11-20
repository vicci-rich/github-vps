package exec

import (
	"errors"
	"math/big"
	"testing"

	"github.com/DSiSc/craft/types"
	"github.com/DSiSc/repository"
)

func TestStubRejectsEveryCall(t *testing.T) {
	ctx := NewWasmChainContext(&types.Transaction{}, &types.Header{}, &repository.Repository{}, types.Address{})
	vm := NewVM(ctx, nil)
	if vm == nil {
		t.Fatal("NewVM returned nil")
	}

	ret, addr, gas, err := vm.Create(types.Address{}, []byte{0x00}, 1000, big.NewInt(0))
	if !errors.Is(err, ErrUnsupported) || ret != nil || addr != (types.Address{}) || gas != 1000 {
		t.Fatalf("Create = (%x,%x,%d,%v)", ret, addr, gas, err)
	}

	ret, gas, err = vm.Call(types.Address{}, types.Address{}, nil, 500, big.NewInt(0))
	if !errors.Is(err, ErrUnsupported) || ret != nil || gas != 500 {
		t.Fatalf("Call = (%x,%d,%v)", ret, gas, err)
	}
}
