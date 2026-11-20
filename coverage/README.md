# Coverage artifacts

Every module that ships has a report here; `make cover` regenerates them.

| Module | Artifact | Result |
|---|---|---|
| `route` | `route.txt` / `route.out` | 68.4% statements (72.3% with `ROUTE_PG_DSN` set, which enables the Postgres store tests) |
| `contractsManage` | `contractsManage.txt` | 12.9% — the pure selector/encoding/default helpers are covered; `EvmCall` paths need a running node |
| `wasm-stub` | `wasm-stub.txt` | 100.0% |
| `runner` | `runner-jacoco.csv` | 85.9% instructions / 82.5% branches (JaCoCo HTML under `runner/target/site/jacoco`) |
| `client` | `client.txt` | 9 tests, 7 pass + 2 skipped in CI mode; Node reports 100% for `app.js` |
| `producer` | `producer.txt` | upstream test log — see below |

## `producer` note

The fork carries three changes for the demo (state commit, receipts accessor,
`CommitState` switch). Its **upstream tests rely on `monkey` patching**, which
does not survive Go 1.25 (inlining/ABI): the same two tests
(`TestProducer_MakeBlock`, `Test_signBlock`) fail on an **unmodified copy** of
`producer@v1.1.0` with the same toolchain. They are therefore not evidence
about this fork either way; the fork's behaviour is covered by the end-to-end
run recorded in `../E2E.md`.
