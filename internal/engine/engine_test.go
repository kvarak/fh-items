package engine

import (
	"strings"
	"testing"

	"fh-items/internal/catalog"
	"fh-items/internal/state"
)

// baseState returns a minimal, valid state with the given building levels
// already met so items are unlocked, ready for a test to customize.
func baseState() state.State {
	return state.State{
		SchemaVersion: state.SchemaVersion,
		Character: state.Character{
			Materials: map[catalog.ResourceType]int{},
			Herbs:     map[catalog.ResourceType]int{},
		},
		SharedSupply: state.SharedSupply{Herbs: map[catalog.ResourceType]int{}},
		HerbSourcing: state.HerbSourceSharedFirst,
		Items:        map[int]state.ItemState{},
	}
}

func unlockedItemState(id, availableCopies int) state.ItemState {
	return state.ItemState{ItemID: id, Unlocked: true, AvailableCopies: availableCopies}
}

// crudeShield (id 10) -> reinforcedShield (id 15) is the rulebook's own
// worked example: Reinforced Shield costs 1 lumber plus a Crude Shield.
func crudeShield() catalog.Item {
	return catalog.Item{
		ID: 10, Name: "Crude Shield", Count: 2,
		Cost: catalog.Cost{Materials: map[catalog.ResourceType]int{catalog.ResourceMetal: 1}},
	}
}

func reinforcedShield() catalog.Item {
	return catalog.Item{
		ID: 15, Name: "Reinforced Shield", Count: 2,
		Cost: catalog.Cost{
			Materials:     map[catalog.ResourceType]int{catalog.ResourceLumber: 1},
			RequiredItems: []catalog.ItemRequirement{{ItemID: 10, Quantity: 1}},
		},
	}
}

func TestCheckFeasibility_PrerequisiteOwnedByActiveCharacter(t *testing.T) {
	cat := catalog.Catalog{Items: []catalog.Item{crudeShield(), reinforcedShield()}}
	st := baseState()
	st.Character.OwnedItems = []int{10} // already owns the Crude Shield
	st.Character.Materials[catalog.ResourceLumber] = 1
	st.Items[10] = unlockedItemState(10, 1) // 1 copy still in the supply
	st.Items[15] = unlockedItemState(15, 1)

	result := CheckFeasibility(cat, st, Selection{15: 1})

	if !result.Feasible {
		t.Fatalf("expected feasible, got shortfalls: %v", result.Shortfalls)
	}
	if !hasConsumed(result.ConsumedPrerequisites, 10, 1) {
		t.Errorf("expected item 10 to be consumed as a prerequisite, got: %+v", result.ConsumedPrerequisites)
	}
	// The owned Crude Shield is returned to the available supply.
	if result.FinalAvailableCopies[10] != 2 {
		t.Errorf("FinalAvailableCopies[10] = %d, want 2 (returned to supply)", result.FinalAvailableCopies[10])
	}
	if result.FinalOwnedCount[10] != 0 {
		t.Errorf("FinalOwnedCount[10] = %d, want 0 (consumed)", result.FinalOwnedCount[10])
	}
}

func TestCheckFeasibility_PrerequisiteNotOwned_NoAutoCrafting(t *testing.T) {
	cat := catalog.Catalog{Items: []catalog.Item{crudeShield(), reinforcedShield()}}
	st := baseState()
	// Not owned, but a copy IS available in the supply and personal
	// resources could cover crafting one fresh. The engine must NOT do
	// this automatically - it should report a clear shortfall instead.
	st.Character.Materials[catalog.ResourceLumber] = 1
	st.Character.Materials[catalog.ResourceMetal] = 1
	st.Items[10] = unlockedItemState(10, 1)
	st.Items[15] = unlockedItemState(15, 1)

	result := CheckFeasibility(cat, st, Selection{15: 1})

	if result.Feasible {
		t.Fatal("expected infeasible (no automatic prerequisite crafting), got feasible")
	}
	if !hasShortfall(result.Shortfalls, ShortfallPrerequisite, "requires 1 x item 10 (Crude Shield) owned, but only 0 owned") {
		t.Errorf("expected a clear missing-prerequisite shortfall, got: %v", result.Shortfalls)
	}
	if len(result.ConsumedPrerequisites) != 0 {
		t.Errorf("expected no prerequisites consumed, got: %+v", result.ConsumedPrerequisites)
	}
}

