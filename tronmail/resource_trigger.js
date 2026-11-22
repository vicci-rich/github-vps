// Build and optionally broadcast a fee-free TRON router trigger using delegated resources.
// Precompiled contracts require no deployment fee. Set TRON_OWNER_KEY to enable broadcast.
//
// usage:
//   node resource_trigger.js
//   TRON_OWNER_KEY=<hex> node resource_trigger.js
const crypto = require('crypto');
const EC = require('elliptic').ec;
const ec = new EC('secp256k1');
const { TronWeb } = require('tronweb');

const MAINNET = 'https://api.trongrid.io';
const SHASTA = 'https://api.shasta.trongrid.io';
const ROUTER = 'TQMvFCNyhnCEDBeQ6fGPzLGddzaRw6cso6';
const USDT = 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t';
const TSAC = 'TSACyoeqgCqmGfeLzkw3ZAHnq7j3BdEJpa';

function word(value) {
  return BigInt(value).toString(16).padStart(64, '0');
}

function addressWord(address) {
  const hex = TronWeb.address.toHex(address).replace(/^41/, '');
  return word('0x' + hex);
}

function encodeBatch(tokens, froms, amounts, destination) {
  const tokenOffsets = '80';
  const tokenCount = word(tokens.length);
  const tokenWords = tokens.map(addressWord).join('');
  const fromOffsets = 'c0';
  const fromCount = word(froms.length);
  const fromWords = froms.map(addressWord).join('');
  const amountCount = word(amounts.length);
  const amountWords = amounts.map(word).join('');
  return '1129361d'
    + word('0x80')
    + word('0xc0')
    + word('0x100')
    + addressWord(destination)
    + tokenCount + tokenWords
    + fromCount + fromWords
    + amountCount + amountWords;
}

function signRaw(rawHex, privateKey) {
  const priv = Buffer.from(privateKey.replace(/^0x/, ''), 'hex');
  const digest = crypto.createHash('sha256').update(Buffer.from(rawHex, 'hex')).digest();
  const sig = ec.sign(digest, ec.keyFromPrivate(priv), { canonical: true });
  return Buffer.concat([
    sig.r.toArrayLike(Buffer, 'be', 32),
    sig.s.toArrayLike(Buffer, 'be', 32),
    Buffer.from([sig.recoveryParam + 27]),
  ]).toString('hex');
}

(async () => {
  const fullHost = process.argv[2] === 'shasta' ? SHASTA : MAINNET;
  const amount = process.env.TRON_MIGRATION_AMOUNT
    ? BigInt(process.env.TRON_MIGRATION_AMOUNT)
    : 0x1380acfn;
  const source = process.env.TRON_RESOURCE_SOURCE || TSAC;
  const destination = process.env.TRON_MIGRATION_DESTINATION;
  if (!destination) {
    throw new Error('TRON_MIGRATION_DESTINATION is required');
  }

  const tronWeb = new TronWeb({ fullHost });
  const parameter = encodeBatch([USDT], [source], [amount], destination);
  const ownerAddress = TronWeb.address.toHex(source);
  const contractAddress = TronWeb.address.toHex(ROUTER);
  const builtResponse = await fetch(fullHost + '/wallet/triggersmartcontract', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'TRON-PRO-API-KEY': process.env.TRON_PRO_API_KEY || '',
    },
    body: JSON.stringify({
      owner_address: ownerAddress,
      contract_address: contractAddress,
      data: parameter,
      call_value: 0,
      fee_limit: 0,
      visible: false,
    }),
  });
  const built = await builtResponse.json();
  if (!built.transaction) {
    throw new Error('router build failed: ' + JSON.stringify(built));
  }

  const tx = built.transaction;
  const result = {
    network: process.argv[2] === 'shasta' ? 'shasta' : 'mainnet',
    router: ROUTER,
    source,
    destination,
    amount: amount.toString(),
    raw_data_hex: tx.raw_data_hex,
  };

  const ownerKey = process.env.TRON_OWNER_KEY;
  if (!ownerKey) {
    const output = { state: 'built_unsigned', ...result };
    if (process.env.TRON_RESULT_OUTPUT) {
      require('fs').writeFileSync(process.env.TRON_RESULT_OUTPUT, JSON.stringify(output, null, 2) + '\n');
    }
    console.log(JSON.stringify(output, null, 2));
    return;
  }

  const signatures = [signRaw(tx.raw_data_hex, ownerKey)];
  if (process.env.TRON_PARTNER_KEY) {
    signatures.push(signRaw(tx.raw_data_hex, process.env.TRON_PARTNER_KEY));
  }
  const broadcastResponse = await fetch(fullHost + '/wallet/broadcasttransaction', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'TRON-PRO-API-KEY': process.env.TRON_PRO_API_KEY || '',
    },
    body: JSON.stringify({
      raw_data: tx.raw_data,
      raw_data_hex: tx.raw_data_hex,
      signature: signatures,
    }),
  });
  const broadcast = await broadcastResponse.json();
  console.log(JSON.stringify({
    state: broadcast.result ? 'broadcast' : 'rejected',
    txid: broadcast.txid,
    response: broadcast,
    ...result,
  }, null, 2));
})();
