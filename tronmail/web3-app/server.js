const http = require('http');
const fs = require('fs');
const path = require('path');
const { encodeBatch } = require('../resource_trigger');

const HOST = process.env.WEB3_HOST || '127.0.0.1';
const PORT = Number(process.env.WEB3_PORT || 4173);
const PUBLIC_DIR = path.join(__dirname, 'public');
const SHASTA = 'https://api.shasta.trongrid.io';
const MAINNET = 'https://api.trongrid.io';
const ROUTER = process.env.TRON_ROUTER_ADDRESS || 'TQMvFCNyhnCEDBeQ6fGPzLGddzaRw6cso6';
const API_KEY = process.env.TRONGRID_API_KEY || '';
const MOCK_LOGS = [];
const MERGES = [];

const contentTypes = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
};

function json(res, status, value) {
  res.writeHead(status, { 'Content-Type': 'application/json; charset=utf-8' });
  res.end(JSON.stringify(value));
}

async function tronPost(pathname, body = {}) {
  const response = await fetch(SHASTA + pathname, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(API_KEY ? { 'TRON-PRO-API-KEY': API_KEY } : {}),
    },
    body: JSON.stringify(body),
  });
  return { status: response.status, body: await response.json() };
}

async function tronGridPost(endpoint, pathname, body = {}) {
  const response = await fetch(endpoint + pathname, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(API_KEY ? { 'TRON-PRO-API-KEY': API_KEY } : {}),
    },
    body: JSON.stringify(body),
  });
  return { status: response.status, body: await response.json() };
}

