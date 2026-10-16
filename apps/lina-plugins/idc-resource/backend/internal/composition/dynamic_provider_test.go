// These tests keep connection proof freshness in the trusted resource owner,
// including guests whose WASI wall clock is a deterministic epoch clock.
package composition

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

// verificationClockRegistrar emulates the isolated guest's authentication call
// and independent returned proof without granting the guest a real wall clock.
type verificationClockRegistrar struct {
	dynamicRegistrarFake
	step  *resourcecap.ConfidentialHTTPRequest
	proof json.RawMessage
}

func (f *verificationClockRegistrar) InvokeProviderMethod(ctx context.Context, input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	f.invocation = input
	if f.step != nil {
		payload, err := json.Marshal(f.step)
		if err != nil {
			return nil, err
		}
		if _, err = f.registration.Handler(ctx, &pluginhost.ConfidentialOutboundRequest{Payload: payload}); err != nil {
			return nil, err
		}
	}
	return &capmodel.BusinessExtensionMethodResult{Payload: f.proof}, nil
}

func TestDynamicVerificationUsesOwnerAuthenticationClock(t *testing.T) {
	for _, tc := range []struct {
		name          string
		kind          string
		callback      bool
		authenticated bool
		status        int
		verified      bool
		accepted      bool
	}{
		{"epoch guest accepted after owner authentication", resourcecap.ConfidentialHTTPKindAuthenticate, true, true, 200, true, true},
		{"guest cannot invent owner authentication", resourcecap.ConfidentialHTTPKindAuthenticate, false, true, 200, true, false},
		{"false proof cannot be promoted", resourcecap.ConfidentialHTTPKindAuthenticate, true, true, 200, false, false},
		{"owner did not authenticate", resourcecap.ConfidentialHTTPKindAuthenticate, true, false, 200, true, false},
		{"business request is not authentication", resourcecap.ConfidentialHTTPKindRequest, true, true, 200, true, false},
		{"unsuccessful status is not authentication", resourcecap.ConfidentialHTTPKindAuthenticate, true, true, 401, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(resourcecap.ConnectionVerification{Verified: tc.verified, ObservedAt: time.Unix(0, 0).UTC()})
			if err != nil {
				t.Fatal(err)
			}
			registrar := &verificationClockRegistrar{proof: payload}
			if tc.callback {
				registrar.step = &resourcecap.ConfidentialHTTPRequest{Kind: tc.kind}
			}
			p := &dynamicProvider{
				extensions:    registrar,
				configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Settings: []byte("{}")}},
				outbound: func(context.Context, channelconfig.Configuration, string, []resourcecap.SecretBinding) (pluginhost.ConfidentialOutboundHandler, func(), error) {
					return func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
						response, marshalErr := json.Marshal(resourcecap.ConfidentialHTTPResponse{StatusCode: tc.status, Authenticated: tc.authenticated})
						return &pluginhost.ConfidentialOutboundResult{Payload: response}, marshalErr
					}, func() {}, nil
				},
			}
			startedAt := time.Now().UTC()
			result, err := p.VerifyConnection(context.Background())
			if !tc.accepted {
				if !bizerr.Is(err, resourcecap.CodeProviderInvalidResponse) || result != nil {
					t.Fatalf("unproven connection accepted: %+v %v", result, err)
				}
				return
			}
			if err != nil || result == nil || result.ObservedAt.Before(startedAt) || result.ObservedAt.After(time.Now().UTC()) {
				t.Fatalf("owner authentication time not retained: %+v %v", result, err)
			}
		})
	}
}

func TestDynamicCapabilityDeclarationUsesReceiptClockWithoutAuthentication(t *testing.T) {
	capabilities := make([]resourcecap.CapabilityDescriptor, 0, resourcecap.MaximumCapabilityCount)
	for _, code := range resourcecap.SupportedCapabilityCodes() {
		capabilities = append(capabilities, resourcecap.CapabilityDescriptor{Code: code, Support: resourcecap.SupportLevelSupported})
	}
	payload, err := json.Marshal(resourcecap.CapabilitySnapshot{AdapterVersion: "epoch-guest", Capabilities: capabilities, ObservedAt: time.Unix(0, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	registrar := &verificationClockRegistrar{proof: payload}
	p := &dynamicProvider{
		extensions:    registrar,
		configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Settings: []byte("{}")}},
		outbound: func(context.Context, channelconfig.Configuration, string, []resourcecap.SecretBinding) (pluginhost.ConfidentialOutboundHandler, func(), error) {
			return func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
				t.Fatal("capability declaration must not authenticate or call the supplier")
				return nil, nil
			}, func() {}, nil
		},
	}
	startedAt := time.Now().UTC()
	result, err := p.DiscoverCapabilities(context.Background())
	if err != nil || result == nil || result.ObservedAt.Before(startedAt) || result.ObservedAt.After(time.Now().UTC()) {
		t.Fatalf("declaration receipt time invalid: %+v %v", result, err)
	}
}
