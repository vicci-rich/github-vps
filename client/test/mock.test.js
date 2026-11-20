'use strict';

const test = require('node:test');
const assert = require('node:assert');
const { bootstrap } = require('./helpers');

test('ngMock $httpBackend stubs /markets.json', () => {
  const env = bootstrap();
  const injector = env.angular.injector(['ng', 'ngMock', 'routeApp']);
  const $httpBackend = injector.get('$httpBackend');
  const markets = injector.get('marketsService');

  let rows;
  $httpBackend.expectGET('/markets.json').respond([{ trade_count: 3, pair: 'XLM/USDC' }]);
  markets.markets().then(function (data) { rows = data; });
  $httpBackend.flush();

  assert.equal(rows.length, 1);
  assert.equal(rows[0].trade_count, 3);
  $httpBackend.verifyNoOutstandingExpectation();
  $httpBackend.verifyNoOutstandingRequest();
});

test('ngMock $httpBackend stubs /assets.json', () => {
  const env = bootstrap();
  const injector = env.angular.injector(['ng', 'ngMock', 'routeApp']);
  const $httpBackend = injector.get('$httpBackend');
  const markets = injector.get('marketsService');

  let assets;
  $httpBackend.whenGET('/assets.json').respond([{ code: 'XLM' }]);
  markets.assets().then(function (data) { assets = data; });
  $httpBackend.flush();

  assert.equal(assets[0].code, 'XLM');
});

test('ngMockE2E whenGET stub drives the same service', async () => {
  const env = bootstrap();
  const injector = env.angular.injector(['ng', 'ngMockE2E', 'routeApp']);
  const $httpBackend = injector.get('$httpBackend');
  const $rootScope = injector.get('$rootScope');
  const markets = injector.get('marketsService');

  // ngMockE2E responds through the real XHR path, so there is no flush():
  // wait for the promise and digest until it settles.
  $httpBackend.whenGET('/markets.json').respond([{ trade_count: 9 }]);
  let rows;
  markets.markets().then(function (data) { rows = data; });
  for (let i = 0; i < 20 && !rows; i++) {
    await new Promise((resolve) => setTimeout(resolve, 50));
    $rootScope.$digest();
  }
  assert.ok(rows, 'the e2e stub never responded');
  assert.equal(rows[0].trade_count, 9);
});

test('the controller populates its model from the stubbed backend', () => {
  const env = bootstrap();
  const injector = env.angular.injector(['ng', 'ngMock', 'routeApp']);
  const $httpBackend = injector.get('$httpBackend');
  const $controller = injector.get('$controller');

  $httpBackend.expectGET('/markets.json').respond([{ trade_count: 1 }, { trade_count: 2 }]);
  const vm = $controller('MarketsController');
  assert.deepEqual(vm.markets, []);
  $httpBackend.flush();
  assert.equal(vm.markets.length, 2);
});

test('a failing backend surfaces the controller error flag', () => {
  const env = bootstrap();
  const injector = env.angular.injector(['ng', 'ngMock', 'routeApp']);
  const $httpBackend = injector.get('$httpBackend');
  const $controller = injector.get('$controller');

  $httpBackend.expectGET('/markets.json').respond(503, 'down');
  const vm = $controller('MarketsController');
  $httpBackend.flush();
  assert.equal(vm.error, 'markets unavailable');
});

test('recordService asks /proof/{sha}', () => {
  const env = bootstrap();
  const injector = env.angular.injector(['ng', 'ngMock', 'routeApp']);
  const $httpBackend = injector.get('$httpBackend');
  const records = injector.get('recordService');

  let doc;
  $httpBackend.expectGET('/proof/abc1234').respond({ sha: 'abc1234', gpg: { verified: 'true' } });
  records.get('abc1234').then(function (data) { doc = data; });
  $httpBackend.flush();

  assert.equal(doc.gpg.verified, 'true');
});
