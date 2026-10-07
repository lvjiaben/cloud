// These tests keep optional catalog reads under the existing resource owner
// and new-business gate without expanding the original eight provider methods.

package backend

import (
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestCatalogInvocationRequiresResourceOwnerAndNewBusinessPurpose(t *testing.T) {
	for _, method := range []string{resourcecap.DynamicProviderMethodListCatalog, resourcecap.DynamicProviderMethodReadCatalogProduct} {
		invocation := &capmodel.BusinessExtensionBridgeRequest{
			Version: capmodel.BusinessExtensionInvocationProtocolV1, ProviderPluginID: pluginID,
			OwnerPluginID: resourcecap.OwnerPluginID, Point: resourcecap.DynamicProviderPoint,
			ContractVersion: resourcecap.DynamicProviderContractVersion, ReleaseVersion: "v0.2.1",
			ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
			Method: method, Purpose: capmodel.BusinessExtensionInvocationPurposeNewBusiness,
			RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: "v10-catalog"},
		}
		if !validInvocation(invocation) {
			t.Fatalf("resource-owned catalog method rejected: %s", method)
		}
		invocation.Purpose = capmodel.BusinessExtensionInvocationPurposeExistingBusiness
		if validInvocation(invocation) {
			t.Fatalf("existing-business invocation gained catalog read: %s", method)
		}
		invocation.Purpose = capmodel.BusinessExtensionInvocationPurposeNewBusiness
		invocation.OwnerPluginID = "idc-core"
		if validInvocation(invocation) {
			t.Fatal("product owner bypassed resource-owned supplier access")
		}
	}
	if len(resourcecap.DynamicProviderMethods()) != 8 {
		t.Fatal("optional catalog methods changed the required provider contract")
	}
}
