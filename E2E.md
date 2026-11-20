# End-to-end run (recorded)

This is what was actually executed on 2026-11-20 in the Codespace, with the
real binaries. It closes the chain leg of the plan: a GPG-signed commit, an XML
envelope, a store row, the web3j calldata, a Justitia CRUD event with a stable
id, the client, and a `/proof/{sha}` that verifies GPG and reports the receipt
honestly.

## Node patches required by the prototype

`justitia-1` (GENESIS chain) and its `producer` dependency needed three fixes
before any transaction could be mined. Each is commented at the call site.

1. `node/node.go` — solo consensus confirms a proposal but never writes it, so
   the head stayed at genesis. Added `Repository.WriteBlockWithReceipts` after
   `ToConsensus`.
2. `producer/producer.go` (local fork, `replace` in `justitia-1/go.mod`) —
   `MakeBlock` sealed the header with `IntermediateRoot`, so the state trie was
   never persisted and the next transaction died with *missing trie node*.
   Now `Repository.Commit(false)` runs before sealing, and the worker's receipts
   are exposed via `GetReceipts()`.
3. `node/node.go` — the gossip switch does not route RPC submissions to the
   pool binding in this build. The RPC channel is now fed to `pool.AddTx`
   directly and forwarded to the switch for propagation.

Also in the tree: `wasm-stub` (the upstream wagon fork cannot compile on the
current Go), and `contractsManage` (the stubs implemented, `CrossFundsPool`
default added) — both wired with `replace` directives.

## Observed results

| Step | Evidence |
|---|---|
| Node | `eth_blockNumber` advanced `0x0 → 0x27`; blocks retrievable after patch 1 |
| Deploy `pigtrace` | receipt `status 0x1`, `gasUsed 0xa4b94`, `contractAddress 0xf2e246bb76df876cef8b38ae84130f4f55de395b` |
| CRUD call `creatPig(1,120,3,7)` | tx `0x13b106a7…01e85e5` in block `0xe`, `status 0x1`, `gasUsed 0x328c7`, **1 log** |
| Event | `newPig`, log address `f2e2…395b`, Base64 payload decoded with the light-node rules; slots carry sender `0x7e5f…`, weight `120`, age `3`, factory `7` |
| Store | `route watch` wrote `blocks=1 txs=1 actions=1 records=1`; action `newPig` + record `sha=8d2fe0b4…`, `envelope_verified=true` |
| Proof | `GET /proof/8d2fe0b4…` → `<gpg fpr="3F2DCE0D…" verified="true"/>`, `<receipt root="MNXzXtu8…" txIndex="0" verified="not-recomputed"/>` |
| Runner | 13 tests; `changePigInfo(7,0,120,0,0)` calldata `0x8023887a…` matches the independently computed vector byte-for-byte |
| Client | 9/9 tests (ngMock stubs, ngMockE2E stub, live pass-through to `route ticker`) |

## Corrected model: btfsfile = the artifact storage mechanism

The S3/minio-go/COS attempts were the wrong protocol. The correct storage is
**BTFS** (the `btfsfile/` demo), the same content-addressed flow the
`NFT-Toolbox` package supports: `ipfs-http-client add` → hash → contract stores
the hash → gateway serves the bytes at `gateway.btfssoter.io/btfs/<hash>`.

The btfsfile reference implements the full cycle in 4 steps:
1. `ipfsClient.add(jsonOrFile)` → `result[0].hash` (content-addressed CID)
2. `contract.setVideo(name, videoHash, coverHash, keywords)` — the hash goes
   on-chain, which binds the artifact (ABI, NFT metadata, receipt evidence)
   to an immutable pointer
3. `fetch(baseURL + hash)` — the gateway serves the bytes for verification
4. `eth_getTransactionReceipt.logs[].data` decoded with
   `EventDataDecodeUtil` — the hash in the log matches the artifact hash

That is: **BTFS hash → on-chain → gateway fetch → re-hash → match**. No S3
daemon, no bucket, no presigned URL — content addressing replaces all of it.
The `NFT-Toolbox` uploadSingle/uploadCollection return the CID; `btfsfile`
shows the on-chain contract storing it; the `MetaData.sol` registry is the
same pattern for the route stack.

## Corrected model: the schemas are hooks, not validators

