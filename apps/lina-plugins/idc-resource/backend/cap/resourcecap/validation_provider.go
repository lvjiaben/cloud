// This file validates Provider construction, capability, logging, and metric boundaries.

package resourcecap

import (
	"encoding/hex"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	// maximumObservationFutureSkew tolerates small adapter and owner clock drift.
	maximumObservationFutureSkew = 5 * time.Minute
	// maximumCapabilityObservationAge prevents stale support matrices from
	// driving current product and action availability.
	maximumCapabilityObservationAge = 24 * time.Hour
	// minimumObservationYear excludes timestamps outside the supported runtime era.
	minimumObservationYear = 2000
	// maximumObservationYear leaves arithmetic headroom and rejects year-9999 sentinels.
	maximumObservationYear = 9998
)

// Validate verifies a detached Provider channel configuration before a factory
// receives it. Runtime endpoint policy must still verify DNS, certificates,
// redirects, and response limits when the channel is enabled.
func (c ProviderConfig) Validate() error {
	if err := ValidateChannelRef(c.ChannelRef); err != nil {
		return err
	}
	if err := validateEndpoint(c.Endpoint); err != nil {
		return err
	}
	if c.TLSCertificateSHA256 != "" {
		decoded, err := hex.DecodeString(c.TLSCertificateSHA256)
		if err != nil || len(decoded) != 32 {
			return gerror.New("invalid certificate fingerprint")
		}
	}
	if err := validateJSONObject("settings", c.Settings, false); err != nil {
		return err
	}
	return validateSecretBindings(c.SecretBindings)
}

// Validate verifies one named binding contains a non-zero core-owned handle.
func (b SecretBinding) Validate() error {
	if err := validateSafeName("secret binding name", b.Name, maximumNameBytes); err != nil {
		return err
	}
	if b.Handle.IsZero() {
		return gerror.New("resource Provider secret binding requires a non-zero handle")
	}
	return nil
}

// validateSecretBindings verifies one bounded, uniquely named handle set.
func validateSecretBindings(bindings []SecretBinding) error {
	if len(bindings) > MaximumSecretBindingCount {
		return gerror.Newf("resource Provider secret bindings exceed %d entries", MaximumSecretBindingCount)
	}
	seen := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		if err := binding.Validate(); err != nil {
			return err
		}
		if _, exists := seen[binding.Name]; exists {
			return gerror.New("resource Provider secret binding is duplicated")
		}
		seen[binding.Name] = struct{}{}
	}
	return nil
}

// Validate verifies every Provider construction capability is explicitly
// supplied. Owner composition remains responsible for scoping each dependency
// to the declared plugin, Provider, and channel.
func (e ProviderEnv) Validate() error {
	if err := ValidatePluginID(e.PluginID); err != nil {
		return err
	}
	if err := ValidateProviderID(e.ProviderID); err != nil {
		return err
	}
	dependencies := []struct {
		name  string
		value any
	}{
		{name: "secret accessor", value: e.Secrets},
		{name: "secret sink", value: e.SecretSink},
		{name: "authentication sessions", value: e.AuthenticationSessions},
		{name: "HTTP client", value: e.HTTPClient},
		{name: "clock", value: e.Clock},
		{name: "logger", value: e.Logger},
		{name: "metrics", value: e.Metrics},
	}
	for _, dependency := range dependencies {
		if isNilInterface(dependency.value) {
			return gerror.Newf("resource Provider %s is required", dependency.name)
		}
	}
	return nil
}

