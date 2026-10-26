package catalog

import (
	"testing"
)

func TestGenerateFromSource(t *testing.T) {
	source := `[
		{
			"id": 1, "name": "Spyglass", "slot": "head", "count": 2,
			"resources": {"metal": 1},
			"source": "Craftsman 1", "desc": "gain advantage",
			"unlockCrafstmanLevel": 1
		},
		{
			"id": 2, "name": "Reinforced Shield", "slot": "1h", "count": 2,
			"resources": {"item": [1], "lumber": 1},
			"source": "Craftsman 2", "desc": "block more",
			"unlockCrafstmanLevel": 2
		},
		{
			"id": 3, "name": "Deadly Mixture", "slot": "small", "count": 2,
			"resources": {"any": [2, 1]},
			"source": "Alchemy", "desc": "wound poison"
		},
		{
			"id": 4, "name": "Amulet of Life", "slot": "head", "count": 2,
			"cost": 15, "source": "Initially Available", "desc": "heal",
			"unlockTradingPostLevel": -1
		},
		{
			"id": 5, "name": "Moon Earring", "slot": "small", "count": 2,
			"cost": 20, "source": "Jeweler 1", "desc": "recover items",
			"unlockJewelerLevel": 1
		}
	]`

	cat, err := GenerateFromSource([]byte(source))
	if err != nil {
		t.Fatalf("GenerateFromSource() error = %v", err)
	}
	if cat.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", cat.SchemaVersion, SchemaVersion)
	}
	if cat.SourceChecksum == "" {
		t.Error("SourceChecksum is empty, want a checksum of the source bytes")
	}
	if len(cat.Items) != 5 {
		t.Fatalf("len(Items) = %d, want 5", len(cat.Items))
	}

	byID := make(map[int]Item, len(cat.Items))
	for _, item := range cat.Items {
		byID[item.ID] = item
	}

	spyglass := byID[1]
	if spyglass.Cost.Materials[ResourceMetal] != 1 {
		t.Errorf("Spyglass metal cost = %d, want 1", spyglass.Cost.Materials[ResourceMetal])
	}
	if spyglass.RequiredBuilding != BuildingCraftsman || spyglass.RequiredBuildingLevel != 1 {
		t.Errorf("Spyglass building gate = %s/%d, want craftsman/1", spyglass.RequiredBuilding, spyglass.RequiredBuildingLevel)
	}

	shield := byID[2]
	if len(shield.Cost.RequiredItems) != 1 || shield.Cost.RequiredItems[0].ItemID != 1 || shield.Cost.RequiredItems[0].Quantity != 1 {
		t.Errorf("Reinforced Shield RequiredItems = %v, want [{1 1}]", shield.Cost.RequiredItems)
	}
	if shield.Cost.Materials[ResourceLumber] != 1 {
		t.Errorf("Reinforced Shield lumber cost = %d, want 1", shield.Cost.Materials[ResourceLumber])
	}

	mixture := byID[3]
	if len(mixture.Cost.AnyHerb) != 2 || mixture.Cost.AnyHerb[0] != 1 || mixture.Cost.AnyHerb[1] != 2 {
		t.Errorf("Deadly Mixture AnyHerb = %v, want [1 2] (sorted)", mixture.Cost.AnyHerb)
	}

	amulet := byID[4]
	if amulet.Cost.Gold != 15 {
		t.Errorf("Amulet of Life gold cost = %d, want 15", amulet.Cost.Gold)
	}
	if amulet.RequiredBuilding != BuildingTradingPost || amulet.RequiredBuildingLevel != 1 {
		t.Errorf("Amulet of Life building gate = %s/%d, want tradingPost/1 (unlockTradingPostLevel -1 is the level 1 stock)",
			amulet.RequiredBuilding, amulet.RequiredBuildingLevel)
	}

	earring := byID[5]
	if earring.RequiredBuilding != BuildingJeweler || earring.RequiredBuildingLevel != 1 {
		t.Errorf("Moon Earring building gate = %s/%d, want jeweler/1", earring.RequiredBuilding, earring.RequiredBuildingLevel)
	}

	if err := cat.Validate(); err != nil {
		t.Errorf("generated catalog failed validation: %v", err)
	}
}

func TestGenerateFromSource_DeduplicatesRepeatedIDs(t *testing.T) {
	source := `[
		{"id": 245, "name": "Ancient Coin", "count": 1, "cost": 0, "source": "Loot", "desc": ""},
		{"id": 245, "name": "Ancient Coin", "count": 1, "cost": 0, "source": "Loot", "desc": ""}
	]`
	cat, err := GenerateFromSource([]byte(source))
	if err != nil {
		t.Fatalf("GenerateFromSource() error = %v", err)
	}
	if len(cat.Items) != 1 {
		t.Fatalf("len(Items) = %d, want 1 (duplicate ids collapsed)", len(cat.Items))
	}
}

func TestGenerateFromSource_RejectsUnknownResourceKey(t *testing.T) {
	source := `[{"id": 1, "name": "Mystery", "count": 1, "resources": {"unobtainium": 1}}]`
	_, err := GenerateFromSource([]byte(source))
	if err == nil {
		t.Fatal("expected an error for an unknown resource key, got nil")
	}
}
