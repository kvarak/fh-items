# fh-items - Plan

A webapp to help Frosthaven players plan what to craft: enter your
character's resources and owned items, select what to craft, and
immediately see whether it's affordable - and if not, exactly why.

## Status: v1 complete

Implemented and tested (`go test ./...`, and the same in Docker via
`make test`):

- App data schema + source transform/validation (`internal/catalog`),
  generated from the immutable `.github/docs/items.json`.
- Runtime state model (`internal/state`): one active crafting character,
  a shared Frosthaven herb supply, building levels, per-item
  unlock/availability.
- A strict, owned-items-only affordability check (`internal/engine`) that
  reports structured, categorized shortfalls (resource / missing
  prerequisite / locked-or-unavailable) rather than plain text.
- No server-side persistence: the server holds no save file and no
  per-user state. All state (supplies, building levels, discovered items)
  lives in browser cookies, applied per-request.
- A 2-endpoint JSON API (`internal/api`): `GET /api/state`,
  `POST /api/check`, plus the server-rendered `GET /` page.
- A server-rendered single-page UI with a live running total and a
  per-item status column, verified end-to-end in a real browser.
- A Dockerized dev workflow (`make build`/`test`/`gendata`/`serve`)
  scoped entirely to this directory.

See [README.md](README.md) for how to run it, where state lives,
how to regenerate catalog data, and what it deliberately doesn't do.

**Definition of done (v1): met.** A user can enter their supplies, select
items, and immediately see whether they can craft them, what resources
remain, and which requirement is missing.

## Possible future additions

Not planned, just plausible next steps if this grows beyond v1:

- Recommendation/search (best-value suggestions, combination enumeration)
  instead of only checking a manually selected set.
- The rulebook's prerequisite-substitution rule (pay an owned-by-another
  item's cost in place of owning it) - deliberately left out of v1 to
  keep the check predictable; see the modeling note below.
- Multi-character support (pooled or per-character supplies/ownership).
- Optional server-side persistence (e.g. a shareable, multi-device save)
  if cookie-only state ever proves too limiting - deliberately left out
  for now to keep the server stateless and avoid migration/locking/backup
  concerns entirely.
- An accessibility/responsive design pass beyond native form semantics.

## Core problem

Given the active character's current resources and owned items, and the
catalog's building-level/availability gating, determine whether a
manually selected set of items (with quantities) can be crafted right
now. This is a strict yes/no/shortfall check, not a search for the best
or any affordable combination - the user picks the items; the tool
answers whether that exact pick is affordable.

## Modeling notes

- **Resource types are separate, not a generic pool.** Lumber, metal, hide,
  herbs, and gold are tracked independently.
- **Supply rules differ per resource (confirmed against the rulebook, p.
  56):** "A character can use the material resources in their personal
  supply to craft items. All characters can use the material resources in
  the Frosthaven supply to pay various costs for buildings, but not to
  craft items." and "A character can use the herb resources in their
  personal supply to craft items and brew potions. All characters can use
  the herb resources in the Frosthaven supply to craft items and brew
  potions." So: lumber/metal/hide are personal-only for crafting (the
  Frosthaven supply's material resources are for building costs, never
  item crafting); herbs can be used from either the character's personal
  supply or the shared Frosthaven supply for crafting, per a configurable
  rule (e.g. "shared first, then personal"); gold is also personal to the
  crafting character, kept separate from material/herb resources.
- **Ownership/context (v1 scope).** The user selects one *active crafting
  character* plus the shared Frosthaven herb supply. Lumber/metal/hide/gold
  costs are paid from that one character's personal supply only; herbs
  follow the configurable shared-vs-personal rule. Joint optimization
  across multiple characters' pooled supplies, or assigning different
  items to different characters in one recommendation, is an explicit
  later feature, not v1.
