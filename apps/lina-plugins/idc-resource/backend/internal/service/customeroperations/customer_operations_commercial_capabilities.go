// This file separates quote entry eligibility from executable free management.
// Merchant pricing, stock and financial rules remain authoritative at Quote.
package customeroperations

import (
	"slices"
	"time"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// CommercialCapability allows a quote request, never direct execution or a
// promise that merchant rules will admit a purchase.
type CommercialCapability struct {
	Action              string
	Supported, CanQuote bool
	ReasonCode          string
}

func commercialCapabilities(canOperate, busy bool, observation *resourcecap.ResourceSnapshot, observationErr error, capabilities []serveroperationresourcecap.CapabilityProjection, capabilityErr error, choices *Options) []CommercialCapability {
	actions := []serveroperationresourcecap.Action{serveroperationresourcecap.ActionSnapshotCreate, serveroperationresourcecap.ActionBackupCreate, serveroperationresourcecap.ActionTrafficAdd, serveroperationresourcecap.ActionChangeIP, serveroperationresourcecap.ActionResize}
	result := make([]CommercialCapability, 0, len(actions))
	known := capabilityErr == nil && len(capabilities) == 1 && !capabilities[0].ObservedAt.IsZero()
	for _, action := range actions {
		item := CommercialCapability{Action: string(action), Supported: known && slices.Contains(capabilities[0].Actions, action)}
		switch {
		case !known:
			item.ReasonCode = "capability_unavailable"
		case !item.Supported:
			item.ReasonCode = "unsupported"
		case !canOperate:
			item.ReasonCode = "entitlement_unavailable"
		case busy:
			item.ReasonCode = "operation_in_progress"
		case observationErr != nil || observation == nil || observation.State == resourcecap.ResourceStateUnknown:
			item.ReasonCode = "runtime_unconfirmed"
		case observation.State != resourcecap.ResourceStateRunning && observation.State != resourcecap.ResourceStateActive && observation.State != resourcecap.ResourceStateStopped:
			item.ReasonCode = "runtime_state_conflict"
		case choices == nil || choices.Accessories == nil || choices.Accessories.SnapshotNo == "" || !choices.Accessories.ExpiresAt.After(time.Now()) || !slices.Contains(choices.Accessories.Actions, action):
			item.ReasonCode = "choices_unavailable"
		default:
			item.CanQuote = true
		}
		result = append(result, item)
	}
	return result
}
