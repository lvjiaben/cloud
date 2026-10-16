// Identity-only selections must reload the exact public provider before
// compiling input; missing or changed authority cannot reach admission writes.
package productautomation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestCreateInterfaceRejectsChangedSelectionAuthorityBeforeAdmission(t *testing.T) {
	for _, scenario := range []string{"unavailable", "release", "declaration", "methods", "metadata"} {
		t.Run(scenario, func(t *testing.T) {
			selected := automationEvidence(nil)
			exact := cloneAutomationEvidence(selected)
			extensions := &automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{exact}}
			switch scenario {
			case "unavailable":
				extensions.exactErr = errors.New("unavailable")
			case "release":
				exact.ReleaseDigest = strings.Repeat("2", 64)
			case "declaration":
				exact.DeclarationDigest = strings.Repeat("2", 64)
			case "methods":
				exact.AuthorizedMethods = []string{"provision"}
			case "metadata":
				exact.ProviderMetadata = nil
			}
			svc := &serviceImpl{authorizer: &automationTestAuthorizer{}, codec: &automationTestCodec{evidence: selected, identityOnly: true}, extensions: extensions}
			result, err := svc.CreateInterface(context.Background(), CreateInterfaceInput{DisplayName: "Provider", SelectionToken: "selection"})
			if err == nil || result != nil || extensions.exactCalls != 1 || extensions.admissionCalls != 0 {
				t.Fatalf("changed declaration reached write: %v", err)
			}
		})
	}
}
