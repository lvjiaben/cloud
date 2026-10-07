// This file implements the bounded, route-declared payload canonicalization
// used by the local process-authentication frame.

package processauth

import (
	"unicode/utf8"

	"github.com/gowebpki/jcs"
)

// canonicalizePayload applies the exact payload rule selected by the server route.
func canonicalizePayload(mode PayloadMode, body []byte) ([]byte, error) {
	switch mode {
	case PayloadModeEmpty:
		if len(body) != 0 {
			return nil, ErrInvalid
		}
		return []byte{}, nil
	case PayloadModeJSON:
		if len(body) == 0 || !utf8.Valid(body) {
			return nil, ErrInvalid
		}
		canonical, err := jcs.Transform(body)
		if err != nil || len(canonical) == 0 || len(canonical) > MaximumPayloadBytes {
			clearBytes(canonical)
			return nil, ErrInvalid
		}
		return canonical, nil
	case PayloadModeRaw:
		return bytesClone(body), nil
	default:
		return nil, ErrInvalid
	}
}

// bytesClone returns an independent copy that can be cleared by the caller.
func bytesClone(value []byte) []byte {
	result := make([]byte, len(value))
	copy(result, value)
	return result
}
