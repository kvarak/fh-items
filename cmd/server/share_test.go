package main

import (
	"encoding/base64"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// encodeShare mirrors encodeShare in app.js.
func encodeShare(u unlocks) string {
	raw := []byte{byte(u.Buildings.Craftsman), byte(u.Buildings.TradingPost), byte(u.Buildings.Jeweler)}
	for _, ids := range [][]int{u.DiscoveredItemIDs, u.GloomhavenIDs} {
		length := 0
		for _, id := range ids {
			if id/8+1 > length {
				length = id/8 + 1
			}
		}
		bits := make([]byte, length)
		for _, id := range ids {
			bits[id/8] |= 1 << (id % 8)
		}
		raw = binary.AppendUvarint(raw, uint64(length))
		raw = append(raw, bits...)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func TestDecodeShare_RoundTrip(t *testing.T) {
	want := unlocks{
		Buildings:         buildingLevels{Craftsman: 3, TradingPost: 2, Jeweler: 1},
		DiscoveredItemIDs: []int{3, 7, 200},
		GloomhavenIDs:     []int{10, 21},
	}
	got, ok := decodeShare(encodeShare(want))
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v (ok=%v), want %+v", got, ok, want)
	}
}

func TestDecodeShare_Empty(t *testing.T) {
	got, ok := decodeShare(encodeShare(unlocks{}))
	if !ok || got.Buildings != (buildingLevels{}) || len(got.DiscoveredItemIDs) != 0 || len(got.GloomhavenIDs) != 0 {
		t.Errorf("got %+v (ok=%v), want empty unlocks", got, ok)
	}
}

func TestDecodeShare_Malformed(t *testing.T) {
	for _, value := range []string{"", "!!!", "AQI", encodeShare(unlocks{}) + "AA"} {
		if _, ok := decodeShare(value); ok {
			t.Errorf("decodeShare(%q) accepted malformed input", value)
		}
	}
}

func TestStoreShareInCookie(t *testing.T) {
	want := unlocks{Buildings: buildingLevels{Craftsman: 3}, DiscoveredItemIDs: []int{7}}
	req := httptest.NewRequest(http.MethodGet, "/?share="+encodeShare(want), nil)
	rec := httptest.NewRecorder()

	if !storeShareInCookie(rec, req) {
		t.Fatal("storeShareInCookie did not detect the share link")
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}

	stored := httptest.NewRequest(http.MethodGet, "/", nil)
	stored.AddCookie(&http.Cookie{Name: cookies[0].Name, Value: cookies[0].Value})
	if got := unlocksFromRequest(stored); !reflect.DeepEqual(got, want) {
		t.Errorf("stored %+v, want %+v", got, want)
	}
}

func TestStoreShareInCookie_NoShareParam(t *testing.T) {
	rec := httptest.NewRecorder()
	if storeShareInCookie(rec, httptest.NewRequest(http.MethodGet, "/", nil)) {
		t.Error("reported a share link without a share parameter")
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Error("set a cookie without a share parameter")
	}
}