// SupportedCapabilityCodes returns the complete deterministic Provider matrix.
func SupportedCapabilityCodes() []CapabilityCode {
	return []CapabilityCode{
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
}

// Validate fails closed because capability freshness cannot be established
// without an explicit resource-owner clock. Call ValidateAt with Clock.Now().
func (s CapabilitySnapshot) Validate() error {
	return gerror.New("resource Provider capability snapshot requires explicit owner clock validation")
}

// ValidateAt verifies a capability observation is fresh, complete, unique,
// bounded, and internally consistent using the caller-owned current time.
func (s CapabilitySnapshot) ValidateAt(now time.Time) error {
	if err := validateSafeDiagnostic("adapter version", s.AdapterVersion, maximumAdapterVersionBytes, true); err != nil {
		return err
	}
	want := SupportedCapabilityCodes()
	if len(s.Capabilities) != len(want) || len(s.Capabilities) != MaximumCapabilityCount {
		return gerror.Newf("resource Provider capability matrix must contain exactly %d entries", len(want))
	}
	if err := validateCapabilityAggregateBudget(s); err != nil {
		return err
	}
	if err := validateObservationTimeAt(
		"capability observation time",
		s.ObservedAt,
		now,
		maximumCapabilityObservationAge,
	); err != nil {
		return err
	}
	for _, descriptor := range s.Capabilities {
		if err := validateMachineValue("capability code", string(descriptor.Code)); err != nil {
			return err
		}
		if err := validateMachineValue("support level", string(descriptor.Support)); err != nil {
			return err
		}
	}
	remaining := make(map[CapabilityCode]struct{}, len(want))
	for _, code := range want {
		remaining[code] = struct{}{}
	}
	for _, descriptor := range s.Capabilities {
		if err := descriptor.Validate(); err != nil {
			return err
		}
		if _, exists := remaining[descriptor.Code]; !exists {
			return gerror.New("resource Provider capability is unknown or duplicated")
		}
		delete(remaining, descriptor.Code)
	}
	if len(remaining) != 0 {
		return gerror.New("resource Provider capability matrix is incomplete")
	}
	return nil
}

// Validate verifies one capability support declaration and its constraints.
func (d CapabilityDescriptor) Validate() error {
	if err := validateCapabilityCode(d.Code); err != nil {
		return err
	}
	if err := validateSupportLevel(d.Support); err != nil {
		return err
	}
	switch d.Support {
	case SupportLevelUnsupported:
		if len(d.Constraints) != 0 {
			return gerror.New("resource Provider unsupported capability must not declare constraints")
		}
		if err := validateSafeDiagnostic(
			"unsupported capability reason",
			d.UnsupportedReason,
			maximumCapabilityReasonBytes,
			true,
		); err != nil {
			return err
		}
	case SupportLevelConditional:
		if len(d.Constraints) == 0 {
			return gerror.New("resource Provider conditional capability requires constraints")
		}
		if d.UnsupportedReason != "" {
			return gerror.New("resource Provider conditional capability must not have an unsupported reason")
		}
	case SupportLevelSupported:
		if d.UnsupportedReason != "" {
			return gerror.New("resource Provider supported capability must not have an unsupported reason")
		}
	default:
		return gerror.New("resource Provider capability support level is invalid")
	}
	if len(d.Constraints) > maximumConstraintCount {
		return gerror.Newf("resource Provider capability exceeds %d constraints", maximumConstraintCount)
	}
	seen := make(map[string]struct{}, len(d.Constraints))
	for _, constraint := range d.Constraints {
		if err := constraint.Validate(); err != nil {
			return err
		}
		if _, exists := seen[constraint.Name]; exists {
			return gerror.New("resource Provider capability duplicates a constraint")
		}
		seen[constraint.Name] = struct{}{}
	}
	return nil
}

// Validate verifies one Provider parameter constraint is bounded and usable.
func (c ParameterConstraint) Validate() error {
	if err := validateSafeName("constraint name", c.Name, maximumNameBytes); err != nil {
		return err
	}
	if len(c.AllowedValues) > maximumAllowedValueCount {
		return gerror.New("resource Provider constraint has too many allowed values")
	}
	seen := make(map[string]struct{}, len(c.AllowedValues))
	for _, value := range c.AllowedValues {
		if err := validateSafeDiagnostic("constraint allowed value", value, maximumMessageBytes, true); err != nil {
			return err
		}
		if _, exists := seen[value]; exists {
			return gerror.New("resource Provider constraint duplicates an allowed value")
		}
		seen[value] = struct{}{}
	}
	if c.Minimum != nil && c.Maximum != nil && *c.Minimum > *c.Maximum {
		return gerror.New("resource Provider constraint minimum exceeds maximum")
	}
	if err := validateSafeDiagnostic("constraint pattern", c.Pattern, maximumPatternBytes, false); err != nil {
		return err
	}
	if c.Pattern != "" {
		if _, err := regexp.Compile(c.Pattern); err != nil {
			return gerror.New("resource Provider constraint pattern is invalid")
		}
	}
	return nil
}

// ValidateLogEntry verifies one redacted Provider diagnostic before an owner
// logger wrapper accepts it.
func ValidateLogEntry(level LogLevel, message string, fields ...LogField) error {
	if err := validateLogLevel(level); err != nil {
		return err
	}
	if err := validateSafeDiagnostic("log message", message, maximumMessageBytes, true); err != nil {
		return err
	}
	if len(fields) > MaximumLogFieldCount {
		return gerror.Newf("resource Provider log fields exceed %d entries", MaximumLogFieldCount)
	}
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if err := validateSafeName("log field key", field.Key, maximumNameBytes); err != nil {
			return err
		}
		if sensitiveDiagnosticKey(field.Key) {
			return gerror.New("resource Provider log field key is sensitive")
		}
		if err := validateSafeDiagnostic("log field value", field.Value, maximumMessageBytes, false); err != nil {
			return err
		}
		if _, exists := seen[field.Key]; exists {
			return gerror.New("resource Provider log field is duplicated")
		}
		seen[field.Key] = struct{}{}
	}
	return nil
}

