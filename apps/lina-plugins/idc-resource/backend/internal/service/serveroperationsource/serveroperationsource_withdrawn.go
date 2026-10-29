// This file withdraws a disproven provider contract immediately, including
// persisted capability snapshots from older releases. It does not mutate old
// evidence, disable other providers, or alter observation of already sent tasks.
package serveroperationsource

import (
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const zjmfCloudPluginID = "idc-zjmfcloud"

// The frozen administrator-v1 endpoint adjusts quantity and has no verified
// address replacement semantics. A previously cached conditional flag is unsafe.
func withdrawnProviderCapability(pluginID string, capability resourcecap.CapabilityCode) bool {
	return pluginID == zjmfCloudPluginID && capability == resourcecap.CapabilityChangeIP
}

// Restoration cannot rely on this connector's earlier synthetic available flag.
// Keep the exact provider/action restriction until readiness is independently proven.
func withdrawnProviderAction(pluginID string, action serveroperationresourcecap.Action) bool {
	capability, ok := capabilityFor(action)
	return ok && withdrawnProviderCapability(pluginID, capability) || pluginID == zjmfCloudPluginID && (action == serveroperationresourcecap.ActionSnapshotRestore || action == serveroperationresourcecap.ActionBackupRestore)
}

// Directory membership proves identity, not readiness, for snapshots and backups.
func providerAccessoryState(pluginID string, kind resourcecap.AccessoryKind, state resourcecap.AccessoryState) resourcecap.AccessoryState {
	if pluginID == zjmfCloudPluginID && (kind == resourcecap.AccessoryKindSnapshot || kind == resourcecap.AccessoryKindBackup) {
		return resourcecap.AccessoryStateUnknown
	}
	return state
}
