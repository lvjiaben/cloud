package dcim

import (
	"encoding/json"
	"strings"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func validPlan() resourcecap.ServerProvisionPlan {
	return resourcecap.ServerProvisionPlan{
		ExternalPrincipalRef: "customer-42", Hostname: "server-42",
		Selector:              json.RawMessage(`{}`),
		ProductConfiguration:  json.RawMessage(`{"server_group":"8","ip_address_count":2,"backup_count":1,"snapshot_count":3}`),
		CustomerConfiguration: json.RawMessage(`{"operating_system":"9"}`),
		Components: []resourcecap.ServerProvisionComponent{
			{Type: resourcecap.ServerProvisionComponentSystemDisk, Gigabytes: 40, SelectionQuantity: 1},
			{Type: resourcecap.ServerProvisionComponentDataDisk, Gigabytes: 100, SelectionQuantity: 2},
		},
	}
}

func TestBuildProvisionParametersSupportsOnlyBMS(t *testing.T) {
	t.Parallel()
	if _, err := buildProvisionParameters(ProtocolLegacy, validPlan()); err == nil {
		t.Fatal("legacy provisioning must remain unsupported")
	}
	raw, err := buildProvisionParameters(ProtocolBMS, validPlan())
	if err != nil {
		t.Fatal(err)
	}
	var parameters bmsProvisionParameters
	if json.Unmarshal(raw, &parameters) != nil || parameters.Type != 2 || parameters.Size != 40 || parameters.DataSize != 200 || parameters.GroupID != "8" || parameters.OSID != "9" {
		t.Fatalf("unexpected parameters: %s", raw)
	}
	if string(raw) == "" || containsAny(string(raw), "password", "access_token", "access-user", "endpoint") {
		t.Fatalf("secret or endpoint leaked into parameters: %s", raw)
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
