# runner — web3j + Fibonacci step

Reads one store row, advances the Fibonacci chain (one step per linkage hop)
and builds the Justitia calldata. Dry-run by default.

## Numbers

- `F(62) = 4 052 739 537 881` (not 4.05e18 — that constant belonged to a
  larger index); `F(92) = 7 540 113 804 746 346 429` is the last value that fits
  a signed 64-bit integer, so `MAX_SAFE_HOPS = 92` and the default chain is 62
  hops (two levels below the EVM `MAX_DEPTH = 64`).
- `changePigInfo(uint256,uint256,uint256,uint256,uint256)` selector is
  `0x8023887a`; `creatPig(uint256,uint256,uint256,uint256)` is `0x58255f64`.
  Both are pinned by `CalldataTest` against vectors computed independently
  (Keccak-256 of the signature + 32-byte words), so the encoder is checked
  byte-for-byte, not by round-tripping the same library.

## Usage

```
mvn -q -B package
mvn -q -B dependency:build-classpath -Dmdep.outputFile=target/cp.txt

java -cp "target/runner-0.1.0.jar:$(cat target/cp.txt)" route.runner.Runner \
  --row '{"token_id":7,"breed":0,"weight":120,"age":0,"factory":0}' --hops 62
```

Output:

```
{"token_id":7,...,"hops":62,"fibonacci":4052739537881,"calldata":"0x8023887a…","contract":""}
```

Flags: `--row JSON` or `--token/--breed/--weight/--age/--factory`, `--hops`
(default 62), `--contract`, `--send`, `--rpc`, `--key-env` (default
`ROUTE_SIGNER_KEY`).

Sending is off by default. It requires `--send --rpc --contract` **and** the
signer key in the environment; the key is never read from a flag, a file or
the repository.

## Tests and coverage

```
mvn -q -B test          # 13 tests: Fibonacci, calldata vectors, runner, send guards
open target/site/jacoco/index.html
```

JaCoCo 0.8.14 is used because 0.8.12 cannot instrument Java 25 class files.

## Licensing

Apache-2.0 (see `LICENSE`).
