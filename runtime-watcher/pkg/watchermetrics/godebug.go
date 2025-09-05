package watchermetrics

import (
	"strings"
)

// parseGodebugFipsMode parses the provided value to determine the FIPS mode.
// The argument to this function should be a value of the GODEBUG environment variable.
// We need to parse it manually because the provided fips140.Enabled() function doesn't distinguish
// between "on", "debug" and "only" modes.
func parseGodebugFipsMode(goDebugEnvVar string) string {
	const off = "off"

	result := off

	if len(goDebugEnvVar) == 0 {
		return result
	}
	for keyValuePair := range strings.SplitSeq(goDebugEnvVar, ",") {
		keyValuePair = strings.TrimSpace(keyValuePair)
		if justValue := strings.TrimPrefix(keyValuePair, "fips140="); len(justValue) < len(keyValuePair) {
			result = justValue
			// We're not leaving the loop here to align with the Go standard library parsing logic,
			// where "the last provided value wins"
		}
	}
	switch result {
	case off, "on", "debug", "only":
		return result
	default:
		// If the value is not one of the expected values, return "off" to have exhaustive matching.
		// Go standard library panics in this case.
		return off
	}
}
