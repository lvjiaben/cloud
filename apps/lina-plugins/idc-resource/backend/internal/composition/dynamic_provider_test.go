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
				outbound: func(context.Context, channelconfig.Configuration, string, []resourcecap.SecretBinding, ...func(resourcecap.Evidence)) (pluginhost.ConfidentialOutboundHandler, func(), error) {
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
		outbound: func(context.Context, channelconfig.Configuration, string, []resourcecap.SecretBinding, ...func(resourcecap.Evidence)) (pluginhost.ConfidentialOutboundHandler, func(), error) {
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

// Host receipt, rather than an epoch WASI clock, establishes observation time
// for task reconciliation and current customer runtime admission.
func TestDynamicObservationTimeUsesOwnerReceipt(t *testing.T) {

	epoch := time.Unix(0, 0).UTC()
	payload, err := json.Marshal(resourcecap.ResourceSnapshot{ExternalResourceRef: "41", State: resourcecap.ResourceStateActive, Data: json.RawMessage(`{"addresses":["203.0.113.9"]}`), ObservedAt: epoch, Evidence: []resourcecap.Evidence{{Kind: "resource.query", Reference: "41", ObservedAt: epoch}}})
	if err != nil {
		t.Fatal(err)
	}
	registrar := &verificationClockRegistrar{proof: payload}
	p := &dynamicProvider{extensions: registrar, configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Settings: []byte(`{}`)}}, outbound: func(context.Context, channelconfig.Configuration, string, []resourcecap.SecretBinding, ...func(resourcecap.Evidence)) (pluginhost.ConfidentialOutboundHandler, func(), error) {
		return func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
			t.Fatal("fake guest must not call network")
			return nil, nil
		}, func() {}, nil
	}}
	before := time.Now().UTC()
	result, err := p.QueryResource(context.Background(), resourcecap.ResourceQuery{ExternalResourceRef: "41"})
	if err != nil || result == nil || result.ObservedAt.Before(before) || result.ObservedAt.After(time.Now().UTC()) || len(result.Evidence) != 1 || !result.Evidence[0].ObservedAt.Equal(result.ObservedAt) {
		t.Fatalf("guest clock reached owner observation: %+v %v", result, err)
	}
}

func TestDynamicCapacityPreservesObjectFilterWire(t *testing.T) {
	epoch := time.Unix(0, 0).UTC()
	payload, err := json.Marshal(resourcecap.ProviderCapacityPage{
		Items:    []resourcecap.ProviderCapacityItem{{Kind: resourcecap.ProviderCapacityKindLine, ExternalRef: "5", Data: json.RawMessage(`{"id":5,"area_id":2}`), ObservedAt: epoch}},
		Complete: true, Evidence: []resourcecap.Evidence{{Kind: "upstream_page", Reference: "line:1", ObservedAt: epoch}},
	})
	if err != nil {
		t.Fatal(err)
	}
	registrar := &verificationClockRegistrar{proof: payload}
	p := &dynamicProvider{extensions: registrar, configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Settings: json.RawMessage(`{}`)}}, outbound: func(context.Context, channelconfig.Configuration, string, []resourcecap.SecretBinding, ...func(resourcecap.Evidence)) (pluginhost.ConfidentialOutboundHandler, func(), error) {
		return func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
			t.Fatal("wire fixture must not call a supplier")
			return nil, nil
		}, func() {}, nil
	}}
	request := resourcecap.ProviderCapacityRequest{Kind: resourcecap.ProviderCapacityKindLine, Limit: 5}
	before := time.Now().UTC()
	result, err := p.ListProviderCapacity(context.Background(), request)
	if err != nil || result == nil || len(result.Items) != 1 {
		t.Fatalf("capacity failed: %+v %v", result, err)
	}
	var received resourcecap.ProviderCapacityRequest
	if _, err = resourcecap.DecodeDynamicProviderInvocationEnvelope(registrar.invocation.Payload, &received); err != nil {
		t.Fatal(err)
	}
	if received.Validate() != nil || string(received.Filters) != `{}` {
		t.Fatalf("empty filter changed on the managed wire: %s", received.Filters)
	}
	if request.Filters != nil {
		t.Fatal("capacity serialization mutated caller input")
	}
	if result.Items[0].ObservedAt.Before(before) || result.Items[0].ObservedAt.After(time.Now().UTC()) || !result.Items[0].ObservedAt.Equal(result.Evidence[0].ObservedAt) {
		t.Fatal("capacity observations did not use owner receipt time")
	}
	if _, err = p.ListProviderCapacity(context.Background(), resourcecap.ProviderCapacityRequest{Kind: request.Kind, Limit: 5, Filters: json.RawMessage(`null`)}); err == nil {
		t.Fatal("explicit invalid filters bypassed validation")
	}
}

// swallowedFailureRegistrar models a real guest converting a masked callback
// error to a valid unknown result. The owner must keep its reviewed category.
type swallowedFailureRegistrar struct{ verificationClockRegistrar }

func (f *swallowedFailureRegistrar) InvokeProviderMethod(ctx context.Context, input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	if _, err := f.registration.Handler(ctx, &pluginhost.ConfidentialOutboundRequest{Sequence: 2}); err == nil {
		return nil, context.Canceled
	}
	return &capmodel.BusinessExtensionMethodResult{Payload: f.proof}, nil
}
func TestDynamicUnknownOperationRetainsOwnerFailure(t *testing.T) {
	for _, code := range []*bizerr.Code{resourcecap.CodeProviderTimeout, resourcecap.CodeProviderDNS, resourcecap.CodeProviderNetwork, resourcecap.CodeProviderEndpointNotFound, resourcecap.CodeProviderInvalidResponse, resourcecap.CodeProviderAuthRejected, resourcecap.CodeProviderLocalAuthPolicy} {
		t.Run(code.RuntimeCode(), func(t *testing.T) {
			proof, err := json.Marshal(resourcecap.OperationResult{Status: resourcecap.TaskStatusUnknown, Failure: &resourcecap.ProviderFailure{Code: "UPSTREAM_RESULT_UNKNOWN", Message: "unknown", ResultUnknown: true}})
			if err != nil {
				t.Fatal(err)
			}
			registrar := &swallowedFailureRegistrar{verificationClockRegistrar: verificationClockRegistrar{proof: proof}}
			provider := &dynamicProvider{extensions: registrar, configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Settings: []byte("{}")}}, outbound: func(context.Context, channelconfig.Configuration, string, []resourcecap.SecretBinding, ...func(resourcecap.Evidence)) (pluginhost.ConfidentialOutboundHandler, func(), error) {
				return func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
					return nil, bizerr.WrapCode(context.Canceled, code)
				}, func() {}, nil
			}}
			var result resourcecap.OperationResult
			if err = provider.invoke(t.Context(), resourcecap.DynamicProviderMethodExecute, struct{}{}, nil, &result); err != nil {
				t.Fatal(err)
			}
			if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil || result.Failure.Code != code.RuntimeCode() || !result.Failure.ResultUnknown || result.Failure.Retryable || len(result.Evidence) != 2 || result.Evidence[1].Kind != resourcecap.EvidenceKindCallDiagnostic {
				t.Fatalf("classification lost or replay allowed: %+v", result)
			}
			if err = result.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestOperationFailureNeverPromotesUnreviewedError(t *testing.T) {
	result := resourcecap.OperationResult{Status: resourcecap.TaskStatusUnknown}
	if retainOperationFailure(&result, context.Canceled) || result.Failure != nil {
		t.Fatal("unreviewed error exposed")
	}
}
