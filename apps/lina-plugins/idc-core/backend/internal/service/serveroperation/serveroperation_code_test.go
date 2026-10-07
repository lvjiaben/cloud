package serveroperation

import (
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

func TestPublicErrorMapsServerOperationBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		code *bizerr.Code
	}{
		{name: "invalid", err: ErrInvalidInput, code: CodeInvalid},
		{name: "unavailable", err: ErrServiceUnavailable, code: CodeUnavailable},
		{name: "action unavailable", err: ErrActionUnavailable, code: CodeActionUnavailable},
		{name: "conflict", err: ErrOperationConflict, code: CodeConflict},
		{name: "output unavailable", err: serveroperationresourcecap.ErrOutputUnavailable, code: CodeOutputUnavailable},
		{name: "output consumed", err: serveroperationresourcecap.ErrOutputConsumed, code: CodeOutputConsumed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := PublicError(test.err); !bizerr.Is(err, test.code) {
				t.Fatalf("PublicError(%v) = %v", test.err, err)
			}
		})
	}
}
