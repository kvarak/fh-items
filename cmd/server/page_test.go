package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"fh-items/internal/catalog"
	"fh-items/internal/state"
)

func testCatalog() catalog.Catalog {
	return catalog.Catalog{Items: []catalog.Item{
		{ID: 20, Name: "Spyglass", Count: 2, RequiredBuilding: catalog.BuildingCraftsman, RequiredBuildingLevel: 2},
		{ID: 30, Name: "Random Blueprint Item", Count: 2},
	}}
}

func requestWithUnlocksCookie(t *testing.T, value string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if value != "" {
		req.AddCookie(&http.Cookie{Name: unlocksCookieName, Value: url.PathEscape(value)})
	}
	return req
}

func TestStateFromRequest_NoCookieReturnsDefault(t *testing.T) {
	cat := testCatalog()
	st := state.New(cat)
	req := requestWithUnlocksCookie(t, "")

	got := stateFromRequest(req, cat, st)

	if got.Buildings.Craftsman != 0 {
		t.Errorf("Buildings.Craftsman = %d, want 0", got.Buildings.Craftsman)
	}
	if got.Items[20].Unlocked {
		t.Error("item 20 should still be locked with no cookie")
	}
}

func TestStateFromRequest_AppliesBuildingsAndUnlockedItems(t *testing.T) {
	cat := testCatalog()
	st := state.New(cat)
	req := requestWithUnlocksCookie(t, `{"buildings":{"craftsman":2,"jeweler":0,"tradingPost":0},"unlockedItemIds":[30]}`)

	got := stateFromRequest(req, cat, st)

	if got.Buildings.Craftsman != 2 {
		t.Errorf("Buildings.Craftsman = %d, want 2", got.Buildings.Craftsman)
	}
	if !got.Items[20].Unlocked {
		t.Error("item 20 (Craftsman 2) should be unlocked")
	}
	if !got.Items[30].Unlocked {
		t.Error("item 30 should be unlocked")
	}
}

func TestStateFromRequest_MalformedCookieFallsBackToDefault(t *testing.T) {
	cat := testCatalog()
	st := state.New(cat)
	req := requestWithUnlocksCookie(t, "not json")

	got := stateFromRequest(req, cat, st)

	if got.Buildings.Craftsman != 0 {
		t.Errorf("Buildings.Craftsman = %d, want 0 (malformed cookie should be ignored)", got.Buildings.Craftsman)
	}
}

func TestStateFromRequest_DoesNotMutateInputState(t *testing.T) {
	cat := testCatalog()
	st := state.New(cat)
	req := requestWithUnlocksCookie(t, `{"buildings":{"craftsman":2,"jeweler":0,"tradingPost":0},"unlockedItemIds":[30]}`)

	stateFromRequest(req, cat, st)

	if st.Buildings.Craftsman != 0 {
		t.Errorf("input state was mutated: Buildings.Craftsman = %d, want 0", st.Buildings.Craftsman)
	}
	if st.Items[30].Unlocked {
		t.Error("input state's item map was mutated in place")
	}
}
