package api

import (
	"encoding/json"
	"net/http"

	"fh-items/internal/catalog"
	"fh-items/internal/engine"
	"fh-items/internal/state"
)

// handleGetState serves GET /api/state: the active character's supply,
// the shared herb supply, the herb sourcing rule, building levels, and
// every catalog item merged with its current unlock/availability state.
func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	st := s.state
	s.mu.RUnlock()

	byID := make(map[int]catalog.Item, len(s.Catalog.Items))
	for _, item := range s.Catalog.Items {
		byID[item.ID] = item
	}

	items := make([]ItemView, 0, len(s.Catalog.Items))
	for _, item := range s.Catalog.Items {
		is := st.Items[item.ID]
		items = append(items, itemView(item, itemAvailability{Unlocked: is.Unlocked, AvailableCopies: is.AvailableCopies}, byID))
	}

	writeJSON(w, StateView{
		Character: CharacterView{
			Materials:  st.Character.Materials,
			Herbs:      st.Character.Herbs,
			Gold:       st.Character.Gold,
			OwnedItems: st.Character.OwnedItems,
		},
		SharedHerbs:  st.SharedSupply.Herbs,
		HerbSourcing: string(st.HerbSourcing),
		Buildings: BuildingsView{
			Craftsman:   st.Buildings.Craftsman,
			Jeweler:     st.Buildings.Jeweler,
			TradingPost: st.Buildings.TradingPost,
		},
		Items: items,
	})
}

// CharacterOverride lets a POST /api/check request supply ad-hoc resource
// values for the check instead of the server's persisted character. Any
// field left nil/omitted falls back to the persisted value.
type CharacterOverride struct {
	Materials  map[catalog.ResourceType]int `json:"materials,omitempty"`
	Herbs      map[catalog.ResourceType]int `json:"herbs,omitempty"`
	Gold       *int                         `json:"gold,omitempty"`
	OwnedItems []int                        `json:"ownedItems,omitempty"`
}

// CheckRequest is the POST /api/check request body.
type CheckRequest struct {
	// Items maps item ID to the quantity to check crafting.
	Items map[int]int `json:"items"`
	// Character/SharedHerbs/HerbSourcing/Buildings/UnlockedItemIDs
	// optionally override the persisted state for this one check, so the
	// UI can let a user try out "what if I had these resources/buildings/
	// unlocks" without saving anything.
	Character    *CharacterOverride           `json:"character,omitempty"`
	SharedHerbs  map[catalog.ResourceType]int `json:"sharedHerbs,omitempty"`
	HerbSourcing string                       `json:"herbSourcing,omitempty"`
	Buildings    *BuildingsOverride           `json:"buildings,omitempty"`
	// UnlockedItemIDs lists, by ID only (never by name - this must stay
	// spoiler-free), which items with no building requirement (found via
	// blueprints/loot/scenario rewards) are considered discovered for this
	// check. Items not listed are treated as not yet discovered.
	UnlockedItemIDs []int `json:"unlockedItemIds,omitempty"`
}

// BuildingsOverride lets a POST /api/check request supply ad-hoc building
// levels instead of the server's persisted levels. Any field left
// nil/omitted falls back to the persisted value.
type BuildingsOverride struct {
	Craftsman   *int `json:"craftsman,omitempty"`
	Jeweler     *int `json:"jeweler,omitempty"`
	TradingPost *int `json:"tradingPost,omitempty"`
}

// ConsumedView is a prerequisite item consumed from owned items.
type ConsumedView struct {
	ItemID   int `json:"itemId"`
	Quantity int `json:"quantity"`
}

// ShortfallView categorizes one reason a check failed: "locked" (not
// unlocked or no copies available), "resource" (insufficient materials/
// herbs/gold), "prerequisite" (a required item isn't owned), or "other".
type ShortfallView struct {
	Kind    string `json:"kind"`
	ItemID  int    `json:"itemId"`
	Message string `json:"message"`
}

// CheckResponse is the POST /api/check response body.
type CheckResponse struct {
	Feasible              bool            `json:"feasible"`
	Shortfalls            []ShortfallView `json:"shortfalls"`
	ConsumedPrerequisites []ConsumedView  `json:"consumedPrerequisites"`
	Spent                 ResourceDelta   `json:"spent"`
	Remaining             ResourceDelta   `json:"remaining"`
}

// ResourceDelta groups personal materials, combined herbs (personal +
// shared), and gold - either spent by a check, or remaining afterward.
type ResourceDelta struct {
	Materials map[catalog.ResourceType]int `json:"materials"`
	Herbs     map[catalog.ResourceType]int `json:"herbs"`
	Gold      int                          `json:"gold"`
}

