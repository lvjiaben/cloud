// This file publishes the exact administrator v1 capability matrix.

package zjmfcloud

import (
	"context"
	"time"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const upstreamTicketUnsupportedReason = "administrator v1 has no ticket endpoint in the frozen contract; finance ticket forwarding is a separate protocol"

// DiscoverCapabilities returns the complete administrator v1 capability matrix.
func (p *Provider) DiscoverCapabilities(ctx context.Context) (*resourcecap.CapabilitySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	snapshot := DeclaredCapabilitySnapshot(p.now())
	if err := snapshot.ValidateAt(snapshot.ObservedAt); err != nil {
		return nil, err
	}
	return snapshot, nil
}

// DeclaredCapabilitySnapshot returns the connector's complete server action
// matrix without constructing a credential-bearing provider instance. The
// managed artifact contract test uses the same declaration as runtime
// capability discovery, so the two cannot drift silently.
func DeclaredCapabilitySnapshot(observedAt time.Time) *resourcecap.CapabilitySnapshot {
	snapshot := &resourcecap.CapabilitySnapshot{
		AdapterVersion: adapterVersion,
		ObservedAt:     observedAt,
		Capabilities: []resourcecap.CapabilityDescriptor{
			supported(resourcecap.CapabilityAuthentication),
			supported(resourcecap.CapabilityDiscovery),
			conditional(resourcecap.CapabilityProviderCapacity, constraint(
				"kind", true, "region", "line", "plan", "image", "stock",
			)),
			unsupported(resourcecap.CapabilityCost, "administrator v1 does not expose resource cost"),
			unsupported(resourcecap.CapabilityBalance, "administrator v1 does not expose channel balance"),
			unsupported(resourcecap.CapabilityExpiry, "administrator v1 does not expose resource expiry"),
			conditional(resourcecap.CapabilityProvision,
				constraint("num", false, "1"),
				minimumConstraint("cpu", true, 1),
				minimumConstraint("memory", true, 128),
				constraint("os", true),
				minimumConstraint("client", true, 1),
				constraint("hostname", true),
			),
			supported(resourcecap.CapabilityResourceQuery),
			supported(resourcecap.CapabilityStart),
			conditional(resourcecap.CapabilityStop, constraint("mode", false, "graceful", "hard")),
			conditional(resourcecap.CapabilityReboot, constraint("mode", false, "graceful", "hard")),
			conditional(resourcecap.CapabilitySuspend, constraint("reason", false, "traffic", "due", "other")),
			supported(resourcecap.CapabilityResume),
			unsupported(resourcecap.CapabilityRenew, "administrator v1 does not expose resource renewal"),
			supported(resourcecap.CapabilityResetPassword),
			supported(resourcecap.CapabilityConsole),
			conditional(resourcecap.CapabilityReinstall,
				minimumConstraint("os", true, 1),
				boundedConstraint("port", false, 1, 65535),
				constraint("format_data_disk", false, "0", "1"),
				minimumConstraint("system_disk_size", false, 1),
			),
			conditional(resourcecap.CapabilityEnterRescue,
				constraint("system", true, "linux", "windows"),
			),
			supported(resourcecap.CapabilityExitRescue),
			conditional(resourcecap.CapabilityResize,
				minimumConstraint("cpu", false, 1),
				minimumConstraint("memory", false, 1),
				minimumConstraint("in_bw", false, 1),
				minimumConstraint("out_bw", false, 1),
				minimumConstraint("system_disk_size", false, 1),
			),
			unsupported(resourcecap.CapabilityChangeIP, "administrator v1 IP count changes do not prove address replacement"),
			conditional(resourcecap.CapabilityManageSnapshot,
				constraint("action", true, "create", "delete"),
				minimumConstraint("disk_ref", false, 1),
				minimumConstraint("snapshot_ref", false, 1),
				constraint("name", false),
			),
			conditional(resourcecap.CapabilityBackup,
				constraint("action", true, "create", "delete"),
				minimumConstraint("disk_ref", false, 1),
				minimumConstraint("snapshot_ref", false, 1),
				constraint("name", false),
			),
			conditional(resourcecap.CapabilitySecurityGroup,
				constraint("action", true, "attach", "detach"),
				minimumConstraint("security_group_ref", false, 1),
			),
			conditional(resourcecap.CapabilityNAT,
				constraint("action", true, "create", "delete"),
				minimumConstraint("nat_ref", false, 1),
				constraint("name", false),
				boundedConstraint("external_port", false, 1, 65535),
				boundedConstraint("internal_port", false, 1, 65535),
				constraint("protocol", false, "tcp", "udp", "tcp_udp"),
			),
			conditional(resourcecap.CapabilityVirtualMedia,
				constraint("action", true, "mount", "unmount"),
				minimumConstraint("image_ref", false, 1),
			),
			conditional(resourcecap.CapabilityTrafficPackage,
				constraint("action", true, "add"),
				boundedConstraint("traffic_gb", true, 1, maximumTrafficPackageGB),
			),
			unsupported(resourcecap.CapabilityUpstreamTicket, upstreamTicketUnsupportedReason),
			supported(resourcecap.CapabilityTerminate),
		},
	}
	return snapshot
}

func supported(code resourcecap.CapabilityCode) resourcecap.CapabilityDescriptor {
	return resourcecap.CapabilityDescriptor{Code: code, Support: resourcecap.SupportLevelSupported}
}

func conditional(
	code resourcecap.CapabilityCode,
	constraints ...resourcecap.ParameterConstraint,
) resourcecap.CapabilityDescriptor {
	return resourcecap.CapabilityDescriptor{
		Code:        code,
		Support:     resourcecap.SupportLevelConditional,
		Constraints: constraints,
	}
}

func unsupported(code resourcecap.CapabilityCode, reason string) resourcecap.CapabilityDescriptor {
	return resourcecap.CapabilityDescriptor{
		Code:              code,
		Support:           resourcecap.SupportLevelUnsupported,
		UnsupportedReason: reason,
	}
}

func constraint(name string, required bool, allowed ...string) resourcecap.ParameterConstraint {
	return resourcecap.ParameterConstraint{Name: name, Required: required, AllowedValues: allowed}
}

func minimumConstraint(name string, required bool, minimum int64) resourcecap.ParameterConstraint {
	return resourcecap.ParameterConstraint{Name: name, Required: required, Minimum: &minimum}
}

func boundedConstraint(
	name string,
	required bool,
	minimum int64,
	maximum int64,
) resourcecap.ParameterConstraint {
	return resourcecap.ParameterConstraint{
		Name: name, Required: required, Minimum: &minimum, Maximum: &maximum,
	}
}
