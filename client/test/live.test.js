'use strict';

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const http = require('node:http');
const { execFile, spawn } = require('node:child_process');
const { bootstrap } = require('./helpers');

const SKIP_LIVE = process.env.ROUTE_SKIP_LIVE === '1';

const ROUTE_BIN = path.join(__dirname, '..', '..', 'route', 'bin', 'route');
const ADDR = '127.0.0.1:18080';
const BASE = 'http://' + ADDR;

function buildRoute() {
  return new Promise((resolve, reject) => {
    execFile('go', ['build', '-o', ROUTE_BIN, './cmd/route'], {
      cwd: path.join(__dirname, '..', '..', 'route')
    }, (err) => (err ? reject(err) : resolve()));
  });
}

function writeFixture() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'route-ticker-'));
  const file = path.join(dir, 'markets.json');
  fs.writeFileSync(file, JSON.stringify({
    markets: [{ trade_count: 7, pair: 'XLM/USDC' }],
    assets: [{ code: 'XLM' }]
  }));
  return file;
}

function waitForServer(timeoutMs = 5000) {
  const started = Date.now();
  return new Promise((resolve, reject) => {
    const tick = () => {
      const req = http.get(BASE + '/markets.json', (res) => {
        res.resume();
        if (res.statusCode === 200) {
          resolve();
        } else {
          retry();
        }
      });
      req.on('error', retry);
      req.setTimeout(500, () => req.destroy());
    };
    const retry = () => {
      if (Date.now() - started > timeoutMs) {
        reject(new Error('ticker did not start'));
        return;
      }
      setTimeout(tick, 100);
    };
    tick();
  });
}

test('live pass-through: ngMockE2E forwards to the running ticker', { skip: SKIP_LIVE ? 'ROUTE_SKIP_LIVE=1' : false }, async (t) => {
  await buildRoute();
  const server = spawn(ROUTE_BIN, ['ticker', '-f', writeFixture(), '-addr', ADDR], {
    stdio: 'ignore'
  });
  t.after(() => server.kill('SIGTERM'));
  await waitForServer();

  const env = bootstrap(BASE + '/');
  const injector = env.angular.injector([
    'ng', 'ngMockE2E', 'routeApp',
    function ($provide) { $provide.constant('API_BASE', BASE); }
  ]);
  const $httpBackend = injector.get('$httpBackend');
  const $rootScope = injector.get('$rootScope');
  $httpBackend.whenGET(/.*/).passThrough();

  const markets = injector.get('marketsService');
  let rows;
  markets.markets().then(function (data) { rows = data; });

  for (let i = 0; i < 50 && !rows; i++) {
    await new Promise((resolve) => setTimeout(resolve, 100));
    $rootScope.$digest();
  }

  assert.ok(rows, 'the live request never resolved');
  assert.equal(rows.length, 1);
  assert.equal(rows[0].trade_count, 7);
});

test('live direct $http call reaches the ticker', { skip: SKIP_LIVE ? 'ROUTE_SKIP_LIVE=1' : false }, async (t) => {
  await buildRoute();
  const server = spawn(ROUTE_BIN, ['ticker', '-f', writeFixture(), '-addr', ADDR], {
    stdio: 'ignore'
  });
  t.after(() => server.kill('SIGTERM'));
  await waitForServer();

  const env = bootstrap(BASE + '/');
  const injector = env.angular.injector([
    'ng', 'routeApp',
    function ($provide) { $provide.constant('API_BASE', BASE); }
  ]);
  const $http = injector.get('$http');
  const $rootScope = injector.get('$rootScope');

  let body;
  $http.get(BASE + '/markets.json').then(function (resp) { body = resp.data; });
  for (let i = 0; i < 50 && !body; i++) {
    await new Promise((resolve) => setTimeout(resolve, 100));
    $rootScope.$digest();
  }

  assert.ok(Array.isArray(body) && body.length === 1);
  assert.equal(body[0].trade_count, 7);
});
