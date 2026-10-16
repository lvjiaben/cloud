// This file normalizes explicit channel certificate trust and clears stale trust
// when an administrator changes the supplier endpoint.
package providerconfiguration

import (
	"encoding/hex"
	"github.com/gogf/gf/v2/errors/gerror"
	"strings"
)

func resolveCertificateFingerprint(input *string, previous string, endpointChanged bool) (string, error) {
	if input == nil {
		if endpointChanged {
			return "", nil
		}
		return previous, nil
	}
	value := strings.ToLower(strings.TrimSpace(*input))
	if value == "" {
		return "", nil
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return "", gerror.New("invalid certificate fingerprint")
	}
	return value, nil
}
