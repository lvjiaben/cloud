// This file carries reviewed local preparation diagnostics without exposing error causes.

package provisionresourcecap

import (
	"encoding/json"
	"errors"

	"github.com/gogf/gf/v2/errors/gerror"
)

// PreparationFailureCode identifies an owner-reviewed local failure, never supplier free text.
type PreparationFailureCode string

const (
	PreparationFailureRequest     PreparationFailureCode = "resource_preparation_request_invalid"
	PreparationFailureLookup      PreparationFailureCode = "resource_preparation_lookup_failed"
	PreparationFailureParameters  PreparationFailureCode = "resource_preparation_parameters_unavailable"
	PreparationFailureCredential  PreparationFailureCode = "resource_preparation_credential_failed"
	PreparationFailureCallback    PreparationFailureCode = "resource_preparation_callback_unavailable"
	PreparationFailureTaskInput   PreparationFailureCode = "resource_preparation_task_input_invalid"
	PreparationFailureTaskStorage PreparationFailureCode = "resource_preparation_task_storage_failed"
)

type preparationError struct {
	code  PreparationFailureCode
	cause error
}

func (e *preparationError) Error() string { return string(e.code) }
func (e *preparationError) Unwrap() error { return e.cause }

// WrapPreparationFailure retains an internal cause while exposing only a reviewed category.
// It does not prove global non-acceptance or authorize recovery of an unknown task.
func WrapPreparationFailure(code PreparationFailureCode, cause error) error {
	if cause == nil {
		return nil
	}
	var existing *preparationError
	if errors.As(cause, &existing) {
		return cause
	}
	if _, known := preparationFailureProperties(code); !known {
		return gerror.Wrap(cause, "resource provisioning preparation failed")
	}
	return gerror.Wrap(&preparationError{code: code, cause: cause}, "resource provisioning preparation failed")
}

// PreparationDiagnostic returns bounded constant-only evidence. Persistence and lookup
// failures omit send status because a concurrent or committed task may already exist.
func PreparationDiagnostic(err error) (string, string) {
	var failure *preparationError
	if !errors.As(err, &failure) || failure == nil {
		return "", ""
	}
	notStarted, known := preparationFailureProperties(failure.code)
	if !known {
		return "", ""
	}
	var started *bool
	if notStarted {
		value := false
		started = &value
	}
	body, marshalErr := json.Marshal(struct {
		Stage                  string                 `json:"stage"`
		ErrorCode              PreparationFailureCode `json:"errorCode"`
		BusinessRequestStarted *bool                  `json:"businessRequestStarted,omitempty"`
	}{Stage: "owner_setup", ErrorCode: failure.code, BusinessRequestStarted: started})
	if marshalErr != nil {
		return "", ""
	}
	return string(failure.code), string(body)
}

func preparationFailureProperties(code PreparationFailureCode) (notStarted, known bool) {
	switch code {
	case PreparationFailureRequest, PreparationFailureParameters, PreparationFailureCredential, PreparationFailureCallback, PreparationFailureTaskInput:
		return true, true
	case PreparationFailureLookup, PreparationFailureTaskStorage:
		return false, true
	default:
		return false, false
	}
}
