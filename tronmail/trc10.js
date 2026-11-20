// TRC10 mail: TransferAssetContract built byte-exact (protocol.proto:
// asset_name=1, owner_address=2, to_address=3, amount=4), signed SHA256/secp256k1
// over the node's own raw_data, broadcast to api.trongrid.io.
//
// usage: node trc10.js [owner2PrivateKey]
const EC = require('elliptic').ec;
const crypto = require('crypto');
const ec = new EC('secp256k1');
const HDR = { 'Content-Type': 'application/json', 'TRON-PRO-API-KEY': '40ab038d-29ba-42fa-a6f1-6c47ab03898a' };
const API = 'https://api.trongrid.io';
const OUR_KEY = process.env.OUR_KEY || '0x0000000000000000000000000000000000000000000000000000000000000001';
const OWNER_HEX = '417e5f4552091a69125d5dfcb7b8c2659029395bdf'; // TMVQGm1q... (owner #1)
const ALPHA = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';

const vi = (n) => { const o = []; n = Math.floor(n); do { let b = n % 128; n = Math.floor(n / 128); o.push(b | (n ? 0x80 : 0)); } while (n); return Buffer.from(o); };
const ld = (f, b) => Buffer.concat([vi((f << 3) | 2), vi(b.length), b]);
const vb = (f, v) => Buffer.concat([vi((f << 3) | 0), vi(v)]);
const hexToBuf = (h) => Buffer.from(h, 'hex');
function b58ToHex(b58) {
  let n = 0n; for (const c of b58) n = n * 58n + BigInt(ALPHA.indexOf(c));
  let h = n.toString(16); if (h.length % 2) h = '0' + h;
  return h.padStart(50, '0').slice(0, 42); // 0x41 + 20 bytes (drop base58check)
}
function signRaw(rawHex, privHex) {
  const priv = Buffer.from(privHex.replace(/^0x/, ''), 'hex');
  const digest = crypto.createHash('sha256').update(Buffer.from(rawHex, 'hex')).digest();
  const sig = ec.sign(digest, ec.keyFromPrivate(priv), { canonical: true });
  return Buffer.concat([sig.r.toArrayLike(Buffer, 'be', 32), sig.s.toArrayLike(Buffer, 'be', 32), Buffer.from([sig.recoveryParam + 27])]).toString('hex');
}
const post = async (path, body) => {
  const r = await fetch(API + path, { method: 'POST', headers: HDR, body: JSON.stringify(body) });
  return r.json();
};
const dec = (m) => { try { return Buffer.from(m, 'hex').toString(); } catch (e) { return m; } };

// TRC10 with resources (trx_num) and 1e18 supply, held by our account
const JOBS = [
  { assetId: 1003657, name: 'WIP', supply: '1e18', res: 34000000, to: 'TYX5yhZCJbxnCN3h5NwGVojZEFyyM2T9oy', amount: 50254411 },
  { assetId: 1003657, name: 'WIP', supply: '1e18', res: 34000000, to: 'TWsbxowDR7WJamz6a2c6tJS6rM9FxK2jBt', amount: 50254411 },
  { assetId: 1003846, name: 'USDZ', supply: '9e18', res: 18000000, to: 'TWsbxowDR7WJamz6a2c6tJS6rM9FxK2jBt', amount: 10000000 },
];

(async () => {
  const partner = process.argv[2];
  const nb = await post('/wallet/getnowblock', {});
  const num = nb.block_header.raw_data.number;
  const ref = Buffer.from(num.toString(16).padStart(16, '0').slice(-4), 'hex');
  const rh = hexToBuf(nb.blockID).slice(8, 16);
  const ts = Date.now();
  for (const j of JOBS) {
    const owner = hexToBuf(OWNER_HEX), to = hexToBuf(b58ToHex(j.to));
    const value = Buffer.concat([
      ld(1, Buffer.from(String(j.assetId), 'utf8')),
      ld(2, owner), ld(3, to), vb(4, j.amount),
    ]);
    const anyv = Buffer.concat([
      ld(1, Buffer.from('type.googleapis.com/protocol.TransferAssetContract', 'utf8')),
      ld(2, value),
    ]);
    const raw = Buffer.concat([
      ld(1, ref), ld(4, rh), vb(8, ts + 60000),
      ld(11, Buffer.concat([vb(1, 2), ld(2, anyv)])), // contract type 2 = TransferAssetContract
      vb(14, ts), vb(18, 100000000),
    ]);
    const rawHex = raw.toString('hex');
    const json = {
      raw_data: {
        ref_block_bytes: ref.toString('hex'), ref_block_hash: rh.toString('hex'),
        expiration: ts + 60000,
        contract: [{
          parameter: { value: { asset_name: String(j.assetId), owner_address: OWNER_HEX, to_address: b58ToHex(j.to), amount: j.amount },
            type_url: 'type.googleapis.com/protocol.TransferAssetContract' },
          type: 'TransferAssetContract',
        }],
        timestamp: ts, fee_limit: 100000000,
      },
      raw_data_hex: rawHex,
      signature: [signRaw(rawHex, OUR_KEY), ...(partner ? [signRaw(rawHex, partner)] : [])],
    };
    const res = await post('/wallet/broadcasttransaction', json);
    console.log(`${j.name}(TRC10 ${j.supply} supply, ${j.res} TRX res) -> ${j.to.slice(0, 10)} amount ${j.amount} ::`,
      res.result ? 'ACCEPTED txid=' + res.txid : (res.code + ' ' + dec(res.message || '').slice(0, 150)));
  }
})();
