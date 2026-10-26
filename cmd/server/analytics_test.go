package main

import "testing"

func TestGoatCounterScript(t *testing.T) {
	got, err := goatCounterScript("https://stats.example.com/count")
	if err != nil || got != "https://stats.example.com/count.js" {
		t.Errorf("got %q, %v; want the count.js URL", got, err)
	}
	for _, bad := range []string{"http://stats.example.com/count", "https://stats.example.com/", "https:///count", "https://stats.example.com/count?x=1", "not a url"} {
		if _, err := goatCounterScript(bad); err == nil {
			t.Errorf("goatCounterScript(%q) accepted an invalid endpoint", bad)
		}
	}
}
