// Package store defines the single authoritative row model (see schema.sql)
// and an in-memory implementation used by the watcher, the tests and the
// fixture-driven `route watch` command. Postgres implements the same
// interface.
package store

import (
	_ "embed"
	"fmt"
	"sort"
	"sync"
)

// Block is one row of blocks.
type Block struct {
	Chain      string
	Height     int64
	Hash       string
	ParentHash string
	Timestamp  int64
	TxCount    int
}

// Tx is one row of transactions.
type Tx struct {
	Chain    string
	Height   int64
	TxID     string
	FromAddr string
	ToAddr   string
	Value    string // decimal string, numeric in SQL
	Status   string
}

// Action is one row of actions (the eos splitter shape).
type Action struct {
	Chain   string
	TxID    string
	Index   int
	Account string
	Name    string
	Data    string
}

// Vout is one row of vouts (the btc splitter shape).
type Vout struct {
	Chain      string
	Height     int64
	TxID       string
	N          int
	Value      string
	Address    string
	ScriptType string
	IsCoinbase bool
	IsUsed     bool
}

// Omni is one row of omni_transfers; PropertyID 31 is USDT.
type Omni struct {
	Chain            string
	TxID             string
	PropertyID       int64
	Amount           string
	SendingAddress   string
	ReferenceAddress string
	Valid            bool
	BlockHeight      int64
}

// Record is one row of route_records: an envelope bound to a chain record.
type Record struct {
	Sha              string
	ID               int64
	Entity           string
	State            string
	Author           string
	EnvelopeVerified string
}

// Store is the persistence contract shared by the watcher and the services.
type Store interface {
	PutBlock(Block) error
	PutTx(Tx) error
	PutAction(Action) error
	PutVout(Vout) error
	PutOmni(Omni) error
	PutRecord(Record) error
	BlockHeight(chain string) (int64, bool, error)
	RevertFrom(chain string, height int64) error
	Count(table string) int
}

// Mem is an in-memory Store. It is the reference implementation for the row
// semantics, including revert-by-height.
type Mem struct {
	mu     sync.RWMutex
	blocks map[string]map[int64]Block
	txs    map[string]map[string]Tx
	acts   map[string]map[string]Action
	vouts  map[string]map[string]Vout
	omni   map[string]map[string]Omni
	recs   map[string]Record
}

// NewMem returns an empty in-memory store.
func NewMem() *Mem {
	return &Mem{
		blocks: map[string]map[int64]Block{},
		txs:    map[string]map[string]Tx{},
		acts:   map[string]map[string]Action{},
		vouts:  map[string]map[string]Vout{},
		omni:   map[string]map[string]Omni{},
		recs:   map[string]Record{},
	}
}

func chainMap[V any](m map[string]map[string]V, chain string) map[string]V {
	if _, ok := m[chain]; !ok {
		m[chain] = map[string]V{}
	}
	return m[chain]
}

// PutBlock inserts or replaces a block row.
func (s *Mem) PutBlock(b Block) error {
	if b.Chain == "" || b.Hash == "" {
		return fmt.Errorf("store: block needs chain and hash")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.blocks[b.Chain]; !ok {
		s.blocks[b.Chain] = map[int64]Block{}
	}
	s.blocks[b.Chain][b.Height] = b
	return nil
}

// PutTx inserts or replaces a transaction row.
func (s *Mem) PutTx(t Tx) error {
	if t.Chain == "" || t.TxID == "" {
		return fmt.Errorf("store: tx needs chain and txid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	chainMap[Tx](s.txs, t.Chain)[t.TxID] = t
	return nil
}

// PutAction inserts or replaces an action row keyed by (txid, idx).
func (s *Mem) PutAction(a Action) error {
	if a.Chain == "" || a.TxID == "" {
		return fmt.Errorf("store: action needs chain and txid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	chainMap[Action](s.acts, a.Chain)[fmt.Sprintf("%s#%d", a.TxID, a.Index)] = a
	return nil
}

// PutVout inserts or replaces a vout row keyed by (txid, n).
func (s *Mem) PutVout(v Vout) error {
	if v.Chain == "" || v.TxID == "" {
		return fmt.Errorf("store: vout needs chain and txid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	chainMap[Vout](s.vouts, v.Chain)[fmt.Sprintf("%s#%d", v.TxID, v.N)] = v
	return nil
}

// PutOmni inserts or replaces an Omni transfer row.
func (s *Mem) PutOmni(o Omni) error {
	if o.Chain == "" || o.TxID == "" {
		return fmt.Errorf("store: omni needs chain and txid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	chainMap[Omni](s.omni, o.Chain)[o.TxID] = o
	return nil
}

// PutRecord inserts or replaces an envelope-backed record.
func (s *Mem) PutRecord(r Record) error {
	if r.Sha == "" || r.Entity == "" {
		return fmt.Errorf("store: record needs sha and entity")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs[r.Sha] = r
	return nil
}

// BlockHeight returns the highest block height for a chain.
func (s *Mem) BlockHeight(chain string) (int64, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	heights, ok := s.blocks[chain]
	if !ok || len(heights) == 0 {
		return 0, false, nil
	}
	var max int64
	for h := range heights {
		if h > max {
			max = h
		}
	}
	return max, true, nil
}

// RevertFrom deletes every row at height >= h for the chain, in one pass.
func (s *Mem) RevertFrom(chain string, h int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for height := range s.blocks[chain] {
		if height >= h {
			delete(s.blocks[chain], height)
		}
	}
	for id, tx := range s.txs[chain] {
		if tx.Height >= h {
			delete(s.txs[chain], id)
		}
	}
	for id, a := range s.acts[chain] {
		if aTxHeight(s.txs[chain], a) >= h {
			delete(s.acts[chain], id)
		}
	}
	for id, v := range s.vouts[chain] {
		if v.Height >= h {
			delete(s.vouts[chain], id)
		}
	}
	for id, o := range s.omni[chain] {
		if o.BlockHeight >= h {
			delete(s.omni[chain], id)
		}
	}
	return nil
}

func aTxHeight(txs map[string]Tx, a Action) int64 {
	if t, ok := txs[a.TxID]; ok {
		return t.Height
	}
	return 0
}

// Count returns the number of rows in a table (blocks, transactions, actions,
// vouts, omni_transfers, route_records) across all chains.
func (s *Mem) Count(table string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	switch table {
	case "blocks":
		n := 0
		for _, m := range s.blocks {
			n += len(m)
		}
		return n
	case "transactions":
		n := 0
		for _, m := range s.txs {
			n += len(m)
		}
		return n
	case "actions":
		n := 0
		for _, m := range s.acts {
			n += len(m)
		}
		return n
	case "vouts":
		n := 0
		for _, m := range s.vouts {
			n += len(m)
		}
		return n
	case "omni_transfers":
		n := 0
		for _, m := range s.omni {
			n += len(m)
		}
		return n
	case "route_records":
		return len(s.recs)
	}
	return 0
}

// Tables lists the table names the store knows, in schema order.
func Tables() []string {
	return []string{"blocks", "transactions", "actions", "vouts", "omni_transfers", "route_records"}
}

// OrderedHeights returns the block heights of a chain in ascending order.
func (s *Mem) OrderedHeights(chain string) []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	heights := make([]int64, 0, len(s.blocks[chain]))
	for h := range s.blocks[chain] {
		heights = append(heights, h)
	}
	sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })
	return heights
}

//go:embed schema.sql
var schemaSQL string

// Schema returns the Postgres DDL for the store.
func Schema() string { return schemaSQL }
