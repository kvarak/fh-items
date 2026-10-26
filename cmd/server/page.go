package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"fh-items/internal/catalog"
)

// unlocksCookieName is the cookie written by the Update button in app.js.
const unlocksCookieName = "fhUnlocks"

// shareParamName is the query parameter carrying a compact shared unlocks value.
const shareParamName = "share"

type buildingLevels struct {
	Craftsman   int `json:"craftsman"`
	Jeweler     int `json:"jeweler"`
	TradingPost int `json:"tradingPost"`
}

func (b buildingLevels) level(bt catalog.BuildingType) int {
	switch bt {
	case catalog.BuildingCraftsman:
		return b.Craftsman
	case catalog.BuildingJeweler:
		return b.Jeweler
	case catalog.BuildingTradingPost:
		return b.TradingPost
	default:
		return 0
	}
}

// unlocks is the JSON shape of the fhUnlocks cookie.
type unlocks struct {
	Buildings         buildingLevels `json:"buildings"`
	DiscoveredItemIDs []int          `json:"unlockedItemIds"`
	GloomhavenIDs     []int          `json:"ghItemIds"`
}

// unlocksFromRequest reads the fhUnlocks cookie; a missing or malformed
// cookie means a fresh campaign.
func unlocksFromRequest(r *http.Request) unlocks {
	cookie, err := r.Cookie(unlocksCookieName)
	if err != nil {
		return unlocks{}
	}
	value, err := url.PathUnescape(cookie.Value)
	if err != nil {
		return unlocks{}
	}
	return parseUnlocks(value)
}

func parseUnlocks(value string) unlocks {
	var u unlocks
	if err := json.Unmarshal([]byte(value), &u); err != nil {
		return unlocks{}
	}
	return u
}

// unlockedSet returns the IDs of unlocked items. Building-gated items unlock
// by building level; all others only once marked discovered.
func (u unlocks) unlockedSet(cat catalog.Catalog) map[int]bool {
	discovered := make(map[int]bool, len(u.DiscoveredItemIDs))
	for _, id := range u.DiscoveredItemIDs {
		discovered[id] = true
	}
	unlocked := make(map[int]bool, len(cat.Items))
	for _, item := range cat.Items {
		if item.RequiredBuilding != "" {
			unlocked[item.ID] = u.Buildings.level(item.RequiredBuilding) >= item.RequiredBuildingLevel
		} else {
			unlocked[item.ID] = discovered[item.ID]
		}
	}
	return unlocked
}

// buildingLabels uses building numbers instead of names to avoid spoilers.
var buildingLabels = map[catalog.BuildingType]string{
	catalog.BuildingCraftsman:   "5",
	catalog.BuildingJeweler:     "39",
	catalog.BuildingTradingPost: "37",
}

// pageItem is one row in the Items table. Key is unique across Frosthaven
// and Gloomhaven items, whose IDs overlap.
type pageItem struct {
	Key         string
	ID          int
	Name        string
	ImagePath   string
	Slot        string
	Building    string
	Craftable   bool
	CostSummary string
}

// pageItemCost is the per-item data app.js needs for the basket total and
// prerequisite auto-add.
type pageItemCost struct {
	Materials       map[catalog.ResourceType]int `json:"materials,omitempty"`
	Herbs           map[catalog.ResourceType]int `json:"herbs,omitempty"`
	AnyHerb         []int                        `json:"anyHerb,omitempty"`
	Gold            int                          `json:"gold,omitempty"`
	RequiredItemIDs []int                        `json:"requiredItemIds,omitempty"`
}

// pageUnlockable is a discoverable item, shown by ID only to avoid spoilers.
type pageUnlockable struct {
	ID         int
	Discovered bool
}

// pageGloomhavenSet is one Trading Post import set; Visible once the
// Trading Post reaches Level.
type pageGloomhavenSet struct {
	Level   int
	Visible bool
	Items   []pageUnlockable
}

type pageData struct {
	Buildings      buildingLevels
	Items          []pageItem
	Unlockables    []pageUnlockable
	GloomhavenSets []pageGloomhavenSet
	ItemCostJSON   template.JS
}

func gloomhavenKey(id int) string { return fmt.Sprintf("gh%d", id) }

