// This file locks down the minimal public projection and fail-closed constructor boundary.

package secret

import (
	"bytes"
	"context"
	"reflect"
	"testing"
	"time"
)

// TestMetadataProjectionRemainsMinimal prevents future callers from receiving
// handles, versions, key identifiers, ciphertext, or plaintext-derived proof.
func TestMetadataProjectionRemainsMinimal(t *testing.T) {
	metadataType := reflect.TypeOf(Metadata{})
	want := []string{"Configured", "MaskedTail", "UpdatedAt"}
	if metadataType.NumField() != len(want) {
		t.Fatalf("metadata projection gained fields: %+v", metadataType)
	}
	for index, fieldName := range want {
		if metadataType.Field(index).Name != fieldName {
			t.Fatalf("metadata field %d changed: got=%s want=%s", index, metadataType.Field(index).Name, fieldName)
		}
	}
}

// TestConstructorRequiresSecurityDependencies proves no permissive key or
// authorization fallback can be activated by omission.
func TestConstructorRequiresSecurityDependencies(t *testing.T) {
	keys := fixedTestKeyProvider{}
	authorizer := allowTestAuthorizer{}
	if _, err := newService(Config{}, nil, authorizer, time.Now, bytes.NewReader(make([]byte, 64))); err == nil {
		t.Fatal("constructor accepted a missing key provider")
	}
	if _, err := newService(Config{}, keys, nil, time.Now, bytes.NewReader(make([]byte, 64))); err == nil {
		t.Fatal("constructor accepted a missing authorizer")
	}
	if _, err := newService(Config{}, keys, authorizer, nil, bytes.NewReader(make([]byte, 64))); err == nil {
		t.Fatal("constructor accepted a missing clock")
	}
	if _, err := newService(Config{}, keys, authorizer, time.Now, nil); err == nil {
		t.Fatal("constructor accepted missing cryptographic randomness")
	}
	var typedNilKeys *pointerTestKeyProvider
	if _, err := newService(Config{}, typedNilKeys, authorizer, time.Now, bytes.NewReader(make([]byte, 64))); err == nil {
		t.Fatal("constructor accepted a typed-nil key provider")
	}
	var typedNilAuthorizer *pointerTestAuthorizer
	if _, err := newService(Config{}, keys, typedNilAuthorizer, time.Now, bytes.NewReader(make([]byte, 64))); err == nil {
		t.Fatal("constructor accepted a typed-nil authorizer")
	}
	var typedNilRandom *bytes.Reader
	if _, err := newService(Config{}, keys, authorizer, time.Now, typedNilRandom); err == nil {
		t.Fatal("constructor accepted typed-nil cryptographic randomness")
	}
}

// TestScopeValidationRejectsAmbiguousValues verifies tenant and caller routing
// cannot be widened through whitespace or ambiguous separator forms.
func TestScopeValidationRejectsAmbiguousValues(t *testing.T) {
	valid := Scope{TenantID: 0, OwnerPluginID: "provider-test", CallerScope: "provider.authentication"}
	if _, err := normalizeScope(valid); err != nil {
		t.Fatalf("valid scope rejected: %v", err)
	}
	invalid := []Scope{
		{TenantID: 1, OwnerPluginID: valid.OwnerPluginID, CallerScope: valid.CallerScope},
		{TenantID: 0, OwnerPluginID: "Linapro-IDC", CallerScope: valid.CallerScope},
		{TenantID: 0, OwnerPluginID: valid.OwnerPluginID, CallerScope: "provider"},
		{TenantID: 0, OwnerPluginID: valid.OwnerPluginID, CallerScope: "provider..authentication"},
		{TenantID: 0, OwnerPluginID: valid.OwnerPluginID, CallerScope: "provider.-authentication"},
	}
	for _, scope := range invalid {
		if _, err := normalizeScope(scope); err == nil {
			t.Fatalf("ambiguous scope unexpectedly accepted: %+v", scope)
		}
	}
}

type fixedTestKeyProvider struct{}

func (fixedTestKeyProvider) Active(context.Context) (KeyMaterial, error) {
	return KeyMaterial{ID: "test-key", Key: bytes.Repeat([]byte{0x44}, dekSize)}, nil
}

func (fixedTestKeyProvider) Resolve(context.Context, []string) (map[string][]byte, error) {
	return map[string][]byte{"test-key": bytes.Repeat([]byte{0x44}, dekSize)}, nil
}

type allowTestAuthorizer struct{}

func (allowTestAuthorizer) Authorize(context.Context, AuthorizationRequest) error {
	return nil
}

type pointerTestKeyProvider struct{}

func (*pointerTestKeyProvider) Active(context.Context) (KeyMaterial, error) {
	return KeyMaterial{}, nil
}

func (*pointerTestKeyProvider) Resolve(context.Context, []string) (map[string][]byte, error) {
	return nil, nil
}

type pointerTestAuthorizer struct{}

func (*pointerTestAuthorizer) Authorize(context.Context, AuthorizationRequest) error {
	return nil
}
