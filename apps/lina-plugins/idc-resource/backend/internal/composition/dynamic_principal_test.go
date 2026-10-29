// This file checks principal correlations against frozen guest wire contracts,
// while credentials remain in owner-only secret bindings.

package composition

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

func TestDynamicPrincipalProjectsStableCorrelationForFrozenGuests(t *testing.T) {
	const taskNo = "prt_69f153f7e109152720c6bc7221325356"
	for _, creating := range []bool{false, true} {
		name := "lookup"
		if creating {
			name = "create"
		}
		t.Run(name, func(t *testing.T) {
			username, err := secretcap.NewHandle("sec_principal_username")
			if err != nil {
				t.Fatal(err)
			}
			password, err := secretcap.NewHandle("sec_principal_password")
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			evidence := []resourcecap.Evidence{{Kind: "upstream_principal", Reference: "task_69f153f7e109152720c6bc7221325356", Summary: "exact principal observed", ObservedAt: now}}
			var proof any = resourcecap.PrincipalLookupResult{Exists: false, ObservedAt: now, Evidence: evidence}
			if creating {
				proof = resourcecap.PrincipalCreateResult{ExternalPrincipalRef: "42", ObservedAt: now, Evidence: evidence}
			}
			raw, err := json.Marshal(proof)
			if err != nil {
				t.Fatal(err)
			}
			registrar := &verificationClockRegistrar{proof: raw}
			var bindings []resourcecap.SecretBinding
			provider := &dynamicProvider{
				extensions:    registrar,
				configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Settings: json.RawMessage(`{}`)}},
				outbound: func(_ context.Context, _ channelconfig.Configuration, _ string, secrets []resourcecap.SecretBinding, _ ...func(resourcecap.Evidence)) (pluginhost.ConfidentialOutboundHandler, func(), error) {
					bindings = append([]resourcecap.SecretBinding(nil), secrets...)
					return func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
						t.Fatal("wire projection test must not call a supplier")
						return nil, nil
					}, func() {}, nil
				},
			}
			if creating {
				_, err = provider.CreatePrincipal(t.Context(), resourcecap.PrincipalCreateRequest{RequestID: taskNo, SecretBindings: []resourcecap.SecretBinding{{Name: resourcecap.PrincipalBindingUsername, Handle: username}, {Name: resourcecap.PrincipalBindingPassword, Handle: password}}})
			} else {
				_, err = provider.LookupPrincipal(t.Context(), resourcecap.PrincipalLookupRequest{RequestID: taskNo, Username: username})
			}
			if err != nil {
				t.Fatal(err)
			}
			var envelope resourcecap.DynamicProviderInvocationEnvelope
			if err = json.Unmarshal(registrar.invocation.Payload, &envelope); err != nil {
				t.Fatal(err)
			}
			var request resourcecap.DynamicPrincipalRequest
			if err = json.Unmarshal(envelope.Payload, &request); err != nil {
				t.Fatal(err)
			}
			if request.RequestID != "task_69f153f7e109152720c6bc7221325356" || request.Validate(creating) != nil {
				t.Fatal("frozen guest did not receive the established opaque task format")
			}
			expectedBindings := 1
			if creating {
				expectedBindings = 2
			}
			if len(bindings) != len(request.ConfidentialInputNames) || len(bindings) != expectedBindings || bindings[0].Handle != username || bindings[0].Name != resourcecap.PrincipalBindingUsername {
				t.Fatal("owner-only secret bindings changed")
			}
			if creating && (bindings[1].Handle != password || bindings[1].Name != resourcecap.PrincipalBindingPassword) {
				t.Fatal("principal password left its exact owner slot")
			}
			if strings.Contains(string(registrar.invocation.Payload), "sec_principal_") {
				t.Fatal("opaque secret handle leaked into the guest payload")
			}
		})
	}
}

func TestPrincipalCorrelationPreservesExistingNonOwnerFormats(t *testing.T) {
	for _, reference := range []string{"principal-request-1", "task_69f153f7e109152720c6bc7221325356", "prt_short", "prt_69F153F7e109152720c6bc7221325356"} {
		if principalInvocationReference(reference) != reference {
			t.Fatal("unrelated correlation was rewritten")
		}
	}
}
