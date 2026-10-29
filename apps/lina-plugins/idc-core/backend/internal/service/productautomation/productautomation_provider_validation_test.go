// This file verifies pure configuration validation fails closed, preserves
// exact provider identity, and revokes its context after every invocation.

package productautomation

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/errors/gcode"
	"lina-core/pkg/bizerr"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func TestProviderConfigurationValidationBeforeSave(t *testing.T) {
	for _, response := range []string{`{"version":"v1","valid":true}`, `{"version":"v1","valid":false}`, `{"version":"v2","valid":true}`, `{"version":"v1"}`, `{"version":"v1","valid":true} {}`, `{"version":"v1","valid":true,"secret":"forged"}`, `{"version":"v1","valid":false,"reason":"supplier_capabilities_unconfirmed"}`, `{"version":"v1","valid":true,"reason":"supplier_capabilities_unconfirmed"}`, `{"version":"v1","valid":true,"reason":"forged"}`} {
		t.Run(response, func(t *testing.T) {
			ctx := context.Background()
			exact := dynamicCandidateEvidence("1.0.0")
			exact.AuthorizedMethods = append(exact.AuthorizedMethods, productfulfillmentcap.MethodValidateProductConfiguration)
			exact.ProductConfiguration = nil
			extensions := &automationTestExtensions{}
			service := &serviceImpl{extensions: extensions, dynamicCandidates: newDynamicCandidateCache(), runtimeContexts: newSupplierRuntimeContextStore(rand.Reader), secretAccessor: newConfigurationSecretManager()}
			var contextID string
			extensions.invoke = func(input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
				contextID = input.RuntimeContext.ContextID
				current, ok := service.runtimeContexts.get(contextID, exact.PluginID)
				if !ok || current.configurationView != nil || current.interfaceInstanceNo != "" || current.configurationRevision != 0 || input.Method != productfulfillmentcap.MethodValidateProductConfiguration || input.Purpose != capmodel.BusinessExtensionInvocationPurposeNewBusiness || input.ReleaseDigest != exact.ReleaseDigest || input.DeclarationDigest != exact.DeclarationDigest {
					t.Fatalf("validation received wrong evidence or credentials: %#v", input)
				}
				if err := service.runtimeSecretAccess(ctx, exact.PluginID, contextID, "password", func([]byte) error {
					t.Fatal("pure validation consumed a supplier secret")
					return nil
				}); err == nil {
					t.Fatal("pure validation was allowed to request supplier secrets")
				}
				var request productfulfillmentcap.ConfigurationValidationRequest
				if err := json.Unmarshal(input.Payload, &request); err != nil || string(request.ProductConfiguration) != "{}" {
					t.Fatalf("canonical values missing: %s %v", input.Payload, err)
				}
				return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: json.RawMessage(response)}, nil
			}
			_, err := service.compileConfigurationResolved(ctx, exact, nil, nil, configurationSurfaceProduct, dynamicCandidateScope{interfaceInstanceNo: "supplier-interface", configurationRevision: 7})
			if (err == nil) != (response == `{"version":"v1","valid":true}`) || extensions.invokeCalls != 1 {
				t.Fatalf("validation did not fail closed: err=%v calls=%d", err, extensions.invokeCalls)
			}
			if response == `{"version":"v1","valid":false,"reason":"supplier_capabilities_unconfirmed"}` && !bizerr.Is(err, codeSupplierCapabilitiesUnconfirmed) {
				t.Fatalf("supplier capability rejection lost its actionable reason: %v", err)
			}
			if response == `{"version":"v1","valid":true,"reason":"forged"}` && !bizerr.Is(err, catalog.CodeManagementInvalid) {
				t.Fatalf("unknown provider reason accepted: %v", err)
			}
			if _, exists := service.runtimeContexts.get(contextID, exact.PluginID); exists {
				t.Fatal("validation context survived invocation")
			}
		})
	}
}

func TestProviderConfigurationValidationOptionalForExistingPlugins(t *testing.T) {
	service := &serviceImpl{}
	if err := service.validateProviderProductConfiguration(context.Background(), dynamicCandidateEvidence("1.0.0"), json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
}

func TestBoundTextAlternativesUseOnePureProviderBatch(t *testing.T) {
	exact := dynamicCandidateEvidence("1.0.0")
	exact.AuthorizedMethods = append(exact.AuthorizedMethods, productfulfillmentcap.MethodValidateProductConfiguration)
	exact.ProductConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "system_disk_io_limit", Type: capmodel.ConfigurationFieldSingleLine, MaximumLength: 43}}}
	ext := &automationTestExtensions{}
	service := &serviceImpl{extensions: ext, runtimeContexts: newSupplierRuntimeContextStore(rand.Reader)}
	ext.invoke = func(input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
		var req productfulfillmentcap.ConfigurationValidationRequest
		if err := json.Unmarshal(input.Payload, &req); err != nil {
			t.Fatal(err)
		}
		choices := req.ProductChoiceValues["system_disk_io_limit"]
		if len(choices) != 2 || string(choices[1]) != `"500,500,2000,2000"` {
			t.Fatalf("profiles not batched: %s", input.Payload)
		}
		state, ok := service.runtimeContexts.get(input.RuntimeContext.ContextID, exact.PluginID)
		if !ok || state.configurationView != nil || state.interfaceInstanceNo != "" {
			t.Fatal("choice validation received supplier access")
		}
		return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: json.RawMessage(`{"version":"v1","valid":true}`)}, nil
	}
	selections := []CustomerFieldSelection{{FieldKey: "local_disk", ProductFieldKey: "system_disk_io_limit", LocalField: &capmodel.ConfigurationFieldDeclaration{Type: capmodel.ConfigurationFieldSingleSelect, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "0,0,0,0"}, {Value: "500,500,2000,2000"}}}}}
	if err := service.validateBoundTextChoices(context.Background(), exact, json.RawMessage(`{}`), selections); err != nil {
		t.Fatal(err)
	}
	if ext.invokeCalls != 1 {
		t.Fatal("one call per option")
	}
}