// ValidateMetric verifies one bounded metric identity and label set before an
// owner metrics wrapper accepts it.
func ValidateMetric(name string, labels ...MetricLabel) error {
	if err := validateSafeName("metric name", name, maximumNameBytes); err != nil {
		return err
	}
	if len(labels) > MaximumMetricLabelCount {
		return gerror.Newf("resource Provider metric labels exceed %d entries", MaximumMetricLabelCount)
	}
	seen := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		if err := validateSafeName("metric label name", label.Name, maximumNameBytes); err != nil {
			return err
		}
		if sensitiveDiagnosticKey(label.Name) {
			return gerror.New("resource Provider metric label is sensitive")
		}
		if err := validateSafeDiagnostic("metric label value", label.Value, maximumMetricValueBytes, true); err != nil {
			return err
		}
		if _, exists := seen[label.Name]; exists {
			return gerror.New("resource Provider metric label is duplicated")
		}
		seen[label.Name] = struct{}{}
	}
	return nil
}

// ValidateMetricObservation verifies a finite observation and its dimensions.
func ValidateMetricObservation(name string, value float64, labels ...MetricLabel) error {
	if !validMetricValue(value) {
		return gerror.New("resource Provider metric observation must be finite")
	}
	return ValidateMetric(name, labels...)
}

// capabilityValidationBudget tracks total capability graph size before any
// descriptor can be accepted as a publishable snapshot.
type capabilityValidationBudget struct {
	// nodes is the aggregate descriptor, constraint, bound, and value count.
	nodes int
	// bytes is the aggregate byte length of all capability strings and bounds.
	bytes int
}

// add accounts for aggregate capability nodes and string bytes without
// allowing integer overflow or work beyond the configured limits.
func (b *capabilityValidationBudget) add(nodes int, bytes int) error {
	if nodes < 0 || nodes > maximumCapabilityNodes-b.nodes {
		return gerror.Newf("resource Provider capability graph exceeds %d nodes", maximumCapabilityNodes)
	}
	if bytes < 0 || bytes > maximumCapabilityBytes-b.bytes {
		return gerror.Newf("resource Provider capability graph exceeds %d bytes", maximumCapabilityBytes)
	}
	b.nodes += nodes
	b.bytes += bytes
	return nil
}

