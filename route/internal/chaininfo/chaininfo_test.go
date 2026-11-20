package chaininfo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func fakeGateway(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var result any
		switch req.Method {
		case "eth_blockNumber":
			result = "0x27"
		case "eth_getBalance":
			result = "0x7fffffffffffffff"
		case "eth_getTransactionCount":
			result = "0x2"
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"code": -32601, "message": "Method not found"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
}

func TestRead(t *testing.T) {
	srv := fakeGateway(t)
	defer srv.Close()

	s, err := New(srv.URL).Read("0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b")
	if err != nil {
		t.Fatal(err)
	}
	if s.BlockNumber != 39 || s.Balance != "9223372036854775807" || s.Nonce != 2 {
		t.Fatalf("snapshot = %+v", s)
	}
}

func TestReadWithoutAddress(t *testing.T) {
	srv := fakeGateway(t)
	defer srv.Close()

	s, err := New(srv.URL).Read("")
	if err != nil {
		t.Fatal(err)
	}
	if s.BlockNumber != 39 || s.Balance != "" || s.Nonce != 0 {
		t.Fatalf("snapshot = %+v", s)
	}
}

func TestErrors(t *testing.T) {
	srv := fakeGateway(t)
	defer srv.Close()
	c := New(srv.URL)
	if _, err := c.call("nope", []any{}); err == nil {
		t.Fatal("expected an error for an unknown method")
	}
	if _, err := New("http://127.0.0.1:1").BlockNumber(); err == nil {
		t.Fatal("expected a transport error")
	}
	if _, err := hexBig(json.RawMessage(`"0xzz"`)); err == nil {
		t.Fatal("expected an error for a bad hex quantity")
	}
}
