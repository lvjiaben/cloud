// This file verifies preparation diagnostics preserve categories without disclosing causes.

package provisionresourcecap

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/errors/gerror"
)

func TestPreparationDiagnosticKeepsOnlyReviewedFields(t *testing.T) {
	cause := gerror.New("password=MUST_REDACT token=MUST_REDACT SQL secret_ref=MUST_REDACT")
	for _, code := range []PreparationFailureCode{PreparationFailureRequest, PreparationFailureLookup, PreparationFailureParameters, PreparationFailureCredential, PreparationFailureCallback, PreparationFailureTaskInput, PreparationFailureTaskStorage} {
		wrapped := WrapPreparationFailure(code, cause)
		if !errors.Is(wrapped, cause) {
			t.Fatal("internal cause lost")
		}
		actual, body := PreparationDiagnostic(wrapped)
		if actual != string(code) || strings.Contains(body, "MUST_REDACT") || len(body) > 4096 {
			t.Fatal("diagnostic exposed a cause or lost its reviewed category")
		}
		var value map[string]any
		if json.Unmarshal([]byte(body), &value) != nil || value["stage"] != "owner_setup" || value["errorCode"] != string(code) {
			t.Fatal("invalid preparation evidence")
		}
		started, exists := value["businessRequestStarted"]
		if code == PreparationFailureLookup || code == PreparationFailureTaskStorage {
			if exists {
				t.Fatal("ambiguous local persistence incorrectly asserted send status")
			}
		} else if !exists || started != false {
			t.Fatal("local pre-task validation lost its send-stage information")
		}
	}
}

func TestPreparationDiagnosticRejectsUnreviewedAndPreservesSpecificCause(t *testing.T) {
	cause := gerror.New("private test-only cause")
	for _, err := range []error{nil, cause, WrapPreparationFailure(PreparationFailureCode("password=forbidden"), cause)} {
		code, body := PreparationDiagnostic(err)
		if code != "" || body != "" {
			t.Fatal("unreviewed diagnostic was exposed")
		}
	}
	if WrapPreparationFailure(PreparationFailureTaskStorage, nil) != nil {
		t.Fatal("nil cause became an error")
	}
	specific := WrapPreparationFailure(PreparationFailureTaskInput, cause)
	code, _ := PreparationDiagnostic(WrapPreparationFailure(PreparationFailureTaskStorage, specific))
	if code != string(PreparationFailureTaskInput) {
		t.Fatal("specific reviewed failure was overwritten")
	}
}
