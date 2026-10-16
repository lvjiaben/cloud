// This file verifies the public descriptor guest requires an exact frozen invocation.
package websitecap

import (
	"lina-core/pkg/plugin/capability/capmodel"
	"strings"
	"testing"
)

// TestDescriptorIdentity protects the template descriptor from cross-owner or missing-runtime calls.
func TestDescriptorIdentity(t *testing.T) {
	invocation := &capmodel.BusinessExtensionBridgeRequest{Version: capmodel.BusinessExtensionInvocationProtocolV1, OwnerPluginID: Owner, ProviderPluginID: "website-default", Point: Point, ContractVersion: Version, Method: MethodDescribe, ReleaseVersion: "v0.1.0", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), Purpose: capmodel.BusinessExtensionInvocationPurposeNewBusiness, RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: "fixture"}}
	if !validInvocation(invocation, "website-default") {
		t.Fatal("valid descriptor rejected")
	}
	if validInvocation(invocation, "website-minimal") {
		t.Fatal("other provider accepted")
	}
	invocation.Method = "run"
	if validInvocation(invocation, "website-default") {
		t.Fatal("other method accepted")
	}
	invocation.Method = MethodDescribe
	invocation.RuntimeContext = nil
	if validInvocation(invocation, "website-default") {
		t.Fatal("missing context accepted")
	}
}
