// Package watch applies a recorded chain feed (EventCenter blocks, eos-style
// actions, btc vouts and Omni transfers) to the store. It is the adapter seam:
// the bds listener, the ZMQ subscriber and the light-node poller all produce
// the same Event stream.
package watch

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"route/internal/store"
)

// Kind values of an Event.
const (
	KindBlock  = "block"
	KindTx     = "tx"
	KindAction = "action"
	KindVout   = "vout"
	KindOmni   = "omni"
	KindRecord = "record"
	KindRevert = "revert"
)

// Event is one line of the feed.
type Event struct {
	Kind string `json:"kind"`

	// block
	Height     int64  `json:"height"`
	Hash       string `json:"hash"`
	ParentHash string `json:"parent_hash"`
	Timestamp  int64  `json:"timestamp"`
	TxCount    int    `json:"tx_count"`

	// tx
	TxID   string `json:"txid"`
	From   string `json:"from"`
	To     string `json:"to"`
	Value  string `json:"value"`
	Status string `json:"status"`

	// action
	Index   int    `json:"index"`
	Account string `json:"account"`
	Name    string `json:"name"`
	Data    string `json:"data"`

	// vout
	N          int    `json:"n"`
	Address    string `json:"address"`
	ScriptType string `json:"script_type"`
	IsCoinbase bool   `json:"is_coinbase"`
	IsUsed     bool   `json:"is_used"`

	// omni
	PropertyID       int64  `json:"property_id"`
	Amount           string `json:"amount"`
	SendingAddress   string `json:"sending_address"`
	ReferenceAddress string `json:"reference_address"`
	Valid            bool   `json:"valid"`

	// record
	Sha      string `json:"sha"`
	ID       int64  `json:"id"`
	Entity   string `json:"entity"`
	State    string `json:"state"`
	Author   string `json:"author"`
	Verified string `json:"verified"`
}

// Stats is what one feed run produced.
type Stats struct {
	Blocks   int `json:"blocks"`
	Txs      int `json:"txs"`
	Actions  int `json:"actions"`
	Vouts    int `json:"vouts"`
	Omni     int `json:"omni"`
	USDT     int `json:"usdt"`
	Records  int `json:"records"`
	Reverted int `json:"reverted"`
	Height   int64
}

// Apply reads JSONL events and writes them to the store. `chain` is the
// default chain for events that do not carry one.
func Apply(s store.Store, chain string, r io.Reader) (Stats, error) {
	var stats Stats
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		raw := sc.Bytes()
		if len(bytesTrim(raw)) == 0 {
			continue
		}
		var ev Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			return stats, fmt.Errorf("watch: line %d: %w", line, err)
		}
		switch ev.Kind {
		case KindBlock:
			if err := s.PutBlock(store.Block{Chain: chain, Height: ev.Height, Hash: ev.Hash,
				ParentHash: ev.ParentHash, Timestamp: ev.Timestamp, TxCount: ev.TxCount}); err != nil {
				return stats, err
			}
			stats.Blocks++
			if ev.Height > stats.Height {
				stats.Height = ev.Height
			}
		case KindTx:
			if err := s.PutTx(store.Tx{Chain: chain, Height: ev.Height, TxID: ev.TxID,
				FromAddr: ev.From, ToAddr: ev.To, Value: ev.Value, Status: ev.Status}); err != nil {
				return stats, err
			}
			stats.Txs++
		case KindAction:
			if err := s.PutAction(store.Action{Chain: chain, TxID: ev.TxID, Index: ev.Index,
				Account: ev.Account, Name: ev.Name, Data: ev.Data}); err != nil {
				return stats, err
			}
			stats.Actions++
		case KindVout:
			if err := s.PutVout(store.Vout{Chain: chain, Height: ev.Height, TxID: ev.TxID, N: ev.N,
				Value: ev.Value, Address: ev.Address, ScriptType: ev.ScriptType,
				IsCoinbase: ev.IsCoinbase, IsUsed: ev.IsUsed}); err != nil {
				return stats, err
			}
			stats.Vouts++
		case KindOmni:
			if err := s.PutOmni(store.Omni{Chain: chain, TxID: ev.TxID, PropertyID: ev.PropertyID,
				Amount: ev.Amount, SendingAddress: ev.SendingAddress,
				ReferenceAddress: ev.ReferenceAddress, Valid: ev.Valid, BlockHeight: ev.Height}); err != nil {
				return stats, err
			}
			stats.Omni++
			if ev.PropertyID == 31 {
				stats.USDT++
			}
		case KindRecord:
			if err := s.PutRecord(store.Record{Sha: ev.Sha, ID: ev.ID, Entity: ev.Entity,
				State: ev.State, Author: ev.Author, EnvelopeVerified: ev.Verified}); err != nil {
				return stats, err
			}
			stats.Records++
		case KindRevert:
			if err := s.RevertFrom(chain, ev.Height); err != nil {
				return stats, err
			}
			stats.Reverted++
		default:
			return stats, fmt.Errorf("watch: line %d: unknown kind %q", line, ev.Kind)
		}
	}
	if err := sc.Err(); err != nil {
		return stats, fmt.Errorf("watch: %w", err)
	}
	if h, ok, err := s.BlockHeight(chain); err != nil {
		return stats, err
	} else if ok {
		stats.Height = h
	}
	return stats, nil
}

func bytesTrim(b []byte) []byte {
	start := 0
	for start < len(b) && (b[start] == ' ' || b[start] == '\t' || b[start] == '\r' || b[start] == '\n') {
		start++
	}
	end := len(b)
	for end > start && (b[end-1] == ' ' || b[end-1] == '\t' || b[end-1] == '\r' || b[end-1] == '\n') {
		end--
	}
	return b[start:end]
}
