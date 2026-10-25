package state

import (
	"strings"
	"testing"

	"fh-items/internal/catalog"
)

func testCatalog() catalog.Catalog {
	return catalog.Catalog{
		SchemaVersion:  catalog.SchemaVersion,
		SourceChecksum: "abc",
		Items: []catalog.Item{
			{ID: 1, Name: "Crude Shield", Count: 2, Cost: catalog.Cost{Materials: map[catalog.ResourceType]int{catalog.ResourceMetal: 1}},
				RequiredBuilding: catalog.BuildingCraftsman, RequiredBuildingLevel: 1},
			{ID: 2, Name: "Reinforced Shield", Count: 2,
				Cost:                  catalog.Cost{Materials: map[catalog.ResourceType]int{catalog.ResourceLumber: 1}, RequiredItems: []int{1}},
				RequiredBuilding:      catalog.BuildingCraftsman,
				RequiredBuildingLevel: 3},
			{ID: 3, Name: "Random Blueprint Item", Count: 2, Cost: catalog.Cost{Materials: map[catalog.ResourceType]int{catalog.ResourceHide: 1}}},
			{ID: 4, Name: "Amulet of Life", Count: 2, Cost: catalog.Cost{Gold: 15}, RequiredBuilding: catalog.BuildingTradingPost},
		},
	}
}

func TestNew_InitializesItemStateFromCatalog(t *testing.T) {
	cat := testCatalog()
	s := New(cat)

	if err := s.Validate(); err != nil {
		t.Fatalf("fresh state failed validation: %v", err)
	}

	// Craftsman-1-gated item: building starts at level 0, so it's locked.
	if s.Items[1].Unlocked {
		t.Error("item 1 (requires Craftsman 1) should start locked when Craftsman is level 0")
	}
	// Trading-post item with no level requirement (source -1): unlocked
	// as soon as the building exists conceptually, independent of level.
	if !s.Items[4].Unlocked {
		t.Error("item 4 (no required level) should start unlocked")
	}
	// Blueprint/loot item with no RequiredBuilding: explicit, starts
	// locked until the player marks it discovered.
	if s.Items[3].Unlocked {
		t.Error("item 3 (no RequiredBuilding) should start locked, unlock is tracked explicitly")
	}
	if s.Items[1].AvailableCopies != 2 {
		t.Errorf("item 1 AvailableCopies = %d, want 2 (from catalog Count)", s.Items[1].AvailableCopies)
	}
}

func TestRefreshBuildingGatedUnlocks(t *testing.T) {
	cat := testCatalog()
	s := New(cat)

	s.Buildings.Craftsman = 1
	s.RefreshBuildingGatedUnlocks(cat)
	if !s.Items[1].Unlocked {
		t.Error("item 1 (requires Craftsman 1) should unlock once Craftsman reaches level 1")
	}
	if s.Items[2].Unlocked {
		t.Error("item 2 (requires Craftsman 3) should stay locked at Craftsman level 1")
	}

	// Explicit, non-building-gated items are untouched by refresh.
	s.Items[3] = ItemState{ItemID: 3, Unlocked: true, AvailableCopies: 1}
	s.RefreshBuildingGatedUnlocks(cat)
	if !s.Items[3].Unlocked {
		t.Error("item 3 (no RequiredBuilding) should be left untouched by RefreshBuildingGatedUnlocks")
	}
}

func TestValidate_RejectsUnknownHerbSourcingRule(t *testing.T) {
	s := New(testCatalog())
	s.HerbSourcing = "madeUpRule"
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "unknown herb sourcing rule") {
		t.Errorf("Validate() error = %v, want it to mention unknown herb sourcing rule", err)
	}
}

func TestValidate_RejectsNegativeQuantities(t *testing.T) {
	s := New(testCatalog())
	s.Character.Materials[catalog.ResourceMetal] = -1
	s.Character.Gold = -5
	err := s.Validate()
	if err == nil {
		t.Fatal("expected an error for negative quantities, got nil")
	}
	if !strings.Contains(err.Error(), "materials[metal] must not be negative") {
		t.Errorf("error = %v, want it to mention negative metal", err)
	}
	if !strings.Contains(err.Error(), "gold must not be negative") {
		t.Errorf("error = %v, want it to mention negative gold", err)
	}
}

func TestValidate_RejectsNegativeAvailableCopies(t *testing.T) {
	s := New(testCatalog())
	st := s.Items[1]
	st.AvailableCopies = -1
	s.Items[1] = st
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "availableCopies must not be negative") {
		t.Errorf("Validate() error = %v, want it to mention negative availableCopies", err)
	}
}

func TestValidate_RejectsSchemaVersionMismatch(t *testing.T) {
	s := New(testCatalog())
	s.SchemaVersion = 999
	err := s.Validate()
	if err == nil || !strings.Contains(err.Error(), "schema version") {
		t.Errorf("Validate() error = %v, want it to mention schema version", err)
	}
}

func TestBuildingLevels_Level(t *testing.T) {
	b := BuildingLevels{Craftsman: 3, Jeweler: 2, TradingPost: 4}
	cases := []struct {
		building catalog.BuildingType
		want     int
	}{
		{catalog.BuildingCraftsman, 3},
		{catalog.BuildingJeweler, 2},
		{catalog.BuildingTradingPost, 4},
		{catalog.BuildingType("unknown"), 0},
	}
	for _, tc := range cases {
		if got := b.Level(tc.building); got != tc.want {
			t.Errorf("Level(%s) = %d, want %d", tc.building, got, tc.want)
		}
	}
}
