// This file verifies secret handles fail closed across serialization, logging, and compatibility checks.

package secretcap

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"reflect"
	"strings"
	"testing"
)

// Compile-time assertions keep the accidental-output defenses part of the
// public Handle contract.
var (
	_ fmt.Stringer             = Handle{}
	_ fmt.GoStringer           = Handle{}
	_ fmt.Formatter            = Handle{}
	_ slog.LogValuer           = Handle{}
	_ json.Marshaler           = Handle{}
	_ json.Unmarshaler         = (*Handle)(nil)
	_ encoding.TextMarshaler   = Handle{}
	_ encoding.TextUnmarshaler = (*Handle)(nil)
)

// TestHandleNeverEchoesThroughJSONOrLogs verifies the only field containing
// the opaque reference cannot accidentally reach common serialization paths.
func TestHandleNeverEchoesThroughJSONOrLogs(t *testing.T) {
	const reference = "sec_test_reference_that_must_not_escape"
	handle, err := NewHandle(reference)
	if err != nil {
		t.Fatalf("create handle: %v", err)
	}
	request, err := NewAccessRequest(handle, Purpose("provider.authentication"))
	if err != nil {
		t.Fatalf("create access request: %v", err)
	}

	for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%d"} {
		if output := fmt.Sprintf(format, handle); strings.Contains(output, reference) {
			t.Fatalf("format %q leaked handle reference: %q", format, output)
		}
	}
	if output := fmt.Sprintf("%+v", request); strings.Contains(output, reference) {
		t.Fatalf("access request formatting leaked handle reference: %q", output)
	}

	var logBuffer bytes.Buffer
	logger := log.New(&logBuffer, "", 0)
	logger.Printf("handle=%+v request=%+v", handle, request)
	if strings.Contains(logBuffer.String(), reference) {
		t.Fatalf("standard log leaked handle reference: %q", logBuffer.String())
	}

	logBuffer.Reset()
	slog.New(slog.NewTextHandler(&logBuffer, nil)).Info(
		"secret access",
		"handle", handle,
		"request", request,
	)
	if strings.Contains(logBuffer.String(), reference) {
		t.Fatalf("structured log leaked handle reference: %q", logBuffer.String())
	}

	if payload, marshalErr := json.Marshal(handle); marshalErr == nil {
		t.Fatalf("expected handle JSON encoding to fail, got %q", payload)
	} else if strings.Contains(marshalErr.Error(), reference) {
		t.Fatalf("JSON error leaked handle reference: %v", marshalErr)
	}
	if payload, marshalErr := json.Marshal(request); marshalErr == nil {
		t.Fatalf("expected access request JSON encoding to fail, got %q", payload)
	} else if strings.Contains(marshalErr.Error(), reference) {
		t.Fatalf("request JSON error leaked handle reference: %v", marshalErr)
	}
	if payload, marshalErr := handle.MarshalText(); marshalErr == nil {
		t.Fatalf("expected text encoding to fail, got %q", payload)
	}
}

// TestPurposeCodesRejectFreeTextAndCredentialShapes verifies audit metadata is
// a bounded, low-entropy code rather than a credential-bearing text channel.
func TestPurposeCodesRejectFreeTextAndCredentialShapes(t *testing.T) {
	valid := []string{
		"provider.authentication",
		"payment.callback-signature",
		"compliance.identity-query",
		"ai.model-call",
	}
	for _, value := range valid {
		if _, err := NewPurpose(value); err != nil {
			t.Fatalf("valid purpose %q was rejected: %v", value, err)
		}
	}
	invalid := []string{
		"",
		"authentication",
		"Provider.authentication",
		"provider.Authentication",
		"provider..authentication",
		"provider.-authentication",
		"provider.authentication-",
		"provider.auth--refresh",
		"Authorization: Bearer plaintext",
		"api_key=plaintext",
		"unknown.authentication",
		"provider." + strings.Repeat("a", maxPurposeSegmentBytes+1),
		"provider." + strings.Repeat("a", maxPurposeBytes),
	}
	for _, value := range invalid {
		if _, err := NewPurpose(value); err == nil {
			t.Fatalf("invalid purpose %q was accepted", value)
		}
	}
}

// TestHandleHasNoExportedStorage proves reflection-based encoders cannot find
// an exported reference or future plaintext field to echo.
func TestHandleHasNoExportedStorage(t *testing.T) {
	handleType := reflect.TypeOf(Handle{})
	for index := 0; index < handleType.NumField(); index++ {
		field := handleType.Field(index)
		if field.IsExported() {
			t.Fatalf("secret handle field %q must not be exported", field.Name)
		}
	}
}

// TestAccessRequestExposesOnlySafeIntent verifies a bridge payload can carry a
// purpose without serializing the governed handle reference.
func TestAccessRequestExposesOnlySafeIntent(t *testing.T) {
	handle, err := NewHandle("sec_safe_intent")
	if err != nil {
		t.Fatalf("create handle: %v", err)
	}
	request, err := NewAccessRequest(handle, Purpose("provider.authentication"))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	payload, err := json.Marshal(request.Intent())
	if err != nil {
		t.Fatalf("marshal safe intent: %v", err)
	}
	if strings.Contains(string(payload), handle.OpaqueRef()) {
		t.Fatalf("safe intent leaked governed handle reference: %s", payload)
	}
}

