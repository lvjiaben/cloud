// This file verifies server provisioning plans cannot carry or return Provider secrets.

package resourcecap

import (
	"context"
	"encoding/json"
	"testing"
)

// TestBuildProvisionParametersKeepsSecretsOutOfBothConnectorDirections proves
// the future managed guest boundary contains only the reviewed safe plan and
// one non-secret parameter projection.
func TestBuildProvisionParametersKeepsSecretsOutOfBothConnectorDirections(t *testing.T) {
	t.Parallel()
	plan := validServerProvisionPlan()
	parameters, err := BuildProvisionParameters(context.Background(), func(
		_ context.Context,
		got ServerProvisionPlan,
	) (json.RawMessage, error) {
		if got.ExternalPrincipalRef != "88" || got.Hostname != "server-1.example.test" {
			t.Fatalf("unexpected detached provision plan: %#v", got)
		}
		return json.RawMessage(`{"client":"88","cpu":2,"memory":4096,"os":"3"}`), nil
	}, plan)
	if err != nil {
		t.Fatalf("BuildProvisionParameters() error = %v", err)
	}
	if string(parameters) != `{"client":"88","cpu":2,"memory":4096,"os":"3"}` {
		t.Fatalf("unexpected parameters %s", parameters)
	}

	secretInputs := []ServerProvisionPlan{
		func() ServerProvisionPlan {
			value := validServerProvisionPlan()
			value.Selector = json.RawMessage(`{"password":"plaintext"}`)
			return value
		}(),
		func() ServerProvisionPlan {
			value := validServerProvisionPlan()
			value.Components[0].Code = "access-token=plaintext"
			return value
		}(),
	}
	for _, value := range secretInputs {
		if _, err = BuildProvisionParameters(context.Background(), func(
			context.Context,
			ServerProvisionPlan,
		) (json.RawMessage, error) {
			t.Fatal("builder must not be called for a secret-bearing plan")
			return nil, nil
		}, value); err == nil {
			t.Fatal("secret-bearing plan was accepted")
		}
	}

	if _, err = BuildProvisionParameters(context.Background(), func(
		context.Context,
		ServerProvisionPlan,
	) (json.RawMessage, error) {
		return json.RawMessage(`{"access_token":"plaintext"}`), nil
	}, validServerProvisionPlan()); err == nil {
		t.Fatal("secret-bearing connector result was accepted")
	}
}

func validServerProvisionPlan() ServerProvisionPlan {
	return ServerProvisionPlan{
		ExternalPrincipalRef:  "88",
		Hostname:              "server-1.example.test",
		Selector:              json.RawMessage(`{"provider":"zjmfcloud-v1"}`),
		ProductConfiguration:  json.RawMessage(`{}`),
		CustomerConfiguration: json.RawMessage(`{}`),
		Components: []ServerProvisionComponent{{
			Type: ServerProvisionComponentCPU, Cores: 2, SelectionQuantity: 1,
		}},
	}
}

// TestServerProvisionPlanAcceptsOnlyResourceOwnedServerComponents keeps the
// complete server shape in resourcecap instead of the generic commerce model.
func TestServerProvisionPlanAcceptsOnlyResourceOwnedServerComponents(t *testing.T) {
	t.Parallel()
	types := []ServerProvisionComponentType{
		ServerProvisionComponentRegion,
		ServerProvisionComponentLine,
		ServerProvisionComponentCPU,
		ServerProvisionComponentMemory,
		ServerProvisionComponentOperatingSystem,
		ServerProvisionComponentSystemDisk,
		ServerProvisionComponentDataDisk,
		ServerProvisionComponentBandwidth,
	}
	plan := validServerProvisionPlan()
	plan.Components = make([]ServerProvisionComponent, 0, len(types))
	for _, componentType := range types {
		plan.Components = append(plan.Components, ServerProvisionComponent{
			Type: componentType, Code: "server-value", SelectionQuantity: 1,
		})
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("resource-owned server configuration was rejected: %v", err)
	}
}
