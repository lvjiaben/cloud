// This file retains owner-only proof when local preparation exhausts its retry
// budget without calling a provider. It never converts a transport failure or
// an accepted operation into a replayable provisioning failure.
package resourcetask

import (
	"encoding/json"
	"time"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const localPreparationDiagnosticStage = "request_preparation"

// localProvisionFailureFacts binds the no-call proof to this exact task while
// preserving the original fixed preparation code and message for operators.
func localProvisionFailureFacts(taskNo, code, summary string) ([]resourcecap.Evidence, error) {
	observedAt := time.Now().UTC()
	proof, err := resourcecap.NewProvisionNonSubmissionEvidence(taskNo, observedAt)
	if err != nil {
		return nil, err
	}
	value := struct {
		Stage     string `json:"stage"`
		Code      string `json:"errorCode"`
		Detail    string `json:"detail"`
		Started   bool   `json:"businessRequestStarted"`
		Responded bool   `json:"responseReceived"`
		Attempted int    `json:"businessRequestsAttempted"`
	}{Stage: localPreparationDiagnosticStage, Code: code, Detail: summary}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	diagnostic := resourcecap.Evidence{Kind: resourcecap.EvidenceKindCallDiagnostic, Summary: string(raw), ObservedAt: observedAt}
	if err = diagnostic.Validate(); err != nil {
		return nil, err
	}
	return []resourcecap.Evidence{diagnostic, proof}, nil
}
