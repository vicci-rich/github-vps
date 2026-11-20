<!-- Route INPUT OBJ: every migration/update request opens a PR with this
     template. The `route wspull` trigger parses these fields and the WebSocket
     pull carries the on-chain block that the PR references. All fields are
     required except notes. -->

## Record

- sha: <commit sha of the GPG-signed envelope>
- block_num: <chain block height that carries the tx>
- token_id: <numeric id, e.g. 31 for USDT on Omni>
- amount: <migrated amount>
- entity: <pig>
- state: <create|change|transfer|remove>
- author: <author email verified by gpg>

## Notes
<smaller notes, optional>
