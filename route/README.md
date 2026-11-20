# route — git as the backend, XML as the interface

`route` turns a git commit into a signed `<envelope>` document and verifies it
against a pinned keyring. The same five documents (`envelope`, `record`,
`operation`, `proof`, `endpoint`) are what the CRUD daemon and the light-node
web app exchange, so the backend, the services and the client cannot drift.

## The interface (public domain)

- `interface/route.xsd` — the schema, namespace `urn:route:1`, **CC0 1.0**
  (see `interface/LICENSE-CC0`). Java types via `xjc`, Go types via the
  `internal/xmlio` package.
- `interface/route.xml` — a worked example.

## Commands

```
route envelope [rev]         build outbox/<sha>.xml for a commit (default HEAD)
route verify <file>          validate the XML and verify the commit signature
route get <rev>              print the envelope for a commit
route put [-m msg] <paths>   git add -N, commit (signed by default), write envelope
route amend [-m msg]         rewrite the last commit, then write the envelope
route rm <paths>             git rm, commit, then write the envelope
route install-hooks [-H d]   set core.hooksPath to the hooks directory
route version
```

Flags per command: `-C repo` (default `.`), `-o outbox` (default `outbox`),
`-k keys` (default `keys`, pinned `*.asc` keyring). `put`, `amend` and `rm`
also take `-g gpgprogram` to override `gpg.program` for the signing call.

### Codespaces note

`/etc/gitconfig` sets `gpg.program=/.codespaces/bin/gh-gpgsign`; that helper
signs through GitHub and returns `403` when GPG signing is not enabled on the
repository (and can consume API quota). `route` therefore verifies with the
local `gpg` binary, and signing falls back to the local binary when the
configured program is the Codespaces helper. Pass `-g /usr/bin/gpg` explicitly
to pin the signer for a one-off test key.

## Hooks

`hooks/post-commit`, `hooks/post-merge`, `hooks/post-checkout` emit the
envelope for the new HEAD and never fail the git operation. Install with
`make hooks` or `route install-hooks -H hooks`; the binary is found through
`ROUTE_BIN`, `git config route.bin`, or `PATH`.

`git add -b` and `git remote update --recursive` do not exist; use
`git add -N` and `git fetch --all --prune --tags --recurse-submodules`.

## Store (P1)

`internal/store/schema.sql` is the one authoritative schema: `blocks`,
`transactions`, `actions`, `vouts`, `omni_transfers` (property 31 = USDT) and
`route_records` (envelope-backed rows). `internal/store` ships two
implementations of the same interface — `Mem` (tests, fixtures) and `PG`
(Postgres, `INSERT … ON CONFLICT` upserts, transactional revert-by-height).
`internal/watch` applies a JSONL event feed (`block`, `tx`, `action`, `vout`,
`omni`, `record`, `revert`) — the seam the bds listener plugs into.

```
route watch -f feed.jsonl [-chain justitia] [-dsn postgres://…] [-revert N]
route schema
route ticker -f markets.json [-addr 127.0.0.1:8000]   # /markets.json, /assets.json
route proof [-o outbox] [-k keys] <sha>               # the <proof> document
route serve [-o outbox] [-k keys] [-receipts f] [-addr 127.0.0.1:8081]
```

Local Postgres for the tests:

```
docker run -d --name route-pg -e POSTGRES_PASSWORD=route -e POSTGRES_DB=route \
  -p 55432:5432 postgres:16-alpine
ROUTE_PG_DSN='postgres://postgres:route@127.0.0.1:55432/route?sslmode=disable' \
  go test ./internal/store
```

Without `ROUTE_PG_DSN` the Postgres tests skip; the in-memory tests still run.

## ABIs off-chain — BTFS (the btfsfile pattern)

The artifact store is **BTFS** (the `btfsfile/` reference), not S3. The flow:

```
ipfsClient.add(jsonOrFile)   → result[0].hash  (CID)
contract.setMetaData(id, cid) → hash on-chain
gateway.btfssoter.io/btfs/<cid>  → fetch → re-hash → match
```

`route/internal/artifacts` wraps minio-go and can serve as a local/dev
replacement; the production path is BTFS via the IPFS HTTP API at
`api.btfssoter.io`, served from `gateway.btfssoter.io/btfs/<cid>`. No bucket,
no presigned URL — content addressing replaces all of it (see `btfsfile/`
`contract/Video.sol` for the hash-in-contract pattern and
`applications/.../source.js` for the `ipfsClient.add` → gateway fetch cycle).

After publishing, the chain side runs `POST /wallet/clearabi` (unsigned),
signs, broadcasts, and `wallet/getcontract` returns `"abi": "{}"` — the ABI
lives at the BTFS gateway URL, served as `<baseURL>/<cid>/abi/<contract>.json`.

## Proof (`/proof/{sha}`)

`internal/proof` assembles the `<proof>` document: the stored envelope, the GPG
verdict (recomputed against `-k keys` when a repository is given, otherwise
`verified="not-recomputed"`), and the receipt status. The receipt is only
`verified="true"` when a receipts file supplies it; with no evidence the
document says `not-recomputed` — the old light-node stub that returned `true`
unconditionally is exactly what this replaces. `route serve` exposes the handler
at `GET /proof/{sha}` (XML, CORS for the dev origin).

Coverage on this tree (with the Postgres tests enabled): **74.0%** statements.

## Verification

- `route verify` parses the XML, applies the schema rules and verifies the
  commit signature. With `-k keys` it uses only the pinned keyring; without it
  it falls back to `git verify-commit` and the local keyring.
- An unsigned commit yields `verified="false"` and `route verify` exits
  non-zero — it never reports success without a signature.

## Build and test

```
make build      # bin/route
make test       # go test ./...
make cover      # coverage.out + per-function summary
make vet
```

## Licensing

- Code: Apache-2.0 (`LICENSE`).
- Interface (`interface/route.xsd`, `interface/route.xml`): CC0 1.0
  (`interface/LICENSE-CC0`).

## Layout

```
cmd/route/            CLI
internal/xmlio/       the five documents: parse, marshal, validate
internal/gitx/        git plumbing: commit, diffstat, graph digest, verify
internal/gpgx/        isolated keyring: generate, sign, verify, export/import
internal/envelope/    build/write/verify envelopes
interface/            route.xsd (CC0) + route.xml
hooks/                post-commit, post-merge, post-checkout
```
