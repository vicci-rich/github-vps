package store

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

// numeric converts a decimal string to a driver value: empty becomes NULL for
// nullable columns and "0" for NOT NULL columns, because Postgres cannot cast
// ” to numeric.
func numeric(s string, notNull bool) any {
	s = strings.TrimSpace(s)
	if s == "" {
		if notNull {
			return "0"
		}
		return nil
	}
	return s
}

// PG is the authoritative Postgres store. It implements the same interface as
// Mem, so the watcher and the services do not care which one they hold.
type PG struct{ db *sql.DB }

// Open connects, pings, and applies schema.sql (idempotent).
func Open(dsn string) (*PG, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	if _, err := db.Exec(Schema()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: schema: %w", err)
	}
	return &PG{db: db}, nil
}

// Close releases the connection pool.
func (p *PG) Close() error { return p.db.Close() }

// PutBlock upserts a block row.
func (p *PG) PutBlock(b Block) error {
	if b.Chain == "" || b.Hash == "" {
		return fmt.Errorf("store: block needs chain and hash")
	}
	_, err := p.db.Exec(`INSERT INTO blocks (chain, height, hash, parent_hash, "timestamp", tx_count)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (chain, height) DO UPDATE SET
		  hash=EXCLUDED.hash, parent_hash=EXCLUDED.parent_hash,
		  "timestamp"=EXCLUDED."timestamp", tx_count=EXCLUDED.tx_count`,
		b.Chain, b.Height, b.Hash, b.ParentHash, b.Timestamp, b.TxCount)
	return err
}

// PutTx upserts a transaction row.
func (p *PG) PutTx(t Tx) error {
	if t.Chain == "" || t.TxID == "" {
		return fmt.Errorf("store: tx needs chain and txid")
	}
	_, err := p.db.Exec(`INSERT INTO transactions (chain, height, txid, from_addr, to_addr, value, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (chain, txid) DO UPDATE SET
		  height=EXCLUDED.height, from_addr=EXCLUDED.from_addr, to_addr=EXCLUDED.to_addr,
		  value=EXCLUDED.value, status=EXCLUDED.status`,
		t.Chain, t.Height, t.TxID, t.FromAddr, t.ToAddr, numeric(t.Value, false), t.Status)
	return err
}

// PutAction upserts an action row.
func (p *PG) PutAction(a Action) error {
	if a.Chain == "" || a.TxID == "" {
		return fmt.Errorf("store: action needs chain and txid")
	}
	_, err := p.db.Exec(`INSERT INTO actions (chain, txid, idx, account, name, data)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (chain, txid, idx) DO UPDATE SET
		  account=EXCLUDED.account, name=EXCLUDED.name, data=EXCLUDED.data`,
		a.Chain, a.TxID, a.Index, a.Account, a.Name, a.Data)
	return err
}

// PutVout upserts a vout row.
func (p *PG) PutVout(v Vout) error {
	if v.Chain == "" || v.TxID == "" {
		return fmt.Errorf("store: vout needs chain and txid")
	}
	_, err := p.db.Exec(`INSERT INTO vouts (chain, height, txid, n, value, address, script_type, is_coinbase, is_used)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (chain, txid, n) DO UPDATE SET
		  height=EXCLUDED.height, value=EXCLUDED.value, address=EXCLUDED.address,
		  script_type=EXCLUDED.script_type, is_coinbase=EXCLUDED.is_coinbase, is_used=EXCLUDED.is_used`,
		v.Chain, v.Height, v.TxID, v.N, numeric(v.Value, true), v.Address, v.ScriptType, v.IsCoinbase, v.IsUsed)
	return err
}

// PutOmni upserts an Omni transfer row.
func (p *PG) PutOmni(o Omni) error {
	if o.Chain == "" || o.TxID == "" {
		return fmt.Errorf("store: omni needs chain and txid")
	}
	_, err := p.db.Exec(`INSERT INTO omni_transfers
		(chain, txid, property_id, amount, sending_address, reference_address, valid, block_height)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (chain, txid) DO UPDATE SET
		  property_id=EXCLUDED.property_id, amount=EXCLUDED.amount,
		  sending_address=EXCLUDED.sending_address, reference_address=EXCLUDED.reference_address,
		  valid=EXCLUDED.valid, block_height=EXCLUDED.block_height`,
		o.Chain, o.TxID, o.PropertyID, numeric(o.Amount, true), o.SendingAddress, o.ReferenceAddress, o.Valid, o.BlockHeight)
	return err
}

// PutRecord upserts an envelope-backed record.
func (p *PG) PutRecord(r Record) error {
	if r.Sha == "" || r.Entity == "" {
		return fmt.Errorf("store: record needs sha and entity")
	}
	_, err := p.db.Exec(`INSERT INTO route_records (sha, id, entity, state, author, envelope_verified, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6, now())
		ON CONFLICT (sha) DO UPDATE SET
		  id=EXCLUDED.id, entity=EXCLUDED.entity, state=EXCLUDED.state,
		  author=EXCLUDED.author, envelope_verified=EXCLUDED.envelope_verified, updated_at=now()`,
		r.Sha, r.ID, r.Entity, r.State, r.Author, r.EnvelopeVerified)
	return err
}

// BlockHeight returns the highest height for a chain.
func (p *PG) BlockHeight(chain string) (int64, bool, error) {
	var h sql.NullInt64
	if err := p.db.QueryRow(`SELECT max(height) FROM blocks WHERE chain=$1`, chain).Scan(&h); err != nil {
		return 0, false, err
	}
	if !h.Valid {
		return 0, false, nil
	}
	return h.Int64, true, nil
}

// RevertFrom deletes rows at height >= h in one transaction, children first.
func (p *PG) RevertFrom(chain string, h int64) error {
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	stmts := []string{
		`DELETE FROM actions WHERE chain=$1 AND txid IN (SELECT txid FROM transactions WHERE chain=$1 AND height>=$2)`,
		`DELETE FROM vouts WHERE chain=$1 AND height>=$2`,
		`DELETE FROM omni_transfers WHERE chain=$1 AND block_height>=$2`,
		`DELETE FROM transactions WHERE chain=$1 AND height>=$2`,
		`DELETE FROM blocks WHERE chain=$1 AND height>=$2`,
	}
	for _, q := range stmts {
		if _, err := tx.Exec(q, chain, h); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

var countSQL = map[string]string{
	"blocks":         `SELECT count(*) FROM blocks`,
	"transactions":   `SELECT count(*) FROM transactions`,
	"actions":        `SELECT count(*) FROM actions`,
	"vouts":          `SELECT count(*) FROM vouts`,
	"omni_transfers": `SELECT count(*) FROM omni_transfers`,
	"route_records":  `SELECT count(*) FROM route_records`,
}

// Count returns the row count of a known table (0 for an unknown name).
func (p *PG) Count(table string) int {
	q, ok := countSQL[table]
	if !ok {
		return 0
	}
	var n int
	if err := p.db.QueryRow(q).Scan(&n); err != nil {
		return 0
	}
	return n
}

// Truncate empties every table; used by tests and by `route db-reset`.
func (p *PG) Truncate() error {
	for _, table := range Tables() {
		if _, err := p.db.Exec("TRUNCATE TABLE " + table); err != nil {
			return err
		}
	}
	return nil
}
