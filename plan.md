# fh-items - Plan

A webapp to help Frosthaven players plan what to craft. Backend logic/data
prep in Go, minimal JS on the frontend for interaction.

**Status:** Milestones 1-5 implemented (scaffolding, app data schema,
source transform, validation, runtime state model). Stopped for review
before the affordability engine (milestone 6+). See the correction note
below - it affects how that engine must work.

**Correction from implementation:** `.github/docs/items.json` shows that
~20% of items (55/267) are crafted by consuming *other owned items* as
ingredients (e.g. Reinforced Shield requires owning/consuming a Crude
Shield plus 1 lumber), not just raw materials. So nested recipes/item
dependencies *do* exist, contrary to the earlier assumption in this plan.
The data model now tracks this explicitly (`Cost.RequiredItems`,
`Character.OwnedItems`), but the affordability/recommendation engine
(milestone 6+) will need to account for crafting order and item
dependencies, not just a flat resource budget.

## Core problem

This is a constrained resource-allocation problem: given a resource pool,
current building levels, and the set of unlocked/available item IDs, find
feasible sets of items whose total costs do not exceed available resources.
This is not a greedy "buy the best item first" problem - multiple items
compete for the same limited resources, so the interesting question is which
*combination* is feasible or best, not which single item to pick first.

## Modeling notes

- **Resource types are separate, not a generic pool.** Lumber, metal, hide,
  herbs, and gold are tracked independently.
- **Supply rules differ per resource.** Lumber/metal/hide must come from the
  selected character's personal supply. Herbs may come from the shared
  Frosthaven supply or a character's personal supply, per a configurable
  rule (e.g. "shared first, then personal"). Gold is a separate currency,
  not pooled with crafting materials.
- **Ownership/context (v1 scope).** The user selects one *active crafting
  character* plus an optional shared Frosthaven herb supply. All non-herb
  costs are paid from that one character's personal supply; herbs follow
  the configurable shared-vs-personal rule. Joint optimization across
  multiple characters' pooled supplies, or assigning different items to
  different characters in one recommendation, is an explicit later
  feature, not v1.
- **Item availability is explicit, not inferred.** An item is selectable
  only if `unlocked` is true, `requiredBuildingLevel` is met, and
  `availableCopies` > 0. These are distinct fields in the data model, not
  derived solely from building level.
- **Duplicate selections are real purchases.** Selecting the same item N
  times consumes N copies from `availableCopies` and adds N times its cost;
  it is never free-form "quantity" without a per-unit cost and supply
  check.
- **"Best value" is a precise objective, not implicit.** Each item carries
  a `value` field (from source data where meaningful, otherwise a
  user-assigned priority/weight at runtime). The recommendation objective
  must be one of, and the API must say which one it's running:
  1. maximize total value within the available resources;
  2. maximize total value within the available resources, subject to a
     user-supplied maximum item count;
  3. maximize number of items (value ignored), as a fallback when no value
     is assigned.
- **"Combinations" is explicitly scoped.** Three distinct operations:
  1. Exact affordability check for a manually selected set of items.
  2. "Best value" recommendation per the objective above.
  3. (Stretch) Enumerate all affordable sets.
- **Search is bounded.** Exhaustive search is only used below an explicit
  limit (e.g. a maximum of 20 selectable items, or a configurable result
  cap). Beyond that limit, the API returns a clear "too many combinations"
  error instead of hanging; DP or an ILP/CP-SAT solver is only introduced
  if/when that bound proves too restrictive in practice.

## Data sources

Authoritative item data already exists at `.github/docs/items.json` (267
items, with resource costs, gold costs, and Craftsman/Jeweler/Trading Post
unlock levels) and `.github/docs/fh-rule-book.pdf` for full rules context.
This source file is treated as immutable; app data is generated/derived
from it into a versioned, validated format, with a documented field mapping
and a checksum of the source file.

