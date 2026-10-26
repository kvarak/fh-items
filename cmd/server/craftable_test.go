package main

import (
	"testing"

	"fh-items/internal/catalog"
	"fh-items/internal/state"
)

func TestIsCraftable_RequiresUnlockedPrerequisites(t *testing.T) {
	byID := map[int]catalog.Item{
		1: {ID: 1, Name: "Unhealthy Mixture"},
		2: {ID: 2, Name: "Cured Leather Armor", Cost: catalog.Cost{RequiredItems: []catalog.ItemRequirement{{ItemID: 1, Quantity: 1}}}},
	}
	st := state.State{Items: map[int]state.ItemState{
		2: {ItemID: 2, Unlocked: true, AvailableCopies: 1},
	}}

	if isCraftable(2, byID, st, map[int]bool{}) {
		t.Error("item 2 should not be craftable while its prerequisite is undiscovered")
	}

	st.Items[1] = state.ItemState{ItemID: 1, Unlocked: true, AvailableCopies: 1}
	if !isCraftable(2, byID, st, map[int]bool{}) {
		t.Error("item 2 should be craftable once its prerequisite is unlocked and in stock")
	}
}

func TestRequiredItemsSummary_NamesOnlyUnlockedPrerequisites(t *testing.T) {
	byID := map[int]catalog.Item{
		1: {ID: 1, Name: "Unhealthy Mixture"},
	}
	cost := catalog.Cost{RequiredItems: []catalog.ItemRequirement{{ItemID: 1, Quantity: 2}}}

	locked := requiredItemsSummary(cost, byID, state.State{Items: map[int]state.ItemState{}})
	if locked != "2x item 1" {
		t.Errorf("locked prerequisite: got %q, want %q", locked, "2x item 1")
	}

	st := state.State{Items: map[int]state.ItemState{1: {ItemID: 1, Unlocked: true}}}
	unlocked := requiredItemsSummary(cost, byID, st)
	if unlocked != "2x Unhealthy Mixture" {
		t.Errorf("unlocked prerequisite: got %q, want %q", unlocked, "2x Unhealthy Mixture")
	}
}
