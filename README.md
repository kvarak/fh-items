# fh-items

A Frosthaven crafting planner. Set your building levels and discovered
items, then pick items into a shopping basket to see the total cost.
Prerequisite items are added to the basket automatically.

## Running it

```sh
make serve                       # via Docker, on http://localhost:8080
go run ./cmd/server              # without Docker, on :8080
go run ./cmd/server -addr :9000
```

## Where state lives

Nothing is stored server-side. Everything lives in browser cookies:

- `fhUnlocks`: building levels and discovered item IDs, saved when you
  click Update.
- `fhBasket`: the item IDs in the shopping basket.

Clearing cookies for the site resets everything.

## Spoilers

Locked items are never named. Discovered items are picked by number,
buildings are shown by number, and a locked prerequisite appears as
`item N` in the Cost column.

## Regenerating catalog data

The catalog is generated from `../.github/docs/items.json` into
`internal/catalog/data/catalog.gen.json` and embedded in the binary:

```sh
make gendata          # via Docker
go run ./cmd/gendata  # without Docker
```

## Running tests

```sh
make test       # via Docker
go test ./...   # without Docker
```
