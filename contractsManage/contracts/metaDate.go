package contracts

import (
	"fmt"
	"github.com/DSiSc/contractsManage/utils"
	"github.com/DSiSc/craft/log"
	"github.com/DSiSc/craft/types"
	"github.com/DSiSc/repository"
	"math/big"
)

const (
	GetContractById  = "8ba9ac6f" // getContractById(uint256)
	ContractMetaDate = "1e7936bd"
	RegisteContract  = "507cbfd0" // registerContract(uint256,uint256,address)
	UpdateContract   = "70ea9c72" // updateContract(uint256,uint256,address)
	UpdateWhiteList  = "355acedc" // updateWhiteListAddress(uint256,address)
)

type MetaData interface {

	// get contract address by contract id
	GetContractById(types.ContractType) types.Address

	// issue a proposal to update contract address
	UpdateContractAddress(uint64, types.ContractType, types.Address) error

	// voteForContractProposal
	UpdateWhiteListAddress(uint64, types.Address) error
}

type MetaDataContract struct {
	Address types.Address
	MetaMap map[types.ContractType]types.Address
}

func NewMetaDataContract() MetaData {
	return &MetaDataContract{
		Address: utils.HexToAddress(types.MetaDataContractAddress),
		MetaMap: make(map[types.ContractType]types.Address),
	}
}

func (instance *MetaDataContract) GetContractById(contractType types.ContractType) types.Address {
	chain, err := repository.NewLatestStateRepository()
	if err != nil {
		panic(fmt.Errorf("failed to create init-state block chain, as: %v", err))
	}
	block := chain.GetCurrentBlock()
	nonce := utils.GetAccountNonce(block, types.Address{})
	callCode := fmt.Sprintf("%s%s", GetContractById, utils.EncodeUint256(int(contractType)))
	result, err := utils.EvmCall(nonce+1, &instance.Address, big.NewInt(0),
		uint64(0), big.NewInt(0), utils.Hex2Bytes(callCode), nil)
	if nil != err {
		log.Error("evm called to get contract by id %d error with %v.", contractType, err)
		return utils.HexToAddress(defaultAddressFor(contractType))
	}
	address := utils.BytesToAddress(result[12:])
	return address
}

// callData builds selector | proposalId | second word | address for the two
// governance-gated registry calls.
func callData(selector string, proposalID uint64, second uint64, addr types.Address) []byte {
	code := fmt.Sprintf("%s%s%s%s", selector,
		utils.EncodeUint256(int(proposalID)),
		utils.EncodeUint256(int(second)),
		utils.EncodingAddress(utils.Bytes2Hex(addr[:])))
	return utils.Hex2Bytes(code)
}

// UpdateContractAddress asks the registry to point a contract id at a new
// implementation. The whitelist proposal must already be approved on-chain,
// which is enforced by MetaData.updateContract itself.
func (instance *MetaDataContract) UpdateContractAddress(proposalID uint64, contractType types.ContractType, addr types.Address) error {
	chain, err := repository.NewLatestStateRepository()
	if err != nil {
		return fmt.Errorf("update contract %d: %w", contractType, err)
	}
	block := chain.GetCurrentBlock()
	nonce := utils.GetAccountNonce(block, types.Address{})
	_, err = utils.EvmCall(nonce+1, &instance.Address, big.NewInt(0),
		uint64(0), big.NewInt(0), callData(UpdateContract, proposalID, uint64(contractType), addr), nil)
	if err != nil {
		return fmt.Errorf("update contract %d: %w", contractType, err)
	}
	return nil
}

// UpdateWhiteListAddress repoints the whitelist contract after the proposal
// has been approved.
func (instance *MetaDataContract) UpdateWhiteListAddress(proposalID uint64, addr types.Address) error {
	chain, err := repository.NewLatestStateRepository()
	if err != nil {
		return fmt.Errorf("update whitelist: %w", err)
	}
	block := chain.GetCurrentBlock()
	nonce := utils.GetAccountNonce(block, types.Address{})
	code := fmt.Sprintf("%s%s%s", UpdateWhiteList,
		utils.EncodeUint256(int(proposalID)),
		utils.EncodingAddress(utils.Bytes2Hex(addr[:])))
	_, err = utils.EvmCall(nonce+1, &instance.Address, big.NewInt(0),
		uint64(0), big.NewInt(0), utils.Hex2Bytes(code), nil)
	if err != nil {
		return fmt.Errorf("update whitelist: %w", err)
	}
	return nil
}

// defaultAddressFor returns the genesis address for a contract type without
// panicking on CrossFundsPool (id 5), which the old helper did.
func defaultAddressFor(contractType types.ContractType) string {
	switch contractType {
	case types.MetaDataContractType:
		return types.MetaDataContractAddress
	case types.WhiteListContractType:
		return types.WhiteListContractTypeDefaultAddress
	case types.VoteContractType:
		return types.VotingContractDefaultAddress
	case types.JustitiaRightContractType:
		return types.JustiitaContractDefaultAddress
	case types.CrossFundsPoolContractType:
		return types.CrossFundsPoolDefaultAddress
	}
	return types.MetaDataContractAddress
}
