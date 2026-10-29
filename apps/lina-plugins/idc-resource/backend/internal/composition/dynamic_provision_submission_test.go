// These tests require owner-confirmed absence of every business HTTP attempt;
// a guest claim, a failed later step or an acknowledgement must never suffice.
package composition

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

type preflightFailureRegistrar struct{ verificationClockRegistrar }

// saturatedDiagnosticRegistrar fills the saved diagnostic budget before a real
// attempted business request, reproducing the dangerous bounded-log edge case.
type saturatedDiagnosticRegistrar struct{ verificationClockRegistrar }

func (f *saturatedDiagnosticRegistrar) InvokeProviderMethod(ctx context.Context, _ pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	for sequence := uint8(1); sequence <= 5; sequence++ {
		payload := json.RawMessage(`{"kind":"request","method":"POST","path":"/clouds","useAuthentication":true,"responseFields":["data"],"secretFormBindings":[{"field":"rootpass","slot":"not_granted"}]}`)
		if sequence == 5 {
			payload = json.RawMessage(`{"kind":"request","method":"POST","path":"/clouds","useAuthentication":true,"responseFields":["data"]}`)
		}
		if _, err := f.registration.Handler(ctx, &pluginhost.ConfidentialOutboundRequest{Sequence: sequence, Payload: payload}); err == nil {
			return nil, context.Canceled
		}
	}
	return &capmodel.BusinessExtensionMethodResult{Payload: f.proof}, nil
}

func (f *preflightFailureRegistrar) InvokeProviderMethod(ctx context.Context, _ pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	if _, err := f.registration.Handler(ctx, &pluginhost.ConfidentialOutboundRequest{Sequence: 2, Payload: []byte(`{"kind":"request","method":"POST","path":"/clouds","useAuthentication":true,"responseFields":["data"],"secretFormBindings":[{"field":"rootpass","slot":"not_granted"}]}`)}); err == nil {
		return nil, context.Canceled
	}
	return &capmodel.BusinessExtensionMethodResult{Payload: f.proof}, nil
}

func TestProvisionOwnerPreflightFailureIsDefiniteWithoutSubmission(t *testing.T) {
	proof, err := json.Marshal(resourcecap.OperationResult{Status: resourcecap.TaskStatusUnknown, Failure: &resourcecap.ProviderFailure{Code: "upstream_result_unknown", Message: "Outcome unconfirmed", ResultUnknown: true}})
	if err != nil {
		t.Fatal(err)
	}
	registrar := &preflightFailureRegistrar{verificationClockRegistrar: verificationClockRegistrar{proof: proof}}
	p := &dynamicProvider{extensions: registrar, configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Settings: []byte(`{}`)}}, outbound: func(_ context.Context, _ channelconfig.Configuration, _ string, _ []resourcecap.SecretBinding, observers ...func(resourcecap.Evidence)) (pluginhost.ConfidentialOutboundHandler, func(), error) {
		h := &confidentialHTTPHandler{providerMethod: resourcecap.DynamicProviderMethodExecute, observeResponse: observers[0]}
		return h.Execute, func() {}, nil
	}}
	var result resourcecap.OperationResult
	err = p.invoke(t.Context(), resourcecap.DynamicProviderMethodExecute, resourcecap.DynamicOperationRequest{RequestID: "task_local_preflight", Capability: resourcecap.CapabilityProvision}, nil, &result)
	if err != nil || result.Status != resourcecap.TaskStatusFailed || result.Failure == nil || result.Failure.ResultUnknown || result.Failure.Retryable {
		t.Fatalf("owner-proven absence did not finish as a non-retried failure: %+v %v", result, err)
	}
	if result.Failure.Code != provisionresourcecap.FailureRequestNotSent || resourcecap.ValidateProvisionNonSubmissionEvidence(result.Evidence[len(result.Evidence)-1], "task_local_preflight") != nil {
		t.Fatal("owner proof was not bound to the request")
	}
}

func TestProvisionLaterPreflightFailurePreservesPriorAttemptAndAcknowledgement(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		attempted  int
		taskRef    string
		capability resourcecap.CapabilityCode
	}{
		{"previous HTTP attempt", 1, "", resourcecap.CapabilityProvision},
		{"acknowledged task", 0, "upstream-task-71", resourcecap.CapabilityProvision},
		{"different operation", 0, "", resourcecap.CapabilityStart},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			proof, err := json.Marshal(resourcecap.OperationResult{Status: resourcecap.TaskStatusUnknown, ExternalTaskRef: scenario.taskRef, Failure: &resourcecap.ProviderFailure{Code: "upstream_result_unknown", Message: "Outcome unconfirmed", ResultUnknown: true}})
			if err != nil {
				t.Fatal(err)
			}
			registrar := &preflightFailureRegistrar{verificationClockRegistrar: verificationClockRegistrar{proof: proof}}
			p := &dynamicProvider{extensions: registrar, configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Settings: []byte(`{}`)}}, outbound: func(_ context.Context, _ channelconfig.Configuration, _ string, _ []resourcecap.SecretBinding, observers ...func(resourcecap.Evidence)) (pluginhost.ConfidentialOutboundHandler, func(), error) {
				h := &confidentialHTTPHandler{providerMethod: resourcecap.DynamicProviderMethodExecute, observeResponse: observers[0], businessHTTPStarted: scenario.attempted}
				return h.Execute, func() {}, nil
			}}
			var result resourcecap.OperationResult
			if err = p.invoke(t.Context(), resourcecap.DynamicProviderMethodExecute, resourcecap.DynamicOperationRequest{RequestID: "task_preserve_unknown", Capability: scenario.capability}, nil, &result); err != nil || result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil || !result.Failure.ResultUnknown || result.ExternalTaskRef != scenario.taskRef {
				t.Fatalf("failed later step erased acceptance uncertainty: %+v %v", result, err)
			}
			for _, item := range result.Evidence {
				if item.Kind == resourcecap.EvidenceKindRequestNotSent {
					t.Fatal("unproven local absence became recoverable")
				}
			}
		})
	}
}

