package backend

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestManifestDeclaresManagedMultiInstanceProtocolProfiles(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../plugin.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if yaml.Unmarshal(raw, &manifest) != nil || manifest["id"] != "idc-dcim" || manifest["type"] != "dynamic" || manifest["distribution"] != "managed" {
		t.Fatalf("invalid managed plugin manifest: %#v", manifest)
	}
	encoded := string(raw)
	for _, required := range []string{"interface_configuration_cardinality: multiple", "interfaceSettingsProjection:", "authenticationProfiles:", "per_request_form", "per_request_headers"} {
		if !strings.Contains(encoded, required) {
			t.Fatalf("manifest is missing %q", required)
		}
	}
}
