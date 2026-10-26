package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/url"
)

// shareCookieMaxAge matches the one-year lifetime app.js gives the cookie.
const shareCookieMaxAge = 60 * 60 * 24 * 365

// storeShareInCookie saves a share link's unlocks to the fhUnlocks cookie and
// reports whether the request carried a share link (the caller should redirect).
func storeShareInCookie(w http.ResponseWriter, r *http.Request) bool {
	if !r.URL.Query().Has(shareParamName) {
		return false
	}
	if u, ok := decodeShare(r.URL.Query().Get(shareParamName)); ok {
		value, _ := json.Marshal(u)
		http.SetCookie(w, &http.Cookie{
			Name:     unlocksCookieName,
			Value:    url.PathEscape(string(value)),
			Path:     "/",
			MaxAge:   shareCookieMaxAge,
			SameSite: http.SameSiteLaxMode,
		})
	}
	return true
}

// maxShareBitsetBytes caps each ID bitset so a crafted link cannot allocate much.
const maxShareBitsetBytes = 512

// decodeShare parses a share value built by the share button in app.js:
// base64url of three level bytes (craftsman, tradingPost, jeweler), then two
// ID bitsets (discovered, Gloomhaven). Each bitset is a uvarint byte length
// followed by the bytes; bit i of byte b means ID b*8+i.
func decodeShare(value string) (unlocks, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) < 3 {
		return unlocks{}, false
	}
	u := unlocks{Buildings: buildingLevels{
		Craftsman:   int(raw[0]),
		TradingPost: int(raw[1]),
		Jeweler:     int(raw[2]),
	}}
	rest := raw[3:]
	var ok bool
	if u.DiscoveredItemIDs, rest, ok = readIDBitset(rest); !ok {
		return unlocks{}, false
	}
	if u.GloomhavenIDs, rest, ok = readIDBitset(rest); !ok || len(rest) != 0 {
		return unlocks{}, false
	}
	return u, true
}

func readIDBitset(b []byte) ([]int, []byte, bool) {
	n, k := binary.Uvarint(b)
	if k <= 0 || n > maxShareBitsetBytes || uint64(len(b)-k) < n {
		return nil, nil, false
	}
	set := b[k : k+int(n)]
	var ids []int
	for i, by := range set {
		for bit := 0; bit < 8; bit++ {
			if by&(1<<bit) != 0 {
				ids = append(ids, i*8+bit)
			}
		}
	}
	return ids, b[k+int(n):], true
}
