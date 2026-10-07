package v10

import (
	"time"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const ProviderID = "idcsmart-business-v10"

func capabilities(observedAt time.Time) resourcecap.CapabilitySnapshot {
	return resourcecap.CapabilitySnapshot{
		AdapterVersion: "idcsmart-business-v10-v0.2.0",
		ObservedAt:     observedAt,
		Capabilities: []resourcecap.CapabilityDescriptor{
			supported(resourcecap.CapabilityAuthentication),
			supported(resourcecap.CapabilityDiscovery),
			conditional(resourcecap.CapabilityProviderCapacity, constraint("kind", true, "plan")),
			unsupported(resourcecap.CapabilityCost, "V10 product discovery does not expose normalized supplier cost"),
			unsupported(resourcecap.CapabilityBalance, "V10 balance belongs to the upstream payment workflow"),
			unsupported(resourcecap.CapabilityExpiry, "V10 resource detail does not prove a stable normalized expiry contract"),
			conditional(resourcecap.CapabilityProvision, constraint("workflow", true, "console_cart_v1")),
			supported(resourcecap.CapabilityResourceQuery),
			unsupported(resourcecap.CapabilityStart, "V10 delegates power actions to product modules with module-specific contracts"),
			unsupported(resourcecap.CapabilityStop, "V10 delegates power actions to product modules with module-specific contracts"),
			unsupported(resourcecap.CapabilityReboot, "V10 delegates power actions to product modules with module-specific contracts"),
			supported(resourcecap.CapabilitySuspend),
			supported(resourcecap.CapabilityResume),
			unsupported(resourcecap.CapabilityRenew, "V10 renewal creates a separate upstream financial transaction"),
			unsupported(resourcecap.CapabilityResetPassword, "V10 password reset requires a separately reviewed module contract"),
			unsupported(resourcecap.CapabilityConsole, "V10 console results can contain confidential session material"),
			unsupported(resourcecap.CapabilityReinstall, "V10 reinstall is module-specific"),
			unsupported(resourcecap.CapabilityEnterRescue, "V10 rescue mode is module-specific"),
			unsupported(resourcecap.CapabilityExitRescue, "V10 rescue mode is module-specific"),
			unsupported(resourcecap.CapabilityResize, "V10 resize creates a separate upstream financial transaction"),
			unsupported(resourcecap.CapabilityChangeIP, "V10 change-IP is module-specific"),
			unsupported(resourcecap.CapabilityManageSnapshot, "V10 snapshot handling is module-specific"),
			unsupported(resourcecap.CapabilityBackup, "V10 backup handling is module-specific"),
			unsupported(resourcecap.CapabilitySecurityGroup, "V10 security groups are module-specific"),
			unsupported(resourcecap.CapabilityNAT, "V10 NAT handling is module-specific"),
			unsupported(resourcecap.CapabilityVirtualMedia, "V10 virtual media is module-specific"),
			unsupported(resourcecap.CapabilityTrafficPackage, "V10 traffic packages create a separate upstream transaction"),
			unsupported(resourcecap.CapabilityUpstreamTicket, "V10 ticket forwarding is outside the server provider contract"),
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