func TestCheckFeasibility_PrerequisiteOwnedByAnotherCharacter_NoSubstitution(t *testing.T) {
	cat := catalog.Catalog{Items: []catalog.Item{crudeShield(), reinforcedShield()}}
	st := baseState()
	// Not owned by the active character, and none left in the supply
	// (i.e. presumably held by another party member). The engine must
	// not substitute its cost - it's simply missing for this check.
	st.Character.Materials[catalog.ResourceLumber] = 1
	st.Character.Materials[catalog.ResourceMetal] = 1
	st.Items[10] = unlockedItemState(10, 0)
	st.Items[15] = unlockedItemState(15, 1)

	result := CheckFeasibility(cat, st, Selection{15: 1})

	if result.Feasible {
		t.Fatal("expected infeasible (no cost substitution), got feasible")
	}
	if !hasShortfall(result.Shortfalls, ShortfallPrerequisite, "requires 1 x item 10 (Crude Shield) owned, but only 0 owned") {
		t.Errorf("expected a clear missing-prerequisite shortfall, got: %v", result.Shortfalls)
	}
}

func TestCheckFeasibility_PrerequisiteUnavailableAndNotOwned(t *testing.T) {
	cat := catalog.Catalog{Items: []catalog.Item{crudeShield(), reinforcedShield()}}
	st := baseState()
	st.Character.Materials[catalog.ResourceLumber] = 1
	st.Items[10] = unlockedItemState(10, 0)
	st.Items[15] = unlockedItemState(15, 1)

	result := CheckFeasibility(cat, st, Selection{15: 1})

	if result.Feasible {
		t.Fatal("expected infeasible, got feasible")
	}
	if len(result.Shortfalls) == 0 {
		t.Error("expected at least one shortfall explaining why")
	}
}

func TestCheckFeasibility_ChainedUpgrades_AllOwned(t *testing.T) {
	// A (1) -> B (2) -> C (3): each requires 1 of the previous. Crafting C
	// is feasible only because A and B are both already owned.
	a := catalog.Item{ID: 1, Name: "A", Count: 2, Cost: catalog.Cost{Materials: map[catalog.ResourceType]int{catalog.ResourceMetal: 1}}}
	b := catalog.Item{ID: 2, Name: "B", Count: 2, Cost: catalog.Cost{
		Materials: map[catalog.ResourceType]int{catalog.ResourceMetal: 1}, RequiredItems: []catalog.ItemRequirement{{ItemID: 1, Quantity: 1}},
	}}
	c := catalog.Item{ID: 3, Name: "C", Count: 2, Cost: catalog.Cost{
		Materials: map[catalog.ResourceType]int{catalog.ResourceMetal: 1}, RequiredItems: []catalog.ItemRequirement{{ItemID: 2, Quantity: 1}},
	}}
	cat := catalog.Catalog{Items: []catalog.Item{a, b, c}}

	st := baseState()
	st.Character.Materials[catalog.ResourceMetal] = 1 // only C's own cost
	st.Character.OwnedItems = []int{1, 2}             // A and B already owned
	st.Items[1] = unlockedItemState(1, 2)
	st.Items[2] = unlockedItemState(2, 2)
	st.Items[3] = unlockedItemState(3, 2)

	result := CheckFeasibility(cat, st, Selection{3: 1})

	if !result.Feasible {
		t.Fatalf("expected feasible (both prerequisites owned), got shortfalls: %v", result.Shortfalls)
	}
	if result.Totals.Materials[catalog.ResourceMetal] != 1 {
		t.Errorf("metal spent = %d, want 1 (only C's own cost)", result.Totals.Materials[catalog.ResourceMetal])
	}
	if !hasConsumed(result.ConsumedPrerequisites, 2, 1) {
		t.Errorf("expected B (item 2) to be consumed as C's prerequisite, got: %+v", result.ConsumedPrerequisites)
	}
}

