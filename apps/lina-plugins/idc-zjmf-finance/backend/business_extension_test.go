package backend

import (
	"encoding/json"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestProductOwnerCannotInvokeSupplierOutboundDirectly(t *testing.T) {
	invocation := validResourceInvocation()
	invocation.Point = capmodel.BusinessExtensionProductFulfillment
	invocation.OwnerPluginID = "idc-core"
	invocation.Method = "provision"
	if validInvocation(invocation) {
		t.Fatal("product fulfillment context was allowed to enter resource-owned supplier execution")
	}
}

func TestResourceCandidatePurposeIsNewBusinessOnly(t *testing.T) {
	invocation := validResourceInvocation()
	invocation.Method = resourcecap.DynamicProviderMethodConfigurationCandidates
	invocation.Purpose = capmodel.BusinessExtensionInvocationPurposeExistingBusiness
	if validInvocation(invocation) {
		t.Fatal("candidate discovery accepted an existing-business purpose")
	}
	invocation.Purpose = capmodel.BusinessExtensionInvocationPurposeNewBusiness
	if !validInvocation(invocation) {
		t.Fatal("resource owner candidate invocation was rejected")
	}
}

func TestResourceCatalogPurposeIsNewBusinessOnly(t *testing.T) {
	for _, method := range []string{resourcecap.DynamicProviderMethodListCatalog, resourcecap.DynamicProviderMethodReadCatalogProduct} {
		invocation := validResourceInvocation()
		invocation.Method = method
		invocation.Purpose = capmodel.BusinessExtensionInvocationPurposeExistingBusiness
		if validInvocation(invocation) {
			t.Fatalf("catalog method %s accepted an existing-business purpose", method)
		}
		invocation.Purpose = capmodel.BusinessExtensionInvocationPurposeNewBusiness
		if !validInvocation(invocation) {
			t.Fatalf("resource-owned catalog method %s was rejected", method)
		}
	}
}

func validResourceInvocation() *capmodel.BusinessExtensionBridgeRequest {
	return &capmodel.BusinessExtensionBridgeRequest{
		Version:           capmodel.BusinessExtensionInvocationProtocolV1,
		ProviderPluginID:  pluginID,
		OwnerPluginID:     resourcecap.OwnerPluginID,
		Point:             resourcecap.DynamicProviderPoint,
		ContractVersion:   resourcecap.DynamicProviderContractVersion,
		ReleaseVersion:    "v0.1.0",
		ReleaseDigest:     strings.Repeat("a", 64),
		DeclarationDigest: strings.Repeat("b", 64),
		Method:            resourcecap.DynamicProviderMethodDiscoverCapabilities,
		Purpose:           capmodel.BusinessExtensionInvocationPurposeExistingBusiness,
		Payload:           json.RawMessage(`{}`),
		RuntimeContext:    &capmodel.BusinessExtensionRuntimeContext{ContextID: "ctx-1"},
	}
}
