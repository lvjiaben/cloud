package finance

import (
	"time"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const ProviderID = "zjmf-finance-native"

func DeclaredCapabilitySnapshot(observedAt time.Time) *resourcecap.CapabilitySnapshot {
	return &resourcecap.CapabilitySnapshot{
		AdapterVersion: "zjmf-finance-native-v0.1.0",
		ObservedAt:     observedAt,
		Capabilities: []resourcecap.CapabilityDescriptor{
			supported(resourcecap.CapabilityAuthentication),
			supported(resourcecap.CapabilityDiscovery),
			conditional(resourcecap.CapabilityProviderCapacity, constraint("kind", true, "plan")),
			unsupported(resourcecap.CapabilityCost, "native finance resource contract does not expose normalized cost"),
			unsupported(resourcecap.CapabilityBalance, "balance is part of the upstream commerce workflow"),
			unsupported(resourcecap.CapabilityExpiry, "expiry projection is not proven by the native server route"),
			conditional(resourcecap.CapabilityProvision, constraint("workflow", true, "serialized_cart_v1")),
			supported(resourcecap.CapabilityResourceQuery),
			supported(resourcecap.CapabilityStart),
			conditional(resourcecap.CapabilityStop, constraint("mode", false, "graceful", "hard")),
			conditional(resourcecap.CapabilityReboot, constraint("mode", false, "graceful", "hard")),
			supported(resourcecap.CapabilitySuspend),
			supported(resourcecap.CapabilityResume),
			unsupported(resourcecap.CapabilityRenew, "renewal performs a separate upstream billing transaction"),
			unsupported(resourcecap.CapabilityResetPassword, "password reset awaits operation-secret request binding"),
			unsupported(resourcecap.CapabilityConsole, "console response may contain confidential session material"),
			unsupported(resourcecap.CapabilityReinstall, "reinstall awaits reviewed operation-secret request binding"),
			unsupported(resourcecap.CapabilityEnterRescue, "rescue mode awaits reviewed operation-secret request binding"),
			unsupported(resourcecap.CapabilityExitRescue, "native source does not prove a distinct exit-rescue action"),
			unsupported(resourcecap.CapabilityResize, "upgrade routes perform a separate upstream billing transaction"),
			unsupported(resourcecap.CapabilityChangeIP, "native source does not prove a stable change-IP contract"),
			unsupported(resourcecap.CapabilityManageSnapshot, "native source does not prove snapshot routes"),
			unsupported(resourcecap.CapabilityBackup, "native source does not prove backup routes"),
			unsupported(resourcecap.CapabilitySecurityGroup, "native source does not prove security-group routes"),
			unsupported(resourcecap.CapabilityNAT, "native source does not prove NAT routes"),
			unsupported(resourcecap.CapabilityVirtualMedia, "native source does not prove virtual-media routes"),
			unsupported(resourcecap.CapabilityTrafficPackage, "native source does not prove traffic-package routes"),
			unsupported(resourcecap.CapabilityUpstreamTicket, "ticket forwarding is outside the native server contract"),
			supported(resourcecap.CapabilityTerminate),
		},
	}
}

func supported(code resourcecap.CapabilityCode) resourcecap.CapabilityDescriptor {
	return resourcecap.CapabilityDescriptor{Code: code, Support: resourcecap.SupportLevelSupported}
}

func unsupported(code resourcecap.CapabilityCode, reason string) resourcecap.CapabilityDescriptor {
	return resourcecap.CapabilityDescriptor{Code: code, Support: resourcecap.SupportLevelUnsupported, UnsupportedReason: reason}
}

func conditional(code resourcecap.CapabilityCode, constraints ...resourcecap.ParameterConstraint) resourcecap.CapabilityDescriptor {
	return resourcecap.CapabilityDescriptor{Code: code, Support: resourcecap.SupportLevelConditional, Constraints: constraints}
}

func constraint(name string, required bool, values ...string) resourcecap.ParameterConstraint {
	return resourcecap.ParameterConstraint{Name: name, Required: required, AllowedValues: values}
}
