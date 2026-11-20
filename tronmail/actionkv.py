#!/usr/bin/env python3
"""Action KV writer for the justitia fork — mirrors bds/splitter/eos (method.go ParseBlock)
and bds/service/model/eos/action.go:

  block doc:  {"block": {...}, "transactions": [ {"trx": {"id", "transaction": {..., "actions": [...]},
               "packed_trx"} } ] }
  rows:       action(transaction_hash, block_num, account, name, authorization, data, hex_data)
              transaction(...), block(...)
  reorg:      DELETE from <t>_action WHERE block_num = ?

The `trx` field accepts either a packed string or an object (the oneOf in the eos schema);
we emit the object form and keep the packed RLP in packed_trx.
"""
import json, base64, sqlite3, sys, urllib.request

RPC = "http://127.0.0.1:47768"
DB = "/tmp/justitia.db"

def rpc(method, params):
    req = urllib.request.Request(RPC, data=json.dumps({"jsonrpc":"2.0","method":method,"params":params,"id":1}).encode(),
                                 headers={"Content-Type":"application/json"})
    with urllib.request.urlopen(req, timeout=20) as r:
        return json.loads(r.read())["result"]

def b64topics(log):
    return [base64.b64decode(t).hex() for t in log["topics"]]

KNOWN = {
    "ddf252ad": "Transfer", "727200b4": "Wrapped", "e5c32bc9": "Triggered",
    "60452eb7": "Deposit", "f0537507": "ExitStarted", "8c379a00": "Approval",
    "e048c935": "Noise", "d39c4205": "Migrated",
}

def build_doc(block_num):
    blk = rpc("eth_getBlockByNumber", [hex(block_num), True])
    if not blk: return None, []
    doc = {"block": {"number": block_num, "hash": blk["hash"], "timestamp": int(blk["timestamp"], 16),
                     "tx_count": len(blk["transactions"])},
           "transactions": []}
    actions = []
    for tx in blk["transactions"]:
        rec = rpc("eth_getTransactionReceipt", [tx["hash"]]) or {}
        acts = []
        for log in (rec.get("logs") or []):
            t = b64topics(log)
            name = KNOWN.get(t[0][:8], "0x" + t[0][:8])
            data = base64.b64decode(log["data"]).hex()
            acts.append({"account": "0x" + base64.b64decode(log["address"]).hex(),
                         "name": name,
                         "authorization": tx["from"],
                         "data": data,
                         "hex_data": tx["input"]})
        doc["transactions"].append({
            "trx": {"id": tx["hash"],
                    "transaction": {"ref_block_num": block_num,
                                    "actions": acts,
                                    "receipt_status": rec.get("status"),
                                    "gas_used": int(rec.get("gasUsed", "0x0"), 16)},
                    "packed_trx": tx["input"]},
            "status": rec.get("status"),
            "from": tx["from"], "to": tx["to"], "value": int(tx.get("value", "0x0"), 16)})
        actions.extend(acts)
    return doc, actions

def ensure(con):
    con.execute("""CREATE TABLE IF NOT EXISTS justitia_block(
        block_num INTEGER PRIMARY KEY, block_hash TEXT, timestamp INTEGER, tx_count INTEGER)""")
    con.execute("""CREATE TABLE IF NOT EXISTS justitia_transaction(
        id INTEGER PRIMARY KEY AUTOINCREMENT, transaction_hash TEXT, block_num INTEGER,
        status TEXT, from_addr TEXT, to_addr TEXT, value INTEGER, receipt_status TEXT,
        gas_used INTEGER, packed_trx TEXT)""")
    con.execute("""CREATE TABLE IF NOT EXISTS justitia_action(
        id INTEGER PRIMARY KEY AUTOINCREMENT, transaction_hash TEXT, block_num INTEGER,
        account TEXT, name TEXT, authorization TEXT, data TEXT, hex_data TEXT)""")

def ingest(con, block_num):
    doc, actions = build_doc(block_num)
    b = doc["block"]
    con.execute("INSERT OR REPLACE INTO justitia_block(block_num,block_hash,timestamp,tx_count) VALUES (?,?,?,?)",
                (b["number"], b["hash"], b["timestamp"], b["tx_count"]))
    con.execute("DELETE FROM justitia_action WHERE block_num=?", (block_num,))
    con.execute("DELETE FROM justitia_transaction WHERE block_num=?", (block_num,))
    for t in doc["transactions"]:
        tr = t["trx"]
        con.execute("""INSERT INTO justitia_transaction(transaction_hash,block_num,status,from_addr,to_addr,
                       value,receipt_status,gas_used,packed_trx) VALUES (?,?,?,?,?,?,?,?,?)""",
                    (tr["id"], b["number"], t["status"], t["from"], t["to"], t["value"],
                     tr["transaction"]["receipt_status"], tr["transaction"]["gas_used"], tr["packed_trx"]))
        for a in tr["transaction"]["actions"]:
            con.execute("""INSERT INTO justitia_action(transaction_hash,block_num,account,name,authorization,data,hex_data)
                           VALUES (?,?,?,?,?,?,?)""",
                        (tr["id"], b["number"], a["account"], a["name"], a["authorization"], a["data"], a["hex_data"]))
    con.commit()
    return len(doc["transactions"]), len(actions)

if __name__ == "__main__":
    blocks = [int(x, 0) for x in sys.argv[1:]] or [0x364, 0x364 + 5]
    con = sqlite3.connect(DB)
    ensure(con)
    for bn in blocks:
        txs, acts = ingest(con, bn)
        print(f"block {hex(bn)}: {txs} txs, {acts} actions -> {DB}")
    rows = con.execute("""SELECT block_num, name, account, authorization, data FROM justitia_action
                          ORDER BY block_num, id""").fetchall()
    print(f"\njustitia_action rows: {len(rows)}")
    for r in rows[:12]:
        print("  ", hex(r[0]), r[1], r[2][:12], "<-", r[3][:12], "data", (int(r[4], 16) if r[4] else 0))
