// TRON multisig "mail" -> api.trongrid.io
//
// Flow (same as bds-tron/src/test/java/stest/tron/wallet): build the tx node-side
// (/wallet/triggersmartcontract | /wallet/transferasset), sign the node's own raw_data
// bytes with each owner key (SHA256 + secp256k1, low-S, v=27+rec), broadcast all
// signatures to /wallet/broadcasttransaction.
//
// usage: node cosign.js [owner2PrivateKey]
const { TronWeb } = require('tronweb');
const EC = require('elliptic').ec;
const { getTronGridApiKey } = require('./trongrid_config');
const ec = new EC('secp256k1');
const crypto = require('crypto');
const HDR = { 'Content-Type': 'application/json', 'TRON-PRO-API-KEY': getTronGridApiKey() };
const API = 'https://api.trongrid.io';
// owner #1 of the 2-of-2 that holds the assets (417e5f4552091a69125d5dfcb7b8c2659029395bdf)
const OUR_KEY = process.env.OUR_KEY || '0x0000000000000000000000000000000000000000000000000000000000000001';
const USDT = 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t';
const dec = (m) => { try { return Buffer.from(m, 'hex').toString(); } catch (e) { return m; } };
const ALPHA = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';
function b58ToHex(b58) {
  let n = 0n;
  for (const c of b58) n = n * 58n + BigInt(ALPHA.indexOf(c));
  let hex = n.toString(16); if (hex.length % 2) hex = '0' + hex;
  hex = hex.padStart(50, '0');           // 21 bytes + 4 byte base58check
  return hex.slice(0, 42);               // 0x41 + 20 byte address
}

function signRaw(rawHex, privHex) {
  const priv = Buffer.from(privHex.replace(/^0x/, ''), 'hex');
  const digest = crypto.createHash('sha256').update(Buffer.from(rawHex, 'hex')).digest();
  const sig = ec.sign(digest, ec.keyFromPrivate(priv), { canonical: true });
  return Buffer.concat([
    sig.r.toArrayLike(Buffer, 'be', 32),
    sig.s.toArrayLike(Buffer, 'be', 32),
    Buffer.from([sig.recoveryParam + 27]),
  ]).toString('hex');
}

const TRC20 = [
  { to: 'TYX5yhZCJbxnCN3h5NwGVojZEFyyM2T9oy', amount: 701408733, label: 'USDT(TRC20) -> TYX5  F(44)=701.408733' },
  { to: 'TWsbxowDR7WJamz6a2c6tJS6rM9FxK2jBt', amount: 102334155, label: 'USDT(TRC20) -> TWsbx F(40)=102.334155' },
];
// TRC10 with resources (trx_num) and 1e18 supply, held by our account
const TRC10 = [
  { assetId: 1003657, name: 'WIP', supply: '1e18', res: 34000000, to: 'TYX5yhZCJbxnCN3h5NwGVojZEFyyM2T9oy', amount: 50254411, label: 'WIP(TRC10 1e18) -> TYX5' },
  { assetId: 1003846, name: 'USDZ', supply: '9e18', res: 18000000, to: 'TWsbxowDR7WJamz6a2c6tJS6rM9FxK2jBt', amount: 10000000, label: 'USDZ(TRC10) -> TWsbx' },
];

(async () => {
  const partner = process.argv[2];
  const tronWeb = new TronWeb({ fullHost: API, headers: HDR });
  tronWeb.setPrivateKey(OUR_KEY);

  const broadcast = async (tx, label) => {
    const sigs = [signRaw(tx.raw_data_hex, OUR_KEY)];
    if (partner) sigs.push(signRaw(tx.raw_data_hex, partner));
    const r = await fetch(API + '/wallet/broadcasttransaction', {
      method: 'POST', headers: HDR,
      body: JSON.stringify({ raw_data: tx.raw_data, raw_data_hex: tx.raw_data_hex, signature: sigs }),
    });
    const res = await r.json();
    console.log(label, '->', res.result ? 'ACCEPTED txid=' + res.txid : (res.code + ' :: ' + dec(res.message || '').slice(0, 200)));
    return res;
  };

  for (const j of TRC20) {
    const built = (await tronWeb.transactionBuilder.triggerSmartContract(
      USDT, 'transfer(address,uint256)', { feeLimit: 100000000, callValue: 0 },
      [{ type: 'address', value: j.to }, { type: 'uint256', value: j.amount }])).transaction;
    await broadcast(built, j.label);
  }

  for (const j of TRC10) {
    const r = await fetch(API + '/wallet/transferasset', {
      method: 'POST', headers: HDR,
      body: JSON.stringify({ owner_address: '417e5f4552091a69125d5dfcb7b8c2659029395bdf', to_address: b58ToHex(j.to), asset_id: String(j.assetId), amount: j.amount }),
    });
    const res = await r.json();
    if (!res.transaction) { console.log(j.label, '-> build failed:', JSON.stringify(res).slice(0, 160)); continue; }
    await broadcast(res.transaction, j.label);
  }
})();
