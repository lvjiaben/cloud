// This file verifies channel-secret errors never expose persistence or credential material.
package channelsecret

import (
	"errors"
	"lina-core/pkg/bizerr"
	"strings"
	"testing"
)

func TestPublicErrorDropsSensitiveCauses(t *testing.T) {
	const canary = "sql-secret-reference-cipher-material"
	raw := errors.New(canary)
	for _, code := range []*bizerr.Code{CodeInvalid, CodeNotAccessible, CodeRecentAuthenticationRequired, CodeConflict, CodeUnavailable} {
		result := publicError(bizerr.WrapCode(raw, code))
		if !bizerr.Is(result, code) || strings.Contains(result.Error(), canary) || errors.Is(result, raw) {
			t.Fatalf("unsafe business error: %v", result)
		}
	}
	result := publicError(raw)
	if !bizerr.Is(result, CodeUnavailable) || strings.Contains(result.Error(), canary) || errors.Is(result, raw) {
		t.Fatalf("unsafe persistence error: %v", result)
	}
	if publicError(nil) != nil {
		t.Fatal("nil error became failure")
	}
}
