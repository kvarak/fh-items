# fh-items

A Frosthaven crafting planner. Set your building levels and discovered
items, then pick items into a shopping basket to see the total cost.
Prerequisite items are added to the basket automatically. Gloomhaven items
imported through the Building 37 can be added too.

## Running it

```sh
make serve                       # via Docker, on http://localhost:8080
go run ./cmd/server              # without Docker, on :8080
go run ./cmd/server -addr :9000
```

## Where state lives

Nothing is stored server-side. Everything lives in browser cookies:

- `fhUnlocks`: building levels, discovered item IDs, and checked
  Gloomhaven item IDs, saved when you click Update.
- `fhBasket`: the item keys in the shopping basket.

Clearing cookies for the site resets everything.

## Analytics

Optional GoatCounter pageview tracking, off by default. To turn it on,
edit `GOATCOUNTER_URL` in [docker-compose.yml](docker-compose.yml)
before running `docker compose up`, or pass `-goatcounter`:

```sh
GOATCOUNTER_URL=https://stats.example.com/count docker compose up -d
go run ./cmd/server -goatcounter https://stats.example.com/count
```

The endpoint must be `https://<host>/count`; `count.js` is loaded from
the same host. Each page load records one pageview with the path
`<hostname><pathname>`, e.g. `items.example.com/`, so several apps can
share one dashboard. GoatCounter sets no cookies, and it skips localhost
and private-network hosts.

## Sharing

"Copy share link" copies a `/?share=...` link holding your unlocks (not
your basket). Opening it saves those unlocks to the recipient's
`fhUnlocks` cookie and redirects to `/`.

## Spoilers

Locked items are never named. Discovered items are picked by number,
buildings are shown by number, and a locked prerequisite appears as
`item N` in the Cost column.

## Running tests

```sh
make test       # via Docker
make check      # all checks (gofmt, vet, staticcheck, govulncheck, race tests, JS syntax), needs Go and Node
go test ./...   # without Docker
```

CI runs `make check` on every push and pull request, and publishes the
Docker image to `ghcr.io` on pushes to `main` for `linux/amd64`,
`linux/arm64`, and `linux/arm/v7` (Raspberry Pi 4 on 64- or 32-bit OS).
