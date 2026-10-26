package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"fh-items/internal/catalog"
	"fh-items/internal/state"
)

func testServer() *Server {
	cat := catalog.Catalog{Items: []catalog.Item{
		{ID: 10, Name: "Crude Shield", Count: 2, Cost: catalog.Cost{Materials: map[catalog.ResourceType]int{catalog.ResourceMetal: 1}}},
		{ID: 15, Name: "Reinforced Shield", Count: 2, Cost: catalog.Cost{
			Materials:     map[catalog.ResourceType]int{catalog.ResourceLumber: 1},
			RequiredItems: []catalog.ItemRequirement{{ItemID: 10, Quantity: 1}},
		}},
		{ID: 20, Name: "Spyglass", Count: 2, Cost: catalog.Cost{Materials: map[catalog.ResourceType]int{catalog.ResourceMetal: 1}},
			RequiredBuilding: catalog.BuildingCraftsman, RequiredBuildingLevel: 2},
		{ID: 30, Name: "Random Blueprint Item", Count: 2, Cost: catalog.Cost{Materials: map[catalog.ResourceType]int{catalog.ResourceHide: 1}}},
	}}
	st := state.New(cat)
	st.Items[10] = state.ItemState{ItemID: 10, Unlocked: true, AvailableCopies: 1}
	st.Items[15] = state.ItemState{ItemID: 15, Unlocked: true, AvailableCopies: 1}
	st.Items[20] = state.ItemState{ItemID: 20, Unlocked: false, AvailableCopies: 2} // Craftsman 2, starts at level 0
	st.Items[30] = state.ItemState{ItemID: 30, Unlocked: false, AvailableCopies: 2} // not yet discovered
	st.Character.OwnedItems = []int{10}
	st.Character.Materials[catalog.ResourceLumber] = 1
	st.Character.Materials[catalog.ResourceMetal] = 5
	st.Character.Materials[catalog.ResourceHide] = 1
	return NewServer(cat, st)
}

func newMux(s *Server) http.Handler {
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	return mux
}

func TestHandleGetState(t *testing.T) {
	s := testServer()
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	rec := httptest.NewRecorder()
	newMux(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}

	var got StateView
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(got.Items) != 4 {
		t.Fatalf("len(Items) = %d, want 4", len(got.Items))
	}
	if got.Character.OwnedItems[0] != 10 {
		t.Errorf("Character.OwnedItems = %v, want [10]", got.Character.OwnedItems)
	}

	var reinforced ItemView
	for _, item := range got.Items {
		if item.ID == 15 {
			reinforced = item
		}
	}
	if len(reinforced.RequiredItems) != 1 || reinforced.RequiredItems[0].Name != "Crude Shield" {
		t.Errorf("Reinforced Shield RequiredItems = %+v, want a resolved name for item 10", reinforced.RequiredItems)
	}
}

func TestHandlePostCheck_FeasibleWithPersistedState(t *testing.T) {
	s := testServer()
	body, _ := json.Marshal(CheckRequest{Items: map[int]int{15: 1}})
	req := httptest.NewRequest(http.MethodPost, "/api/check", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	newMux(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", rec.Code, rec.Body.String())
	}
	var got CheckResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if !got.Feasible {
		t.Fatalf("expected feasible, got shortfalls: %v", got.Shortfalls)
	}
	if got.Remaining.Materials[catalog.ResourceLumber] != 0 {
		t.Errorf("remaining lumber = %d, want 0", got.Remaining.Materials[catalog.ResourceLumber])
	}
	if len(got.ConsumedPrerequisites) != 1 || got.ConsumedPrerequisites[0].ItemID != 10 {
		t.Errorf("ConsumedPrerequisites = %+v, want item 10 consumed", got.ConsumedPrerequisites)
	}
}

func TestHandlePostCheck_InfeasibleReportsShortfall(t *testing.T) {
	s := testServer()
	// Override: explicitly zero lumber (a non-empty map so JSON
	// marshaling doesn't omit it), so the top-level item's own cost fails.
	body, _ := json.Marshal(CheckRequest{
		Items:     map[int]int{15: 1},
		Character: &CharacterOverride{Materials: map[catalog.ResourceType]int{catalog.ResourceLumber: 0}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/check", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	newMux(s).ServeHTTP(rec, req)

	var got CheckResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if got.Feasible {
		t.Fatal("expected infeasible with no lumber override, got feasible")
	}
	if len(got.Shortfalls) == 0 {
		t.Error("expected shortfall details")
	}
	if got.Shortfalls[0].Kind != "resource" {
		t.Errorf("Shortfalls[0].Kind = %q, want %q", got.Shortfalls[0].Kind, "resource")
	}
}

func TestHandlePostCheck_BuildingLevelOverrideUnlocksItem(t *testing.T) {
	s := testServer()
	// Item 20 requires Craftsman 2; the server's persisted level is 0.
	craftsman := 2
	body, _ := json.Marshal(CheckRequest{
		Items:     map[int]int{20: 1},
		Buildings: &BuildingsOverride{Craftsman: &craftsman},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/check", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	newMux(s).ServeHTTP(rec, req)

	var got CheckResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if !got.Feasible {
		t.Fatalf("expected feasible once Craftsman is overridden to level 2, got shortfalls: %v", got.Shortfalls)
	}

	// The override must not leak into the server's persisted state.
	stateReq := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	stateRec := httptest.NewRecorder()
	newMux(s).ServeHTTP(stateRec, stateReq)
	var stateGot StateView
	if err := json.Unmarshal(stateRec.Body.Bytes(), &stateGot); err != nil {
		t.Fatalf("decoding state response: %v", err)
	}
	if stateGot.Buildings.Craftsman != 0 {
		t.Errorf("persisted Buildings.Craftsman = %d, want 0 (override must not persist)", stateGot.Buildings.Craftsman)
	}
	for _, item := range stateGot.Items {
		if item.ID == 20 && item.Unlocked {
			t.Error("item 20 should still be locked in persisted state after a check-only override")
		}
	}
}

func TestHandlePostCheck_UnlockedItemIDsOverride(t *testing.T) {
	s := testServer()
	// Item 30 has no building requirement and starts undiscovered.
	body, _ := json.Marshal(CheckRequest{
		Items:           map[int]int{30: 1},
		UnlockedItemIDs: []int{30},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/check", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	newMux(s).ServeHTTP(rec, req)

	var got CheckResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if !got.Feasible {
		t.Fatalf("expected feasible once item 30 is marked discovered, got shortfalls: %v", got.Shortfalls)
	}
}

func TestHandlePostCheck_UnlockedItemIDsOverride_OmittedStaysLocked(t *testing.T) {
	s := testServer()
	// Sending the override without item 30 in the list must lock it,
	// even though nothing was selected - proving it's not a no-op.
	body, _ := json.Marshal(CheckRequest{
		Items:           map[int]int{30: 1},
		UnlockedItemIDs: []int{999}, // some other, unrelated id
	})
	req := httptest.NewRequest(http.MethodPost, "/api/check", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	newMux(s).ServeHTTP(rec, req)

	var got CheckResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if got.Feasible {
		t.Fatal("expected infeasible: item 30 wasn't in the unlocked list")
	}
}

func TestHandlePostCheck_RejectsInvalidJSON(t *testing.T) {
	s := testServer()
	req := httptest.NewRequest(http.MethodPost, "/api/check", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()
	newMux(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok || errObj["code"] != "INVALID_REQUEST" {
		t.Errorf("error response = %v, want code INVALID_REQUEST", body)
	}
}
