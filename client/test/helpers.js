'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { JSDOM } = require('jsdom');

const angularSrc = fs.readFileSync(require.resolve('angular/angular.js'), 'utf8');
const mocksSrc = fs.readFileSync(require.resolve('angular-mocks/angular-mocks.js'), 'utf8');
const appSrc = fs.readFileSync(path.join(__dirname, '..', 'app.js'), 'utf8');

// bootstrap creates a jsdom window with AngularJS, angular-mocks and the app
// loaded, so the tests exercise the same code the browser runs.
function bootstrap(url = 'http://localhost/') {
  const dom = new JSDOM('<!doctype html><html><body></body></html>', {
    url,
    runScripts: 'outside-only'
  });
  const { window } = dom;
  window.eval(angularSrc);
  window.eval(mocksSrc);
  window.eval(appSrc);
  return { dom, window, angular: window.angular };
}

module.exports = { bootstrap };
