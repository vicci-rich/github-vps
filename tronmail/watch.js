// Watch USDT/TRC10 balances on the two mainnet addresses.
const HDR = { 'Content-Type': 'application/json', 'TRON-PRO-API-KEY': '40ab038d-29ba-42fa-a6f1-6c47ab03898a' };
const API = 'https://api.trongrid.io';
const USDT = '41a614f803b6fd780986a42c78ec9c7f77e6ded13c';
const ADDRS = ['TYX5yhZCJbxnCN3h5NwGVojZEFyyM2T9oy', 'TWsbxowDR7WJamz6a2c6tJS6rM9FxK2jBt'];
const ALPHA = '123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz';
const b58ToHex = (b58) => { let n = 0n; for (const c of b58) n = n * 58n + BigInt(ALPHA.indexOf(c));
  let h = n.toString(16); if (h.length % 2) h = '0' + h; return h.padStart(50, '0').slice(0, 42); };
const post = async (p, b) => (await fetch(API + p, { method: 'POST', headers: HDR, body: JSON.stringify(b) })).json();
(async () => {
  for (;;) {
    for (const a of ADDRS) {
      const h = b58ToHex(a);
      const r = await post('/wallet/triggerconstantcontract', { owner_address: h, contract_address: USDT,
        function_selector: 'balanceOf(address)', parameter: '0'.repeat(24) + h.slice(2) });
      const v = (r.constant_result && r.constant_result[0]) ? BigInt('0x' + r.constant_result[0]) : (r.error ? 'ERR:' + (r.error.message||'').slice(0,40) : 0n);
      console.log(new Date().toISOString(), a, 'USDT', v.toString(), `(${(Number(v) / 1e6).toFixed(6)} USDT)`);
    }
    await new Promise(r => setTimeout(r, 15000));
  }
})();
