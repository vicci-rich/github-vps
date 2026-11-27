# Justitia Web3 app

## Run

```bash
cd tronmail
npm install
npm run web3
```

Open `http://127.0.0.1:4173`.

Set `TRONGRID_API_KEY` to a Shasta-compatible TronGrid API key:

```bash
TRONGRID_API_KEY=<key> npm run web3
```

Set `TRON_ROUTER_ADDRESS` to the router contract deployed by you on Shasta.
The repository's historical `TQM...` address is a Justitia/mainnet artifact,
not evidence that the router exists on Shasta.

## Genesis deployment

Genesis state belongs to a chain's initial node configuration, not to a running
public testnet. Deploy the complete local Justitia stack with:

```bash
npm run deploy:genesis
```

The command creates a fresh foundation key, writes the Justitia genesis
configuration, restarts the local node, and deploys the existing Justitia
contracts and router from `deploy.py`.

## Shasta exchange route

The web UI builds a Shasta unsigned smart-contract transaction from one source
EOA to one destination EOA. Private keys are never accepted by the browser or
written to the app. Broadcasting is intentionally disabled; use a hardware
wallet or a separately approved operator flow to sign and broadcast.

## Push mock merge result to TronGrid

After injecting a mock log with `/api/mock-log`, call
`/api/publish-mock-merge` with the returned merge ID:

```bash
curl -X POST http://127.0.0.1:4173/api/publish-mock-merge \
  -H 'Content-Type: application/json' \
  -d '{"mergeId":"<merge-id>"}'
```

This sends the unsigned trigger request to
`https://api.trongrid.io/wallet/triggersmartcontract`. It does not broadcast,
move balances, or claim a real cross-network settlement.
