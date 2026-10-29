package nativecommercial

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

var (
	errTargetChanged     = errors.New("native commercial resource target changed")
	errCapabilityRemoved = errors.New("native commercial capability was removed")
)

// Transient reads, stale directories, capacity contention and database errors
// remain retryable. Only a proved immutable target/parameter mismatch or a
// complete capability observation removing the action closes unsent admission.
func permanentAdmissionFailure(operation servercommercialcap.Operation, err error) *servercommercialcap.AdmissionFailure {
	var reason string
	switch {
	case errors.Is(err, errTargetChanged), errors.Is(err, serveroperationresourcecap.ErrTaskConflict):
		reason = "target_conflict"
	case errors.Is(err, errCapabilityRemoved):
		reason = "capability_unavailable"
	default:
		return nil
	}
	payload, marshalErr := json.Marshal(struct{ OperationNo, EntitlementNo, IntentDigest, Reason string }{operation.OperationNo, operation.EntitlementNo, operation.ContentDigest, reason})
	if marshalErr != nil {
		return nil
	}
	digest := sha256.Sum256(payload)
	return &servercommercialcap.AdmissionFailure{OperationNo: operation.OperationNo, EntitlementNo: operation.EntitlementNo, ReasonCode: reason, EvidenceDigest: hex.EncodeToString(digest[:])}
}
