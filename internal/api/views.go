package api

import "fh-items/internal/catalog"

// RequiredItemView is a prerequisite item requirement, with its name
// resolved for display.
type RequiredItemView struct {
	ItemID   int    `json:"itemId"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

// ItemView is a catalog item merged with its current per-campaign state.
type ItemView struct {
	ID                    int                          `json:"id"`
	Name                  string                       `json:"name"`
	Slot                  string                       `json:"slot,omitempty"`
	Materials             map[catalog.ResourceType]int `json:"materials,omitempty"`
	Herbs                 map[catalog.ResourceType]int `json:"herbs,omitempty"`
	AnyHerb               []int                        `json:"anyHerb,omitempty"`
	RequiredItems         []RequiredItemView           `json:"requiredItems,omitempty"`
	Gold                  int                          `json:"gold,omitempty"`
	RequiredBuilding      string                       `json:"requiredBuilding,omitempty"`
	RequiredBuildingLevel int                          `json:"requiredBuildingLevel,omitempty"`
	Unlocked              bool                         `json:"unlocked"`
	AvailableCopies       int                          `json:"availableCopies"`
	Source                string                       `json:"source,omitempty"`
	Description           string                       `json:"description,omitempty"`
}

// CharacterView is the active character's supply, for display and as a
// baseline for POST /api/check.
type CharacterView struct {
	Materials  map[catalog.ResourceType]int `json:"materials"`
	Herbs      map[catalog.ResourceType]int `json:"herbs"`
	Gold       int                          `json:"gold"`
	OwnedItems []int                        `json:"ownedItems"`
}

// BuildingsView is the party's current building levels.
type BuildingsView struct {
	Craftsman   int `json:"craftsman"`
	Jeweler     int `json:"jeweler"`
	TradingPost int `json:"tradingPost"`
}

// StateView is the full GET /api/state response.
type StateView struct {
	Character    CharacterView                `json:"character"`
	SharedHerbs  map[catalog.ResourceType]int `json:"sharedHerbs"`
	HerbSourcing string                       `json:"herbSourcing"`
	Buildings    BuildingsView                `json:"buildings"`
	Items        []ItemView                   `json:"items"`
}

func itemView(item catalog.Item, is itemAvailability, byID map[int]catalog.Item) ItemView {
	v := ItemView{
		ID:                    item.ID,
		Name:                  item.Name,
		Slot:                  item.Slot,
		Materials:             item.Cost.Materials,
		Herbs:                 item.Cost.Herbs,
		AnyHerb:               item.Cost.AnyHerb,
		Gold:                  item.Cost.Gold,
		RequiredBuilding:      string(item.RequiredBuilding),
		RequiredBuildingLevel: item.RequiredBuildingLevel,
		Unlocked:              is.Unlocked,
		AvailableCopies:       is.AvailableCopies,
		Source:                item.Source,
		Description:           item.Description,
	}
	for _, req := range item.Cost.RequiredItems {
		name := ""
		if reqItem, ok := byID[req.ItemID]; ok {
			name = reqItem.Name
		}
		v.RequiredItems = append(v.RequiredItems, RequiredItemView{ItemID: req.ItemID, Name: name, Quantity: req.Quantity})
	}
	return v
}

// itemAvailability is the subset of state.ItemState a view needs.
type itemAvailability struct {
	Unlocked        bool
	AvailableCopies int
}
