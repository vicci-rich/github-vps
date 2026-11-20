package watch

import (
	"strings"
	"testing"

	"route/internal/store"
)

const fixture = `
{"kind":"block","height":1,"hash":"a","parent_hash":"0","timestamp":10,"tx_count":1}
{"kind":"tx","height":1,"txid":"t1","from":"x","to":"y","value":"1","status":"ok"}
{"kind":"action","txid":"t1","index":0,"account":"acc","name":"newPig","data":"{}"}
{"kind":"vout","height":1,"txid":"t1","n":0,"value":"0.1","address":"addr","script_type":"pubkeyhash"}
{"kind":"omni","height":1,"txid":"t1","property_id":31,"amount":"1.5","sending_address":"s","reference_address":"r","valid":true}
{"kind":"record","sha":"abc1234","id":7,"entity":"pig","state":"create","author":"a@b.c","verified":"true"}

{"kind":"block","height":2,"hash":"b","parent_hash":"a","timestamp":20,"tx_count":0}
{"kind":"omni","height":2,"txid":"t2","property_id":1,"amount":"2","valid":true}
{"kind":"revert","height":2}
`

func TestApplyAndRevert(t *testing.T) {
	s := store.NewMem()
	stats, err := Apply(s, "justitia", strings.NewReader(fixture))
	if err != nil {
		t.Fatal(err)
	}
	if stats.Blocks != 2 || stats.Txs != 1 || stats.Actions != 1 || stats.Vouts != 1 ||
		stats.Omni != 2 || stats.USDT != 1 || stats.Records != 1 || stats.Reverted != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if stats.Height != 1 {
		t.Fatalf("height after revert = %d, want 1", stats.Height)
	}
	for table, want := range map[string]int{"blocks": 1, "transactions": 1, "actions": 1, "vouts": 1, "omni_transfers": 1, "route_records": 1} {
		if got := s.Count(table); got != want {
			t.Errorf("%s = %d, want %d", table, got, want)
		}
	}
}

func TestApplyRejectsUnknownKind(t *testing.T) {
	_, err := Apply(store.NewMem(), "justitia", strings.NewReader(`{"kind":"frobnicate"}`))
	if err == nil || !strings.Contains(err.Error(), "unknown kind") {
		t.Fatalf("err = %v, want an unknown-kind error", err)
	}
}

func TestApplyRejectsBadJSON(t *testing.T) {
	_, err := Apply(store.NewMem(), "justitia", strings.NewReader("{"))
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("err = %v, want a line-numbered parse error", err)
	}
}
