// This file provides bounded, secret-aware validation primitives shared by the Provider contract.

package resourcecap

import (
	"encoding/json"
	"math"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-resource/backend/cap/resourcecap/internal/credentialscan"
)

const (
	// MaximumProviderCapacityPageSize bounds one in-memory Provider providerCapacity page.
	MaximumProviderCapacityPageSize = 500
	// MaximumEvidenceCount bounds evidence attached to any one Provider value.
	MaximumEvidenceCount = 32
	// MaximumSecretBindingCount bounds the credentials delegated to one Provider instance.
	MaximumSecretBindingCount = 32
	// MaximumSecretOutputCount bounds confidential handles returned by one operation.
	MaximumSecretOutputCount = 16
	// MaximumResidualResourceCount bounds unreclaimed resources returned by one operation.
	MaximumResidualResourceCount = 16
	// MaximumSecretPlaintextBytes bounds one synchronous confidential output.
	MaximumSecretPlaintextBytes = 1 << 20
	// MaximumCapabilityCount is the complete normalized capability matrix size.
	MaximumCapabilityCount = 29
	// MaximumLogFieldCount bounds one Provider diagnostic event.
	MaximumLogFieldCount = 32
	// MaximumMetricLabelCount bounds one Provider metric dimension set.
	MaximumMetricLabelCount = 16

	// maximumChannelRefBytes bounds one resource-owner channel reference.
	maximumChannelRefBytes = 256
	// maximumEndpointBytes bounds one configured upstream endpoint.
	maximumEndpointBytes = 2048
	// maximumPageJSONBytes bounds all normalized JSON items in one page.
	maximumPageJSONBytes = 16 << 20
	// maximumReferenceBytes bounds an opaque non-secret external reference.
	maximumReferenceBytes = 512
	// maximumCursorBytes bounds one opaque providerCapacity cursor.
	maximumCursorBytes = 1024
	// maximumNameBytes bounds canonical machine names.
	maximumNameBytes = 128
	// maximumCodeBytes bounds normalized failure codes.
	maximumCodeBytes = 128
	// maximumMessageBytes bounds one diagnostic message or allowed value.
	maximumMessageBytes = 2048
	// maximumEvidenceSummaryBytes bounds one redacted evidence summary.
	maximumEvidenceSummaryBytes = 4096
	// maximumCapabilityReasonBytes bounds one unsupported-capability reason.
	maximumCapabilityReasonBytes = 1024
	// maximumConstraintCount bounds constraints on one capability.
	maximumConstraintCount = 64
	// maximumAllowedValueCount bounds values on one constraint.
	maximumAllowedValueCount = 256
	// maximumPatternBytes bounds one RE2-compatible constraint expression.
	maximumPatternBytes = 512
	// maximumAdapterVersionBytes bounds an adapter implementation version.
	maximumAdapterVersionBytes = 128
	// maximumMetricValueBytes bounds one metric label value.
	maximumMetricValueBytes = 256
	// maximumMachineValueBytes bounds all typed machine enum inputs.
	maximumMachineValueBytes = 128
	// maximumCapabilityNodes bounds a complete capability graph.
	maximumCapabilityNodes = 4096
	// maximumCapabilityBytes bounds strings in a complete capability graph.
	maximumCapabilityBytes = 1 << 20
)

