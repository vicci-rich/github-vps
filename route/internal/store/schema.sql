-- Route store: the one authoritative Postgres schema (P1).
-- Row shapes follow bds/splitter/eos/schema.json (blocks/transactions/actions)
-- and bds/splitter/btc (vouts + Omni property 31 = USDT).

CREATE TABLE IF NOT EXISTS blocks (
  chain       text    NOT NULL,
  height      bigint  NOT NULL,
  hash        text    NOT NULL,
  parent_hash text    NOT NULL,
  "timestamp" bigint  NOT NULL,
  tx_count    integer NOT NULL,
  PRIMARY KEY (chain, height)
);

CREATE TABLE IF NOT EXISTS transactions (
  chain     text    NOT NULL,
  height    bigint  NOT NULL,
  txid      text    NOT NULL,
  from_addr text,
  to_addr   text,
  value     numeric,
  status    text,
  PRIMARY KEY (chain, txid)
);

CREATE TABLE IF NOT EXISTS actions (
  chain   text    NOT NULL,
  txid    text    NOT NULL,
  idx     integer NOT NULL,
  account text,
  name    text,
  data    text,
  PRIMARY KEY (chain, txid, idx)
);

CREATE TABLE IF NOT EXISTS vouts (
  chain       text    NOT NULL,
  height      bigint  NOT NULL,
  txid        text    NOT NULL,
  n           integer NOT NULL,
  value       numeric NOT NULL,
  address     text,
  script_type text,
  is_coinbase boolean NOT NULL DEFAULT false,
  is_used     boolean NOT NULL DEFAULT false,
  PRIMARY KEY (chain, txid, n)
);

CREATE TABLE IF NOT EXISTS omni_transfers (
  chain             text    NOT NULL,
  txid              text    NOT NULL,
  property_id       bigint  NOT NULL,
  amount            numeric NOT NULL,
  sending_address   text,
  reference_address text,
  valid             boolean NOT NULL,
  block_height      bigint  NOT NULL,
  PRIMARY KEY (chain, txid)
);

CREATE INDEX IF NOT EXISTS omni_transfers_property ON omni_transfers (property_id);

CREATE TABLE IF NOT EXISTS route_records (
  sha               text PRIMARY KEY,
  id                bigint NOT NULL,
  entity            text   NOT NULL,
  state             text   NOT NULL,
  author            text   NOT NULL,
  envelope_verified text   NOT NULL,
  updated_at        timestamptz NOT NULL DEFAULT now()
);
