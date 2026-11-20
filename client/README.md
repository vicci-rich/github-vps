# client — AngularJS view of the route stack

Two services, one page:

- `marketsService` → `/markets.json`, `/assets.json` (the ticker)
- `recordService` → `/proof/{sha}` (the light node)

`API_BASE` is empty when the page is served from the same origin; tests set it
to an absolute URL.

## Run

```
npm install
# terminal 1: the ticker on the fixture
../route/bin/route ticker -f markets.json -addr 127.0.0.1:8000
# terminal 2: any static server for this directory, e.g.
python3 -m http.server 8080
# open http://127.0.0.1:8080/
```

## Tests

```
npm test        # node --test
```

The suite runs AngularJS 1.8.3 in jsdom and covers both modes the plan asks
for:

- **stubs**: `ngMock.$httpBackend.expectGET(...).respond(...)` + `flush()`,
  controller model population, error flag, and `/proof/{sha}` routing;
- **e2e stubs**: `ngMockE2E.$httpBackend.whenGET(...).respond(...)` (no
  `flush()` — the e2e backend answers through the real XHR path);
- **live pass-through**: an `ngMockE2E` backend with
  `whenGET(/.*/).passThrough()` against a real `route ticker` process, plus a
  plain `$http` call to the same server.

9 tests, all passing. Node's built-in coverage (`node --test
--experimental-test-coverage`) reports `app.js` at 100% of its statements,
because the tests drive the actual services and controller.

## Notes

- The ticker here is `route ticker` (same two endpoints as bds-xlm's ticker,
  fixture-backed). bds-xlm's Postgres-backed ticker remains the production
  implementation; the client depends only on the endpoint contract.
- No keys, no network calls to public chains: the live tests bind to
  `127.0.0.1:18080` only.

## Licensing

Apache-2.0 (see `LICENSE`); the vendored `angular` and `angular-mocks`
packages keep their MIT license in `node_modules`.
