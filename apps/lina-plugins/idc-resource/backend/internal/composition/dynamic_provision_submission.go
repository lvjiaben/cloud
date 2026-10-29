// This file lets only trusted owner observations establish that provisioning
// attempted no business HTTP request. Guest claims are stripped, and any prior
// attempt or acceptance identifier preserves the unknown-result fence.
package composition

import (
	"encoding/json"
	"slices"
	"time"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func (o *dynamicCallObservation) finalizeProvision(method string, request any, target any) {
	operation, ok := target.(*resourcecap.OperationResult)
	if !ok || method != resourcecap.DynamicProviderMethodExecute {
		return
	}
	// A reserved statement carried by a guest is not owner proof, even when its
	// body has the expected shape. It can never enable recovery or a refund.
	operation.Evidence = slices.DeleteFunc(operation.Evidence, func(item resourcecap.Evidence) bool { return item.Kind == resourcecap.EvidenceKindRequestNotSent })
	if operation.Failure != nil && operation.Failure.Code == provisionresourcecap.FailureRequestNotSent {
		operation.Status = resourcecap.TaskStatusUnknown
		operation.Failure = &resourcecap.ProviderFailure{Code: "upstream_result_unknown", Message: "Provisioning requires reconciliation", ResultUnknown: true}
	}
	projected, ok := request.(resourcecap.DynamicOperationRequest)
	if !ok || projected.Capability != resourcecap.CapabilityProvision || projected.ResourceRef != "" ||
		(operation.Status != resourcecap.TaskStatusUnknown && operation.Status != resourcecap.TaskStatusFailed) ||
		operation.ExternalTaskRef != "" || operation.ExternalResourceRef != "" || operation.Resource != nil || operation.Progress != nil || len(operation.ResidualResources) != 0 || len(operation.SecretOutputs) != 0 {
		return
	}
	o.mu.Lock()
	observations := append([]resourcecap.Evidence(nil), o.evidence...)
	attempted := o.anyBusinessAttempt
	o.mu.Unlock()
	if attempted || !ownerObservedNoBusinessAttempt(observations) {
		return
	}
	proof, err := resourcecap.NewProvisionNonSubmissionEvidence(projected.RequestID, time.Now().UTC())
	if err != nil {
		return
	}
	operation.Status = resourcecap.TaskStatusFailed
	operation.Failure = &resourcecap.ProviderFailure{Code: provisionresourcecap.FailureRequestNotSent, Message: "The provisioning request was not sent; correct the local configuration before recovery"}
	operation.Evidence = append(operation.Evidence, proof)
}

func ownerObservedNoBusinessAttempt(observations []resourcecap.Evidence) bool {
	confirmed := false
	for _, item := range observations {
		if item.Kind == resourcecap.EvidenceKindHTTPResponse {
			return false
		}
		if item.Kind != resourcecap.EvidenceKindCallDiagnostic {
			continue
		}
		var diagnostic struct {
			Stage     callDiagnosticStage `json:"stage"`
			Attempted *int                `json:"businessRequestsAttempted"`
		}
		if json.Unmarshal([]byte(item.Summary), &diagnostic) != nil || diagnostic.Attempted == nil || *diagnostic.Attempted != 0 {
			return false
		}
		switch diagnostic.Stage {
		case callDiagnosticValidation, callDiagnosticAuthentication, callDiagnosticPreparation:
			confirmed = true
		default:
			return false
		}
	}
	return confirmed
}
