package baota

import (
	"encoding/json"
	"errors"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"testing"
)

func TestBuildSitePlan(t *testing.T) {
	got, err := BuildSitePlan(json.RawMessage(`{"deployment_type":"php_site","php_version":"82"}`), json.RawMessage(`{"primary_domain":"shop.example.com","additional_domains":"www.example.com"}`))
	if err != nil {
		t.Fatal(err)
	}
	var plan FrozenSitePlan
	if json.Unmarshal(got, &plan) != nil || plan.Path != "/www/wwwroot/shop.example.com" || plan.PHPVersion != "82" {
		t.Fatalf("unexpected plan: %s", got)
	}
}

func TestBuildSitePlanRejectsUnsafeInputs(t *testing.T) {
	if _, err := BuildSitePlan(json.RawMessage(`{"deployment_type":"php_site","php_version":"82"}`), json.RawMessage(`{"primary_domain":"../../etc/passwd"}`)); err == nil {
		t.Fatal("unsafe domain accepted")
	}
}

type failingExecutor struct{}

func (failingExecutor) Execute(string, uint8, json.RawMessage) (json.RawMessage, error) {
	return nil, errors.New("not called")
}

func TestResourceCapabilitiesExposeOnlySafeSiteOperations(t *testing.T) {
	data, err := ExecuteResource(failingExecutor{}, "ctx-1", resourcecap.DynamicProviderMethodDiscoverCapabilities, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot resourcecap.CapabilitySnapshot
	if json.Unmarshal(data, &snapshot) != nil || snapshot.ValidateAt(snapshot.ObservedAt) != nil {
		t.Fatal("invalid snapshot")
	}
}
