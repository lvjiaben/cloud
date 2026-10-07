package backend

import (
	"io/fs"
	"strings"
	"testing"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-shared-services/backend/cap/challengecap"
	"lina-plugin-shared-services/backend/cap/inboxcap"
)

func TestPluginRegistrationOwnsBothSharedCapabilities(t *testing.T) {
	registered, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok || registered.GetEmbeddedFiles() == nil {
		t.Fatalf("expected %s source plugin registration", pluginID)
	}
	if len(registered.GetRouteRegistrars()) != 1 || len(registered.GetJobRegistrars()) != 1 {
		t.Fatalf("unexpected shared plugin registrars")
	}
	descriptors := registered.GetCapabilityDescriptors()
	if len(descriptors) != 2 {
		t.Fatalf("unexpected descriptor count: %d", len(descriptors))
	}
	foundChallenge, foundInbox := false, false
	for _, descriptor := range descriptors {
		if descriptor.OwnerPluginID != pluginID || descriptor.Version != "v1" {
			t.Fatalf("unexpected descriptor: %#v", descriptor)
		}
		switch descriptor.Service {
		case challengecap.ServiceName:
			foundChallenge = len(descriptor.Methods) == 3
		case inboxcap.ServiceName:
			foundInbox = len(descriptor.Methods) == 1
		}
	}
	if !foundChallenge || !foundInbox {
		t.Fatalf("missing shared capability descriptor: %#v", descriptors)
	}
	manifest, err := fs.ReadFile(registered.GetEmbeddedFiles(), "plugin.yaml")
	if err != nil || !strings.Contains(string(manifest), "id: "+pluginID) {
		t.Fatalf("embedded manifest mismatch: %v", err)
	}
}