func TestProvisionGuestCannotAssertOwnerNonSubmission(t *testing.T) {
	forged, err := resourcecap.NewProvisionNonSubmissionEvidence("task_forged_absence", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(resourcecap.OperationResult{Status: resourcecap.TaskStatusFailed, Failure: &resourcecap.ProviderFailure{Code: provisionresourcecap.FailureRequestNotSent, Message: "Guest claims absence"}, Evidence: []resourcecap.Evidence{forged}})
	if err != nil {
		t.Fatal(err)
	}
	registrar := &verificationClockRegistrar{proof: payload}
	p := &dynamicProvider{extensions: registrar, configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Settings: []byte(`{}`)}}, outbound: func(context.Context, channelconfig.Configuration, string, []resourcecap.SecretBinding, ...func(resourcecap.Evidence)) (pluginhost.ConfidentialOutboundHandler, func(), error) {
		return func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
			t.Fatal("no owner step was expected")
			return nil, nil
		}, func() {}, nil
	}}
	var result resourcecap.OperationResult
	if err = p.invoke(t.Context(), resourcecap.DynamicProviderMethodExecute, resourcecap.DynamicOperationRequest{RequestID: "task_forged_absence", Capability: resourcecap.CapabilityProvision}, nil, &result); err != nil || result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil || !result.Failure.ResultUnknown {
		t.Fatalf("guest supplied owner authority: %+v %v", result, err)
	}
	for _, item := range result.Evidence {
		if item.Kind == resourcecap.EvidenceKindRequestNotSent {
			t.Fatal("forged owner proof retained")
		}
	}
}

func TestProvisionMissingInvocationCountNeverProvesZero(t *testing.T) {
	for _, summary := range []string{`{"stage":"request_preparation","businessRequestStarted":false}`, `{"stage":"request_preparation","businessRequestsAttempted":1}`, `{"stage":"http_transport","businessRequestsAttempted":0}`} {
		if ownerObservedNoBusinessAttempt([]resourcecap.Evidence{{Kind: resourcecap.EvidenceKindCallDiagnostic, Summary: summary, ObservedAt: time.Now().UTC()}}) {
			t.Fatal("incomplete or contradictory observation proved absence")
		}
	}
}

func TestProvisionSubmissionCannotDisappearWhenDiagnosticBudgetIsFull(t *testing.T) {
	proof, err := json.Marshal(resourcecap.OperationResult{Status: resourcecap.TaskStatusUnknown, Failure: &resourcecap.ProviderFailure{Code: "upstream_result_unknown", Message: "Outcome unconfirmed", ResultUnknown: true}})
	if err != nil {
		t.Fatal(err)
	}
	registrar := &saturatedDiagnosticRegistrar{verificationClockRegistrar: verificationClockRegistrar{proof: proof}}
	client := &diagnosticFailedHTTPClient{}
	p := &dynamicProvider{extensions: registrar, configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Settings: []byte(`{}`)}}, outbound: func(_ context.Context, _ channelconfig.Configuration, _ string, _ []resourcecap.SecretBinding, observers ...func(resourcecap.Evidence)) (pluginhost.ConfidentialOutboundHandler, func(), error) {
		h := &confidentialHTTPHandler{providerMethod: resourcecap.DynamicProviderMethodExecute, observeResponse: observers[0], login: &resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: "POST", Path: "/login"}, configuration: channelconfig.Configuration{Config: resourcecap.ProviderConfig{Endpoint: "https://example.test", Settings: diagnosticLoginSettings()}}, environment: resourcecap.ProviderEnv{HTTPClient: client, AuthenticationSessions: &confidentialHTTPSessions{token: []byte("private-session")}}}
		return h.Execute, func() {}, nil
	}}
	var result resourcecap.OperationResult
	if err = p.invoke(t.Context(), resourcecap.DynamicProviderMethodExecute, resourcecap.DynamicOperationRequest{RequestID: "task_saturated_diagnostics", Capability: resourcecap.CapabilityProvision}, nil, &result); err != nil || result.Status != resourcecap.TaskStatusUnknown || client.calls != 1 {
		t.Fatalf("a transmitted request became falsely recoverable after log truncation: %+v calls=%d %v", result, client.calls, err)
	}
	for _, item := range result.Evidence {
		if item.Kind == resourcecap.EvidenceKindRequestNotSent || item.Kind == businessHTTPAttemptObserved {
			t.Fatal("invalid owner proof or internal signal was exposed")
		}
	}
}