// TestDependencyCompatibilityFailsClosed verifies identity, version, and
// method mismatches never fall back to the current contract.
func TestDependencyCompatibilityFailsClosed(t *testing.T) {
	valid, err := NewDependency()
	if err != nil {
		t.Fatalf("create dependency: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*Dependency)
	}{
		{name: "owner", mutate: func(value *Dependency) { value.OwnerPluginID = "other-owner" }},
		{name: "service", mutate: func(value *Dependency) { value.Service = "other-service" }},
		{name: "version", mutate: func(value *Dependency) { value.Version = "v2" }},
		{name: "missing methods", mutate: func(value *Dependency) { value.Methods = nil }},
		{name: "unknown method", mutate: func(value *Dependency) { value.Methods = []Method{"unknown"} }},
		{name: "duplicate method", mutate: func(value *Dependency) {
			value.Methods = []Method{MethodSecretAccess, MethodSecretAccess}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.Methods = append([]Method(nil), valid.Methods...)
			test.mutate(&candidate)
			if err := ValidateDependency(candidate); err == nil {
				t.Fatalf("expected incompatible %s dependency to fail", test.name)
			}
		})
	}
}

// TestDependencyValidationBoundsAndErrorsDoNotEchoInput verifies untrusted
// dependency metadata is bounded before processing and never reflected in errors.
func TestDependencyValidationBoundsAndErrorsDoNotEchoInput(t *testing.T) {
	var (
		overlongOwner   = "owner-private-" + strings.Repeat("x", maximumDependencyOwnerPluginIDBytes+1)
		overlongService = "service-private-" + strings.Repeat("x", maximumDependencyServiceBytes+1)
		overlongVersion = Version("version-private-" + strings.Repeat("x", maximumDependencyVersionBytes+1))
		overlongMethod  = Method("method-private-" + strings.Repeat("x", maximumDependencyMethodBytes+1))
		tooManyMethods  = make([]Method, maximumDependencyMethodCount+1)
	)
	for index := range tooManyMethods {
		tooManyMethods[index] = MethodSecretAccess
	}
	tooManyMethods[0] = "count-private-method"

	tests := []struct {
		name       string
		dependency Dependency
		private    string
	}{
		{
			name: "overlong owner",
			dependency: Dependency{
				OwnerPluginID: overlongOwner,
				Service:       ServiceName,
				Version:       ContractVersion,
				Methods:       []Method{MethodSecretAccess},
			},
			private: overlongOwner,
		},
		{
			name: "overlong service",
			dependency: Dependency{
				OwnerPluginID: OwnerPluginID,
				Service:       overlongService,
				Version:       ContractVersion,
				Methods:       []Method{MethodSecretAccess},
			},
			private: overlongService,
		},
		{
			name: "overlong version",
			dependency: Dependency{
				OwnerPluginID: OwnerPluginID,
				Service:       ServiceName,
				Version:       overlongVersion,
				Methods:       []Method{MethodSecretAccess},
			},
			private: string(overlongVersion),
		},
		{
			name: "too many methods",
			dependency: Dependency{
				OwnerPluginID: OwnerPluginID,
				Service:       ServiceName,
				Version:       ContractVersion,
				Methods:       tooManyMethods,
			},
			private: string(tooManyMethods[0]),
		},
		{
			name: "overlong method",
			dependency: Dependency{
				OwnerPluginID: OwnerPluginID,
				Service:       ServiceName,
				Version:       ContractVersion,
				Methods:       []Method{overlongMethod},
			},
			private: string(overlongMethod),
		},
		{
			name: "mismatched owner",
			dependency: Dependency{
				OwnerPluginID: "private-owner-mismatch",
				Service:       ServiceName,
				Version:       ContractVersion,
				Methods:       []Method{MethodSecretAccess},
			},
			private: "private-owner-mismatch",
		},
		{
			name: "mismatched service",
			dependency: Dependency{
				OwnerPluginID: OwnerPluginID,
				Service:       "private-service-mismatch",
				Version:       ContractVersion,
				Methods:       []Method{MethodSecretAccess},
			},
			private: "private-service-mismatch",
		},
		{
			name: "mismatched version",
			dependency: Dependency{
				OwnerPluginID: OwnerPluginID,
				Service:       ServiceName,
				Version:       "private-version-mismatch",
				Methods:       []Method{MethodSecretAccess},
			},
			private: "private-version-mismatch",
		},
		{
			name: "unsupported method",
			dependency: Dependency{
				OwnerPluginID: OwnerPluginID,
				Service:       ServiceName,
				Version:       ContractVersion,
				Methods:       []Method{"private-method-mismatch"},
			},
			private: "private-method-mismatch",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateDependency(test.dependency)
			if err == nil {
				t.Fatal("expected dependency validation to fail")
			}
			if strings.Contains(err.Error(), test.private) {
				t.Fatalf("validation error echoed untrusted input: %v", err)
			}
		})
	}
}
