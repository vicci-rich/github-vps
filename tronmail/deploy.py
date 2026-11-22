#!/usr/bin/env python3
"""Clean, ordered deployment of the whole stack on the justitia fork/testnet.

Order (each step depends on the previous):
  0  fresh HORSE foundation key -> genesis account, funded (10^12)
  1  genesis: system contracts (JR / Voting / WhiteList / MetaData) at the
     zero-coinbase CREATE nonces 0..3
  2  HORSE asset issued by the foundation (huge supply; TRC10 analogue)
  3  SR proposal: JR pledge + vote -> MainNetOnlineEvent (pledge >= 1000)
  4  MONKEY router (hidden selector 0x1129361d) + mainnet-style trigger into the mock
  5  token wrapper (TAUV analogue) + trigger/event contracts (TNvi/TSAC analogues)
  6  plasma root/child chain (Validate -> RootChain -> ChildChain)
  7  migration: ONE block = trigger->mock + busy noise + batch pull to both destinations
  8  plasma: deposit F(30) -> exit queue (+2 weeks) -> child credits to both destinations
  9  action KV ingest (bds/eos model) + validation
"""
import json, os, shutil, subprocess, sys, time, base64
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

WS = "/workspaces/github-vps/workspace"
PLASMA = f"{WS}/plasma/contracts"
HERE = os.path.dirname(os.path.abspath(__file__))
HOME = os.path.expanduser("~")
GENESIS = f"{HOME}/.justitia/genesis.json"
NODE_BIN = "/tmp/justitia-node"
STATE, DATA = "/tmp/justitia-state", "/tmp/justitia-data"
TYX5 = "f75a371476c5e6822adf2deef66ca473eecc52d2"   # TYX5yhZCJbxnCN3h5NwGVojZEFyyM2T9oy
TWSBX = "e54b5010a1d1a1493000231e238ea69d895fe055"  # TWsbxowDR7WJamz6a2c6tJS6r9FxK2jBt

def sh(cmd):
    return subprocess.run(cmd, shell=True, capture_output=True, text=True)

def stage_sources():
    os.makedirs("/tmp/build", exist_ok=True)
    for f in os.listdir("/tmp/build"):
        os.remove("/tmp/build/" + f)
    for f in os.listdir(PLASMA):
        if f.endswith(".sol"):
            shutil.copy(f"{PLASMA}/{f}", "/tmp/build/")
    for f in ["TokenWrapper.sol", "TriggerEvent.sol", "CrossFundsPool.sol", "HorseMonkey.sol"]:
        src = f"{HERE}/contracts/{f}"
        if os.path.exists(src):
            shutil.copy(src, "/tmp/build/")

def compile_stage1():
    import solcx
    files = ["/tmp/build/" + f for f in os.listdir("/tmp/build") if f.endswith(".sol") and f != "RootChain.sol"]
    out = solcx.compile_files(files, output_values=["bin"], optimize=False, solc_version="v0.4.25")
    bins = {}
    for k, v in out.items():
        n = k.split(":")[-1]
        if n in ("ERC20", "TokenWrapper", "InnerContract", "TriggerEvent", "CrossFundsPool",
                 "GenAsset", "BusyRouter", "MainnetTrigger", "Validate", "ChildChain"):
            b = v["bin"]
            if n == "CrossFundsPool":
                assert b.count("63fff840d6") == 1
                b = b.replace("63fff840d6", "631129361d")     # hidden dispatch path
            bins[n] = b
    return bins

def compile_rootchain(validate_addr):
    import solcx
    base = PLASMA + "/"
    sources = {base + f: {"content": open(base + f).read()} for f in os.listdir(base) if f.endswith(".sol")}
    std = {"language": "Solidity", "sources": sources,
           "settings": {"optimizer": {"enabled": False},
                        "libraries": {base + "Validate.sol": {"Validate": validate_addr}},
                        "outputSelection": {"*": {"*": ["evm.bytecode.object"]}}}}
    out = solcx.compile_standard(std, solc_version="v0.4.25")
    return out["contracts"][base + "RootChain.sol"]["RootChain"]["evm"]["bytecode"]["object"]

