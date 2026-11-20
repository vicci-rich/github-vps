// Package chaininfo is the second application caller: a small Go JSON-RPC
// client for the Justitia gateway. It is the same surface astraia exposes from
// its console (block number, balance, nonce); astraia itself is a GOPATH-era
// module without a go.mod, so the call path is implemented here rather than
// imported.
package chaininfo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// Client talks to the JSON-RPC gateway.
type Client struct {
	RPC  string
	HTTP *http.Client
}

// New returns a client with a bounded timeout.
func New(rpc string) *Client {
	return &Client{RPC: rpc, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

type response struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) call(method string, params []any) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params, "id": 1})
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Post(c.RPC, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out response
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%s: %v: %s", method, err, raw)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("%s: %s", method, out.Error.Message)
	}
	return out.Result, nil
}

func hexBig(raw json.RawMessage) (*big.Int, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	n := new(big.Int)
	if _, ok := n.SetString(strings.TrimPrefix(s, "0x"), 16); !ok {
		return nil, fmt.Errorf("bad hex quantity %q", s)
	}
	return n, nil
}

// BlockNumber returns the latest height.
func (c *Client) BlockNumber() (uint64, error) {
	raw, err := c.call("eth_blockNumber", []any{})
	if err != nil {
		return 0, err
	}
	n, err := hexBig(raw)
	if err != nil {
		return 0, err
	}
	return n.Uint64(), nil
}

// Balance returns the account balance in wei.
func (c *Client) Balance(address string) (*big.Int, error) {
	raw, err := c.call("eth_getBalance", []any{address, "latest"})
	if err != nil {
		return nil, err
	}
	return hexBig(raw)
}

// Nonce returns the account nonce.
func (c *Client) Nonce(address string) (uint64, error) {
	raw, err := c.call("eth_getTransactionCount", []any{address, "latest"})
	if err != nil {
		return 0, err
	}
	n, err := hexBig(raw)
	if err != nil {
		return 0, err
	}
	return n.Uint64(), nil
}

// Snapshot is what the CLI prints.
type Snapshot struct {
	RPC         string `json:"rpc"`
	Address     string `json:"address"`
	BlockNumber uint64 `json:"block_number"`
	Balance     string `json:"balance_wei"`
	Nonce       uint64 `json:"nonce"`
}

// Read collects the snapshot.
func (c *Client) Read(address string) (*Snapshot, error) {
	height, err := c.BlockNumber()
	if err != nil {
		return nil, err
	}
	s := &Snapshot{RPC: c.RPC, Address: address, BlockNumber: height}
	if address != "" {
		bal, err := c.Balance(address)
		if err != nil {
			return nil, err
		}
		s.Balance = bal.String()
		nonce, err := c.Nonce(address)
		if err != nil {
			return nil, err
		}
		s.Nonce = nonce
	}
	return s, nil
}
