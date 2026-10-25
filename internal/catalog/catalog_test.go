package catalog

import (
	"strings"
	"testing"
)

func TestLoad_EmbeddedCatalogIsValid(t *testing.T) {
	cat, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cat.Items) == 0 {
		t.Fatal("expected embedded catalog to contain items, got none")
	}
}

func TestLoadFrom_RejectsWrongSchemaVersion(t *testing.T) {
	data := []byte(`{"schemaVersion": 999, "sourceChecksum": "abc", "items": []}`)
	_, err := LoadFrom(data)
	if err == nil {
		t.Fatal("expected an error for mismatched schema version, got nil")
	}
	if !strings.Contains(err.Error(), "schema version mismatch") {
		t.Errorf("error = %q, want it to mention schema version mismatch", err.Error())
	}
}

func TestLoadFrom_RejectsInvalidJSON(t *testing.T) {
	_, err := LoadFrom([]byte(`not json`))
	if err == nil {
		t.Fatal("expected an error for invalid JSON, got nil")
	}
}

func TestValidate_DetectsDuplicateIDs(t *testing.T) {
	cat := Catalog{
		SchemaVersion:  SchemaVersion,
		SourceChecksum: "abc",
		Items: []Item{
			{ID: 1, Name: "A"},
			{ID: 1, Name: "B"},
		},
	}
	err := cat.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicate item id 1") {
		t.Errorf("Validate() error = %v, want it to report duplicate item id 1", err)
	}
}

func TestValidate_DetectsUnknownRequiredItem(t *testing.T) {
	cat := Catalog{
		SchemaVersion:  SchemaVersion,
		SourceChecksum: "abc",
		Items: []Item{
			{ID: 1, Name: "A", Cost: Cost{RequiredItems: []int{999}}},
		},
	}
	err := cat.Validate()
	if err == nil || !strings.Contains(err.Error(), "unknown item id 999") {
		t.Errorf("Validate() error = %v, want it to report unknown item id 999", err)
	}
}

func TestValidate_DetectsBadResourceCombinations(t *testing.T) {
	cases := []struct {
		name string
		item Item
		want string
	}{
		{
			name: "non-material resource type in materials",
			item: Item{ID: 1, Name: "A", Cost: Cost{Materials: map[ResourceType]int{"gold": 1}}},
			want: "non-material resource type",
		},
		{
			name: "negative herb quantity",
			item: Item{ID: 1, Name: "A", Cost: Cost{Herbs: map[ResourceType]int{ResourceArrowvine: -1}}},
			want: "must be positive",
		},
		{
			name: "gold and materials both set",
			item: Item{ID: 1, Name: "A", Cost: Cost{Gold: 5, Materials: map[ResourceType]int{ResourceMetal: 1}}},
			want: "both a gold cost and a materials/herb cost",
		},
		{
			name: "building level without building",
			item: Item{ID: 1, Name: "A", RequiredBuildingLevel: 2},
			want: "requiredBuildingLevel set without a requiredBuilding",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cat := Catalog{SchemaVersion: SchemaVersion, SourceChecksum: "abc", Items: []Item{tc.item}}
			err := cat.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Validate() error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestValidate_AcceptsWellFormedCatalog(t *testing.T) {
	cat := Catalog{
		SchemaVersion:  SchemaVersion,
		SourceChecksum: "abc",
		Items: []Item{
			{ID: 1, Name: "Crude Shield", Cost: Cost{Materials: map[ResourceType]int{ResourceMetal: 1}}},
			{
				ID: 2, Name: "Reinforced Shield",
				Cost:                  Cost{Materials: map[ResourceType]int{ResourceLumber: 1}, RequiredItems: []int{1}},
				RequiredBuilding:      BuildingCraftsman,
				RequiredBuildingLevel: 3,
			},
			{ID: 3, Name: "Amulet of Life", Cost: Cost{Gold: 15}},
		},
	}
	if err := cat.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}
