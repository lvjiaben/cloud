package serviceaction

import (
	"testing"

	"lina-core/pkg/bizerr"
)

func TestPublicValidationUsesStableBusinessCodes(t *testing.T) {
	if err := validateActor(Actor{}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("invalid actor returned an unstable error: %v", err)
	}
	service := &service{}
	if _, err := service.RedeemDownload(t.Context(), "bad ticket", Actor{}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("invalid download request returned an unstable error: %v", err)
	}
	if _, err := New(nil, nil); !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("missing service dependencies returned an unstable error: %v", err)
	}
}
