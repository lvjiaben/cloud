package resourcecap

import (
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

func TestPrincipalContractsRequireOpaqueSecretsAndConsistentResults(t *testing.T) {
	t.Parallel()
	username, err := secretcap.NewHandle("sec_principal_username")
	if err != nil {
		t.Fatal(err)
	}
	password, err := secretcap.NewHandle("sec_principal_password")
	if err != nil {
		t.Fatal(err)
	}
	lookup := PrincipalLookupRequest{RequestID: "principal-request-1", Username: username}
	if err = lookup.Validate(); err != nil {
		t.Fatalf("valid lookup request rejected: %v", err)
	}
	create := PrincipalCreateRequest{
		RequestID: "principal-request-1",
		SecretBindings: []SecretBinding{
			{Name: PrincipalBindingUsername, Handle: username},
			{Name: PrincipalBindingPassword, Handle: password},
		},
	}
	if err = create.Validate(); err != nil {
		t.Fatalf("valid create request rejected: %v", err)
	}
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	evidence := []Evidence{{
		Kind: "principal_lookup", Reference: "principal-request-1",
		Summary: "exact username lookup completed", ObservedAt: now,
	}}
	for _, result := range []PrincipalLookupResult{
		{Exists: false, ObservedAt: now, Evidence: evidence},
		{Exists: true, ExternalPrincipalRef: "42", ObservedAt: now, Evidence: evidence},
	} {
		if err = result.Validate(); err != nil {
			t.Fatalf("valid lookup result rejected: %+v err=%v", result, err)
		}
	}
	invalidMissing := PrincipalLookupResult{
		Exists: false, ExternalPrincipalRef: "42", ObservedAt: now, Evidence: evidence,
	}
	if invalidMissing.Validate() == nil {
		t.Fatal("missing principal with an external reference was accepted")
	}
	if (PrincipalLookupRequest{RequestID: "principal-request-1"}).Validate() == nil {
		t.Fatal("zero username handle was accepted")
	}
}

func TestPrincipalCreationAcknowledgementRequiresEvidence(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	result := PrincipalCreateResult{
		ExternalPrincipalRef: "42",
		ObservedAt:           now,
		Evidence: []Evidence{{
			Kind: "principal_acceptance", Reference: "principal-request-1",
			Summary: "upstream identifier observed", ObservedAt: now,
		}},
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("valid creation acknowledgement rejected: %v", err)
	}
	result.Evidence = nil
	if result.Validate() == nil {
		t.Fatal("creation acknowledgement without evidence was accepted")
	}
}
