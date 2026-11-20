package contracts

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/DSiSc/craft/types"
)

// The selectors are pinned to Keccak-256 of the signatures; a drift here would
// silently call the wrong registry function on a live chain.
func TestRegistrySelectors(t *testing.T) {
	want := map[string]string{
		"getContractById(uint256)":                  GetContractById,
		"updateContract(uint256,uint256,address)":   UpdateContract,
		"updateWhiteListAddress(uint256,address)":   UpdateWhiteList,
		"registerContract(uint256,uint256,address)": RegisteContract,
	}
	for sig, got := range want {
		if len(got) != 8 {
			t.Errorf("%s: selector %q is not 4 bytes", sig, got)
		}
		if _, err := hex.DecodeString(got); err != nil {
			t.Errorf("%s: selector %q is not hex", sig, got)
		}
	}
	if GetContractById != "8ba9ac6f" {
		t.Errorf("getContractById selector drifted: %s", GetContractById)
	}
}

func TestCallDataLayout(t *testing.T) {
	addr := types.Address{0x47, 0xc5, 0xe4, 0x08, 0x90, 0xbc, 0xe4, 0xa4, 0x73, 0xa4,
		0x9d, 0x75, 0x01, 0x80, 0x8b, 0x96, 0x33, 0xf2, 0x97, 0x82}
	data := callData(UpdateContract, 7, 5, addr)
	if len(data) != 4+32*3 {
		t.Fatalf("calldata length = %d, want %d", len(data), 4+32*3)
	}
	if got := hex.EncodeToString(data[:4]); got != UpdateContract {
		t.Fatalf("selector = %s, want %s", got, UpdateContract)
	}
	body := hex.EncodeToString(data[4:])
	if !strings.HasSuffix(body, hex.EncodeToString(addr[:])) {
		t.Fatal("address word is not right-aligned")
	}
	if !strings.HasPrefix(body, strings.Repeat("0", 63)+"7") {
		t.Fatal("proposal id is not the first word")
	}
	if !strings.Contains(body, strings.Repeat("0", 63)+"5") {
		t.Fatal("contract type is not the second word")
	}
}

func TestDefaultAddressCoversCrossFundsPool(t *testing.T) {
	if got := defaultAddressFor(types.CrossFundsPoolContractType); got != types.CrossFundsPoolDefaultAddress {
		t.Fatalf("CrossFundsPool default = %s, want %s", got, types.CrossFundsPoolDefaultAddress)
	}
	if got := defaultAddressFor(types.MetaDataContractType); got != types.MetaDataContractAddress {
		t.Fatalf("MetaData default = %s", got)
	}
	if got := defaultAddressFor(types.ContractType(99)); got == "" {
		t.Fatal("unknown type must still return an address, not an empty string")
	}
}
