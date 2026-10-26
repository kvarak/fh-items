# fh-items

A Frosthaven crafting planner: enter your character's resources and owned
items, select what you want to craft, and see immediately whether it's
affordable - and if not, why.

## Running it

```sh
make serve
```

Builds a Docker image (scoped to this directory only) and serves the app
at http://localhost:8080. Override the port with `PORT=...`.

Without Docker:

```sh
go run ./cmd/server            # serves on :8080
go run ./cmd/server -addr :9000
```

## Where state lives

Nothing is persisted server-side - the server holds no save file and no
per-user state. Everything lives in cookies in your browser:

- Personal/shared supply values and herb sourcing are saved to a cookie
  as you change them.
- Building levels and discovered items are saved to a cookie when you
  click Update in the Unlockables box.

Clearing cookies for the site resets everything. Running multiple people
against the same server is fine - each browser's cookies are independent.

## Regenerating catalog data

The item catalog is generated from the immutable source data at
`../.github/docs/items.json` and embedded in the binary at
`internal/catalog/data/catalog.gen.json`. Regenerate it after the source
file changes:

```sh
make gendata          # via Docker
go run ./cmd/gendata  # without Docker
```

## Running tests

```sh
make test       # via Docker
go test ./...   # without Docker
```

## What it does not do

- **No recommendation or search.** It only checks the exact set of items
  you select - it will never suggest the "best" combination, enumerate
  alternatives, or plan a multi-step crafting order for you.
- **No automatic prerequisite crafting.** If an item requires owning
  another item as an ingredient, you must already own it. The app never
  crafts a prerequisite for you from the supply, and never substitutes its
  cost (unlike the official rule for items owned by other party members -
  see `plan.md` for why this was deliberately left out of v1).
- **No multi-character support.** There is one active crafting character;
  joint supplies or assigning items across characters aren't modeled.
- **No server-side persistence.** All state lives in browser cookies; there
  is no save file, no migration tooling, no file locking, no backups, and
  no way to access your state from a different browser/device.