// TestPureValidationTechnicalFailuresKeepOnlySafeCategories verifies the exact
// failure phase remains actionable without replaying the method or exposing the
// provider's arbitrary error/body, including credential-shaped test strings.
func TestPureValidationTechnicalFailuresKeepOnlySafeCategories(t *testing.T) {
	for _, scenario := range []string{"context", "invoke", "busy", "missing", "protocol", "large", "json", "missing-valid", "null-valid"} {
		t.Run(scenario, func(t *testing.T) {
			exact := dynamicCandidateEvidence("1.0.0")
			exact.AuthorizedMethods = append(exact.AuthorizedMethods, productfulfillmentcap.MethodValidateProductConfiguration)
			extensions := &automationTestExtensions{}
			service := &serviceImpl{extensions: extensions, runtimeContexts: newSupplierRuntimeContextStore(rand.Reader)}
			if scenario == "context" {
				service.runtimeContexts = newSupplierRuntimeContextStore(validationFailingRandom{})
			}
			var contextID string
			extensions.invoke = func(input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
				contextID = input.RuntimeContext.ContextID
				switch scenario {
				case "invoke":
					return nil, errors.New("password=PRIVATE provider body")
				case "busy":
					return nil, bizerr.NewCode(bizerr.MustDefine("PLUGIN_LIFECYCLE_ALREADY_RUNNING", "private host explanation", gcode.CodeOperationFailed))
				case "missing":
					return nil, nil
				case "protocol":
					return &capmodel.BusinessExtensionMethodResult{Version: "v2", Payload: json.RawMessage(`{"version":"v1","valid":true}`)}, nil
				case "missing-valid":
					return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: json.RawMessage(`{"version":"v1"}`)}, nil
				case "null-valid":
					return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: json.RawMessage(`{"version":"v1","valid":null}`)}, nil
				case "large":
					return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: bytes.Repeat([]byte("PRIVATE"), productfulfillmentcap.MaximumEvidenceBytes/7+1)}, nil
				default:
					return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: json.RawMessage(`{"password":"PRIVATE"}`)}, nil
				}
			}
			err := service.validateProviderProductConfiguration(t.Context(), exact, json.RawMessage(`{}`))
			expected := codeProviderValidationResponseInvalid
			reason := string(validationReplyInvalid)
			switch scenario {
			case "context":
				expected = codeProviderValidationUnavailable
				reason = string(validationContextUnavailable)
			case "invoke":
				expected = codeProviderValidationUnavailable
				reason = string(validationInvocationFailed)
			case "busy":
				expected = codeProviderBusy
				reason = string(validationAdmissionBusy)
			case "missing":
				reason = string(validationResultMissing)
			case "protocol":
				reason = string(validationProtocolInvalid)
			case "large":
				reason = string(validationReplyTooLarge)
			}
			if !bizerr.Is(err, expected) || !bizerr.Is(normalizeManagementWriteError(err), expected) {
				t.Fatalf("technical classification lost: %v", err)
			}
			metadata, ok := bizerr.As(err)
			if !ok || metadata.Params()["reason"] != reason || strings.Contains(fmt.Sprintf("%+v", err), "PRIVATE") {
				t.Fatal("unsafe or missing technical detail")
			}
			calls := 1
			if scenario == "context" {
				calls = 0
			}
			if extensions.invokeCalls != calls {
				t.Fatal("unconfirmed validation invocation repeated")
			}
			if contextID != "" {
				if _, exists := service.runtimeContexts.get(contextID, exact.PluginID); exists {
					t.Fatal("pure validation context leaked")
				}
			}
		})
	}
	unsafe := providerValidationFailureError(t.Context(), providerValidationFailure("password=PRIVATE"))
	metadata, _ := bizerr.As(unsafe)
	if metadata.Params()["reason"] != string(validationReplyInvalid) {
		t.Fatal("untrusted failure category accepted")
	}
}

// validationFailingRandom fails before a runtime context can be granted.
type validationFailingRandom struct{}

func (validationFailingRandom) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
