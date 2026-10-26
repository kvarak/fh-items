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

function updateBasketTotal() {
  const total = { materials: {}, herbs: {}, anyHerb: [], gold: 0 };
  basketBody.querySelectorAll("tr[data-item-row]").forEach((row) => {
    addCostInto(total, itemCosts[row.dataset.itemRow]);
  });
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

// Moves a row into the basket, or back to its catalog position in the Items table.
function setRowInBasket(row, inBasket) {
  const id = parseInt(row.dataset.itemRow, 10);
  row.querySelector(".item-select").checked = inBasket;
  if (inBasket) {
    row.classList.remove("filtered-out");
    basketBody.appendChild(row);
    addRequiredItemsToBasket(id);
  } else {
    const next = [...itemsBody.querySelectorAll("tr[data-item-row]")].find(
      (other) => parseInt(other.dataset.itemRow, 10) > id
    );
    itemsBody.insertBefore(row, next || null);
  }
  updateBasketTotal();
  applyItemFilters();
  saveBasketCookie();
}

// Basket contents are kept in a cookie so they survive the Update reload.
function saveBasketCookie() {
  const ids = [...basketBody.querySelectorAll("tr[data-item-row]")].map((row) => parseInt(row.dataset.itemRow, 10));
  writeCookie(BASKET_COOKIE, ids);
}

document.addEventListener("change", (event) => {
  if (!event.target.matches(".item-select")) return;
  setRowInBasket(event.target.closest("tr"), event.target.checked);
});

function readBuildingLevels() {
  const out = {};
  document.querySelectorAll(".building-level-input").forEach((input) => {
    out[input.dataset.building] = parseInt(input.value, 10) || 0;
  });
  return out;
}

function readUnlockedItemIDs() {
  const ids = [];
  document.querySelectorAll(".unlockable-checkbox").forEach((input) => {
    if (input.checked) ids.push(parseInt(input.dataset.itemId, 10));
  });
  return ids;
}

// --- Items table slot filter pills. ---

const activeSlotFilters = new Set();

function applyItemFilters() {
  itemsBody.querySelectorAll("tr[data-item-row]").forEach((row) => {
    const matchesSlot = activeSlotFilters.size === 0 || activeSlotFilters.has(row.dataset.slot);
    row.classList.toggle("filtered-out", !matchesSlot);
  });
}

document.querySelectorAll("#slot-filter-pills .pill").forEach((button) => {
  const slot = button.dataset.filterSlot;
  button.addEventListener("click", () => {
    if (!activeSlotFilters.delete(slot)) activeSlotFilters.add(slot);
    button.classList.toggle("pill-active", activeSlotFilters.has(slot));
    applyItemFilters();
  });
});

// Update saves building levels and discovered items, then reloads so the
// server re-renders which items are unlocked.
document.getElementById("unlocks-update-button").addEventListener("click", () => {
  writeCookie(UNLOCKS_COOKIE, {
    buildings: readBuildingLevels(),
    unlockedItemIds: readUnlockedItemIDs(),
  });
  window.location.reload();
});

// Restores the saved basket. Items that are gone or no longer selectable are dropped.
for (const id of readCookie(BASKET_COOKIE) || []) {
  const row = itemsBody.querySelector(`tr[data-item-row="${id}"]`);
  if (row && !row.querySelector(".item-select").disabled) setRowInBasket(row, true);
}
saveBasketCookie();

