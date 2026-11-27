// TRON multisig "mail" -> api.trongrid.io
//
// Flow (same as bds-tron/src/test/java/stest/tron/wallet): build the tx node-side
// (/wallet/triggersmartcontract | /wallet/transferasset), sign the node's own raw_data
// bytes with each owner key (SHA256 + secp256k1, low-S, v=recid 0/1), broadcast all
// signatures to /wallet/broadcasttransaction.
//
// Verified against the live mainnet node:
//   * signature layout  = r(32) || s(32) || recoveryId(0|1)   (java-tron
//     ECDSASignature.toByteArray(): the header byte is v-27, NOT 27+recid)
//   * digest            = SHA256(raw_data bytes)
//   * TaPoS             = ref_block_bytes = block number bytes [6:8]
//                          ref_block_hash  = block id bytes [8:16]
//   * /wallet/transferasset on the public API answers "No asset!" even for assets the
//     account really holds -> the TransferAssetContract raw_data is assembled here
//     (protocol.TransferAssetContract: asset_name=1, owner_address=2, to_address=3,
//     amount=4; Transaction.raw: ref_block_bytes=1, ref_block_hash=4, expiration=8,
//     contract=11, timestamp=14)
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
const OWNER = '417e5f4552091a69125d5dfcb7b8c2659029395bdf';
const USDT = 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t';
const dec = (m) => { try { return Buffer.from(m, 'hex').toString(); } catch (e) { return m; } };
const ALPHA = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';
function b58ToHex(b58) {
  let n = 0n; for (const c of b58) n = n * 58n + BigInt(ALPHA.indexOf(c));
  let hex = n.toString(16); if (hex.length % 2) hex = '0' + hex;
  hex = hex.padStart(50, '0');           // 21 bytes + 4 byte base58check
  return hex.slice(0, 42);               // 0x41 + 20 byte address
}

// ---- minimal protobuf writer (TRON wire format) -------------------------------
function varint(value) {
  let n = typeof value === 'bigint' ? value : BigInt(value);
  const out = [];
  do {
    let b = Number(n & 0x7fn);
    n >>= 7n;
    if (n) b |= 0x80;
    out.push(b);
  } while (n);
  return Buffer.from(out);
}
function pbBytes(num, buf) {
  if (!buf || !buf.length) return Buffer.alloc(0);
  return Buffer.concat([varint((num << 3) | 2), varint(buf.length), buf]);
}
function pbVarint(num, value) {
  if (!value) return Buffer.alloc(0);
  return Buffer.concat([varint((num << 3) | 0), varint(value)]);
}
function pbString(num, text) { return pbBytes(num, Buffer.from(text)); }

const TYPE_IDS = { TransferContract: 1, TransferAssetContract: 2, TriggerSmartContract: 31 };
const TYPE_URL = {
  TransferContract: 'type.googleapis.com/protocol.TransferContract',
  TransferAssetContract: 'type.googleapis.com/protocol.TransferAssetContract',
};

// SHA256 over the raw_data bytes, low-S canonical, header byte = recovery id (0|1)
function signRaw(rawHex, privHex) {
  const priv = Buffer.from(privHex.replace(/^0x/, ''), 'hex');
  const digest = crypto.createHash('sha256').update(Buffer.from(rawHex, 'hex')).digest();
  const key = ec.keyFromPrivate(priv);
  const sig = ec.sign(digest, key);
  // elliptic reports recoveryParam as (r parity | s-is-high); TRON wants the
  // canonical low-s signature and the plain recovery id (0|1)
  const half = ec.n.shrn(1);
  let s = sig.s;
  let recovery = sig.recoveryParam & 1;
  if (s.cmp(half) > 0) { s = ec.n.sub(s); recovery = recovery ^ 1; }
  return Buffer.concat([
    sig.r.toArrayLike(Buffer, 'be', 32),
    s.toArrayLike(Buffer, 'be', 32),
    Buffer.from([recovery]),
  ]).toString('hex');
}

// ---- TRC10: TransferAssetContract raw_data assembled locally ------------------
async function buildTransferAsset(ownerHex, toHex, assetId, amount) {
  const block = await (await fetch(API + '/wallet/getnowblock', { headers: HDR })).json();
  const num = BigInt(block.block_header.raw_data.number);
  const blockId = Buffer.from(block.blockID, 'hex');
  const refBlockBytes = Buffer.from(num.toString(16).padStart(16, '0').slice(12), 'hex'); // bytes [6:8]
  const refBlockHash = blockId.subarray(8, 16);
  const now = Date.now();

  const value = Buffer.concat([
    pbBytes(1, Buffer.from(String(assetId))),                  // asset_name (the id as text)
    pbBytes(2, Buffer.from(ownerHex, 'hex')),                  // owner_address
    pbBytes(3, Buffer.from(toHex, 'hex')),                     // to_address
    pbVarint(4, amount),                                       // amount
  ]);
  const contract = Buffer.concat([
    pbVarint(1, TYPE_IDS.TransferAssetContract),
    pbBytes(2, Buffer.concat([pbString(1, TYPE_URL.TransferAssetContract), pbBytes(2, value)])),
  ]);
  const raw = Buffer.concat([
    pbBytes(1, refBlockBytes),
    pbBytes(4, refBlockHash),
    pbVarint(8, now + 60000000),                          // expiration
    pbBytes(11, contract),
    pbVarint(14, now),                                   // timestamp
  ]);
  const rawData = {
    contract: [{
      parameter: {
        type_url: TYPE_URL.TransferAssetContract,
        // TRON's JSON encodes bytes as hex - asset_name carries the asset id as text
        value: { asset_name: Buffer.from(String(assetId)).toString('hex'), owner_address: ownerHex, to_address: toHex, amount },
      },
      type: 'TransferAssetContract', provider: '', ContractName: '', permission_id: 0,
    }],
    ref_block_bytes: refBlockBytes.toString('hex'),
    ref_block_hash: refBlockHash.toString('hex'),
    expiration: now + 60000000,
    timestamp: now,
  };
  return { raw_data: rawData, raw_data_hex: raw.toString('hex') };
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
    const built = await buildTransferAsset(OWNER, b58ToHex(j.to), j.assetId, j.amount);
    await broadcast(built, j.label);
  }
})();
