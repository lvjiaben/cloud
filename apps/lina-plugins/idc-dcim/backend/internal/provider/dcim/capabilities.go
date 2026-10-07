// This file declares only lifecycle behavior proven for the frozen DCIM protocol.

package dcim

import (
	"time"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// ProviderID is the stable resource-provider identifier published in the manifest.
const ProviderID = "dcim"

func declaredCapabilitySnapshot(protocol Protocol, observedAt time.Time) *resourcecap.CapabilitySnapshot {
	capabilities := []resourcecap.CapabilityDescriptor{
		supported(resourcecap.CapabilityAuthentication),
		supported(resourcecap.CapabilityDiscovery),
		conditional(resourcecap.CapabilityProviderCapacity, constraint("kind", true, "plan")),
		unsupported(resourcecap.CapabilityCost, "DCIM contract does not expose normalized supplier cost"),
		unsupported(resourcecap.CapabilityBalance, "DCIM contract does not expose supplier account balance"),
		unsupported(resourcecap.CapabilityExpiry, "DCIM contract does not expose a stable resource expiry fact"),
		unsupported(resourcecap.CapabilityRenew, "renewal is owned by local billing and is not a DCIM resource action"),
		unsupported(resourcecap.CapabilityUpstreamTicket, "ticket forwarding is outside the DCIM resource contract"),
	}
	if protocol == ProtocolBMS {
		capabilities = append(capabilities,
			conditional(resourcecap.CapabilityProvision, constraint("protocol", true, string(ProtocolBMS))),
			supported(resourcecap.CapabilityResourceQuery), supported(resourcecap.CapabilityStart),
			supported(resourcecap.CapabilityStop), supported(resourcecap.CapabilityReboot),
			supported(resourcecap.CapabilitySuspend), supported(resourcecap.CapabilityResume),
			supported(resourcecap.CapabilityTerminate),
			unsupported(resourcecap.CapabilityResetPassword, "operation-secret output and post-change verification are not yet proven"),
			unsupported(resourcecap.CapabilityConsole, "VNC response contains confidential session material"),
			unsupported(resourcecap.CapabilityReinstall, "reinstall requires additional secret and disk safety review"),
			unsupported(resourcecap.CapabilityEnterRescue, "rescue credentials require confidential output handling"),
			unsupported(resourcecap.CapabilityExitRescue, "rescue exit is not exposed as a distinct stable action"),
			unsupported(resourcecap.CapabilityResize, "BMS resize is a multi-step workflow requiring separate reconciliation"),
			unsupported(resourcecap.CapabilityChangeIP, "BMS IP replacement safety semantics are not proven"),
			unsupported(resourcecap.CapabilityManageSnapshot, "snapshot ownership projection is not yet available"),
			unsupported(resourcecap.CapabilityBackup, "backup ownership projection is not yet available"),
			unsupported(resourcecap.CapabilitySecurityGroup, "BMS source does not prove security-group operations"),
			unsupported(resourcecap.CapabilityNAT, "BMS source does not prove NAT operations"),
			unsupported(resourcecap.CapabilityVirtualMedia, "BMS source does not prove virtual-media operations"),
			unsupported(resourcecap.CapabilityTrafficPackage, "BMS source does not prove traffic-package operations"),
		)
	} else {
		capabilities = append(capabilities,
			unsupported(resourcecap.CapabilityProvision, "legacy provisioning depends on an old local host id, callback token, and confidential supplier output"),
			unsupported(resourcecap.CapabilityResourceQuery, "legacy detail requests depend on the old finance-system local host id"),
			unsupported(resourcecap.CapabilityStart, "legacy power requests depend on the old finance-system local host id"),
			unsupported(resourcecap.CapabilityStop, "legacy power requests depend on the old finance-system local host id"),
			unsupported(resourcecap.CapabilityReboot, "legacy power requests depend on the old finance-system local host id"),
			unsupported(resourcecap.CapabilitySuspend, "legacy suspend requests depend on the old finance-system local host id"),
			unsupported(resourcecap.CapabilityResume, "legacy resume requests depend on the old finance-system local host id"),
			unsupported(resourcecap.CapabilityTerminate, "legacy termination requests depend on the old finance-system local host id"),
			unsupported(resourcecap.CapabilityResetPassword, "legacy password reset safety semantics are not proven"),
			unsupported(resourcecap.CapabilityConsole, "legacy console response contains confidential session material"),
			unsupported(resourcecap.CapabilityReinstall, "legacy reinstall requires additional secret and disk safety review"),
			unsupported(resourcecap.CapabilityEnterRescue, "legacy rescue requires additional secret safety review"),
			unsupported(resourcecap.CapabilityExitRescue, "legacy rescue exit contract is not proven"),
			unsupported(resourcecap.CapabilityResize, "legacy resize is a multi-step workflow requiring separate reconciliation"),
			unsupported(resourcecap.CapabilityChangeIP, "legacy IP replacement safety semantics are not proven"),
			unsupported(resourcecap.CapabilityManageSnapshot, "legacy snapshot routes are not proven"),
			unsupported(resourcecap.CapabilityBackup, "legacy backup routes are not proven"),
			unsupported(resourcecap.CapabilitySecurityGroup, "legacy source does not prove security-group operations"),
			unsupported(resourcecap.CapabilityNAT, "legacy source does not prove NAT operations"),
			unsupported(resourcecap.CapabilityVirtualMedia, "legacy source does not prove virtual-media operations"),
			unsupported(resourcecap.CapabilityTrafficPackage, "legacy traffic-package safety semantics are not proven"),
		)
	}
	return &resourcecap.CapabilitySnapshot{
		AdapterVersion: "dcim-v0.1.0", ObservedAt: observedAt, Capabilities: capabilities,
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
