// This file distinguishes a completed guest invocation from a failed or
// indeterminate business operation in append-only API history. Only fixed
// outcome classes are retained; supplier bodies and free-form errors are not.
package providerexecution

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type callObservationKey struct{}
type callOutcomeCode string

const (
	callOutcomeCompleted        callOutcomeCode = ""
	callOutcomeRuntimeFailed    callOutcomeCode = "provider_call_failed"
	callOutcomeOperationFailed  callOutcomeCode = "provider_operation_failed"
	callOutcomeOperationUnknown callOutcomeCode = "provider_operation_unknown"
	callOutcomeInvalid          callOutcomeCode = "provider_invalid_result"
)

type callObservation struct {
	mu              sync.Mutex
	code            callOutcomeCode
	callbackEntered atomic.Bool
}

func (o *callObservation) resultCode() callOutcomeCode {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.code
}

func observeOperationResult(ctx context.Context, method string, payload json.RawMessage) {
	observation, ok := ctx.Value(callObservationKey{}).(*callObservation)
	if !ok || method != resourcecap.DynamicProviderMethodExecute {
		return
	}
	var operation resourcecap.OperationResult
	code := callOutcomeCompleted
	if json.Unmarshal(payload, &operation) != nil || operation.Validate() != nil {
		code = callOutcomeInvalid
	} else {
		switch operation.Status {
		case resourcecap.TaskStatusFailed:
			code = callOutcomeOperationFailed
		case resourcecap.TaskStatusUnknown:
			code = callOutcomeOperationUnknown
		}
	}
	observation.mu.Lock()
	observation.code = code
	observation.mu.Unlock()
}
