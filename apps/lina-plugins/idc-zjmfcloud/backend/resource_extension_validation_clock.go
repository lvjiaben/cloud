// Guest validation checks result shape without granting the isolated clock
// observation authority. Only freshly decoded local validation copies use this
// neutral date. Returned payload bytes stay unchanged; idc-resource stamps real
// receipt times and validates them after the governed invocation completes.
package backend

import (
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"time"
)

// guestShapeValidationTime belongs only to local shape validation, not evidence.
func guestShapeValidationTime() time.Time {
	return time.Unix(946684800, 0).UTC()
}

// prepareGuestShapeObservation touches only freshly decoded validation copies.
// It leaves identity, status, data, failure and credential constraints intact.
func prepareGuestShapeObservation(target any) {
	now := guestShapeValidationTime()
	stampEvidence := func(items []resourcecap.Evidence) {
		for index := range items {
			items[index].ObservedAt = now
		}
	}
	switch result := target.(type) {
	case *resourcecap.ConnectionVerification:
		result.ObservedAt = now
	case *resourcecap.ProviderCapacityPage:
		stampEvidence(result.Evidence)
		for index := range result.Items {
			result.Items[index].ObservedAt = now
		}
	case *resourcecap.ResourceSnapshot:
		result.ObservedAt = now
		stampEvidence(result.Evidence)
	case *resourcecap.OperationResult:
		stampEvidence(result.Evidence)
		if result.Resource != nil {
			result.Resource.ObservedAt = now
			stampEvidence(result.Resource.Evidence)
		}
	case *resourcecap.PrincipalLookupResult:
		result.ObservedAt = now
		stampEvidence(result.Evidence)
	case *resourcecap.PrincipalCreateResult:
		result.ObservedAt = now
		stampEvidence(result.Evidence)
	}
}
