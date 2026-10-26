const itemCosts = JSON.parse(document.getElementById("item-cost-data").textContent);

const ONE_YEAR_SECONDS = 60 * 60 * 24 * 365;
const BASKET_COOKIE = "fhBasket";
const UNLOCKS_COOKIE = "fhUnlocks";

function writeCookie(name, value) {
  document.cookie = `${name}=${encodeURIComponent(JSON.stringify(value))}; path=/; max-age=${ONE_YEAR_SECONDS}; samesite=lax`;
}

function readCookie(name) {
  const match = document.cookie.match(new RegExp("(?:^|; )" + name + "=([^;]*)"));
  if (!match) return null;
  try {
    return JSON.parse(decodeURIComponent(match[1]));
  } catch {
    return null;
  }
}

function formatResources({ materials, herbs, anyHerb, gold }) {
  const parts = [];
  for (const [rt, qty] of Object.entries(materials)) parts.push(`${qty} ${rt}`);
  for (const [rt, qty] of Object.entries(herbs)) parts.push(`${qty} ${rt}`);
  for (const qty of anyHerb) parts.push(`${qty} any herb`);
  if (gold) parts.push(`${gold} gold`);
  return parts.length ? parts.join(", ") : "nothing";
}

function addCostInto(total, cost) {
  for (const [rt, amount] of Object.entries(cost.materials || {})) {
    total.materials[rt] = (total.materials[rt] || 0) + amount;
  }
  for (const [rt, amount] of Object.entries(cost.herbs || {})) {
    total.herbs[rt] = (total.herbs[rt] || 0) + amount;
  }
  // Each entry is a separate herb-type choice, so entries are listed, not summed.
  total.anyHerb.push(...(cost.anyHerb || []));
  total.gold += cost.gold || 0;
}

// --- Shopping basket: checked items move from the Items table into the basket. ---

const itemsBody = document.getElementById("items-body");
const basketBody = document.getElementById("basket-body");
const basketTotalEl = document.getElementById("basket-total");

function rowsIn(body) {
  return [...body.querySelectorAll("tr[data-item-row]")];
}

function updateBasketTotal() {
  const total = { materials: {}, herbs: {}, anyHerb: [], gold: 0 };
  for (const row of rowsIn(basketBody)) addCostInto(total, itemCosts[row.dataset.itemRow]);
  basketTotalEl.textContent = formatResources(total);
}

// Adds an item's prerequisites to the basket, recursively. Prerequisites
// already in the basket (or not shown in the Items table) are skipped.
function addRequiredItemsToBasket(itemId) {
  // Catalog prerequisites all have quantity 1, so one row per item is enough.
  for (const reqId of itemCosts[itemId].requiredItemIds || []) {
    const reqRow = itemsBody.querySelector(`tr[data-item-row="${reqId}"]`);
    if (reqRow) setRowInBasket(reqRow, true);
  }
}

// Moves a row into the basket, or back to its original position in the Items table.
function setRowInBasket(row, inBasket) {
  const order = parseInt(row.dataset.order, 10);
  row.querySelector(".item-select").checked = inBasket;
  if (inBasket) {
    row.classList.remove("filtered-out");
    basketBody.appendChild(row);
    addRequiredItemsToBasket(row.dataset.itemRow);
  } else {
    const next = rowsIn(itemsBody).find((other) => parseInt(other.dataset.order, 10) > order);
    itemsBody.insertBefore(row, next || null);
  }
  updateBasketTotal();
  applyItemFilters();
  saveBasketCookie();
}

// Basket contents are kept in a cookie so they survive the Update reload.
function saveBasketCookie() {
  writeCookie(BASKET_COOKIE, rowsIn(basketBody).map((row) => row.dataset.itemRow));
}

document.addEventListener("change", (event) => {
  if (!event.target.matches(".item-select")) return;
  setRowInBasket(event.target.closest("tr"), event.target.checked);
});

function readBuildingLevels() {
  const inputs = document.querySelectorAll(".building-level-input");
  return Object.fromEntries([...inputs].map((input) => [input.dataset.building, parseInt(input.value, 10) || 0]));
}

function readCheckedIDs(selector) {
  return [...document.querySelectorAll(selector)]
    .filter((input) => input.checked)
    .map((input) => parseInt(input.dataset.itemId, 10));
}

// --- Items table slot filter pills. ---

const activeSlotFilters = new Set();

function applyItemFilters() {
  for (const row of rowsIn(itemsBody)) {
    const matchesSlot = activeSlotFilters.size === 0 || activeSlotFilters.has(row.dataset.slot);
    row.classList.toggle("filtered-out", !matchesSlot);
  }
}

document.querySelectorAll("#slot-filter-pills .pill").forEach((button) => {
  const slot = button.dataset.filterSlot;
  button.addEventListener("click", () => {
    if (!activeSlotFilters.delete(slot)) activeSlotFilters.add(slot);
    button.classList.toggle("pill-active", activeSlotFilters.has(slot));
    applyItemFilters();
  });
});

function currentUnlocks() {
  return {
    buildings: readBuildingLevels(),
    unlockedItemIds: readCheckedIDs(".unlockable-checkbox"),
    ghItemIds: readCheckedIDs(".gloomhaven-checkbox"),
  };
}

// Update saves building levels and discovered items, then reloads so the
// server re-renders which items are unlocked.
document.getElementById("unlocks-update-button").addEventListener("click", () => {
  writeCookie(UNLOCKS_COOKIE, currentUnlocks());
  window.location.href = "/";
});

// Encodes unlocks as base64url: 3 level bytes, then discovered and Gloomhaven ID bitsets (see cmd/server/share.go).
function pushUvarint(out, n) {
  while (n >= 128) {
    out.push((n & 127) | 128);
    n >>>= 7;
  }
  out.push(n);
}

function encodeShare({ buildings, unlockedItemIds, ghItemIds }) {
  const bytes = [buildings.craftsman, buildings.tradingPost, buildings.jeweler].map((v) => v & 255);
  for (const ids of [unlockedItemIds, ghItemIds]) {
    const length = ids.length ? Math.floor(Math.max(...ids) / 8) + 1 : 0;
    const bits = new Uint8Array(length);
    for (const id of ids) bits[id >> 3] |= 1 << (id & 7);
    pushUvarint(bytes, length);
    bytes.push(...bits);
  }
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

// The server stores a shared link's unlocks in the recipient's cookie, then redirects to "/".
document.getElementById("unlocks-share-button").addEventListener("click", async () => {
  const link = `${window.location.origin}/?share=${encodeShare(currentUnlocks())}`;
  const status = document.getElementById("share-status");
  try {
    await navigator.clipboard.writeText(link);
    status.textContent = "Link copied.";
  } catch {
    status.textContent = link;
  }
});

// Restores the saved basket. Items that are gone or no longer selectable are dropped.
for (const id of readCookie(BASKET_COOKIE) || []) {
  const row = itemsBody.querySelector(`tr[data-item-row="${id}"]`);
  if (row && !row.querySelector(".item-select").disabled) setRowInBasket(row, true);
}
saveBasketCookie();

