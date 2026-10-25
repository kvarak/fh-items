package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// sourceItem mirrors the subset of .github/docs/items.json fields that
// feed the app catalog. The source file is treated as immutable; extra
// fields present in the source but not read here are intentionally
// ignored.
type sourceItem struct {
	ID                     int                        `json:"id"`
	Name                   string                     `json:"name"`
	Slot                   string                     `json:"slot"`
	Count                  int                        `json:"count"`
	Resources              map[string]json.RawMessage `json:"resources"`
	Source                 string                     `json:"source"`
	Desc                   string                     `json:"desc"`
	Cost                   int                        `json:"cost"`
	UnlockCrafstmanLevel   *int                       `json:"unlockCrafstmanLevel"`
	UnlockJewelerLevel     *int                       `json:"unlockJewelerLevel"`
	UnlockTradingPostLevel *int                       `json:"unlockTradingPostLevel"`
}

// GenerateFromSource builds a Catalog from the raw bytes of
// .github/docs/items.json. The returned Catalog's SourceChecksum is the
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
		ID:          src.ID,
		Name:        src.Name,
		Slot:        src.Slot,
		Count:       src.Count,
		Cost:        cost,
		Source:      src.Source,
		Description: src.Desc,
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
		// -1 in the source means "initially available", i.e. no level
		// requirement beyond the building existing.
		if *src.UnlockTradingPostLevel > 0 {
			item.RequiredBuildingLevel = *src.UnlockTradingPostLevel
		}
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
			cost.RequiredItems = append(cost.RequiredItems, ids...)
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
	sort.Ints(cost.RequiredItems)
	sort.Ints(cost.AnyHerb)

	return cost, nil
}