// safeNamePattern accepts only stable ASCII machine names used at public
// contract boundaries.
var safeNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]*$`)

// ValidateChannelRef validates one resource-owner channel reference without
// requiring an otherwise complete ProviderConfig. Startup composition uses it
// before deriving channel-scoped capabilities.
func ValidateChannelRef(value string) error {
	// Channel references are host-generated opaque identifiers. Their random
	// suffix can legitimately resemble a credential, so validate the canonical
	// machine-name shape without running content-oriented credential detection.
	return validateSafeName("channel reference", value, maximumChannelRefBytes)
}

// validateRequiredText validates one required bounded UTF-8 text value.
func validateRequiredText(field string, value string, maximum int) error {
	if value == "" || len(value) > maximum {
		return gerror.Newf("resource Provider %s is invalid", field)
	}
	if value != strings.TrimSpace(value) || !utf8.ValidString(value) ||
		strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return gerror.Newf("resource Provider %s is invalid", field)
	}
	return nil
}

// validateOptionalText validates one optional bounded UTF-8 text value.
func validateOptionalText(field string, value string, maximum int) error {
	if value == "" {
		return nil
	}
	return validateRequiredText(field, value, maximum)
}

// validateSafeName validates one canonical ASCII machine name.
func validateSafeName(field string, value string, maximum int) error {
	if err := validateRequiredText(field, value, maximum); err != nil {
		return err
	}
	if !safeNamePattern.MatchString(value) {
		return gerror.Newf("resource Provider %s must use a canonical machine name", field)
	}
	return nil
}

// validateSafeDiagnostic validates bounded diagnostic text and rejects common
// credential representations before the text can be persisted or logged.
func validateSafeDiagnostic(field string, value string, maximum int, required bool) error {
	var err error
	if required {
		err = validateRequiredText(field, value, maximum)
	} else {
		err = validateOptionalText(field, value, maximum)
	}
	if err != nil {
		return err
	}
	if value != "" && containsCredentialMaterial(value) {
		return gerror.Newf("resource Provider %s contains credential-like material", field)
	}
	return nil
}

// validateMachineValue validates a required bounded machine-controlled value
// before its caller checks membership in a typed enum.
func validateMachineValue(field string, value string) error {
	return validateSafeDiagnostic(field, value, maximumMachineValueBytes, true)
}

// validateRequiredReference validates one required opaque non-secret reference.
func validateRequiredReference(field string, value string) error {
	return validateSafeDiagnostic(field, value, maximumReferenceBytes, true)
}

// validateOptionalReference validates one optional opaque non-secret reference.
func validateOptionalReference(field string, value string) error {
	return validateSafeDiagnostic(field, value, maximumReferenceBytes, false)
}

// validateJSONObject delegates bounded streaming payload inspection to the
// internal credential scanner.
func validateJSONObject(field string, payload json.RawMessage, required bool) error {
	return credentialscan.ValidateJSONObject(field, payload, required)
}

// sensitiveDiagnosticKey delegates diagnostic-key classification to the bounded internal scanner.
func sensitiveDiagnosticKey(key string) bool {
	return credentialscan.SensitiveDiagnosticKey(key)
}

// containsCredentialMaterial delegates all normalization and decoding to the
// bounded internal scanner shared by every public validation path.
func containsCredentialMaterial(value string) bool {
	return credentialscan.Contains(value)
}

// validateEndpoint validates the static endpoint shape. Runtime transport
// policy remains responsible for DNS, certificate, redirect, and network
// destination checks.
func validateEndpoint(endpoint string) error {
	if err := validateSafeDiagnostic("endpoint", endpoint, maximumEndpointBytes, true); err != nil {
		return err
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return gerror.New("resource Provider endpoint is invalid")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return gerror.New("resource Provider endpoint scheme must be http or https")
	}
	if parsed.Hostname() == "" || parsed.Opaque != "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" || parsed.ForceQuery {
		return gerror.New(
			"resource Provider endpoint must have a host and must not contain userinfo, a query, or a fragment",
		)
	}
	if endpointPathContainsCredential(parsed.EscapedPath()) || endpointPathContainsCredential(parsed.Path) {
		return gerror.New("resource Provider endpoint path contains credential-like material")
	}
	return nil
}

// endpointPathContainsCredential applies the shared stable percent-decoding
// credential scan to an endpoint path.
func endpointPathContainsCredential(path string) bool {
	return containsCredentialMaterial(path)
}

// isNilInterface detects nil and typed-nil interface dependencies.
func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	typed := reflect.ValueOf(value)
	switch typed.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return typed.IsNil()
	default:
		return false
	}
}

// validMetricValue reports whether one observation is finite.
func validMetricValue(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
