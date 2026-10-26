package main

import (
	"fmt"
	"net/url"
	"strings"
)

// goatCounterScript validates a GoatCounter endpoint and returns the count.js URL served beside it.
func goatCounterScript(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || !strings.HasSuffix(u.Path, "/count") || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("goatcounter endpoint %q must look like https://host/count", endpoint)
	}
	return endpoint + ".js", nil
}