def fresh_foundation():
    from ecdsa import SigningKey, SECP256k1
    from Crypto.Hash import keccak
    sk = SigningKey.generate(curve=SECP256k1)
    body = keccak.new(digest_bits=256, data=sk.get_verifying_key().to_string()).hexdigest()[-40:]
    return sk.to_string().hex(), body

def write_genesis(foundation_body):
    g = json.load(open(f"{WS}/justitia-1/config/genesis.json"))
    g["GenesisAccounts"].append({"addr": "0x" + foundation_body, "balance": 10**12})
    json.dump(g, open(GENESIS, "w"), indent=1)

def restart_node():
    import signal
    out = subprocess.run(["ps", "-eo", "pid=,args="], capture_output=True, text=True)
    for line in out.stdout.splitlines():
        fields = line.strip().split(None, 1)
        if len(fields) == 2 and "/tmp/justitia-node" in fields[1]:
            os.kill(int(fields[0]), signal.SIGTERM)
    time.sleep(2)
    for d in (STATE, DATA):
        shutil.rmtree(d, ignore_errors=True)
        os.makedirs(d, exist_ok=True)
    subprocess.Popen(f"cd {WS}/justitia-1 && setsid nohup {NODE_BIN} --log_level=1 > /tmp/justitia.out 2>&1 &", shell=True)
    import urllib.request
    for _ in range(60):
        try:
            urllib.request.urlopen("http://127.0.0.1:47768", timeout=2)
            time.sleep(1.5)
            return True
        except Exception:
            time.sleep(0.5)
    return False

