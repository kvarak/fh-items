package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"fh-items/internal/api"
	"fh-items/internal/catalog"
	"fh-items/internal/state"
)

// unlocksCookieName is the client-side cookie (written by app.js's Update
// button) carrying building levels and discovered item IDs. Nothing here
// is persisted server-side - it's read fresh from the request every time.
const unlocksCookieName = "fhUnlocks"

// unlocksCookiePayload is the JSON shape of the fhUnlocks cookie, matching
// api.BuildingsOverride/UnlockedItemIDs so the same application logic
// (api.ApplyBuildingsAndUnlocks) can drive both /api/check's per-request
// overrides and this cookie-driven page render.
type unlocksCookiePayload struct {
	Buildings       api.BuildingsOverride `json:"buildings"`
	UnlockedItemIDs []int                 `json:"unlockedItemIds"`
}

// stateFromRequest applies the fhUnlocks cookie (if present and valid) to
// st and returns the result, leaving st itself untouched. st.Items is
// cloned first since maps are reference types - without it, applying the
// cookie would mutate the server's shared in-memory default state.
func stateFromRequest(r *http.Request, cat catalog.Catalog, st state.State) state.State {
	cookie, err := r.Cookie(unlocksCookieName)
	if err != nil {
		return st
	}
	value, err := url.PathUnescape(cookie.Value)
	if err != nil {
		return st
	}
	var payload unlocksCookiePayload
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return st // malformed/stale cookie - fall back to defaults
	}

	clonedItems := make(map[int]state.ItemState, len(st.Items))
	for id, is := range st.Items {
		clonedItems[id] = is
	}
	st.Items = clonedItems

	api.ApplyBuildingsAndUnlocks(cat, &st, &payload.Buildings, payload.UnlockedItemIDs)
	return st
}

// pageItem is one row in the server-rendered item table.
type pageItem struct {
	ID                    int
	Name                  string
	ImagePath             string
	Slot                  string
	RequiredBuilding      string
	RequiredBuildingLevel int
	Unlocked              bool
	AvailableCopies       int
	Craftable             bool
	CostSummary           string
	// Status/StatusClass explain, in the item table itself, why an item
	// can't be selected at all: locked (building level/discovery) or out
	// of stock. Distinct from affordability shortfalls, which only apply
	// once a selectable item is actually checked.
	Status      string
	StatusClass string
}

// pageRequiredItemCost is one prerequisite entry in pageItemCost.RequiredItems.
type pageRequiredItemCost struct {
	ItemID   int `json:"itemId"`
	Quantity int `json:"quantity"`
}

// pageItemCost is the per-item cost data JS needs to compute a live
// running total as the user changes quantities, and to evaluate the
// items-table filter pills (can purchase/can craft/crafting chains),
// without re-deriving it from the HTML. Embedded as a JSON script tag.
// Includes every catalog item, not just unlocked ones, so a crafting
// chain can be evaluated through prerequisites the UI doesn't show as a
// selectable row.
type pageItemCost struct {
	Materials       map[catalog.ResourceType]int `json:"materials,omitempty"`
	Herbs           map[catalog.ResourceType]int `json:"herbs,omitempty"`
	Gold            int                          `json:"gold,omitempty"`
	RequiredItems   []pageRequiredItemCost       `json:"requiredItems,omitempty"`
	Unlocked        bool                         `json:"unlocked"`
	AvailableCopies int                          `json:"availableCopies"`
}

// pageUnlockable is one spoiler-free entry in the Unlockables box: an
// item with no building requirement (found via blueprints/loot/scenario
// rewards later in the campaign), identified by ID only - never by name.
type pageUnlockable struct {
	ID       int
	Unlocked bool
}

// buildingLabels gives each building its real Frosthaven building/envelope
// reference number, so the UI never names Craftsman/Jeweler/Trading Post
// directly (keeping it spoiler-free for Jeweler/Trading Post, which aren't
// starting buildings).
var buildingLabels = map[catalog.BuildingType]string{
	catalog.BuildingCraftsman:   "5",
	catalog.BuildingJeweler:     "39",
	catalog.BuildingTradingPost: "37",
}

func buildingLabel(bt catalog.BuildingType) string {
	if bt == "" {
		return ""
	}
	return buildingLabels[bt]
}

// pageData is the view model for web/templates/index.html.tmpl.
type pageData struct {
	Character     state.Character
	MaterialTypes []catalog.ResourceType
	HerbTypes     []catalog.ResourceType
	SharedHerbs   map[catalog.ResourceType]int
	HerbSourcing  string
	Buildings     state.BuildingLevels
	Items         []pageItem
	Unlockables   []pageUnlockable
	ItemCostJSON  template.JS
}

