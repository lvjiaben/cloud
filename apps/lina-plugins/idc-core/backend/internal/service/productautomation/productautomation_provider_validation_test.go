// This file verifies pure configuration validation fails closed, preserves
// exact provider identity, and revokes its context after every invocation.

package productautomation

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"testing"

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
