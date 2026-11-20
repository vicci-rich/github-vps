// Package omni parses the Omni Layer payloads used by the btc splitter:
// `omni_gettransaction`, `omni_listblocktransactions`, and the vin/vout view
// of a Bitcoin block. Property 31 is USDT on Omni.
package omni

import (
	"encoding/json"
	"fmt"
	"strings"
)

// USDTPropertyID is the Omni property of Tether USD.
const USDTPropertyID int64 = 31

// Amount accepts both a JSON number and a JSON string, because Omni returns
// amounts as quoted decimals while Bitcoin returns numbers.
type Amount string

// UnmarshalJSON implements json.Unmarshaler.
func (a *Amount) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" {
		s = ""
	}
	*a = Amount(s)
	return nil
}

// Transfer is one omni_gettransaction result.
type Transfer struct {
	TxID             string `json:"txid"`
	SendingAddress   string `json:"sendingaddress"`
	ReferenceAddress string `json:"referenceaddress"`
	PropertyID       int64  `json:"propertyid"`
	Amount           Amount `json:"amount"`
	Valid            bool   `json:"valid"`
	Block            int64  `json:"block"`
	BlockHash        string `json:"blockhash"`
	PositionInBlock  int    `json:"positioninblock"`
	Type             string `json:"type"`
	TypeInt          int    `json:"type_int"`
}

// ParseTransfer decodes one omni_gettransaction result.
func ParseTransfer(raw []byte) (*Transfer, error) {
	var t Transfer
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("omni: %w", err)
	}
	if t.TxID == "" {
		return nil, fmt.Errorf("omni: transfer without txid")
	}
	return &t, nil
}

// ParseTransferList decodes omni_listblocktransactions, which is an array of
// txids.
func ParseTransferList(raw []byte) ([]string, error) {
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, fmt.Errorf("omni: %w", err)
	}
	return ids, nil
}

// IsUSDT reports whether the transfer moves property 31.
func (t *Transfer) IsUSDT() bool { return t.PropertyID == USDTPropertyID }

// Vout is one output of a Bitcoin transaction.
type Vout struct {
	TxID       string
	N          int
	Value      Amount
	Address    string
	ScriptType string
}

type rawBlock struct {
	Tx []rawTx `json:"tx"`
}

type rawTx struct {
	TxID string    `json:"txid"`
	Vout []rawVout `json:"vout"`
}

type rawVout struct {
	Value        Amount `json:"value"`
	N            int    `json:"n"`
	ScriptPubKey struct {
		Addresses []string `json:"addresses"`
		Type      string   `json:"type"`
	} `json:"scriptPubKey"`
}

// ParseVouts extracts every transaction output from a getblock-style payload.
func ParseVouts(raw []byte) ([]Vout, error) {
	var block rawBlock
	if err := json.Unmarshal(raw, &block); err != nil {
		return nil, fmt.Errorf("bitcoin: %w", err)
	}
	out := make([]Vout, 0)
	for _, tx := range block.Tx {
		for _, v := range tx.Vout {
			address := ""
			if len(v.ScriptPubKey.Addresses) > 0 {
				address = v.ScriptPubKey.Addresses[0]
			}
			out = append(out, Vout{
				TxID:       tx.TxID,
				N:          v.N,
				Value:      v.Value,
				Address:    address,
				ScriptType: v.ScriptPubKey.Type,
			})
		}
	}
	return out, nil
}
