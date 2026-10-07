package poolsourcecap

import "testing"

func TestCandidateValidation(t *testing.T) {
	candidate := Candidate{
		PoolNo: "pool_a", DisplayName: "Hong Kong A", ChannelName: "Mofang",
		ProviderID: "zjmf-cloud", ProviderPluginID: "idc-zjmfcloud",
		ResourceKind: ResourceKindVirtualServer,
		Revision:     "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	if err := ValidateCandidate(candidate); err != nil {
		t.Fatalf("ValidateCandidate() error = %v", err)
	}
	if err := ValidateCandidates([]Candidate{candidate, candidate}); err == nil {
		t.Fatal("ValidateCandidates() accepted duplicate pool")
	}
	candidate.Revision = "stale"
	if err := ValidateCandidate(candidate); err == nil {
		t.Fatal("ValidateCandidate() accepted malformed revision")
	}
}

func TestVerifyRequestValidation(t *testing.T) {
	request := VerifyRequest{
		PoolNo: "pool_a", ResourceKind: ResourceKindBareMetal,
		Revision: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	if err := ValidateVerifyRequest(request); err != nil {
		t.Fatalf("ValidateVerifyRequest() error = %v", err)
	}
	request.PoolNo = "pool with space"
	if err := ValidateVerifyRequest(request); err == nil {
		t.Fatal("ValidateVerifyRequest() accepted whitespace pool number")
	}
}
