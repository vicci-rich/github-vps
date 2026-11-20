package store

import (
	"os"
	"strings"
	"testing"
)

func openPG(t *testing.T) *PG {
	t.Helper()
	dsn := os.Getenv("ROUTE_PG_DSN")
	if dsn == "" {
		t.Skip("set ROUTE_PG_DSN to run the Postgres tests")
	}
	pg, err := Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := pg.Truncate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pg.Close() })
	return pg
}

func TestPGMatchesMemSemantics(t *testing.T) {
	pg := openPG(t)

	mem := NewMem()
	applyFixture(t, mem)
	applyFixture(t, pg)

	for table, want := range map[string]int{
		"blocks": 2, "transactions": 2, "actions": 2, "vouts": 2, "omni_transfers": 2, "route_records": 1,
	} {
		if got := pg.Count(table); got != want {
			t.Errorf("PG Count(%s) = %d, want %d", table, got, want)
		}
		if got := mem.Count(table); got != want {
			t.Errorf("Mem Count(%s) = %d, want %d", table, got, want)
		}
	}

	for _, s := range []Store{mem, pg} {
		if err := s.RevertFrom("justitia", 2); err != nil {
			t.Fatalf("%T revert: %v", s, err)
		}
		if got := s.Count("blocks"); got != 1 {
			t.Errorf("%T blocks after revert = %d", s, got)
		}
		if got := s.Count("omni_transfers"); got != 1 {
			t.Errorf("%T omni after revert = %d", s, got)
		}
		if got := s.Count("route_records"); got != 1 {
			t.Errorf("%T records must survive revert, got %d", s, got)
		}
		h, ok, err := s.BlockHeight("justitia")
		if err != nil || !ok || h != 1 {
			t.Errorf("%T height after revert = (%d,%t,%v)", s, h, ok, err)
		}
	}
}

func TestPGSchemaIsIdempotent(t *testing.T) {
	pg := openPG(t)
	if _, err := pg.db.Exec(Schema()); err != nil {
		t.Fatalf("re-applying the schema failed: %v", err)
	}
	if !strings.Contains(Schema(), "CREATE TABLE IF NOT EXISTS omni_transfers") {
		t.Fatal("schema lost the omni table")
	}
}

// fill(nil) is a no-op guard so the Mem helper stays used by both tests.
func TestMemFixtureIsShared(t *testing.T) {
	mem := NewMem()
	applyFixture(t, mem)
	if mem.Count("blocks") != 2 {
		t.Fatal("shared fixture did not reach the in-memory store")
	}
}

func applyFixture(t *testing.T, s Store) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.PutBlock(Block{Chain: "justitia", Height: 1, Hash: "a", Timestamp: 10, TxCount: 1}))
	must(s.PutBlock(Block{Chain: "justitia", Height: 2, Hash: "b", ParentHash: "a", Timestamp: 20}))
	must(s.PutTx(Tx{Chain: "justitia", Height: 1, TxID: "t1"}))
	must(s.PutTx(Tx{Chain: "justitia", Height: 2, TxID: "t2"}))
	must(s.PutAction(Action{Chain: "justitia", TxID: "t1", Index: 0, Name: "newPig"}))
	must(s.PutAction(Action{Chain: "justitia", TxID: "t2", Index: 0, Name: "changePig"}))
	must(s.PutVout(Vout{Chain: "justitia", Height: 1, TxID: "t1", N: 0, Value: "0.1"}))
	must(s.PutVout(Vout{Chain: "justitia", Height: 2, TxID: "t2", N: 1, Value: "0.2"}))
	must(s.PutOmni(Omni{Chain: "justitia", TxID: "t1", PropertyID: 31, Amount: "1.5", BlockHeight: 1}))
	must(s.PutOmni(Omni{Chain: "justitia", TxID: "t2", PropertyID: 1, Amount: "2", BlockHeight: 2}))
	must(s.PutRecord(Record{Sha: "abc1234", ID: 7, Entity: "pig", State: "create", Author: "a@b.c", EnvelopeVerified: "true"}))
}
