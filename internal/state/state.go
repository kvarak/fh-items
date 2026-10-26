// Package state defines the mutable, per-campaign runtime state: the
// active crafting character's personal supply, the shared Frosthaven herb
// supply, building levels, and per-item unlock/availability tracking. This
// is distinct from the static catalog in internal/catalog.
package state

import (
	"fmt"

	"fh-items/internal/catalog"
)

// SchemaVersion is bumped whenever the State JSON shape changes in a
// backward-incompatible way, so save files can be checked for
// compatibility before loading.
const SchemaVersion = 1

// HerbSourcingRule controls where herb costs are paid from when both the
// shared Frosthaven supply and the active character's personal supply have
// some available.
type HerbSourcingRule string

const (
	// HerbSourceSharedFirst spends from the shared supply before dipping
	// into the character's personal herb supply.
	HerbSourceSharedFirst HerbSourcingRule = "sharedFirst"
	// HerbSourcePersonalFirst spends from the character's personal herb
	// supply before the shared supply.
	HerbSourcePersonalFirst HerbSourcingRule = "personalFirst"
	// HerbSourceSharedOnly never draws herbs from personal supply.
	HerbSourceSharedOnly HerbSourcingRule = "sharedOnly"
	// HerbSourcePersonalOnly never draws herbs from the shared supply.
	HerbSourcePersonalOnly HerbSourcingRule = "personalOnly"
)

func validHerbSourcingRule(r HerbSourcingRule) bool {
	switch r {
	case HerbSourceSharedFirst, HerbSourcePersonalFirst, HerbSourceSharedOnly, HerbSourcePersonalOnly:
		return true
	default:
		return false
	}
}

// BuildingLevels records the party's current building levels, which gate
// catalog items whose RequiredBuilding is set.
type BuildingLevels struct {
	Craftsman   int `json:"craftsman"`
	Jeweler     int `json:"jeweler"`
	TradingPost int `json:"tradingPost"`
}

// Level returns the party's current level for the given building type, or
// 0 if the building type is unrecognized.
func (b BuildingLevels) Level(building catalog.BuildingType) int {
	switch building {
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

// Character is the active crafting character: their personal material and
// herb supply, gold, and the items they currently own (which can be
// consumed as crafting ingredients for item upgrades).
type Character struct {
	// Materials holds the character's personal lumber/metal/hide supply.
	Materials map[catalog.ResourceType]int `json:"materials"`
	// Herbs holds the character's personal herb supply, by specific herb
	// type. Consulted per the HerbSourcingRule.
	Herbs map[catalog.ResourceType]int `json:"herbs"`
	Gold  int                          `json:"gold"`
	// OwnedItems lists item IDs this character currently owns, available
	// to be consumed as crafting ingredients for recipes that require
	// them (catalog.Cost.RequiredItems).
	OwnedItems []int `json:"ownedItems"`
}

// SharedSupply is the party-wide Frosthaven supply. For v1 this only
// tracks herbs (consulted per the HerbSourcingRule) since lumber/metal/hide
// and gold are modeled on the active Character.
type SharedSupply struct {
	Herbs map[catalog.ResourceType]int `json:"herbs"`
}

// ItemState is explicit, mutable per-item availability: whether it is
// unlocked and how many physical copies remain. This is tracked
// separately from the static catalog so availability is never inferred
// solely from building level - most items (blueprints, loot, scenario
// rewards) aren't gated by a building at all.
type ItemState struct {
	ItemID          int  `json:"itemId"`
	Unlocked        bool `json:"unlocked"`
	AvailableCopies int  `json:"availableCopies"`
}

// State is the full mutable runtime/campaign state for a single session.
type State struct {
	SchemaVersion int               `json:"schemaVersion"`
	Character     Character         `json:"character"`
	SharedSupply  SharedSupply      `json:"sharedSupply"`
	HerbSourcing  HerbSourcingRule  `json:"herbSourcing"`
	Buildings     BuildingLevels    `json:"buildings"`
	Items         map[int]ItemState `json:"items"`
}

// New builds a fresh State for the given catalog: building-gated items
// start unlocked exactly when their required building level is already
// met (0, i.e. not yet built, for a fresh campaign), and all items start
// with their full copy count available. Items without a RequiredBuilding
// (blueprints, loot, scenario rewards) start locked, since those require
// explicit in-game discovery to unlock.
func New(cat catalog.Catalog) State {
	s := State{
		SchemaVersion: SchemaVersion,
		Character: Character{
			Materials: map[catalog.ResourceType]int{},
			Herbs:     map[catalog.ResourceType]int{},
		},
		SharedSupply: SharedSupply{Herbs: map[catalog.ResourceType]int{}},
		HerbSourcing: HerbSourceSharedFirst,
		Items:        make(map[int]ItemState, len(cat.Items)),
	}
	for _, item := range cat.Items {
		s.Items[item.ID] = ItemState{
			ItemID:          item.ID,
			AvailableCopies: item.Count,
		}
	}
	s.RefreshBuildingGatedUnlocks(cat)
	return s
}

// RefreshBuildingGatedUnlocks recomputes the Unlocked flag for every item
// that has a catalog RequiredBuilding, based on the state's current
// Buildings levels. Items with no RequiredBuilding (blueprints, loot,
// scenario rewards) are left untouched, since their unlock status is
// tracked explicitly rather than derived.
func (s *State) RefreshBuildingGatedUnlocks(cat catalog.Catalog) {
	for _, item := range cat.Items {
		if item.RequiredBuilding == "" {
			continue
		}
		st, ok := s.Items[item.ID]
		if !ok {
			continue
		}
		st.Unlocked = s.Buildings.Level(item.RequiredBuilding) >= item.RequiredBuildingLevel
		s.Items[item.ID] = st
	}
}

// Validate checks structural integrity of the state: known herb sourcing
// rule, non-negative quantities, and non-negative/sane item state.
func (s State) Validate() error {
	var errs []error

	if s.SchemaVersion != SchemaVersion {
		errs = append(errs, fmt.Errorf("state schema version %d does not match expected %d", s.SchemaVersion, SchemaVersion))
	}
	if !validHerbSourcingRule(s.HerbSourcing) {
		errs = append(errs, fmt.Errorf("unknown herb sourcing rule %q", s.HerbSourcing))
	}

	for rt, qty := range s.Character.Materials {
		if qty < 0 {
			errs = append(errs, fmt.Errorf("character materials[%s] must not be negative, got %d", rt, qty))
		}
	}
	for rt, qty := range s.Character.Herbs {
		if qty < 0 {
			errs = append(errs, fmt.Errorf("character herbs[%s] must not be negative, got %d", rt, qty))
		}
	}
	if s.Character.Gold < 0 {
		errs = append(errs, fmt.Errorf("character gold must not be negative, got %d", s.Character.Gold))
	}
	for rt, qty := range s.SharedSupply.Herbs {
		if qty < 0 {
			errs = append(errs, fmt.Errorf("shared supply herbs[%s] must not be negative, got %d", rt, qty))
		}
	}
	for id, st := range s.Items {
		if st.ItemID != id {
			errs = append(errs, fmt.Errorf("items[%d] has mismatched itemId %d", id, st.ItemID))
		}
		if st.AvailableCopies < 0 {
			errs = append(errs, fmt.Errorf("items[%d] availableCopies must not be negative, got %d", id, st.AvailableCopies))
		}
	}

	if len(errs) == 0 {
		return nil
	}
	msg := fmt.Sprintf("%d validation error(s):", len(errs))
	for _, e := range errs {
		msg += "\n  - " + e.Error()
	}
	return fmt.Errorf("%s", msg)
}