def main():
    import driver
    from driver import (deploy, call, rpc, ev, sel, w, sign_tx, send, batch_input,
                         keccak, SigningKey, SECP256k1)
    print("== 0. fresh HORSE foundation")
    priv, body = fresh_foundation()
    open("/tmp/foundation.key", "w").write(priv)
    os.environ["FOUNDATION_KEY"] = priv
    driver.set_key(priv)
    print("   address 0x" + body)

    print("== 1. genesis + restart")
    write_genesis(body)
    stage_sources(); bins = compile_stage1()
    print("   node up:", restart_node(), "height", rpc("eth_blockNumber", []),
          "foundation", int(rpc("eth_getBalance", ["0x" + body, "latest"]), 16))

    A = {}
    def dep(step, name, code, args=b""):
        a, h, r = deploy(code, args)
        A[name] = a
        print(f"   [{step}] {name:13s} -> {a} status {r['status']} block {int(r['blockNumber'],16)}")
        return a

    print("== 2. HORSE asset (huge supply)")
    dep(2, "GenAsset", bins["GenAsset"], (10**24).to_bytes(32, "big"))
    print("== 3. SR proposal")
    jr, vot = "0xbd770416a3345f91e4b34576cb804a576fa48eb1", "0x5a443704dd4b594b382c22a083e2bd3090a6fef3"
    K2 = SigningKey.from_string(bytes.fromhex("02" * 32), curve=SECP256k1)
    K3 = SigningKey.from_string(bytes.fromhex("03" * 32), curve=SECP256k1)
    s2 = keccak.new(digest_bits=256, data=K2.get_verifying_key().to_string()).hexdigest()[-40:]
    s3 = keccak.new(digest_bits=256, data=K3.get_verifying_key().to_string()).hexdigest()[-40:]
    call(jr, "0x" + sel("issueJR(address,uint256)") + "0" * 24 + body + w(400))
    call(jr, "0x" + sel("issueJR(address,uint256)") + "0" * 24 + s2 + w(400))
    call(vot, "0x" + sel("ApplyToCandidate(address,uint256,string,string)") + "0" * 24 + body + w(100)
         + w(0x80) + w(0xc0) + w(0x80) + w(0x60) + w(0x60 + 0x40) + w(5) + "686f727365" + w(0x60 + 0x48) + w(6) + "6d6f6e6b6579" + w(0))
    call(vot, "0x" + sel("Votting(address,uint256)") + "0" * 24 + body + w(200), sk=K2, frm=s2)
    print("   totalPledge", int(ev(vot, "0x" + sel("totalPledge()")), 16),
          "online", ev(vot, "0x" + sel("GetOnlineSymbol()"))[-3:])
    print("== 4. MONKEY router + mainnet trigger")
    monkey = dep(4, "BusyRouter", bins["BusyRouter"])
    trig = dep(4, "MainnetTrigger", bins["MainnetTrigger"], bytes.fromhex(monkey[2:]).rjust(32, b"\x00"))
    print("== 5. token + wrapper + trigger/event contracts")
    erc = dep(5, "ERC20", bins["ERC20"])
    dep(5, "TokenWrapper", bins["TokenWrapper"], bytes.fromhex(erc[2:]).rjust(32, b"\x00"))
    dep(5, "TriggerEvent1", bins["TriggerEvent"])
    dep(5, "TriggerEvent2", bins["TriggerEvent"])
    print("== 6. plasma root/child")
    val = dep(6, "Validate", bins["Validate"])
    rc = dep(6, "RootChain", compile_rootchain(val))
    cc = dep(6, "ChildChain", bins.get("ChildChain") or "")
    if not cc:  # compile ChildChain separately
        import solcx
        out = solcx.compile_files(["/tmp/build/ChildChain.sol"], output_values=["bin"], optimize=False, solc_version="v0.4.25") \
            if os.path.exists("/tmp/build/ChildChain.sol") else None
    json.dump(A, open("/tmp/addresses.json", "w"), indent=1)

    print("== 7. migration: ONE block (trigger -> mock, busy noise, batch pull)")
    for sk, frm in ((K2, s2), (K3, s3)):
        call(erc, "0x" + sel("approve(address,uint256)") + "0" * 24 + monkey[2:] + w(2**200), sk=sk, frm=frm)
    call(erc, "0x" + sel("transfer(address,uint256)") + "0" * 24 + s2 + w(500))
    call(erc, "0x" + sel("transfer(address,uint256)") + "0" * 24 + s3 + w(400))
    raws, nonce = [], int(rpc("eth_getTransactionCount", ["0x" + body, "pending"]), 16)
    raws.append(sign_tx(nonce, trig, 0, bytes.fromhex(sel("fire(uint256)") + w(4052739537881)))); nonce += 1
    raws.append(sign_tx(nonce, monkey, 0, bytes.fromhex(sel("busy(uint256)") + w(2)))); nonce += 1
    for to, amts in ((TYX5, [100, 50]), (TWSBX, [150, 100])):
        raws.append(sign_tx(nonce, monkey, 0, bytes.fromhex(batch_input([erc[2:], erc[2:]], [s2, s3], amts, to)[2:]))); nonce += 1
    hashes = [send(r) for r in raws]
    time.sleep(5)
    blocks = set()
    for h in hashes:
        r = rpc("eth_getTransactionReceipt", [h])
        if r:
            blocks.add(r["blockNumber"])
            print(f"   block {r['blockNumber']} idx {r['transactionIndex']} gas {int(r['gasUsed'],16)} logs {len(r.get('logs') or [])}")
    print("   one block:", len(blocks) == 1, blocks)

    print("== 8. plasma: deposit F(30) -> exit queue -> child credits")
    call(rc, "0x" + sel("deposit()"), value=832040)
    call(rc, "0x" + sel("startDepositExit(uint256,address,uint256)") + w(10**9) + "0" * 64 + w(832040), value=1234567890)
    q = ev(rc, "0x" + sel("getNextExit(address)") + "0" * 64)
    print("   exitableAt", int(q[:64], 16), "utxoPos", int(q[64:128], 16))
    for dst in (TYX5, TWSBX):
        call(cc, "0x" + sel("depositChildChain(address,uint256,bytes,uint256)") + "0" * 24 + dst + w(233) + w(128) + w(1) + w(0))
    for n, a in (("TYX5", TYX5), ("TWSBX", TWSBX)):
        print("   JT", n, int(ev(cc, "0x" + sel("balanceOf(address)") + "0" * 24 + a), 16))

    print("== 9. action KV ingest + validation")
    migration_block = int(next(iter(blocks)), 16)
    out = subprocess.run([sys.executable, f"{HERE}/actionkv.py", hex(migration_block)],
                         capture_output=True, text=True)
    print("\n".join(out.stdout.strip().splitlines()[-6:]))
    print("   addresses:", json.dumps(A))

if __name__ == "__main__":
    main()
