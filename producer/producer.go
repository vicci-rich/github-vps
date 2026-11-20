package producer

import (
	"bytes"
	"fmt"
	"github.com/DSiSc/craft/log"
	"github.com/DSiSc/craft/types"
	"github.com/DSiSc/producer/common"
	"github.com/DSiSc/producer/config"
	"github.com/DSiSc/producer/tools"
	"github.com/DSiSc/repository"
	"github.com/DSiSc/txpool"
	"github.com/DSiSc/validator/tools/account"
	"github.com/DSiSc/validator/tools/signature"
	"github.com/DSiSc/validator/worker"
	"time"
)

type Producer struct {
	txpool txpool.TxsPool
	time   uint64
	// TODO: we support many workers to promote verification speed  in the future
	workers          *worker.Worker
	account          account.Account
	enableSignVerify bool
	chainId          uint64
	receipts         types.Receipts
	// CommitState makes MakeBlock persist the state trie. The node sets it;
	// the upstream tests keep the legacy in-memory behaviour.
	CommitState bool
}

func NewProducer(pool txpool.TxsPool, account account.Account, producerConfig config.ProducerConfig) *Producer {
	return &Producer{
		txpool:           pool,
		account:          account,
		enableSignVerify: producerConfig.EnableSignatureVerify,
		chainId:          producerConfig.ChainId,
	}
}

func (self *Producer) MakeBlock() (*types.Block, error) {
	// Get latest block store and state store
	blockStore, ok := repository.NewLatestStateRepository()
	if nil != ok {
		log.Error("Get NewLatestStateRepository failed.")
		return nil, fmt.Errorf("get NewLatestStateRepository failed")
	}
	// make block
	block, err := self.assembleBlock(blockStore)
	if nil != err {
		log.Error("Assemble block failed.")
		return nil, fmt.Errorf("assemble block failed")
	}
	// verify block
	err = self.verifyBlock(block, blockStore)
	if nil != err {
		log.Error("The block verified failed.")
		return nil, err
	}
	if self.workers != nil {
		self.receipts = self.workers.GetReceipts()
	}
	// Commit the state trie before sealing the header: without this the state
	// root in the block points at nodes that were never written, and the next
	// transaction fails with "missing trie node". Mock repositories used by
	// tests have no trie database; there we keep the intermediate root.
	stateRoot := blockStore.IntermediateRoot(false)
	if self.CommitState {
		if committed, ok := commitState(blockStore); ok {
			stateRoot = committed
		}
	}
	block.Header.MixDigest = common.HeaderDigest(block.Header)
	// set state root
	block.Header.StateRoot = stateRoot
	// sign
	err = self.signBlock(block)
	if nil != err {
		log.Error("Sign block failed.")
		return nil, fmt.Errorf("signature error: %v", err)
	}
	log.Info("Block %d make success", block.Header.Height)
	return block, nil
}

// GetReceipts returns the receipts produced while verifying the last block so
// the node can persist them together with the block.
func (self *Producer) GetReceipts() types.Receipts {
	return self.receipts
}

func (self *Producer) assembleBlock(blockStore *repository.Repository) (*types.Block, error) {
	txs := self.txpool.GetTxs()
	txHash := make([]types.Hash, 0, len(txs))
	for _, t := range txs {
		txHash = append(txHash, common.TxHash(t))
	}
	txRoot := tools.ComputeMerkleRoot(txHash)
	currentBlock := blockStore.GetCurrentBlock()
	block := &types.Block{
		Header: &types.Header{
			ChainID:       self.chainId,
			TxRoot:        txRoot,
			CoinBase:      self.account.Address,
			PrevBlockHash: currentBlock.HeaderHash,
			Timestamp:     uint64(time.Now().Unix()),
			Height:        currentBlock.Header.Height + 1,
		},
		Transactions: txs,
	}
	log.Info("Block %d assemble success with %d txs.", block.Header.Height, len(txs))
	return block, nil
}

// commitState commits the trie and reports whether it succeeded. A mock
// repository panics instead of returning an error, so the panic is contained
// here rather than taking the producer down in tests.
func commitState(repo *repository.Repository) (root types.Hash, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			log.Warn("Commit state panicked, keeping the intermediate root: %v", r)
			ok = false
		}
	}()
	root, err := repo.Commit(false)
	if err != nil {
		log.Warn("Commit state failed, keeping the intermediate root: %v", err)
		return types.Hash{}, false
	}
	return root, true
}

func (self *Producer) verifyBlock(block *types.Block, blockStore *repository.Repository) error {
	// we support num of works to verify the block
	work := worker.NewWorker(blockStore, block, self.enableSignVerify)
	self.workers = work
	// verify the block
	err := work.VerifyBlock()
	if err != nil {
		log.Error("The block %d verified failed with err %v.", block.Header.Height, err)
		return err
	}
	log.Info("The block %d verified success.", block.Header.Height)
	return nil
}

func (self *Producer) signBlock(block *types.Block) error {
	sign, err := signature.Sign(&self.account, block.Header.MixDigest[:])
	if nil != err {
		log.Error("signature error.")
		return err
	}

	notSigned := true
	for _, value := range block.Header.SigData {
		if bytes.Equal(value, sign) {
			notSigned = false
			log.Warn("Duplicate sign")
		}
	}
	if notSigned {
		block.Header.SigData = append(block.Header.SigData, sign)
	}
	return nil
}