**Compatibility rule:** the app refuses to load app data, or a save file,
whose schema version or source checksum doesn't match what it expects. It
fails with a clear error naming the expected vs. found version/checksum
and points to migration guidance, rather than silently loading mismatched
data.

## API surface (contract defined before UI work)

- `GET /api/items`
- `GET /api/state`
- `POST /api/affordability`
- `POST /api/recommend`

Each endpoint needs a documented request/response schema, examples,
validation error cases, and clear units/field names before any UI consumes
it.

**Error standard:** all endpoints use consistent HTTP status codes (4xx
for client/validation errors, 5xx for server errors) and a single JSON
error envelope shape: `code`, `message`, `details`, and field-level
validation errors where applicable. The "too many combinations" case and
the data-version mismatch case both use this same envelope.

## UX notes

- Resource-entry screen: active crafting character's personal supply plus
  one optional shared Frosthaven herb supply, with a configurable herb
  sourcing rule.
- Item browser: shows ID, name, slot/type, gold cost, material/herb costs,
  required building + level, availability, and (if unavailable) why.
- Running total updates immediately, showing remaining resources per type
  and any shortfalls.

## Implementation constraints

Keep it simple: Go standard library or a minimal router, static assets
embedded in the binary, no database for v1. Plain, accessible, responsive
HTML/CSS with progressive enhancement; JS limited to interaction and API
calls.

**Persistence:** a local JSON save file (not browser localStorage). It
survives browser changes, can be inspected/version-controlled, and fits a
Go-embedded webapp better. The save-file schema (including its own version
field) is defined explicitly, with load/migrate/save tests from day one.

## Developer commands

- `go test ./...` runs the full test suite.
- A local data-generation command regenerates app data from
  `.github/docs/items.json`.
- A single command runs the server against sample data for manual/smoke
  testing.

## Steps

Milestone order: schema & validation -> affordability engine + tests -> API
-> minimal UI -> recommendations -> polish.

1. Project scaffolding (Go module, static file server/embed, base folder
   layout, no database) plus the developer commands (`go test ./...`,
   data-generation command, run-with-sample-data command)
2. Define app data schema: resource types, item fields (`unlocked`,
   `availableCopies`, `requiredBuildingLevel`, `value`/priority, costs per
   resource type, gold cost), and a schema version field
3. Build source-to-app data transform from `.github/docs/items.json`
   (immutable source) into versioned, checksummed app data, with a
   documented field mapping
4. Add schema validation, plus the data-version/checksum compatibility
   check, for the generated app data
5. Define the runtime state model: one active crafting character with a
   personal supply, an optional shared Frosthaven herb supply, the herb
   sourcing rule, and building levels
6. Define the versioned save-file schema for persisted user state
   (local JSON file); implement load/migrate/save with tests
7. Implement the affordability engine: exact check for a manually selected
   set of items against the modeled supplies, where duplicate selections
   consume `availableCopies` and duplicate cost
8. Implement bounded combination search (exhaustive under an explicit
   item-count/result cap, returning a clear "too many combinations" error
   beyond it) and "best value" recommendation per the defined objective
   (maximize value / maximize value under an item-count cap / maximize
   item count as a fallback)
9. Unit tests for the affordability/recommendation engine (insufficient
   resources; herb shared-vs-personal sourcing; building-level gating;
   unlocked/available filtering; duplicate selection; zero-cost/gold-only
   items; boundary cases; search-bound exceeded)
10. Define the API contract (`/api/items`, `/api/state`,
    `/api/affordability`, `/api/recommend`): schemas, examples, the shared
    error envelope (`code`/`message`/`details`/field errors), field/unit
    documentation
11. Implement the API endpoints backed by the engine
12. Frontend: resource-entry screen (active character supply + shared herb
    supply, herb sourcing rule)
13. Frontend: item browser with availability/unavailability reasons and a
    live running total with shortfalls
14. Frontend: recommendation view (best-value suggestions, optional
    item-count cap)
15. Integration testing against sample data end-to-end (incl. save/load/
    migrate)
16. Polish: accessibility/responsive pass, documentation