func buildPageData(cat catalog.Catalog, gh []catalog.GloomhavenItem, u unlocks) pageData {
	byID := make(map[int]catalog.Item, len(cat.Items))
	for _, item := range cat.Items {
		byID[item.ID] = item
	}
	unlocked := u.unlockedSet(cat)

	data := pageData{Buildings: u.Buildings}
	costByID := map[string]pageItemCost{}
	craftableMemo := map[int]bool{}
	for _, item := range cat.Items {
		if item.RequiredBuilding == "" {
			data.Unlockables = append(data.Unlockables, pageUnlockable{ID: item.ID, Discovered: unlocked[item.ID]})
		}
		if !unlocked[item.ID] {
			continue
		}

		var building string
		if item.RequiredBuilding != "" {
			building = fmt.Sprintf("%s.%d", buildingLabels[item.RequiredBuilding], item.RequiredBuildingLevel)
		}
		data.Items = append(data.Items, pageItem{
			Key:         strconv.Itoa(item.ID),
			ID:          item.ID,
			Name:        item.Name,
			ImagePath:   fmt.Sprintf("/static/items/%03d.png", item.ID),
			Slot:        item.Slot,
			Building:    building,
			Craftable:   isCraftable(item.ID, byID, unlocked, craftableMemo),
			CostSummary: costSummary(item.Cost, byID, unlocked),
		})

		var reqIDs []int
		for _, req := range item.Cost.RequiredItems {
			reqIDs = append(reqIDs, req.ItemID)
		}
		costByID[strconv.Itoa(item.ID)] = pageItemCost{
			Materials:       item.Cost.Materials,
			Herbs:           item.Cost.Herbs,
			AnyHerb:         item.Cost.AnyHerb,
			Gold:            item.Cost.Gold,
			RequiredItemIDs: reqIDs,
		}
	}

	ghChecked := make(map[int]bool, len(u.GloomhavenIDs))
	for _, id := range u.GloomhavenIDs {
		ghChecked[id] = true
	}
	setIndex := map[int]int{}
	for _, item := range gh {
		available := u.Buildings.TradingPost >= item.TradingPostLevel
		idx, ok := setIndex[item.TradingPostLevel]
		if !ok {
			idx = len(data.GloomhavenSets)
			setIndex[item.TradingPostLevel] = idx
			data.GloomhavenSets = append(data.GloomhavenSets, pageGloomhavenSet{Level: item.TradingPostLevel, Visible: available})
		}
		set := &data.GloomhavenSets[idx]
		set.Items = append(set.Items, pageUnlockable{ID: item.ID, Discovered: ghChecked[item.ID]})

		if !available || !ghChecked[item.ID] {
			continue
		}
		data.Items = append(data.Items, pageItem{
			Key:         gloomhavenKey(item.ID),
			ID:          item.ID,
			Name:        item.Name,
			ImagePath:   fmt.Sprintf("/static/items/gh/%03d.png", item.ID),
			Slot:        item.Slot,
			Building:    fmt.Sprintf("%s.%d", buildingLabels[catalog.BuildingTradingPost], item.TradingPostLevel),
			Craftable:   true,
			CostSummary: fmt.Sprintf("%d gold", item.Gold),
		})
		costByID[gloomhavenKey(item.ID)] = pageItemCost{Gold: item.Gold}
	}

	costJSON, _ := json.Marshal(costByID)
	data.ItemCostJSON = template.JS(costJSON) //nolint:gosec // server-generated JSON, not user input
	return data
}

// costSummary lists resources, gold, and prerequisite items. Prerequisites
// are named only when unlocked, otherwise shown by ID to avoid spoilers.
func costSummary(cost catalog.Cost, byID map[int]catalog.Item, unlocked map[int]bool) string {
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
	for _, req := range cost.RequiredItems {
		name := fmt.Sprintf("item %d", req.ItemID)
		if unlocked[req.ItemID] {
			name = byID[req.ItemID].Name
		}
		parts = append(parts, fmt.Sprintf("%dx %s", req.Quantity, name))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " + ")
}

// isCraftable reports whether an item and its whole prerequisite chain are
// unlocked. memo caches results per ID.
func isCraftable(id int, byID map[int]catalog.Item, unlocked map[int]bool, memo map[int]bool) bool {
	if v, ok := memo[id]; ok {
		return v
	}
	ok := unlocked[id]
	for _, req := range byID[id].Cost.RequiredItems {
		ok = ok && isCraftable(req.ItemID, byID, unlocked, memo)
	}
	memo[id] = ok
	return ok
}
