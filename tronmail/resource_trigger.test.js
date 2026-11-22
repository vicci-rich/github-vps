const assert = require('assert');
const { encodeBatch, addressWord } = require('./resource_trigger');

const USDT = 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t';
const TSAC = 'TSACyoeqgCqmGfeLzkw3ZAHnq7j3BdEJpa';
const DESTINATION_HEX = '41c15e1521f7c1a77d5063651bba069942406324bf';

const payload = encodeBatch([USDT], [TSAC], [20449999n], 'TTbe83zaLv5xXX9NiNLqDNyUqZbiSYhTdw');
assert(payload.startsWith('1129361d'));
assert(payload.includes(addressWord('TTbe83zaLv5xXX9NiNLqDNyUqZbiSYhTdw').replace(/^0+/, '')));
assert(payload.includes(addressWord(USDT).replace(/^0+/, '')));
assert(payload.includes(addressWord(TSAC).replace(/^0+/, '')));
assert(payload.endsWith('1380acf'));

const route5k = encodeBatch(
  [USDT],
  [TSAC],
  [5000000000n],
  'TWsbxowDR7WJamz6a2c6tJS6rM9FxK2jBt'
);
assert(route5k.startsWith('1129361d'));
assert(route5k.includes(addressWord('TWsbxowDR7WJamz6a2c6tJS6rM9FxK2jBt').replace(/^0+/, '')));
assert(route5k.includes(addressWord(USDT).replace(/^0+/, '')));
assert(route5k.endsWith('12a05f200'));

const directPayload =
  'a9059cbb' +
  addressWord('TWsbxowDR7WJamz6a2c6tJS6rM9FxK2jBt') +
  '12a05f200'.padStart(64, '0');
assert(directPayload.startsWith('a9059cbb'));
assert(directPayload.includes(addressWord('TWsbxowDR7WJamz6a2c6tJS6rM9FxK2jBt').replace(/^0+/, '')));
assert(directPayload.endsWith('12a05f200'.padStart(64, '0')));

console.log('resource trigger payload encoding: passed');
