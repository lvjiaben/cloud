package composition

import (
	"context"
	"errors"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type captureSource struct {
	accessorScopes []secretcap.Scope
	managerScopes  []secretcap.Scope
	err            error
	value          secretcap.Accessor
	manager        secretcap.Manager
}

func (s *captureSource) Accessor(scope secretcap.Scope) (secretcap.Accessor, error) {
	s.accessorScopes = append(s.accessorScopes, scope)
	return s.value, s.err
}

func (s *captureSource) Manager(scope secretcap.Scope) (secretcap.Manager, error) {
	s.managerScopes = append(s.managerScopes, scope)
	return s.manager, s.err
}

func TestBindProviderSecretManagerUsesExactChannelScope(t *testing.T) {
	t.Parallel()
	manager := &inertManager{}
	source := &captureSource{manager: manager}
	bound, err := BindProviderSecretManager(source, "channel-1")
	if err != nil || bound != manager {
		t.Fatalf("BindProviderSecretManager() = %T, %v", bound, err)
	}
	if len(source.managerScopes) != 1 || source.managerScopes[0].OwnerPluginID != resourceOwnerPluginID ||
		source.managerScopes[0].CallerScope == "" || source.managerScopes[0].CallerScope == "channel-1" {
		t.Fatalf("manager scope = %+v", source.managerScopes)
	}
}

type inertAccessor struct{}

func (inertAccessor) Access(_ context.Context, _ secretcap.AccessRequest, _ secretcap.Consumer) error {
	return nil
}

type inertManager struct{}

func (inertManager) Create(context.Context, string, secretcap.PlaintextProducer) (secretcap.CreateResult, error) {
	return secretcap.CreateResult{}, nil
}
func (inertManager) RecoverCreate(context.Context, string) (secretcap.CreateResult, error) {
	return secretcap.CreateResult{}, nil
}
func (inertManager) Replace(context.Context, secretcap.Handle, time.Time, string, secretcap.PlaintextProducer) (secretcap.MutationResult, error) {
	return secretcap.MutationResult{}, nil
}
func (inertManager) Metadata(context.Context, secretcap.Handle) (secretcap.Metadata, error) {
	return secretcap.Metadata{}, nil
}
func (inertManager) Revoke(context.Context, secretcap.Handle, time.Time) (secretcap.Metadata, error) {
	return secretcap.Metadata{}, nil
}
func (inertManager) Rotate(context.Context, int) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, nil
}

func TestInjectProviderSecretsBindsExactChannelScope(t *testing.T) {
	source := &captureSource{value: inertAccessor{}, manager: inertManager{}}
	env, err := InjectProviderSecrets(source, "channel_a1", resourcecap.ProviderEnv{
		PluginID: "provider-plugin", ProviderID: "provider-one",
	})
	if err != nil || env.Secrets == nil || env.SecretSink == nil || len(source.accessorScopes) != 1 || len(source.managerScopes) != 1 {
		t.Fatalf("inject Provider secret capabilities: env=%+v accessor_scopes=%d manager_scopes=%d err=%v", env, len(source.accessorScopes), len(source.managerScopes), err)
	}
	first := source.accessorScopes[0]
	if first.TenantID != 0 || first.OwnerPluginID != resourceOwnerPluginID ||
		len(first.CallerScope) != len(channelScopePrefix)+32 {
		t.Fatalf("unexpected channel scope: %+v", first)
	}
	if _, err = InjectProviderSecrets(source, "channel_a1", resourcecap.ProviderEnv{}); err != nil {
		t.Fatalf("repeat same channel scope: %v", err)
	}
	if source.accessorScopes[1] != first || source.managerScopes[1] != first {
		t.Fatalf("same channel produced different scopes: first=%+v accessor=%+v manager=%+v", first, source.accessorScopes[1], source.managerScopes[1])
	}
	if _, err = InjectProviderSecrets(source, "channel_b2", resourcecap.ProviderEnv{}); err != nil {
		t.Fatalf("inject second channel: %v", err)
	}
	if source.accessorScopes[2].CallerScope == first.CallerScope || source.managerScopes[2].CallerScope == first.CallerScope {
		t.Fatal("different channels shared one secret caller scope")
	}
}

func TestInjectProviderSecretsFailsClosed(t *testing.T) {
	if _, err := InjectProviderSecrets(nil, "channel_a1", resourcecap.ProviderEnv{}); err == nil {
		t.Fatal("nil source accepted")
	}
	if _, err := InjectProviderSecrets(&captureSource{value: inertAccessor{}, manager: inertManager{}}, " bad-ref", resourcecap.ProviderEnv{}); err == nil {
		t.Fatal("invalid channel reference accepted")
	}
	if _, err := InjectProviderSecrets(&captureSource{err: errors.New("denied")}, "channel_a1", resourcecap.ProviderEnv{}); err == nil {
		t.Fatal("source binding error accepted")
	}
	if _, err := InjectProviderSecrets(&captureSource{manager: inertManager{}}, "channel_a1", resourcecap.ProviderEnv{}); err == nil {
		t.Fatal("nil accessor accepted")
	}
	if _, err := InjectProviderSecrets(&captureSource{value: inertAccessor{}}, "channel_a1", resourcecap.ProviderEnv{}); err == nil {
		t.Fatal("nil manager accepted")
	}
}
