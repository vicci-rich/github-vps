#!/usr/bin/env python3
"""Justitia fork driver: deploys the stack, runs the migration, batches txs in one block,
and pushes the result to api.trongrid.io as raw protobuf mail."""
import json, hashlib, os, time, urllib.request
from ecdsa import SigningKey, SECP256k1
from ecdsa.util import sigdecode_string
from ecdsa.keys import VerifyingKey
from Crypto.Hash import keccak

RPC = os.environ.get("JUSTITIA_RPC", "http://127.0.0.1:47768")
KEY = os.environ.get("FOUNDATION_KEY","").strip()
SK = None
FROM = ""

def set_key(priv_hex):
    """Bind the signing key (foundation) used for sends."""
    global KEY, SK, FROM
    KEY = priv_hex.strip()
    SK = SigningKey.from_string(bytes.fromhex(KEY), curve=SECP256k1)
    FROM = keccak.new(digest_bits=256, data=SK.get_verifying_key().to_string()).hexdigest()[-40:]
    return FROM

if KEY:
    set_key(KEY)

def rpc(method, params):
    req = urllib.request.Request(RPC, data=json.dumps({"jsonrpc":"2.0","method":method,"params":params,"id":1}).encode(),
                                 headers={"Content-Type":"application/json"})
    with urllib.request.urlopen(req, timeout=20) as r:
        return json.loads(r.read())["result"]

def rlp(item):
    if isinstance(item, int):
        if item == 0: return b"\x80"
        item = item.to_bytes((item.bit_length()+7)//8, "big")
    if isinstance(item, (bytes, bytearray)):
        b = bytes(item)
        if len(b) == 1 and b[0] < 0x80: return b
        if len(b) <= 55: return bytes([0x80+len(b)])+b
        lb = len(b).to_bytes((len(b).bit_length()+7)//8, "big")
        return bytes([0xb7+len(lb)])+lb+b
    payload = b"".join(rlp(x) for x in item)
    if len(payload) <= 55: return bytes([0xc0+len(payload)])+payload
    lb = len(payload).to_bytes((len(payload).bit_length()+7)//8, "big")
    return bytes([0xf7+len(lb)])+lb+payload

def sign_tx(nonce, to, value, data, gas=3000000, gas_price=1, sk=None):
    sk = sk or SK
    if isinstance(to, str): to = bytes.fromhex(to[2:] if to.startswith("0x") else to)
    if isinstance(data, str): data = bytes.fromhex(data[2:] if data.startswith("0x") else data)
    unsigned = [nonce, gas_price, gas, to, value, data]
    pre = rlp(unsigned + [0, b"", b""])
    digest = keccak.new(digest_bits=256, data=pre).digest()
    sig = sk.sign_digest_deterministic(digest, hashfunc=hashlib.sha256,
                                       sigencode=lambda r, s, o: r.to_bytes(32,"big")+s.to_bytes(32,"big"))
    recid = 0
    for c in (0,1):
        try:
            vk = VerifyingKey.from_public_key_recovery_with_digest(sig, digest, SECP256k1, hashfunc=hashlib.sha256, sigdecode=sigdecode_string)[c]
            if vk.to_string() == sk.get_verifying_key().to_string(): recid = c; break
        except Exception: pass
    # canonical (low-S) signature, as required by the txpool
    N = SECP256k1.order
    rs = sig[:32]; ss = sig[32:]
    s_int = int.from_bytes(ss, "big")
    if s_int > N // 2:
        ss = (N - s_int).to_bytes(32, "big")
        recid ^= 1
    sig = rs + ss
    v = recid + 35   # EIP-155 chainId 0
    signed = rlp(unsigned + [v, sig[:32], sig[32:]])
    return "0x" + signed.hex()

def send(raw):
    return rpc("eth_sendRawTransaction", [raw])

def wait(hash_, tries=40):
    for _ in range(tries):
        r = rpc("eth_getTransactionReceipt", [hash_])
        if r: return r
        time.sleep(0.4)
    return None

def deploy(code, args=b""):
    nonce = int(rpc("eth_getTransactionCount", ["0x"+FROM, "pending"]), 16)
    h = send(sign_tx(nonce, b"", 0, bytes.fromhex(code[2:] if code.startswith("0x") else code)+args))
    r = wait(h)
    return r["contractAddress"], h, r

def call(to, data, value=0, gas=3000000, sk=None, frm=None):
    frm = frm or (keccak.new(digest_bits=256, data=(sk or SK).get_verifying_key().to_string()).hexdigest()[-40:])
    nonce = int(rpc("eth_getTransactionCount", ["0x"+frm, "pending"]), 16)
    h = send(sign_tx(nonce, to, value, data, gas, sk=sk))
    return h, wait(h)

def w(x): return format(x, "064x")
def sel(sig): return keccak.new(digest_bits=256, data=sig.encode()).hexdigest()[:8]
def addr_call(to, sig, args): return "0x"+sel(sig)+args
def batch_input(tokens, froms, amounts, to):
    def A(a): return "0"*24+a
    tok = w(len(tokens)) + "".join(A(t) for t in tokens)
    frm = w(len(froms)) + "".join(A(f) for f in froms)
    amt = w(len(amounts)) + "".join(w(a) for a in amounts)
    off_t = 0x80
    off_f = off_t + len(tok)//2
    off_a = off_f + len(frm)//2
    head = w(off_t)+w(off_f)+w(off_a)+w(int(to,16))
    return "0x"+"1129361d"+head+tok+frm+amt

def ev(to, data): return rpc("eth_call", [{"to":to,"data":data}, "latest"])