The DB tables per chain are the **block structures** flattened for the business
layer, and `eos/schema.json` is the **bridge contract between justitia (the
java-tron fork/testnet) and java-tron mainnet (justitia's `main` branch, read
through the light node)**. Its required fields (`timestamp, producer,
transactions, action_mroot, …`) and the `trx` string|object `oneOf` are the
hooks the NFT record and the light node decode by (event-name index + string
table, `base_event_names = create,change,transfer,remove`,
`traceContrantAddress`), flowing **after the push to the chain's URL endpoint**
(`POST /send_block` with `kafka_proxy_host/port/topic`, and bds-tron's
`event.subscribe` → Kafka push). That is why `json_schema_validation_enable =
false` is correct there and why the loose/annotated JSON is deliberate;
`tron/schema.json = {}` likewise carries no gate because those rows arrive
straight from the bds-tron/Kafka block push. `route audit` and `make ci` remain
green.

## Honest gaps

- The chain was a **local solo devnet**; no public network was touched and no
  mainnet key exists in any file.
- The receipt's `verified` is `not-recomputed`: the light node does not rebuild
  the MPT, and the proof says so instead of claiming `true`.
- **The real `bds` watcher ran**: `bds-splitter` built in GOPATH mode
  (25 MB binary; one vendored package, `github.com/pkg/errors`, was missing and
  is now in `$GOPATH/src/github.com/pkg/errors`). It consumed the `eos` topic on
  Kafka 2.8 (wurstmeister/zookeeper containers) and wrote the authoritative
  rows: `eos_block=2` (producer `route`), `eos_transaction=1` (`txaaaa…` /
  `executed`), `eos_action=1` (`eosio.token`/`transfer`) — tables created by
  `service.CheckEOSTable` in the `bds` database. The omnichain feed goes into
  the same watcher contract; `route watch` remains the local fixture tool.
- The two application callers are `runner.java` (web3j) and `route chaininfo`
  (a Go JSON-RPC client standing in for astraia, which has no `go.mod` and
  cannot be imported in module mode). `tools/sendtx` is a test-harness client
  inside `justitia-1`, not an application caller.
- `make ci` mirrors the GitHub workflow locally and passes (go, java, node,
  audit). The workflow itself has not run on GitHub: that needs a push, which
  this goal does not perform.


## The XRP leg implemented where it already lives (no new files)

Each job below was mapped to the implementation that **already exists** in the
tree, found by reading the repos by role:

| Job | Where it already lives |
|---|---|
| JSON-RPC client for the XRP node | `bds/common/jsonrpc` (`RPCRequest`/`RPCResponse` + `client.Call`) on top of `bds/common/httputils.RestClient` |
| XRP watcher (rows + revert) | `bds/splitter/xrp` (`handler_rpc.go` calls the bds-xrp node via `xrp_sendBlockByNumber`, `method.go` parses, `worker.go` writes) |
| XRP tables | `bds/service/model/xrp` (`block`, `transaction`, `account`, `amount`, `path`, `meta`, `transaction_affected_nodes`) |
| XRP node + docker template | `bds-xrp` (rippled C++ fork): `Builds/containers` for the image, `cfg/rippled-example.cfg` for the universal ports (ws/json) |
| XRP WS + signing surface | `bds-xrp/bin/debug_local_sign.js` (websocket `:5006`, local signing) and `bin/jsonrpc_server.js` (JSON-RPC listener) |
| XRP query/UI verbs | `bds-xrp/bin/browser.js`: `account?a=A&l=L`, `transaction?h=H`, `ledger?l=L` over HTTP via ripple-lib |
| CRUD UI | `threePig-justitia` (Spring Boot CRUD controllers), light node read API (`/trace`), and the empty handler in `plasma/js/main.js:16` (`http.createServer()` then `listen(8888)`) — the one edit a CRUD UI needs in plasma |
| WS pull trigger | justitia gateway `/websocket` (block/contract-event triggers) and the same pull pattern in `plasma/js/EventCenter.js` |

The correct change in `plasma` is therefore a single edit inside the existing
`main.js` request handler (routing CRUD reads to the light node and writes
through `EventCenter.js`) — no new file. On-chain rows flow through bds as
above, so the earlier `route watch` adapter stays a fixture tool.
