#!/usr/bin/env python3
import hashlib
import json
from pathlib import Path


TWSBX_HEX = "e54b5010a1d1a1493000231e238ea69d895fe055"
REPO_ROOT = Path(__file__).resolve().parents[1]
GAME_SOURCE = Path(__file__).resolve().with_name("contracts") / "FibonacciPlasma.sol"
DEPLOY_SOURCE = REPO_ROOT / "tronmail/deploy.py"


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

    deploy = DEPLOY_SOURCE.read_text()
    assert "lockCount(address,uint256)" in deploy
    assert "two-week Plasma deposit exit" in deploy
    game_source = GAME_SOURCE.read_text()
    assert "routeAmount(uint baseAmount, uint steps)" in game_source
    config = (REPO_ROOT / "workspace/plasma/js/config.json").read_text()
    assert "0xe54b5010a1d1a1493000231e238ea69d895fe055" in config
    assert '"fibonacciSteps": 62' in config
    child = (REPO_ROOT / "workspace/plasma/contracts/ChildChain.sol").read_text()
    assert "depositRoutedChild" in child
    assert "getFibonacciAmount" in child
    root = (REPO_ROOT / "workspace/plasma/contracts/RootChain.sol").read_text()
    assert "startDepositExitFor" in root

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
