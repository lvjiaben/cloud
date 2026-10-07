package resourcecap

import (
	"strings"
	"testing"
)

func TestResidualResourcesAreBoundedTerminateOnlyAndSecretFree(t *testing.T) {
	base := OperationResult{Status: TaskStatusUnknown,
		Failure: &ProviderFailure{Code: "partial_reclaim", Message: "manual review required", ResultUnknown: true},
		ResidualResources: []ResidualResource{{ResourceKind: "volume", ResourceRef: "volume_600100",
			Status: ResidualResourceReleaseFailed, ReasonCode: "provider_rejected"}}}
	request := OperationRequest{RequestID: "terminate-request-1", Capability: CapabilityTerminate,
		ResourceRef: "server_600100", Parameters: []byte(`{}`)}
	if err := base.ValidateFor(request); err != nil {
		t.Fatalf("valid residual resource rejected: %v", err)
	}
	nonTerminate := request
	nonTerminate.Capability = CapabilityReboot
	if err := base.ValidateFor(nonTerminate); err == nil {
		t.Fatal("non-terminate result accepted residual resources")
	}
	secret := base
	// Raw URL-safe Base64 for "token=must-not-pass" still matches the
	// canonical reason-code alphabet, so the credential scanner must catch it.
	secret.ResidualResources[0].ReasonCode = "dG9rZW49bXVzdC1ub3QtcGFzcw"
	if err := secret.ValidateFor(request); err == nil {
		t.Fatal("secret-like residual reason was accepted")
	}
	overflow := base
	overflow.ResidualResources = make([]ResidualResource, MaximumResidualResourceCount+1)
	for index := range overflow.ResidualResources {
		overflow.ResidualResources[index] = ResidualResource{ResourceKind: "volume",
			ResourceRef: "volume_" + strings.Repeat("a", index+1),
			Status:      ResidualResourceUnknown, ReasonCode: "state_unknown"}
	}
	if err := overflow.ValidateFor(request); err == nil {
		t.Fatal("residual resource overflow was accepted")
	}
}
