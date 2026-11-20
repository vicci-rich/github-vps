package store

import (
	"strings"
	"testing"
)

func fill(t *testing.T, s *Mem) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.PutBlock(Block{Chain: "justitia", Height: 1, Hash: "a", ParentHash: "0", Timestamp: 10, TxCount: 1}))
	must(s.PutBlock(Block{Chain: "justitia", Height: 2, Hash: "b", ParentHash: "a", Timestamp: 20, TxCount: 1}))
	must(s.PutTx(Tx{Chain: "justitia", Height: 1, TxID: "t1", FromAddr: "x", ToAddr: "y", Value: "1", Status: "ok"}))
	must(s.PutTx(Tx{Chain: "justitia", Height: 2, TxID: "t2"}))
	must(s.PutAction(Action{Chain: "justitia", TxID: "t1", Index: 0, Account: "acc", Name: "newPig", Data: "{}"}))
	must(s.PutAction(Action{Chain: "justitia", TxID: "t2", Index: 0, Name: "changePig"}))
	must(s.PutVout(Vout{Chain: "justitia", Height: 1, TxID: "t1", N: 0, Value: "0.1", Address: "addr"}))
	must(s.PutVout(Vout{Chain: "justitia", Height: 2, TxID: "t2", N: 1, Value: "0.2"}))
	must(s.PutOmni(Omni{Chain: "justitia", TxID: "t1", PropertyID: 31, Amount: "1.5", BlockHeight: 1}))
	must(s.PutOmni(Omni{Chain: "justitia", TxID: "t2", PropertyID: 1, Amount: "2", BlockHeight: 2}))
	must(s.PutRecord(Record{Sha: "abc1234", ID: 7, Entity: "pig", State: "create", Author: "a@b.c", EnvelopeVerified: "true"}))
}

func TestCountsAndHeight(t *testing.T) {
	t.Parallel()
	s := NewMem()
	fill(t, s)
	for table, want := range map[string]int{
		"blocks": 2, "transactions": 2, "actions": 2, "vouts": 2, "omni_transfers": 2, "route_records": 1,
	} {
		if got := s.Count(table); got != want {
			t.Errorf("Count(%s) = %d, want %d", table, got, want)
		}
	}
	if got := s.Count("nope"); got != 0 {
		t.Errorf("Count(unknown) = %d", got)
	}
	h, ok, err := s.BlockHeight("justitia")
	if err != nil || !ok || h != 2 {
		t.Fatalf("BlockHeight = (%d,%t,%v), want (2,true,nil)", h, ok, err)
	}
	if _, ok, _ := s.BlockHeight("missing"); ok {
		t.Fatal("missing chain reported a height")
	}
	if got := s.OrderedHeights("justitia"); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("OrderedHeights = %v", got)
	}
}

func TestRevertFrom(t *testing.T) {
	t.Parallel()
	s := NewMem()
	fill(t, s)
	if err := s.RevertFrom("justitia", 2); err != nil {
		t.Fatal(err)
	}
	if got := s.Count("blocks"); got != 1 {
		t.Errorf("blocks after revert = %d, want 1", got)
	}
	if got := s.Count("transactions"); got != 1 {
		t.Errorf("transactions after revert = %d, want 1", got)
	}
	if got := s.Count("vouts"); got != 1 {
		t.Errorf("vouts after revert = %d, want 1", got)
	}
	if got := s.Count("omni_transfers"); got != 1 {
		t.Errorf("omni after revert = %d, want 1", got)
	}
	if got := s.Count("route_records"); got != 1 {
		t.Errorf("records must survive a chain revert, got %d", got)
	}
	h, ok, _ := s.BlockHeight("justitia")
	if !ok || h != 1 {
		t.Fatalf("height after revert = %d", h)
	}
}

func TestValidation(t *testing.T) {
	t.Parallel()
	s := NewMem()
	cases := map[string]error{
		"block":  s.PutBlock(Block{}),
		"tx":     s.PutTx(Tx{Chain: "c"}),
		"action": s.PutAction(Action{Chain: "c"}),
		"vout":   s.PutVout(Vout{Chain: "c"}),
		"omni":   s.PutOmni(Omni{Chain: "c"}),
		"record": s.PutRecord(Record{Sha: "x"}),
	}
	for name, err := range cases {
		if err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
}

func TestSchemaCoversEveryTable(t *testing.T) {
	t.Parallel()
	ddl := Schema()
	for _, table := range Tables() {
		if !strings.Contains(ddl, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Errorf("schema is missing table %s", table)
		}
	}
	for _, col := range []string{"property_id", "sending_address", "reference_address", "is_coinbase", "is_used", "envelope_verified"} {
		if !strings.Contains(ddl, col) {
			t.Errorf("schema is missing column %s", col)
		}
	}
}
