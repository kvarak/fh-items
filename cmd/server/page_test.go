package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"fh-items/internal/catalog"
)

func testCatalog() catalog.Catalog {
	return catalog.Catalog{Items: []catalog.Item{
		{ID: 1, Name: "Spyglass", RequiredBuilding: catalog.BuildingCraftsman, RequiredBuildingLevel: 2},
		{ID: 2, Name: "Amulet of Life", RequiredBuilding: catalog.BuildingTradingPost, RequiredBuildingLevel: 1},
		{ID: 3, Name: "Unhealthy Mixture"},
		{ID: 4, Name: "Cured Leather Armor", Cost: catalog.Cost{RequiredItems: []catalog.ItemRequirement{{ItemID: 3, Quantity: 1}}}},
	}}
}

func requestWithUnlocksCookie(value string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if value != "" {
		req.AddCookie(&http.Cookie{Name: unlocksCookieName, Value: url.PathEscape(value)})
	}
	return req
}

func TestUnlocksFromRequest(t *testing.T) {
	cases := []struct {
		name      string
		cookie    string
		craftsman int
		ids       int
	}{
		{"no cookie", "", 0, 0},
		{"valid cookie", `{"buildings":{"craftsman":2,"jeweler":0,"tradingPost":0},"unlockedItemIds":[3,4]}`, 2, 2},
		{"malformed cookie", "not json", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := unlocksFromRequest(requestWithUnlocksCookie(tc.cookie))
			if u.Buildings.Craftsman != tc.craftsman || len(u.DiscoveredItemIDs) != tc.ids {
				t.Errorf("got craftsman %d, %d ids; want %d, %d", u.Buildings.Craftsman, len(u.DiscoveredItemIDs), tc.craftsman, tc.ids)
			}
		})
	}
}

func TestUnlockedSet(t *testing.T) {
	cat := testCatalog()

	fresh := unlocks{}.unlockedSet(cat)
	if fresh[1] || fresh[2] || fresh[3] {
		t.Errorf("fresh campaign: got %v, want nothing unlocked", fresh)
	}

	u := unlocks{Buildings: buildingLevels{Craftsman: 2}, DiscoveredItemIDs: []int{1, 3}}
	got := u.unlockedSet(cat)
	if !got[1] || !got[3] || got[4] {
		t.Errorf("got %v, want items 1 (by building) and 3 (discovered) unlocked, 4 locked", got)
	}

	u = unlocks{DiscoveredItemIDs: []int{1}}
	if u.unlockedSet(cat)[1] {
		t.Error("building-gated item 1 must not unlock by being marked discovered")
	}
}

func TestIsCraftable_RequiresUnlockedPrerequisites(t *testing.T) {
	cat := testCatalog()
	byID := map[int]catalog.Item{3: cat.Items[2], 4: cat.Items[3]}

	if isCraftable(4, byID, map[int]bool{4: true}, map[int]bool{}) {
		t.Error("item 4 should not be craftable while its prerequisite is locked")
	}
	if !isCraftable(4, byID, map[int]bool{3: true, 4: true}, map[int]bool{}) {
		t.Error("item 4 should be craftable once its prerequisite is unlocked")
	}
}

func TestBuildPageData_GloomhavenItemsNeedLevelAndCheck(t *testing.T) {
	gh := []catalog.GloomhavenItem{
		{ID: 10, Name: "War Hammer", Slot: "2h", Gold: 30, TradingPostLevel: 1},
		{ID: 21, Name: "Stun Powder", Slot: "small", Gold: 20, TradingPostLevel: 2},
		{ID: 25, Name: "Jagged Sword", Slot: "1h", Gold: 30, TradingPostLevel: 1},
	}
	u := unlocks{Buildings: buildingLevels{TradingPost: 1}, GloomhavenIDs: []int{10, 21}}
	data := buildPageData(catalog.Catalog{}, gh, u)

	if len(data.Items) != 1 || data.Items[0].Key != "gh10" {
		t.Fatalf("got items %+v, want only gh10 (checked and level reached)", data.Items)
	}
	if len(data.GloomhavenSets) != 2 || !data.GloomhavenSets[0].Visible || data.GloomhavenSets[1].Visible {
		t.Errorf("got sets %+v, want level 1 visible and level 2 hidden", data.GloomhavenSets)
	}
	if !strings.Contains(string(data.ItemCostJSON), `"gh10":{"gold":30}`) {
		t.Errorf("cost JSON %s missing gh10 gold cost", data.ItemCostJSON)
	}
}

func TestCostSummary_NamesOnlyUnlockedPrerequisites(t *testing.T) {
	byID := map[int]catalog.Item{1: {ID: 1, Name: "Unhealthy Mixture"}}
	cost := catalog.Cost{
		Materials:     map[catalog.ResourceType]int{catalog.ResourceHide: 1},
		RequiredItems: []catalog.ItemRequirement{{ItemID: 1, Quantity: 2}},
	}

	if got, want := costSummary(cost, byID, map[int]bool{}), "1 hide + 2x item 1"; got != want {
		t.Errorf("locked prerequisite: got %q, want %q", got, want)
	}
	if got, want := costSummary(cost, byID, map[int]bool{1: true}), "1 hide + 2x Unhealthy Mixture"; got != want {
		t.Errorf("unlocked prerequisite: got %q, want %q", got, want)
	}
	if got := costSummary(catalog.Cost{}, byID, nil); got != "-" {
		t.Errorf("empty cost: got %q, want %q", got, "-")
	}
}
