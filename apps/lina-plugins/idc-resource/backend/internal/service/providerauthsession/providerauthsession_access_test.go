// Authentication failures retain reviewed diagnostics while discarding secrets.
package providerauthsession

import (
	"errors"
	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"strings"
	"testing"
)

func TestAuthenticationRefreshFailurePreservesReviewedCodeWithoutCause(t *testing.T) {
	for _, code := range []*bizerr.Code{resourcecap.CodeProviderAuthRejected, resourcecap.CodeProviderTimeout, resourcecap.CodeProviderLocalAuthPolicy, resourcecap.CodeProviderInvalidResponse} {
		cause := bizerr.WrapCode(errors.New("private-credential-and-endpoint"), code)
		got := authenticationRefreshFailure(cause)
		if !bizerr.Is(got, code) || strings.Contains(got.Error(), "private-") || errors.Is(got, cause) {
			t.Fatal("reviewed failure lost its category or exposed its producer cause")
		}
	}
}

func TestAuthenticationRefreshFailureMasksUnknownProducerErrors(t *testing.T) {
	arbitrary := bizerr.MustDefine("UNREVIEWED_AUTH_FAILURE", "private-credential", gcode.CodeOperationFailed)
	for _, cause := range []error{nil, errors.New("private-credential-and-endpoint"), bizerr.NewCode(arbitrary)} {
		got := authenticationRefreshFailure(cause)
		if got == nil || got.Error() != "resource Provider authentication refresh failed" || errors.Unwrap(got) != nil {
			t.Fatal("unknown producer error escaped the authentication owner")
		}
	}
}
