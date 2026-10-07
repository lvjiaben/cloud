// This file verifies the Provider SPI capability coverage and environment boundary.

package resourcecap

import (
	"reflect"
	"testing"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

// TestProviderCapabilityCodesAreCompleteAndUnique guards the normalized actions
// required by the first Provider contract.
func TestProviderCapabilityCodesAreCompleteAndUnique(t *testing.T) {
	codes := []CapabilityCode{
		CapabilityAuthentication,
		CapabilityDiscovery,
		CapabilityProviderCapacity,
		CapabilityCost,
		CapabilityBalance,
		CapabilityExpiry,
		CapabilityProvision,
		CapabilityResourceQuery,
		CapabilityStart,
		CapabilityStop,
		CapabilityReboot,
		CapabilitySuspend,
		CapabilityResume,
		CapabilityRenew,
		CapabilityResetPassword,
		CapabilityConsole,
		CapabilityReinstall,
		CapabilityEnterRescue,
		CapabilityExitRescue,
		CapabilityResize,
		CapabilityChangeIP,
		CapabilityManageSnapshot,
		CapabilityBackup,
		CapabilitySecurityGroup,
		CapabilityNAT,
		CapabilityVirtualMedia,
		CapabilityTrafficPackage,
		CapabilityUpstreamTicket,
		CapabilityTerminate,
	}
	seen := make(map[CapabilityCode]struct{}, len(codes))
	for _, code := range codes {
		if code == "" {
			t.Fatal("capability code must not be empty")
		}
		if _, exists := seen[code]; exists {
			t.Fatalf("duplicate capability code %q", code)
		}
		seen[code] = struct{}{}
	}
}

// TestProviderEnvHasOnlyApprovedConstructionCapabilities prevents accidental
// exposure of host service directories or database handles to adapters.
func TestProviderEnvHasOnlyApprovedConstructionCapabilities(t *testing.T) {
	envType := reflect.TypeOf(ProviderEnv{})
	wantFields := []string{
		"PluginID",
		"ProviderID",
		"Secrets",
		"SecretSink",
		"AuthenticationSessions",
		"HTTPClient",
		"Clock",
		"Logger",
		"Metrics",
	}
	if envType.NumField() != len(wantFields) {
		t.Fatalf("ProviderEnv exposes %d fields, want %d approved fields", envType.NumField(), len(wantFields))
	}
	for index, fieldName := range wantFields {
		if got := envType.Field(index).Name; got != fieldName {
			t.Fatalf("ProviderEnv field %d=%q, want %q", index, got, fieldName)
		}
	}
	secretsField, _ := envType.FieldByName("Secrets")
	wantAccessorType := reflect.TypeOf((*secretcap.Accessor)(nil)).Elem()
	if secretsField.Type != wantAccessorType {
		t.Fatalf("ProviderEnv.Secrets type=%v, want core secretcap.Accessor", secretsField.Type)
	}
	secretSinkField, _ := envType.FieldByName("SecretSink")
	wantSecretSinkType := reflect.TypeOf((*SecretSink)(nil)).Elem()
	if secretSinkField.Type != wantSecretSinkType {
		t.Fatalf("ProviderEnv.SecretSink type=%v, want resourcecap.SecretSink", secretSinkField.Type)
	}
	authSessionsField, _ := envType.FieldByName("AuthenticationSessions")
	wantAuthSessionsType := reflect.TypeOf((*AuthenticationSessions)(nil)).Elem()
	if authSessionsField.Type != wantAuthSessionsType {
		t.Fatalf("ProviderEnv.AuthenticationSessions type=%v, want resourcecap.AuthenticationSessions", authSessionsField.Type)
	}
	bindingType := reflect.TypeOf(SecretBinding{})
	handleField, ok := bindingType.FieldByName("Handle")
	if !ok || handleField.Type != reflect.TypeOf(secretcap.Handle{}) {
		t.Fatalf("SecretBinding must carry a core secretcap.Handle, got %#v", handleField)
	}
}