func TestCheckFeasibility_ChainedUpgrades_MissingLinkReportsShortfall(t *testing.T) {
	// Same chain, but B is not owned: crafting C must fail clearly, and
	// must not fall back to auto-crafting B from A.
	a := catalog.Item{ID: 1, Name: "A", Count: 2, Cost: catalog.Cost{Materials: map[catalog.ResourceType]int{catalog.ResourceMetal: 1}}}
	b := catalog.Item{ID: 2, Name: "B", Count: 2, Cost: catalog.Cost{
		Materials: map[catalog.ResourceType]int{catalog.ResourceMetal: 1}, RequiredItems: []catalog.ItemRequirement{{ItemID: 1, Quantity: 1}},
	}}
	c := catalog.Item{ID: 3, Name: "C", Count: 2, Cost: catalog.Cost{
		Materials: map[catalog.ResourceType]int{catalog.ResourceMetal: 1}, RequiredItems: []catalog.ItemRequirement{{ItemID: 2, Quantity: 1}},
	}}
	cat := catalog.Catalog{Items: []catalog.Item{a, b, c}}

	st := baseState()
	st.Character.Materials[catalog.ResourceMetal] = 3
	st.Character.OwnedItems = []int{1} // owns A, but not B
	st.Items[1] = unlockedItemState(1, 2)
	st.Items[2] = unlockedItemState(2, 2)
	st.Items[3] = unlockedItemState(3, 2)

	result := CheckFeasibility(cat, st, Selection{3: 1})

	if result.Feasible {
		t.Fatal("expected infeasible (B is not owned, must not auto-craft it), got feasible")
	}
	if !hasShortfall(result.Shortfalls, ShortfallPrerequisite, "requires 1 x item 2 (B) owned, but only 0 owned") {
		t.Errorf("expected a shortfall naming the missing link (B), got: %v", result.Shortfalls)
	}
}

func TestCheckFeasibility_HerbSourcingFromSharedSupply(t *testing.T) {
	potion := catalog.Item{
		ID: 100, Name: "Stamina Potion", Count: 2,
		Cost: catalog.Cost{Herbs: map[catalog.ResourceType]int{catalog.ResourceArrowvine: 1}},
	}
	cat := catalog.Catalog{Items: []catalog.Item{potion}}

	st := baseState()
	// None in personal supply; the shared Frosthaven supply covers it.
	st.SharedSupply.Herbs[catalog.ResourceArrowvine] = 1
	st.Items[100] = unlockedItemState(100, 1)

	result := CheckFeasibility(cat, st, Selection{100: 1})

	if !result.Feasible {
		t.Fatalf("expected feasible using shared herb supply, got shortfalls: %v", result.Shortfalls)
	}
	if result.Totals.Herbs[catalog.ResourceArrowvine] != 1 {
		t.Errorf("arrowvine spent = %d, want 1", result.Totals.Herbs[catalog.ResourceArrowvine])
	}
}

func TestCheckFeasibility_InsufficientPersonalMaterials(t *testing.T) {
	cat := catalog.Catalog{Items: []catalog.Item{crudeShield()}}
	st := baseState()
	// No metal at all, and metal is a personal-only resource - the shared
	// supply must not be able to cover it.
	st.SharedSupply.Herbs = map[catalog.ResourceType]int{}
	st.Items[10] = unlockedItemState(10, 2)

	result := CheckFeasibility(cat, st, Selection{10: 1})

	if result.Feasible {
		t.Fatal("expected infeasible due to insufficient personal metal, got feasible")
	}
	if !hasShortfall(result.Shortfalls, ShortfallResource, "metal") {
		t.Errorf("expected a resource-kind shortfall mentioning metal, got: %v", result.Shortfalls)
	}
}

