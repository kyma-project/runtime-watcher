package watchermetrics

import (
	"strings"
)

func parseGodebugFipsMode(s string) string {
	result := "off"

	if len(s) == 0 {
		return result
	}
	for keyValuePair := range strings.SplitSeq(s, ",") {
		keyValuePair = strings.TrimSpace(keyValuePair)
		if !strings.HasPrefix(keyValuePair, "fips140=") {
			continue
		}
		result = strings.TrimPrefix(keyValuePair, "fips140=")
	}
	switch result {
	case "off", "on", "debug", "only":
		return result
	default:
		// if the value is not one of the expected values, we return "off"
		return "off"
	}
}
