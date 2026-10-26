# fh-items - Plan

## Status

Done: a server-rendered shopping-basket planner. See [README.md](README.md)
for running it.

## Design

- **Unlocking.** Craftsman (5), Trading Post (37), and Jeweler (39) items
  unlock by building level. Every other item (blueprint, loot, scenario
  reward) unlocks only when marked discovered.
- **Items table.** Only unlocked items are listed. An item can be selected
  only if its whole prerequisite chain is unlocked; otherwise its checkbox
  is disabled and crossed out.
- **Basket.** Selecting an item moves it into the basket and pulls in its
  prerequisites. The basket total sums materials, herbs, and gold.
- **Spoiler-free.** Locked items are never named.
- **Stateless server.** All state is in cookies (`fhUnlocks`, `fhBasket`).
- **Catalog data.** Generated from the immutable
  `.github/docs/items.json`, with a schema version and source checksum.
  The server refuses to load a mismatched schema version.
