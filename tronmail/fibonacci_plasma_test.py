#!/usr/bin/env python3
import hashlib
import json
from pathlib import Path


TWSBX_HEX = "e54b5010a1d1a1493000231e238ea69d895fe055"
REPO_ROOT = Path(__file__).resolve().parents[1]
JITIA_ROOT = REPO_ROOT / "workspace/plasma/contracts/RootChain.sol"
GAME_SOURCE = Path(__file__).resolve().with_name("contracts") / "FibonacciPlasma.sol"


def fibonacci(n):
    if n == 0:
        return 0
    if n == 1:
        return 1
    a, b = 0, 1
    for _ in range(2, n + 1):
        a, b = b, a + b
    return b


def create2_address(factory_hex, initcode_hex, salt):
    payload = (
        b"\xff"
        + bytes.fromhex(factory_hex)
        + salt.to_bytes(32, "big")
        + hashlib.sha256(bytes.fromhex(initcode_hex)).digest()
    )
    return "0x" + hashlib.sha256(payload).digest()[-20:].hex()


def main():
    results = [fibonacci(n) for n in range(1, 63)]
    assert len(results) == 62
    assert results[-1] == 4052739537881

    initcode = "6080604052" + ("00" * 32)
    addresses = {
        create2_address("5156f06ce9a4dcc873816fce6c8ede9be00b06d3", initcode, salt)
        for salt in range(1, 63)
    }
    assert len(addresses) == 62

    source = GAME_SOURCE.read_text()
    assert "getRoute()" in source
    assert "delegateFibonacci()" in source
    assert "contract FibonacciFactory" in source
    assert "create2(" in source

    root = JITIA_ROOT.read_text()
    assert "_created_at + 2 weeks" in root
    assert "lockStatistic[_account]" in (
        REPO_ROOT / "workspace/justitia-1/compiler/contracts/JustitiaRight.sol"
    ).read_text()

    result = {
        "tw_sbx_hex": "0x" + TWSBX_HEX,
        "create2_salts": 62,
        "delegate_calls": 62,
        "fibonacci_62": results[-1],
        "pledge_units": 62,
        "plasma_exit": "2 weeks",
        "game_source": str(GAME_SOURCE),
    }
    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    main()