// handlePostCheck serves POST /api/check: given a selected set of items
// (and optional resource overrides), it runs the strict, owned-items-only
// affordability check and reports feasibility, remaining resources, and
// shortfall details.
func (s *Server) handlePostCheck(w http.ResponseWriter, r *http.Request) {
	var req CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "could not parse request body: "+err.Error())
		return
	}

	s.mu.RLock()
	st := s.state
	s.mu.RUnlock()

	st = applyOverrides(s.Catalog, st, req)

	result := engine.CheckFeasibility(s.Catalog, st, engine.Selection(req.Items))

	consumed := make([]ConsumedView, 0, len(result.ConsumedPrerequisites))
	for _, c := range result.ConsumedPrerequisites {
		consumed = append(consumed, ConsumedView{ItemID: c.ItemID, Quantity: c.Quantity})
	}
	shortfalls := make([]ShortfallView, 0, len(result.Shortfalls))
	for _, s := range result.Shortfalls {
		shortfalls = append(shortfalls, ShortfallView{Kind: string(s.Kind), ItemID: s.ItemID, Message: s.Message})
	}

	writeJSON(w, CheckResponse{
		Feasible:              result.Feasible,
		Shortfalls:            shortfalls,
		ConsumedPrerequisites: consumed,
		Spent: ResourceDelta{
			Materials: result.Totals.Materials,
			Herbs:     result.Totals.Herbs,
			Gold:      result.Totals.Gold,
		},
		Remaining: remainingResources(st, result),
	})
}

// applyOverrides returns a copy of st with any request overrides applied,
// without mutating the server's persisted state. st.Items is cloned
// first since maps in Go are reference types - without this, the
// Buildings/UnlockedItemIDs overrides below would mutate the server's
// live, persisted item states in place.
func applyOverrides(cat catalog.Catalog, st state.State, req CheckRequest) state.State {
	clonedItems := make(map[int]state.ItemState, len(st.Items))
	for id, is := range st.Items {
		clonedItems[id] = is
	}
	st.Items = clonedItems

	if req.Character != nil {
		if req.Character.Materials != nil {
			st.Character.Materials = req.Character.Materials
		}
		if req.Character.Herbs != nil {
			st.Character.Herbs = req.Character.Herbs
		}
		if req.Character.Gold != nil {
			st.Character.Gold = *req.Character.Gold
		}
		if req.Character.OwnedItems != nil {
			st.Character.OwnedItems = req.Character.OwnedItems
		}
	}
	if req.SharedHerbs != nil {
		st.SharedSupply.Herbs = req.SharedHerbs
	}
	if req.HerbSourcing != "" {
		st.HerbSourcing = state.HerbSourcingRule(req.HerbSourcing)
	}
	ApplyBuildingsAndUnlocks(cat, &st, req.Buildings, req.UnlockedItemIDs)
	return st
}

// ApplyBuildingsAndUnlocks mutates st's building levels and item unlock
// flags in place. Shared by /api/check's transient overrides and by
// cmd/server's cookie-driven page rendering, so both apply the exact same
// building-gating/discovery logic.
func ApplyBuildingsAndUnlocks(cat catalog.Catalog, st *state.State, buildings *BuildingsOverride, unlockedItemIDs []int) {
	if buildings != nil {
		if buildings.Craftsman != nil {
			st.Buildings.Craftsman = *buildings.Craftsman
		}
		if buildings.Jeweler != nil {
			st.Buildings.Jeweler = *buildings.Jeweler
		}
		if buildings.TradingPost != nil {
			st.Buildings.TradingPost = *buildings.TradingPost
		}
		st.RefreshBuildingGatedUnlocks(cat)
	}
	if unlockedItemIDs != nil {
		unlocked := make(map[int]bool, len(unlockedItemIDs))
		for _, id := range unlockedItemIDs {
			unlocked[id] = true
		}
		for _, item := range cat.Items {
			if item.RequiredBuilding != "" {
				continue // building-gated items are controlled by Buildings, not this list
			}
			is := st.Items[item.ID]
			is.Unlocked = unlocked[item.ID]
			st.Items[item.ID] = is
		}
	}
}

// remainingResources computes personal materials, combined (personal +
// shared) herbs, and gold left over after a check's spend.
func remainingResources(st state.State, result engine.Result) ResourceDelta {
	materials := map[catalog.ResourceType]int{}
	for rt, qty := range st.Character.Materials {
		materials[rt] = qty - result.Totals.Materials[rt]
	}

	herbs := map[catalog.ResourceType]int{}
	for _, rt := range catalog.HerbTypes {
		combined := st.Character.Herbs[rt] + st.SharedSupply.Herbs[rt]
		herbs[rt] = combined - result.Totals.Herbs[rt]
	}

	return ResourceDelta{
		Materials: materials,
		Herbs:     herbs,
		Gold:      st.Character.Gold - result.Totals.Gold,
	}
}
