// This file records owner-observed call failures without arbitrary error text,
// input parameters, login bodies or credentials. Absence of a response remains
// distinct from proof that the supplier rejected or did not create a resource.
package composition

import (
	"context"
	"encoding/json"
	"lina-core/pkg/logger"
	"sync"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/providerexecution"
)

// ensureCallDiagnostic supplies a bounded fallback only when the owner did not
// retain an actual response or failure observation for this invocation.
func ensureCallDiagnostic(mu *sync.Mutex, observations *[]resourcecap.Evidence, method string, handedOff *bool, stage callDiagnosticStage, failure error) {
	mu.Lock()
	defer mu.Unlock()
	if method != resourcecap.DynamicProviderMethodExecute || len(*observations) != 0 {
		return
	}
	if !*handedOff && stage == callDiagnosticResult {
		stage = callDiagnosticDispatch
	}
	item := newCallDiagnostic(stage, failure, nil, nil, nil)
	if item.Validate() == nil {
		*observations = append(*observations, item)
	}
}

func isBusinessHTTPInstruction(request *pluginhost.ConfidentialOutboundRequest) bool {
	var instruction resourcecap.ConfidentialHTTPRequest
	return request != nil && json.Unmarshal(request.Payload, &instruction) == nil && instruction.Kind == resourcecap.ConfidentialHTTPKindRequest
}

func hasUnconfirmedOperation(target any) bool {
	operation, ok := target.(*resourcecap.OperationResult)
	return ok && (operation.Status == resourcecap.TaskStatusUnknown || operation.Status == resourcecap.TaskStatusFailed)
}

type callDiagnosticStage string

const (
	callDiagnosticValidation     callDiagnosticStage = "request_validation"
	callDiagnosticAuthentication callDiagnosticStage = "authentication"
	callDiagnosticPreparation    callDiagnosticStage = "request_preparation"
	callDiagnosticTransport      callDiagnosticStage = "http_transport"
	callDiagnosticResponse       callDiagnosticStage = "response_processing"
	callDiagnosticDispatch       callDiagnosticStage = "provider_dispatch"
	callDiagnosticResult         callDiagnosticStage = "provider_result"
	callDiagnosticSetup          callDiagnosticStage = "owner_setup"
)

func (h *confidentialHTTPHandler) observeCallFailure(ctx context.Context, kind string, failure error, started, responded int) {
	stage := callDiagnosticValidation
	if kind == resourcecap.ConfidentialHTTPKindAuthenticate {
		stage = callDiagnosticAuthentication
	} else if kind == resourcecap.ConfidentialHTTPKindRequest {
		stage = callDiagnosticPreparation
	}
	httpStarted, httpResponded := h.businessHTTPStarted > started, h.businessHTTPResponded > responded
	if httpStarted {
		stage = callDiagnosticTransport
	}
	if httpResponded {
		stage = callDiagnosticResponse
	}
	attempted := h.businessHTTPStarted
	diagnostic := newCallDiagnostic(stage, failure, &httpStarted, &httpResponded, &attempted)
	// Read-only connector failures need coordinates too; arbitrary errors and
	// authentication bodies never enter logs or durable operation evidence.
	logger.Warningf(ctx, "confidential provider step failed method=%s diagnostic=%s", h.providerMethod, diagnostic.Summary)
	if h.providerMethod == resourcecap.DynamicProviderMethodExecute && h.observeResponse != nil {
		h.observeResponse(diagnostic)
	}
}

// newCallDiagnostic retains only reviewed codes and fixed summaries. It may
// describe a host/guest handoff failure, never a guessed supplier error body.
func newCallDiagnostic(stage callDiagnosticStage, failure error, started, responded *bool, attempted *int) resourcecap.Evidence {
	code, detail := string(stage), "Provider execution did not return a confirmed outcome"
	switch stage {
	case callDiagnosticValidation:
		detail = "The local request instruction was invalid before supplier execution"
	case callDiagnosticAuthentication:
		detail = "Supplier authentication failed; login response bodies are not retained"
	case callDiagnosticPreparation:
		detail = "Local request preparation failed before the business HTTP call"
	case callDiagnosticTransport:
		detail = "The business HTTP call returned no response body; acceptance is unconfirmed"
	case callDiagnosticResponse:
		detail = "A supplier response arrived but could not establish a confirmed outcome"
	case callDiagnosticDispatch:
		detail = "The connector did not hand a business request to the resource owner; governed validation or connector execution did not complete"
	case callDiagnosticSetup:
		detail = "The resource owner could not prepare the confidential execution context"
	}
	if stage == callDiagnosticDispatch && providerexecution.IsAdmissionBusy(failure) {
		code, detail = "PLUGIN_LIFECYCLE_ALREADY_RUNNING", "The connector remained busy after bounded admission waiting; no confirmed provisioning outcome is available"
	}
	if safe := safeConfidentialProviderError(failure); safe != nil {
		if metadata, ok := bizerr.As(safe); ok {
			code, detail = metadata.RuntimeCode(), metadata.Fallback()
		}
	}
	value := struct {
		Stage                     callDiagnosticStage `json:"stage"`
		ErrorCode                 string              `json:"errorCode"`
		Detail                    string              `json:"detail"`
		BusinessRequestStarted    *bool               `json:"businessRequestStarted,omitempty"`
		ResponseReceived          *bool               `json:"responseReceived,omitempty"`
		BusinessRequestsAttempted *int                `json:"businessRequestsAttempted,omitempty"`
	}{stage, code, detail, started, responded, attempted}
	// All values are owner-defined scalars; this structure cannot fail JSON encoding.
	raw, err := json.Marshal(value)
	if err != nil {
		return resourcecap.Evidence{}
	}
	return resourcecap.Evidence{Kind: resourcecap.EvidenceKindCallDiagnostic, Summary: string(raw), ObservedAt: time.Now().UTC()}
}
