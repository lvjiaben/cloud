// This file verifies Provider declaration identifiers and contract-version semantics.

package resourcecap

import (
	"strings"
	"testing"
)

// TestParseContractVersion verifies parsing and the owner's exact current
// compatibility policy.
func TestParseContractVersion(t *testing.T) {
	tests := []struct {
		value string
		want  ParsedContractVersion
	}{
		{value: "v1", want: ParsedContractVersion{Major: 1}},
		{value: "v1.2", want: ParsedContractVersion{Major: 1, Minor: 2}},
		{value: "v12.34.56", want: ParsedContractVersion{Major: 12, Minor: 34, Patch: 56}},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, err := ParseContractVersion(test.value)
			if err != nil {
				t.Fatalf("ParseContractVersion(%q): %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("ParseContractVersion(%q)=%+v, want %+v", test.value, got, test.want)
			}
		})
	}

	v1, err := ParseContractVersion("v1")
	if err != nil {
		t.Fatalf("parse v1: %v", err)
	}
	v1Minor, err := ParseContractVersion("v1.9.4")
	if err != nil {
		t.Fatalf("parse v1.9.4: %v", err)
	}
	v2, err := ParseContractVersion("v2")
	if err != nil {
		t.Fatalf("parse v2: %v", err)
	}
	if !v1.CompatibleWith(v1) {
		t.Fatal("expected the exact contract version to be compatible")
	}
	if v1.CompatibleWith(v1Minor) {
		t.Fatal("expected an unapproved same-major minor version to be incompatible")
	}
	if v1.CompatibleWith(v2) {
		t.Fatal("expected different major versions to be incompatible")
	}
}

// TestParseContractVersionRejectsInvalidValues verifies malformed and aliased
// versions cannot create ambiguous declaration keys.
func TestParseContractVersionRejectsInvalidValues(t *testing.T) {
	invalid := []string{
		"",
		"1",
		"v",
		"v0",
		"v01",
		"v1.02",
		"v1.2.03",
		"v1.",
		"v1..2",
		"v1.2.3.4",
		"v1.0",
		"v1.0.0",
		"v1.2.0",
		"v1.a",
		" v1",
		"v1 ",
		"v1.token=must-not-pass",
		strings.Repeat("v1", maxContractVersionLength),
	}
	for _, value := range invalid {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseContractVersion(value); err == nil {
				t.Fatalf("expected %q to be rejected", value)
			}
		})
	}
}

// TestParsedContractVersionStringHasOneCanonicalForm verifies semantic aliases
// collapse to the only declaration-key representation.
func TestParsedContractVersionStringHasOneCanonicalForm(t *testing.T) {
	tests := []struct {
		version ParsedContractVersion
		want    string
	}{
		{version: ParsedContractVersion{Major: 1}, want: "v1"},
		{version: ParsedContractVersion{Major: 1, Minor: 2}, want: "v1.2"},
		{version: ParsedContractVersion{Major: 1, Minor: 2, Patch: 3}, want: "v1.2.3"},
		{version: ParsedContractVersion{}, want: ""},
	}
	for _, test := range tests {
		if got := test.version.String(); got != test.want {
			t.Fatalf("version %+v canonical form=%q, want %q", test.version, got, test.want)
		}
	}
}

// TestProviderKeyValidateRejectsInvalidIdentifiers verifies declaration identity
// stays aligned with the host's kebab-case runtime boundary.
func TestProviderKeyValidateRejectsInvalidIdentifiers(t *testing.T) {
	valid := []ProviderKey{
		{PluginID: "idc-zjmfcloud", ProviderID: "zjmfcloud", ContractVersion: "v1"},
		{PluginID: "1vendor-provider", ProviderID: "provider-2", ContractVersion: "v1.3"},
	}
	for _, key := range valid {
		if err := key.Validate(); err != nil {
			t.Fatalf("valid key %+v was rejected: %v", key, err)
		}
	}

	invalidIDs := []string{
		"",
		"Uppercase",
		"with_space",
		"with space",
		"-leading",
		"trailing-",
		"double--hyphen",
		strings.Repeat("a", maxIdentifierLength+1),
	}
	for _, value := range invalidIDs {
		t.Run("plugin_"+value, func(t *testing.T) {
			key := ProviderKey{PluginID: value, ProviderID: "provider", ContractVersion: "v1"}
			if err := key.Validate(); err == nil {
				t.Fatalf("expected plugin ID %q to be rejected", value)
			}
		})
		t.Run("provider_"+value, func(t *testing.T) {
			key := ProviderKey{PluginID: "source-plugin", ProviderID: value, ContractVersion: "v1"}
			if err := key.Validate(); err == nil {
				t.Fatalf("expected Provider ID %q to be rejected", value)
			}
		})
	}

	if err := (ProviderKey{
		PluginID:        "source-plugin",
		ProviderID:      "provider",
		ContractVersion: "v1-beta",
	}).Validate(); err == nil {
		t.Fatal("expected malformed contract version to be rejected")
	}
}
