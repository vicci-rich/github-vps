// TRON multisig "mail": fresh USDT transfer, signed by owner#1 (ours) and owner#2 (partner key),
// broadcast with both signatures -> api.trongrid.io  (same flow as bds-tron PublicMethed)
const { TronWeb } = require('tronweb');
const EC = require('elliptic').ec;
const ec = new EC('secp256k1');
const crypto = require('crypto');
const HDR = { 'Content-Type': 'application/json', 'TRON-PRO-API-KEY': '40ab038d-29ba-42fa-a6f1-6c47ab03898a' };
const OUR_KEY = process.env.OUR_KEY || '0x0000000000000000000000000000000000000000000000000000000000000001';
// owner #2 of the 2-of-2 multisig that holds the USDT: TY2YRt63pM7YorXV8WNpfb3qQyvyTZzzuC
// usage: node cosign.js <owner2PrivateKey>
const USDT = 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t';
const dec = (m) => { try { return Buffer.from(m, 'hex').toString(); } catch (e) { return m; } };
function signRaw(rawHex, privHex) {
  const priv = Buffer.from(privHex.replace(/^0x/, ''), 'hex');
  const digest = crypto.createHash('sha256').update(Buffer.from(rawHex, 'hex')).digest();
  const key = ec.keyFromPrivate(priv);
  const sig = ec.sign(digest, key, { canonical: true });
  const r = sig.r.toArrayLike(Buffer, 'be', 32);
  const s = sig.s.toArrayLike(Buffer, 'be', 32);
  return Buffer.concat([r, s, Buffer.from([sig.recoveryParam + 27])]).toString('hex');
}
const JOBS = [
  { to: 'TYX5yhZCJbxnCN3h5NwGVojZEFyyM2T9oy', amount: 701408733, label: 'TYX5  <- F(44) = 701.408733 USDT' },
  { to: 'TWsbxowDR7WJamz6a2c6tJS6rM9FxK2jBt', amount: 102334155, label: 'TWsbx <- F(40) = 102.334155 USDT' },
];
(async () => {
  const partner = process.argv[2];
  const tronWeb = new TronWeb({ fullHost: 'https://api.trongrid.io', headers: HDR });
  tronWeb.setPrivateKey(OUR_KEY);
  for (const job of JOBS) {
    const built = (await tronWeb.transactionBuilder.triggerSmartContract(
      USDT, 'transfer(address,uint256)', { feeLimit: 100000000, callValue: 0 },
      [{ type: 'address', value: job.to }, { type: 'uint256', value: job.amount }])).transaction;
    const sigs = [signRaw(built.raw_data_hex, OUR_KEY)];
    if (partner) sigs.push(signRaw(built.raw_data_hex, partner));
    const r = await fetch('https://api.trongrid.io/wallet/broadcasttransaction', {
      method: 'POST', headers: HDR,
      body: JSON.stringify({ raw_data: built.raw_data, raw_data_hex: built.raw_data_hex, signature: sigs }),
    });
    const res = await r.json();
    console.log(job.label, '->', res.result ? 'ACCEPTED txid=' + res.txid : (res.code + ' :: ' + dec(res.message || '').slice(0, 200)));
    if (res.txid) {
      await new Promise(r2 => setTimeout(r2, 6000));
      const bal = await tronWeb.contract([{ constant: true, inputs: [{ name: 'a', type: 'address' }], name: 'balanceOf', outputs: [{ name: '', type: 'uint256' }], type: 'function' }], USDT).balanceOf(job.to).call();
      console.log('   USDT balance of', job.to, '=', bal.toString());
    }
  }
})();