func buildPageData(cat catalog.Catalog, st state.State) pageData {
	byID := make(map[int]catalog.Item, len(cat.Items))
	for _, item := range cat.Items {
		byID[item.ID] = item
	}

	items := make([]pageItem, 0, len(cat.Items))
	costByID := make(map[int]pageItemCost, len(cat.Items))
	craftableMemo := map[int]bool{}
	for _, item := range cat.Items {
		is := st.Items[item.ID]
		craftable := isCraftable(item.ID, byID, st, craftableMemo)
		status, statusClass := itemStatus(item, is, craftable)
		items = append(items, pageItem{
			ID:                    item.ID,
			Name:                  item.Name,
			ImagePath:             fmt.Sprintf("/static/items/%03d.png", item.ID),
			Slot:                  item.Slot,
			RequiredBuilding:      buildingLabel(item.RequiredBuilding),
			RequiredBuildingLevel: item.RequiredBuildingLevel,
			Unlocked:              is.Unlocked,
			AvailableCopies:       is.AvailableCopies,
			Craftable:             craftable,
			CostSummary:           costCell(item.Cost, byID, st),
			Status:                status,
			StatusClass:           statusClass,
		})
		reqItems := make([]pageRequiredItemCost, 0, len(item.Cost.RequiredItems))
		for _, req := range item.Cost.RequiredItems {
			reqItems = append(reqItems, pageRequiredItemCost{ItemID: req.ItemID, Quantity: req.Quantity})
		}
		costByID[item.ID] = pageItemCost{
			Materials:       item.Cost.Materials,
			Herbs:           item.Cost.Herbs,
			Gold:            item.Cost.Gold,
			RequiredItems:   reqItems,
			Unlocked:        is.Unlocked,
			AvailableCopies: is.AvailableCopies,
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	var unlockables []pageUnlockable
	for _, item := range cat.Items {
		if item.RequiredBuilding != "" {
			continue // gated by building level, not manually marked discovered
		}
		unlockables = append(unlockables, pageUnlockable{ID: item.ID, Unlocked: st.Items[item.ID].Unlocked})
	}
	sort.Slice(unlockables, func(i, j int) bool { return unlockables[i].ID < unlockables[j].ID })

	costJSON, _ := json.Marshal(costByID)

	return pageData{
		Character:     st.Character,
		MaterialTypes: catalog.MaterialTypes,
		HerbTypes:     catalog.HerbTypes,
		SharedHerbs:   st.SharedSupply.Herbs,
		HerbSourcing:  string(st.HerbSourcing),
		Buildings:     st.Buildings,
		Items:         items,
		Unlockables:   unlockables,
		ItemCostJSON:  template.JS(costJSON), //nolint:gosec // server-generated JSON, not user input
	}
}

func costSummary(cost catalog.Cost) string {
	var parts []string
	for _, rt := range catalog.MaterialTypes {
		if qty := cost.Materials[rt]; qty > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", qty, rt))
		}
	}
	for _, rt := range catalog.HerbTypes {
		if qty := cost.Herbs[rt]; qty > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", qty, rt))
		}
	}
	for _, qty := range cost.AnyHerb {
		parts = append(parts, fmt.Sprintf("%d any herb", qty))
	}
	if cost.Gold > 0 {
		parts = append(parts, fmt.Sprintf("%d gold", cost.Gold))
	}
	return strings.Join(parts, " + ")
}

// costCell joins raw material/gold cost and prerequisite items into one Cost column.
func costCell(cost catalog.Cost, byID map[int]catalog.Item, st state.State) string {
	parts := []string{costSummary(cost), requiredItemsSummary(cost, byID, st)}
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	if len(nonEmpty) == 0 {
		return "-"
	}
	return strings.Join(nonEmpty, " + ")
}

// requiredItemsSummary names unlocked prerequisites and shows only the ID for locked ones.
func requiredItemsSummary(cost catalog.Cost, byID map[int]catalog.Item, st state.State) string {
	if len(cost.RequiredItems) == 0 {
		return ""
	}
	var parts []string
	for _, req := range cost.RequiredItems {
		if prereq, ok := byID[req.ItemID]; ok && st.Items[req.ItemID].Unlocked {
			parts = append(parts, fmt.Sprintf("%dx %s", req.Quantity, prereq.Name))
			continue
		}
		parts = append(parts, fmt.Sprintf("%dx item %d", req.Quantity, req.ItemID))
	}
	return strings.Join(parts, " + ")
}

// isCraftable reports whether an item and its whole prerequisite chain can
// be made: each must be unlocked and in stock. memo caches results per ID.
func isCraftable(id int, byID map[int]catalog.Item, st state.State, memo map[int]bool) bool {
	if v, ok := memo[id]; ok {
		return v
	}
	memo[id] = false // assumed false while resolving, so a cycle cannot recurse forever
	item, ok := byID[id]
	is := st.Items[id]
	if !ok || !is.Unlocked || is.AvailableCopies == 0 {
		return false
	}
	for _, req := range item.Cost.RequiredItems {
		if !isCraftable(req.ItemID, byID, st, memo) {
			return false
		}
	}
	memo[id] = true
	return true
}

// itemStatus explains, distinctly, why an item isn't selectable at all:
// locked (building level not met / not yet discovered) takes priority
// over out-of-stock, since an unlocked-but-depleted item could become
// available again, while a locked one needs a different action entirely.
func itemStatus(item catalog.Item, is state.ItemState, craftable bool) (status, class string) {
	switch {
	case !is.Unlocked && item.RequiredBuilding != "":
		return fmt.Sprintf("Locked (needs Building %s, level %d)", buildingLabel(item.RequiredBuilding), item.RequiredBuildingLevel), "status-locked"
	case !is.Unlocked:
		return "Locked (not yet discovered)", "status-locked"
	case is.AvailableCopies == 0:
		return "Out of stock", "status-unavailable"
	case !craftable:
		return "Missing prerequisite", "status-unavailable"
	default:
		return "Available", "status-available"
	}
}
