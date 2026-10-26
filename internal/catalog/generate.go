package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// sourceItem mirrors the subset of the source items.json fields that
// feed the app catalog. The source file is treated as immutable; extra
// fields present in the source but not read here are intentionally
// ignored.
type sourceItem struct {
	ID                     int                        `json:"id"`
	Name                   string                     `json:"name"`
	Slot                   string                     `json:"slot"`
	Resources              map[string]json.RawMessage `json:"resources"`
	Cost                   int                        `json:"cost"`
	UnlockCrafstmanLevel   *int                       `json:"unlockCrafstmanLevel"`
	UnlockJewelerLevel     *int                       `json:"unlockJewelerLevel"`
	UnlockTradingPostLevel *int                       `json:"unlockTradingPostLevel"`
}

// GenerateFromSource builds a Catalog from the raw bytes of the source
// items.json. The returned Catalog's SourceChecksum is the
// sha256 of sourceJSON, so staleness against the immutable source file can
// be detected later.
func GenerateFromSource(sourceJSON []byte) (Catalog, error) {
	var sourceItems []sourceItem
	if err := json.Unmarshal(sourceJSON, &sourceItems); err != nil {
		return Catalog{}, fmt.Errorf("parsing source items.json: %w", err)
	}

	items := make([]Item, 0, len(sourceItems))
	seen := make(map[int]bool, len(sourceItems))
	for _, src := range sourceItems {
		// Some source entries repeat the same id for cosmetic image
		// variants of one physical item (e.g. multiple Ancient Coin
		// faces); keep only the first occurrence.
		if seen[src.ID] {
			continue
		}
		seen[src.ID] = true

		item, err := convertItem(src)
		if err != nil {
			return Catalog{}, fmt.Errorf("converting item %d (%s): %w", src.ID, src.Name, err)
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	sum := sha256.Sum256(sourceJSON)
	return Catalog{
		SchemaVersion:  SchemaVersion,
		SourceChecksum: hex.EncodeToString(sum[:]),
		Items:          items,
	}, nil
}

func convertItem(src sourceItem) (Item, error) {
	cost, err := convertCost(src)
	if err != nil {
		return Item{}, err
	}

	item := Item{
		ID:   src.ID,
		Name: src.Name,
		Slot: src.Slot,
		Cost: cost,
	}
	// The source calls the feet slot "legs"; the game cards say feet.
	if item.Slot == "legs" {
		item.Slot = "feet"
	}

	switch {
	case src.UnlockCrafstmanLevel != nil:
		item.RequiredBuilding = BuildingCraftsman
		item.RequiredBuildingLevel = *src.UnlockCrafstmanLevel
	case src.UnlockJewelerLevel != nil:
		item.RequiredBuilding = BuildingJeweler
		item.RequiredBuildingLevel = *src.UnlockJewelerLevel
	case src.UnlockTradingPostLevel != nil:
		item.RequiredBuilding = BuildingTradingPost
		// -1 in the source marks the starting stock, available from level 1.
		item.RequiredBuildingLevel = max(*src.UnlockTradingPostLevel, 1)
	}

	return item, nil
}

func convertCost(src sourceItem) (Cost, error) {
	cost := Cost{Gold: src.Cost}
	if len(src.Resources) == 0 {
		return cost, nil
	}

	for key, raw := range src.Resources {
		switch key {
		case "item":
			var ids []int
			if err := json.Unmarshal(raw, &ids); err != nil {
				return Cost{}, fmt.Errorf("resources.item: %w", err)
			}
			// Group repeated ids into a quantity; the source data has
			// never actually repeated one within a single item, but this
			// keeps the conversion correct if it ever does.
			qtyByID := make(map[int]int, len(ids))
			order := make([]int, 0, len(ids))
			for _, id := range ids {
				if qtyByID[id] == 0 {
					order = append(order, id)
				}
				qtyByID[id]++
			}
			for _, id := range order {
				cost.RequiredItems = append(cost.RequiredItems, ItemRequirement{ItemID: id, Quantity: qtyByID[id]})
			}
		case "any":
			var qtys []int
			if err := json.Unmarshal(raw, &qtys); err != nil {
				return Cost{}, fmt.Errorf("resources.any: %w", err)
			}
			cost.AnyHerb = append(cost.AnyHerb, qtys...)
		default:
			var qty int
			if err := json.Unmarshal(raw, &qty); err != nil {
				return Cost{}, fmt.Errorf("resources.%s: %w", key, err)
			}
			rt := ResourceType(key)
			if isMaterialType(rt) {
				if cost.Materials == nil {
					cost.Materials = map[ResourceType]int{}
				}
				cost.Materials[rt] = qty
			} else if isHerbType(rt) {
				if cost.Herbs == nil {
					cost.Herbs = map[ResourceType]int{}
				}
				cost.Herbs[rt] = qty
			} else {
				return Cost{}, fmt.Errorf("unknown resource key %q", key)
			}
		}
	}
	sort.Slice(cost.RequiredItems, func(i, j int) bool { return cost.RequiredItems[i].ItemID < cost.RequiredItems[j].ItemID })
	sort.Ints(cost.AnyHerb)

	return cost, nil
}