// validateCapabilityAggregateBudget bounds the complete capability graph,
// including descriptors, constraints, bounds, patterns, and allowed values.
func validateCapabilityAggregateBudget(snapshot CapabilitySnapshot) error {
	budget := capabilityValidationBudget{}
	if err := budget.add(1, len(snapshot.AdapterVersion)); err != nil {
		return err
	}
	for _, descriptor := range snapshot.Capabilities {
		descriptorBytes := len(descriptor.Code) + len(descriptor.Support) + len(descriptor.UnsupportedReason)
		if err := budget.add(1, descriptorBytes); err != nil {
			return err
		}
		for _, constraint := range descriptor.Constraints {
			constraintNodes := 1
			constraintBytes := len(constraint.Name) + len(constraint.Pattern) + 1
			if constraint.Minimum != nil {
				constraintNodes++
				constraintBytes += 8
			}
			if constraint.Maximum != nil {
				constraintNodes++
				constraintBytes += 8
			}
			if err := budget.add(constraintNodes, constraintBytes); err != nil {
				return err
			}
			if err := budget.add(len(constraint.AllowedValues), 0); err != nil {
				return err
			}
			for _, value := range constraint.AllowedValues {
				if err := budget.add(0, len(value)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// isSupportedCapability reports whether a capability code belongs to the
// current complete resource contract matrix.
func isSupportedCapability(code CapabilityCode) bool {
	for _, candidate := range SupportedCapabilityCodes() {
		if code == candidate {
			return true
		}
	}
	return false
}

// validateCapabilityCode applies machine-value bounds before membership.
func validateCapabilityCode(code CapabilityCode) error {
	if err := validateMachineValue("capability code", string(code)); err != nil {
		return err
	}
	if !isSupportedCapability(code) {
		return gerror.New("resource Provider capability is unsupported by the contract")
	}
	return nil
}

// validateSupportLevel applies machine-value bounds before membership.
func validateSupportLevel(level SupportLevel) error {
	if err := validateMachineValue("support level", string(level)); err != nil {
		return err
	}
	switch level {
	case SupportLevelUnsupported, SupportLevelConditional, SupportLevelSupported:
		return nil
	default:
		return gerror.New("resource Provider capability support level is invalid")
	}
}

// validateLogLevel applies machine-value bounds before membership.
func validateLogLevel(level LogLevel) error {
	if err := validateMachineValue("log level", string(level)); err != nil {
		return err
	}
	switch level {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
		return nil
	default:
		return gerror.New("resource Provider log level is invalid")
	}
}

// validateObservationTime verifies intrinsic timestamp sanity when no owner
// clock is available. Freshness-sensitive values must use validateObservationTimeAt.
func validateObservationTime(field string, value time.Time) error {
	if value.IsZero() {
		return gerror.Newf("resource Provider %s is required", field)
	}
	if value.Year() < minimumObservationYear || value.Year() > maximumObservationYear {
		return gerror.Newf("resource Provider %s is outside the trusted time range", field)
	}
	return nil
}

// validateObservationTimeAt verifies one observation against an explicit
// caller-owned clock and inclusive future-skew and age boundaries.
func validateObservationTimeAt(
	field string,
	value time.Time,
	now time.Time,
	maximumAge time.Duration,
) error {
	if err := validateObservationTime(field, value); err != nil {
		return err
	}
	if err := validateObservationTime("validation clock", now); err != nil {
		return err
	}
	if maximumAge <= 0 {
		return gerror.New("resource Provider observation maximum age is invalid")
	}
	if value.After(now.Add(maximumObservationFutureSkew)) {
		return gerror.Newf("resource Provider %s exceeds allowed future clock skew", field)
	}
	if value.Before(now.Add(-maximumAge)) {
		return gerror.Newf("resource Provider %s is stale", field)
	}
	return nil
}