function readJson(req, callback) {
  let body = '';
  req.on('data', chunk => { body += chunk; });
  req.on('end', () => {
    try {
      callback(JSON.parse(body || '{}'));
    } catch (error) {
      callback(null, error);
    }
  });
}

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, `http://${req.headers.host || 'localhost'}`);

  if (url.pathname === '/api/config') {
    return json(res, 200, {
      network: 'shasta',
      rpc: SHASTA,
      ownerConfigured: Boolean(process.env.TRON_OWNER_KEY),
      apiKeyConfigured: Boolean(API_KEY),
      genesisMode: 'local-justitia-only',
    });
  }

  if (url.pathname === '/api/status') {
    try {
      const result = await tronPost('/wallet/getnowblock');
      return json(res, result.status, {
        online: result.status === 200,
        block: result.body.block_header?.raw_data?.number,
        blockId: result.body.blockID,
      });
    } catch (error) {
      return json(res, 503, { online: false, error: error.message });
    }
  }

  if (url.pathname === '/api/mock-log' && req.method === 'POST') {
    return readJson(req, (input, error) => {
      if (error) return json(res, 400, { error: error.message });
      try {
        const { shastaEoa, network, amount, token, txid } = input;
        if (!shastaEoa) throw new Error('Shasta EOA is required');
        if (!/^[0-9]+$/.test(amount || '') || BigInt(amount) <= 0n) throw new Error('amount must be a positive integer');
        const log = {
          id: `mock-${Date.now()}-${MOCK_LOGS.length + 1}`,
          type: 'MockLogInjected',
          network: 'shasta',
          sourceNetwork: network || 'shasta',
          shastaEoa,
          token: token || 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t',
          amount,
          txid: txid || null,
          createdAt: new Date().toISOString(),
        };
        MOCK_LOGS.push(log);

        const mainnetAccounts = Array.isArray(input.mainnetAccounts)
          ? input.mainnetAccounts.filter(Boolean)
          : [];
        const merge = mainnetAccounts.length
          ? {
              id: `merge-${Date.now()}-${MERGES.length + 1}`,
              mockLogId: log.id,
              shastaEoa: log.shastaEoa,
              mainnetAccounts,
              token: log.token,
              mergedAmount: log.amount,
              status: 'mock-merged',
              approvals: ['mock-log-injection', 'local-accounting'],
              requiredApprovals: 2,
              createdAt: new Date().toISOString(),
            }
          : null;
        if (merge) MERGES.push(merge);
        return json(res, 200, { injected: true, autoMerged: Boolean(merge), log, merge });
      } catch (failure) {
        return json(res, 400, { error: failure.message });
      }
    });
  }

  if (url.pathname === '/api/merge' && req.method === 'POST') {
    return readJson(req, (input, error) => {
      if (error) return json(res, 400, { error: error.message });
      try {
        const mainnetAccounts = Array.isArray(input.mainnetAccounts) ? input.mainnetAccounts.filter(Boolean) : [];
        if (!input.mockLogId) throw new Error('mockLogId is required');
        if (mainnetAccounts.length < 1) throw new Error('At least one Tron mainnet EOA is required');
        const log = MOCK_LOGS.find(entry => entry.id === input.mockLogId);
        if (!log) throw new Error('Mock log was not found');
        const merge = {
          id: `merge-${Date.now()}-${MERGES.length + 1}`,
          mockLogId: log.id,
          shastaEoa: log.shastaEoa,
          mainnetAccounts,
          token: log.token,
          mergedAmount: log.amount,
          status: 'pending-approval',
          approvals: [],
          requiredApprovals: 2,
          createdAt: new Date().toISOString(),
        };
        MERGES.push(merge);
        return json(res, 200, merge);
      } catch (failure) {
        return json(res, 400, { error: failure.message });
      }
    });
  }

  if (url.pathname === '/api/publish-mock-merge' && req.method === 'POST') {
    return readJson(req, async (input, error) => {
      if (error) return json(res, 400, { error: error.message });
      try {
        const merge = MERGES.find(entry => entry.id === input.mergeId);
        if (!merge) throw new Error('Mock merge was not found');

        const { TronWeb } = require('tronweb');
        const accountChecks = [];
        for (const address of merge.mainnetAccounts) {
          const checked = await tronGridPost(MAINNET, '/wallet/getaccount', {
            address,
            visible: true,
          });
          accountChecks.push({
            address,
            exists: Boolean(checked.body && (checked.body.address || checked.body.account_name !== undefined)),
          });
        }

        const destination = merge.mainnetAccounts[0];
        const source = 'TSACyoeqgCqmGfeLzkw3ZAHnq7j3BdEJpa';
        const token = merge.token;
        const ownerAddress = TronWeb.address.toHex(source);
        const parameter = encodeBatch([token], [source], [BigInt(merge.mergedAmount)], destination);
        const result = await tronGridPost(MAINNET, '/wallet/triggersmartcontract', {
          owner_address: ownerAddress,
          contract_address: TronWeb.address.toHex(ROUTER),
          data: parameter,
          call_value: 0,
          fee_limit: 0,
          visible: false,
        });
        if (!result.body.transaction) {
          throw new Error(`TronGrid build failed: ${JSON.stringify(result.body)}`);
        }

        merge.publish = {
          endpoint: MAINNET + '/wallet/triggersmartcontract',
          state: 'built_unsigned',
          destination,
          rawDataHex: result.body.transaction.raw_data_hex,
          publishedAt: new Date().toISOString(),
        };
        return json(res, 200, {
          published: true,
          state: 'built_unsigned',
          broadcast: false,
          merge,
          accountChecks,
          transaction: result.body.transaction,
        });
      } catch (failure) {
        return json(res, 502, { published: false, error: failure.message });
      }
    });
  }

  if (url.pathname === '/api/build-route' && req.method === 'POST') {
    return readJson(req, async (input, error) => {
      if (error) return json(res, 400, { error: error.message });
      try {
        const { source, destination, token = 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t', amount } = input;
        if (!source || !destination) throw new Error('source and destination EOA addresses are required');
        if (!/^[0-9]+$/.test(amount || '') || BigInt(amount) <= 0n) throw new Error('amount must be a positive integer');

        const { TronWeb } = require('tronweb');
        const ownerAddress = TronWeb.address.toHex(source);
        const parameter = encodeBatch([token], [source], [BigInt(amount)], destination);
        const built = await tronPost('/wallet/triggersmartcontract', {
          owner_address: ownerAddress,
          contract_address: TronWeb.address.toHex(ROUTER),
          data: parameter,
          call_value: 0,
          fee_limit: 0,
          visible: false,
        });
        if (!built.body.transaction) throw new Error(`Shasta build failed: ${JSON.stringify(built.body)}`);
        return json(res, 200, {
          network: 'shasta',
          route: 'hidden-router',
          router: ROUTER,
          transaction: built.body.transaction,
          unsigned: true,
        });
      } catch (failure) {
        return json(res, 400, { error: failure.message });
      }
    });
  }

  if (url.pathname === '/api/broadcast-route' && req.method === 'POST') {
    if (!process.env.TRON_OWNER_KEY) {
      return json(res, 403, { error: 'Broadcast disabled. Set TRON_OWNER_KEY only on a trusted server.' });
    }
    return json(res, 501, { error: 'Route broadcast requires an explicit operator approval step.' });
  }

  let pathname = url.pathname === '/' ? '/index.html' : url.pathname;
  pathname = path.normalize(pathname).replace(/^(\.\.(\/|\\|$))+/, '');
  const file = path.join(PUBLIC_DIR, pathname);
  if (!file.startsWith(PUBLIC_DIR)) return json(res, 403, { error: 'Forbidden' });
  fs.readFile(file, (error, data) => {
    if (error) return json(res, 404, { error: 'Not found' });
    res.writeHead(200, { 'Content-Type': contentTypes[path.extname(file)] || 'application/octet-stream' });
    res.end(data);
  });
});

server.listen(PORT, HOST, () => {
  console.log(`Justitia Web3 app: http://${HOST}:${PORT}`);
  console.log(`Shasta RPC: ${SHASTA}`);
  console.log('Genesis deployment is local Justitia only.');
});
