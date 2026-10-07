package challengecap

import "testing"

func TestValidateRequestsRejectCrossPurposeAndWeakBindings(t *testing.T) {
	if err := ValidateCreateRequest(CreateRequest{Purpose: "support.presales", Binding: "browser-random-0123456789", Kind: PresentationImage}); err != nil {
		t.Fatalf("valid challenge request rejected: %v", err)
	}
	for _, input := range []CreateRequest{
		{Purpose: "Support Presales", Binding: "browser-random-0123456789", Kind: PresentationImage},
		{Purpose: "support.presales", Binding: "short", Kind: PresentationImage},
		{Purpose: "support.presales", Binding: "browser-random-0123456789", Kind: "audio"},
	} {
		if err := ValidateCreateRequest(input); err == nil {
			t.Fatalf("invalid challenge request accepted: %+v", input)
		}
	}
}