func TestCheckFeasibility_DuplicateItemConsumptionAndAvailableCopyAccounting(t *testing.T) {
	cat := catalog.Catalog{Items: []catalog.Item{crudeShield(), reinforcedShield()}}
	st := baseState()
	st.Character.OwnedItems = []int{10, 10} // owns exactly two Crude Shields
	st.Character.Materials[catalog.ResourceLumber] = 2
	st.Items[10] = unlockedItemState(10, 0)
	st.Items[15] = unlockedItemState(15, 2)

	// Craft two Reinforced Shields: both prerequisites are already owned.
	result := CheckFeasibility(cat, st, Selection{15: 2})

	if !result.Feasible {
		t.Fatalf("expected feasible, got shortfalls: %v", result.Shortfalls)
	}
	if result.Totals.Materials[catalog.ResourceLumber] != 2 {
		t.Errorf("lumber spent = %d, want 2 (one per Reinforced Shield)", result.Totals.Materials[catalog.ResourceLumber])
	}
	if !hasConsumed(result.ConsumedPrerequisites, 10, 2) {
		t.Errorf("expected 2 copies of item 10 consumed, got: %+v", result.ConsumedPrerequisites)
	}
	// Both owned Crude Shields are returned to the supply; none remain owned.
	if result.FinalAvailableCopies[10] != 2 {
		t.Errorf("FinalAvailableCopies[10] = %d, want 2", result.FinalAvailableCopies[10])
	}
	if result.FinalOwnedCount[10] != 0 {
		t.Errorf("FinalOwnedCount[10] = %d, want 0", result.FinalOwnedCount[10])
	}
}

func TestCheckFeasibility_DuplicateSelection_InsufficientOwnedPrerequisites(t *testing.T) {
	cat := catalog.Catalog{Items: []catalog.Item{crudeShield(), reinforcedShield()}}
	st := baseState()
	st.Character.OwnedItems = []int{10} // only one owned, but two are needed
	st.Character.Materials[catalog.ResourceLumber] = 2
	st.Items[10] = unlockedItemState(10, 5) // plenty available in the supply - must not matter
	st.Items[15] = unlockedItemState(15, 2)

	result := CheckFeasibility(cat, st, Selection{15: 2})

	if result.Feasible {
		t.Fatal("expected infeasible: only 1 of 2 needed Crude Shields is owned, and none may be auto-crafted")
	}
	if !hasShortfall(result.Shortfalls, ShortfallPrerequisite, "requires 2 x item 10 (Crude Shield) owned, but only 1 owned") {
		t.Errorf("expected a shortfall naming the exact shortfall quantity, got: %v", result.Shortfalls)
	}
}

func TestCheckFeasibility_UnlockedAndAvailabilityStillEnforced(t *testing.T) {
	cat := catalog.Catalog{Items: []catalog.Item{crudeShield()}}
	st := baseState()
	st.Character.Materials[catalog.ResourceMetal] = 5
	st.Items[10] = state.ItemState{ItemID: 10, Unlocked: false, AvailableCopies: 2}

	result := CheckFeasibility(cat, st, Selection{10: 1})
	if result.Feasible {
		t.Fatal("expected infeasible: item is not unlocked")
	}
	if !hasShortfall(result.Shortfalls, ShortfallLocked, "not unlocked") {
		t.Errorf("expected a shortfall about being locked, got: %v", result.Shortfalls)
	}
}

func hasConsumed(consumed []ConsumedPrerequisite, itemID, qty int) bool {
	for _, c := range consumed {
		if c.ItemID == itemID && c.Quantity == qty {
			return true
		}
	}
	return false
}

func hasShortfall(shortfalls []Shortfall, kind ShortfallKind, substr string) bool {
	for _, s := range shortfalls {
		if s.Kind == kind && strings.Contains(s.Message, substr) {
			return true
		}
	}
	return false
}
