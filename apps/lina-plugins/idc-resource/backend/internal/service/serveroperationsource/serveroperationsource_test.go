package serveroperationsource

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

type inputSecretSourceStub struct{ manager *inputSecretManagerStub }

func (s inputSecretSourceStub) Accessor(secretcap.Scope) (secretcap.Accessor, error) {
	return nil, errors.New("unused")
}
func (s inputSecretSourceStub) Manager(secretcap.Scope) (secretcap.Manager, error) {
	return s.manager, nil
}

type inputSecretManagerStub struct {
	handle  secretcap.Handle
	updated time.Time
	revoked bool
}

func (m *inputSecretManagerStub) Create(_ context.Context, _ string, produce secretcap.PlaintextProducer) (secretcap.CreateResult, error) {
	if err := produce(func(value []byte) error {
		if string(value) != "new-password" {
			return errors.New("wrong plaintext")
		}
		return nil
	}); err != nil {
		return secretcap.CreateResult{}, err
	}
	return secretcap.CreateResult{Handle: m.handle, Metadata: secretcap.Metadata{Configured: true, UpdatedAt: m.updated}}, nil
}
func (*inputSecretManagerStub) RecoverCreate(context.Context, string) (secretcap.CreateResult, error) {
	return secretcap.CreateResult{}, errors.New("unused")
}
func (*inputSecretManagerStub) Replace(context.Context, secretcap.Handle, time.Time, string, secretcap.PlaintextProducer) (secretcap.MutationResult, error) {
	return secretcap.MutationResult{}, errors.New("unused")
}
func (m *inputSecretManagerStub) Metadata(context.Context, secretcap.Handle) (secretcap.Metadata, error) {
	return secretcap.Metadata{Configured: true, UpdatedAt: m.updated}, nil
}
func (m *inputSecretManagerStub) Revoke(context.Context, secretcap.Handle, time.Time) (secretcap.Metadata, error) {
	m.revoked = true
	return secretcap.Metadata{}, nil
}
func (*inputSecretManagerStub) Rotate(context.Context, int) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, errors.New("unused")
}

func TestEveryNormalServerActionMapsToExecutableCapability(t *testing.T) {
	actions := []serveroperationresourcecap.Action{serveroperationresourcecap.ActionStart, serveroperationresourcecap.ActionStop, serveroperationresourcecap.ActionReboot, serveroperationresourcecap.ActionResetPassword, serveroperationresourcecap.ActionConsole, serveroperationresourcecap.ActionReinstall, serveroperationresourcecap.ActionResize, serveroperationresourcecap.ActionChangeIP}
	for _, action := range actions {
		if capability, ok := capabilityFor(action); !ok || capability == "" {
			t.Fatalf("action %q has no capability", action)
		}
	}
}

func TestProjectStatusDoesNotGuessUnknownResult(t *testing.T) {
	status, code := projectStatus(&taskRow{StatusCode: string(resourcetask.StatusResultUnknown), FailureCode: "UPSTREAM_UNKNOWN"})
	if status != serveroperationresourcecap.TaskStatusResultUnknown || code != "upstream_unknown" {
		t.Fatalf("unexpected projection %q %q", status, code)
	}
	status, code = projectStatus(&taskRow{StatusCode: string(resourcetask.StatusSucceeded)})
	if status != serveroperationresourcecap.TaskStatusSucceeded || code != "" {
		t.Fatalf("unexpected success projection %q %q", status, code)
	}
	_ = resourcecap.CapabilityStart
}

func TestCapabilityParametersFollowFrozenProviderConstraints(t *testing.T) {
	constraints := `[{"name":"mode","required":false,"allowedValues":["graceful","hard"]}]`
	if err := validateCapabilityParameters("conditional", constraints, json.RawMessage(`{"mode":"hard"}`)); err != nil {
		t.Fatal(err)
	}
	for _, parameters := range []json.RawMessage{json.RawMessage(`{"mode":"force"}`), json.RawMessage(`{"mode":"hard","force":true}`)} {
		if err := validateCapabilityParameters("conditional", constraints, parameters); !errors.Is(err, serveroperationresourcecap.ErrTaskConflict) {
			t.Fatalf("unsafe parameters %s returned %v", parameters, err)
		}
	}
	numeric := `[{"name":"ip_num","required":true,"minimum":1,"maximum":64}]`
	if err := validateCapabilityParameters("conditional", numeric, json.RawMessage(`{"ip_num":0}`)); !errors.Is(err, serveroperationresourcecap.ErrTaskConflict) {
		t.Fatalf("out-of-range IP count returned %v", err)
	}
	if err := validateCapabilityParameters("supported", `[]`, json.RawMessage(`{"force":true}`)); !errors.Is(err, serveroperationresourcecap.ErrTaskConflict) {
		t.Fatalf("unknown field on parameterless action returned %v", err)
	}
}

func TestInputPasswordIsCreatedAndRevokedInsideResourceScope(t *testing.T) {
	handle, err := secretcap.NewHandle("secret-server-password")
	if err != nil {
		t.Fatal(err)
	}
	manager := &inputSecretManagerStub{handle: handle, updated: time.Now().UTC()}
	service := &Service{secrets: inputSecretSourceStub{manager: manager}}
	prepared, err := service.PrepareInputSecret(context.Background(), serveroperationresourcecap.InputSecretRequest{
		ChannelNo: "channel-1", IdempotencyKey: "password-request-0001", BindingName: "password",
		Produce: func(consume secretcap.Consumer) error { return consume([]byte("new-password")) },
	})
	if err != nil || prepared == nil || !prepared.Handle.Equal(handle) {
		t.Fatalf("prepared=%+v err=%v", prepared, err)
	}
	if err = service.RevokeInputSecret(context.Background(), "channel-1", handle); err != nil || !manager.revoked {
		t.Fatalf("revoke err=%v revoked=%v", err, manager.revoked)
	}
}

func TestDynamicServerActionRequiresFrozenConfigurationRevision(t *testing.T) {
	for _, revision := range []string{"", "rci_short", "rci_11111111111111111111111111111111/other"} {
		if validRouteConfiguration("dynamic", revision) {
			t.Fatal("dynamic operation accepted missing or invalid revision")
		}
	}
	if !validRouteConfiguration("dynamic", "rci_11111111111111111111111111111111") {
		t.Fatal("valid frozen operation revision rejected")
	}
	if !validRouteConfiguration("source", "") {
		t.Fatal("source fixture required dynamic interface configuration")
	}
}
