#!/usr/bin/env python3
"""Publish deployment evidence as a workflow/author-mailing artifact."""
import argparse
import json
import os
import sqlite3
import urllib.request
from pathlib import Path


TRONGRID = "https://api.trongrid.io"
TRONGRID_REFERENCES = [
    "TAUVq7fdL6usuP6ZK4Vpnp8ZfkUu4yAVKj",
    "TWtQzc1EdKCbaKg17ASpoxnx1kVgXxp6Mx",
    "TQMvFCNyhnCEDBeQ6fGPzLGddzaRw6cso6",
    "TNviBTAmdtQC95oGA4YcWjmnVM19FLPQ4F",
    "TSACyoeqgCqmGfeLzkw3ZAHnq7j3BdEJpa",
]


def post_json(path, payload):
    headers = {"Content-Type": "application/json"}
    api_key = os.environ.get("TRONGRID_API_KEY")
    if not api_key:
        directory = Path.cwd().resolve()
        while True:
            candidate = directory / "api.txt"
            if candidate.exists():
                break
            if directory.parent == directory:
                candidate = None
                break
            directory = directory.parent
        if candidate:
            for line in candidate.read_text().splitlines():
                if line.startswith("TRONGRID="):
                    api_key = line.split("=", 1)[1].strip()
                    break
    if api_key:
        headers["TRON-PRO-API-KEY"] = api_key
    req = urllib.request.Request(
        TRONGRID + path,
        data=json.dumps(payload).encode(),
        headers=headers,
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=15) as response:
        return json.loads(response.read())


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--commit", required=True)
    parser.add_argument("--author", default="223556219+Copilot@users.noreply.github.com")
    parser.add_argument("--migration-block", type=lambda value: int(value, 0))
    parser.add_argument("--db", default="/tmp/justitia.db")
    parser.add_argument("--output", default="/tmp/deployment-result.json")
    args = parser.parse_args()

    transactions = []
    if args.migration_block is not None:
        try:
            con = sqlite3.connect(args.db)
            rows = con.execute(
                """SELECT transaction_hash,status,from_addr,to_addr,value
                   FROM justitia_transaction WHERE block_num=? ORDER BY id""",
                (args.migration_block,),
            ).fetchall()
            con.close()
            transactions = [
                {
                    "tx_hash": row[0],
                    "status": row[1],
                    "from": row[2],
                    "to": row[3],
                    "value": row[4],
                }
                for row in rows
            ]
        except sqlite3.Error:
            transactions = []

    refs = []
    for address in TRONGRID_REFERENCES:
        try:
            account = post_json("/wallet/getaccount", {"address": address, "visible": True})
            refs.append({"address": address, "resolved": bool(account)})
        except Exception as exc:
            refs.append({"address": address, "resolved": False, "error": str(exc)})

    result = {
        "commit": args.commit,
        "author": args.author,
        "network": "justitia-fork",
        "migration_block": args.migration_block,
        "transactions": transactions,
        "trongrid_references": refs,
    }
    path = Path(args.output)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    main()
