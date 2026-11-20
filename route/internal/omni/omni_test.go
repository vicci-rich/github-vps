package omni

import "testing"

func TestParseTransferUSDT(t *testing.T) {
	raw := []byte(`{"txid":"aa11","sendingaddress":"1Send","referenceaddress":"1Ref",
		"propertyid":31,"amount":"12.50000000","valid":true,"block":840000,
		"blockhash":"bb22","positioninblock":3,"type":"Simple Send","type_int":0}`)
	tx, err := ParseTransfer(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !tx.IsUSDT() {
		t.Fatalf("property %d must be USDT", tx.PropertyID)
	}
	if string(tx.Amount) != "12.50000000" || tx.SendingAddress != "1Send" || tx.Block != 840000 {
		t.Fatalf("unexpected transfer: %+v", tx)
	}
}

func TestParseTransferAcceptsNumericAmount(t *testing.T) {
	tx, err := ParseTransfer([]byte(`{"txid":"aa","propertyid":1,"amount":1.25}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(tx.Amount) != "1.25" || tx.IsUSDT() {
		t.Fatalf("unexpected transfer: %+v", tx)
	}
}

func TestParseTransferRequiresTxID(t *testing.T) {
	if _, err := ParseTransfer([]byte(`{"propertyid":31}`)); err == nil {
		t.Fatal("expected an error for a transfer without txid")
	}
	if _, err := ParseTransfer([]byte(`not json`)); err == nil {
		t.Fatal("expected an error for invalid json")
	}
}

func TestParseTransferList(t *testing.T) {
	ids, err := ParseTransferList([]byte(`["a","b"]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[1] != "b" {
		t.Fatalf("ids = %v", ids)
	}
}

func TestParseVouts(t *testing.T) {
	raw := []byte(`{"tx":[
		{"txid":"t1","vout":[
			{"value":0.00010000,"n":0,"scriptPubKey":{"addresses":["1A"],"type":"pubkeyhash"}},
			{"value":12.5,"n":1,"scriptPubKey":{"addresses":["1B"],"type":"pubkeyhash"}}]},
		{"txid":"t2","vout":[{"value":0,"n":0,"scriptPubKey":{"type":"nulldata"}}]}]}`)
	vouts, err := ParseVouts(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(vouts) != 3 {
		t.Fatalf("vouts = %d, want 3", len(vouts))
	}
	if vouts[1].TxID != "t1" || vouts[1].N != 1 || vouts[1].Address != "1B" || string(vouts[1].Value) != "12.5" {
		t.Fatalf("unexpected second vout: %+v", vouts[1])
	}
	if vouts[2].Address != "" || vouts[2].ScriptType != "nulldata" {
		t.Fatalf("unexpected third vout: %+v", vouts[2])
	}
	if _, err := ParseVouts([]byte(`{`)); err == nil {
		t.Fatal("expected an error for invalid json")
	}
}