- **Prerequisite items are consumed ingredients, but only if already
  owned.** Crafting an item that lists another item as a cost requires
  the active character to already own enough copies of it; crafting
  consumes those owned copies and returns them to the available supply.
  There is no automatic fresh-crafting of a missing prerequisite and no
  cost-substitution for one owned by someone else - a missing
  prerequisite is simply reported as a shortfall. (The rulebook, p. 65,
  does describe a substitution rule for items owned by other party
  members; v1 deliberately does not implement it, to keep the check a
  strict, predictable "can I craft this right now" answer rather than a
  planner.)
- **Item availability is explicit, not inferred.** An item is selectable
  only if `unlocked` is true, `requiredBuildingLevel` is met, and
  `availableCopies` > 0. These are distinct fields in the data model, not
  derived solely from building level.
- **Duplicate selections are real purchases.** Selecting the same item N
  times consumes N copies from `availableCopies` and adds N times its cost;
  it is never free-form "quantity" without a per-unit cost and supply
  check.
- **No recommendation/search.** "Best value", combination enumeration, and
  bounded search are explicitly out of scope for v1 - cancelled, not
  deferred. The only supported operation is an exact affordability check
  for a manually selected set of items.
- **Shortfalls are categorized, not just text.** `engine.Shortfall` carries
  a `Kind` (`locked`, `resource`, `prerequisite`, or `other`) alongside the
  message, so the API and UI can group/style results by reason instead of
  parsing strings.

## Data sources

Authoritative item data already exists at `.github/docs/items.json` (267
items, with resource costs, gold costs, and Craftsman/Jeweler/Trading Post
unlock levels) and `.github/docs/fh-rule-book.pdf` for full rules context.
This source file is treated as immutable; app data is generated/derived
from it into a versioned, validated format, with a documented field mapping
and a checksum of the source file.

**Compatibility rule:** the app refuses to load app data whose schema
version or source checksum doesn't match what it expects. It fails with a
clear error naming the expected vs. found version/checksum, rather than
silently loading mismatched data.

## API surface

- `GET /` - the server-rendered web UI
- `GET /api/state` - current character, supplies, building levels, and
  every catalog item merged with its unlock/availability state
- `POST /api/check` - takes selected item IDs/quantities (plus optional
  resource overrides, so the UI can try out hypothetical supplies without
  persisting anything) and returns whether the set is affordable,
  remaining resources, and shortfall details

There is no `/api/recommend` and no `/api/items`/`/api/affordability` as
separate endpoints - `/api/check` covers the only supported operation.

**Error standard:** `POST /api/check` uses a JSON error envelope
(`{"error": {"code", "message"}}`) for request-level failures (e.g.
unparseable JSON); a syntactically valid but infeasible check is a normal
200 response with `feasible: false` and `shortfalls`, not an error.

## UX notes

One page, server-rendered from the current state: resource-entry fields
for the active character's personal supply and the shared Frosthaven herb
supply (with a herb-sourcing selector), and an item table (ID, name,
slot/type, cost, prerequisites, required building + level, and a Status
column that distinguishes "Locked (needs X)", "Out of stock", and
"Available"). A small amount of JS (`web/static/app.js`) handles: a live
running total as quantities/resources change; submitting the current form
to `POST /api/check`, rendering the result grouped by shortfall kind
(resources / missing prerequisites / locked-or-unavailable) rather than a
flat list; and reading/writing the cookies that hold all client-side state
(personal/shared supply + herb sourcing saved as changed; building
levels + discovered items saved when the Update button is clicked).

## Implementation constraints

Keep it simple: Go standard library or a minimal router, static assets
embedded in the binary, no database for v1. Plain, accessible, responsive
HTML/CSS with progressive enhancement; JS limited to interaction and API
calls.

**Persistence:** none server-side. The server is stateless across
requests/restarts; every request carries whatever client-side cookie
state it needs (building levels, discovered items), and the UI restores
personal/shared supply values and herb sourcing from a separate cookie on
load. No save file, no database, no migration/locking/backup concerns.

## Developer commands

See [README.md](README.md) for the exact commands (`make build`/`test`/
`gendata`/`serve`, and the non-Docker equivalents).

