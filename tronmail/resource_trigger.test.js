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

console.log('resource trigger payload encoding: passed');
