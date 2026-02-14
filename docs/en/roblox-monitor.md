# Roblox Join Monitor

## Overview

- Site: `roblox`
- Type: `join`
- Events: `enter` and `leave`
- Notify payload enrichments (best effort):
  - game name from Games API (fallback to `lastLocation`)
  - game thumbnail icon from Thumbnails API (fallback to universe thumbnail)
  - default Chinese text template for group notify
- Join URL strategy: try `gamejoin API` first, then fallback to deep link

## Command Examples

```text
/watch -s roblox -t join 123456
/watch -s roblox -t join builderman
/unwatch -s roblox -t join 123456
/list -s roblox
```

Notes:

- `id` accepts both numeric Roblox `UserId` and username.
- If the user does not exist, `watch` returns a readable error and does not subscribe.

## Config (application.v2.yaml only)

Roblox supports v2 keys only. `roblox.*` keys in `application.yaml` are ignored.

```yaml
providers:
  roblox:
    enabled: true
    interval: 10s
    timeout: 8s
    batchSize: 100
    userAgent: "Mozilla/5.0 ..."
    joinApiEnabled: true
    roblosecurity: ""
    emitLeave: true
    usernameCacheTTL: 24h
```

## Field Notes

- `enabled`: enable/disable Roblox polling.
- `interval`: polling interval.
- `timeout`: per-request timeout.
- `batchSize`: Presence batch size, max `100`.
- `joinApiEnabled`: enable `gamejoin` API.
- `roblosecurity`: Roblox `.ROBLOSECURITY` cookie. Empty means deep-link-only mode.
- `emitLeave`: emit leave events.
- `usernameCacheTTL`: TTL for username -> UserId cache.

## Security

- Never expose `.ROBLOSECURITY` in logs/screenshots.
- DDBOT does not print cookie plaintext; it only logs configured/unset and masked length.

## Compatibility

- `providers.roblox.*` keys are v2-only pass-through keys in the compat loader.
- They are loaded without triggering `UNKNOWN_V2_CONFIG`.
