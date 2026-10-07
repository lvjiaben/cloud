// This file defines the normalized Provider capability matrix.

package resourcecap

import "time"

// CapabilityCode identifies one operation or projection a Provider may support.
type CapabilityCode string

// Provider capability codes required by the IDC resource contract.
const (
	CapabilityAuthentication   CapabilityCode = "authentication"
	CapabilityDiscovery        CapabilityCode = "capability.discovery"
	CapabilityProviderCapacity CapabilityCode = "provider-capacity.list"
	CapabilityCost             CapabilityCode = "cost.query"
	CapabilityBalance          CapabilityCode = "balance.query"
	CapabilityExpiry           CapabilityCode = "server.expiry.query"
	CapabilityProvision        CapabilityCode = "server.provision"
	CapabilityResourceQuery    CapabilityCode = "server.query"
	CapabilityStart            CapabilityCode = "server.start"
	CapabilityStop             CapabilityCode = "server.stop"
	CapabilityReboot           CapabilityCode = "server.reboot"
	CapabilitySuspend          CapabilityCode = "server.suspend"
	CapabilityResume           CapabilityCode = "server.resume"
	CapabilityRenew            CapabilityCode = "server.renew"
	CapabilityResetPassword    CapabilityCode = "server.password.reset"
	CapabilityConsole          CapabilityCode = "server.console"
	CapabilityReinstall        CapabilityCode = "server.reinstall"
	CapabilityEnterRescue      CapabilityCode = "server.rescue.enter"
	CapabilityExitRescue       CapabilityCode = "server.rescue.exit"
	CapabilityResize           CapabilityCode = "server.resize"
	CapabilityChangeIP         CapabilityCode = "server.ip.change"
	CapabilityManageSnapshot   CapabilityCode = "server.snapshot"
	CapabilityBackup           CapabilityCode = "server.backup"
	CapabilitySecurityGroup    CapabilityCode = "server.security-group"
	CapabilityNAT              CapabilityCode = "server.nat"
	CapabilityVirtualMedia     CapabilityCode = "server.virtual-media"
	CapabilityTrafficPackage   CapabilityCode = "server.traffic-package"
	CapabilityUpstreamTicket   CapabilityCode = "ticket.manage"
	CapabilityTerminate        CapabilityCode = "server.terminate"
)

// SupportLevel describes how completely a Provider implements one capability.
type SupportLevel string

// Provider capability support levels.
const (
	SupportLevelUnsupported SupportLevel = "unsupported"
	SupportLevelConditional SupportLevel = "conditional"
	SupportLevelSupported   SupportLevel = "supported"
)

// CapabilitySnapshot is one time-bounded Provider capability observation.
type CapabilitySnapshot struct {
	// AdapterVersion identifies the implementation version that produced the snapshot.
	AdapterVersion string
	// Capabilities declares support, constraints, and unsupported reasons by code.
	Capabilities []CapabilityDescriptor
	// ObservedAt records when the adapter produced the snapshot.
	ObservedAt time.Time
}

// CapabilityDescriptor declares support for one normalized capability.
type CapabilityDescriptor struct {
	// Code identifies the normalized capability.
	Code CapabilityCode
	// Support reports whether the capability is unavailable, conditional, or supported.
	Support SupportLevel
	// Constraints describes adapter-visible parameter limits.
	Constraints []ParameterConstraint
	// UnsupportedReason explains why an unavailable capability cannot be used.
	UnsupportedReason string
}

// ParameterConstraint describes one normalized Provider input constraint.
type ParameterConstraint struct {
	// Name identifies the constrained parameter.
	Name string
	// Required reports whether callers must provide the parameter.
	Required bool
	// AllowedValues limits the accepted normalized values when non-empty.
	AllowedValues []string
	// Minimum is an optional inclusive numeric lower bound.
	Minimum *int64
	// Maximum is an optional inclusive numeric upper bound.
	Maximum *int64
	// Pattern is an optional RE2-compatible validation expression.
	Pattern string
}
