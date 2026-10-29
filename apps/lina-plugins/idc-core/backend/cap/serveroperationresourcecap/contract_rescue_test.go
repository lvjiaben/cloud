// These tests preserve the frozen rescue input boundary without creating a
// generated-credential output that the upstream rescue API does not provide.
package serveroperationresourcecap

import (
	"encoding/json"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"testing"
)

func TestRescueTaskContract(t *testing.T) {
	handle, err := secretcap.NewHandle("rescue-test-secret")
	if err != nil {
		t.Fatal(err)
	}
	request := TaskRequest{OperationNo: "operation-rescue", ServiceNo: "resource-1", ChannelNo: "channel-1", PoolNo: "pool-1", ExpectedServiceVersion: 1, Action: ActionEnterRescue, Parameters: json.RawMessage(`{"system":"linux"}`), SecretBindings: []SecretBinding{{Name: "temporary_password", Handle: handle}}}
	request.RequestDigest, err = TaskRequestDigest(request)
	if err != nil || ValidateTaskRequest(request) != nil {
		t.Fatal("valid rescue rejected", err)
	}
	for _, mutate := range []func(*TaskRequest){
		func(r *TaskRequest) { r.SecretBindings = nil },
		func(r *TaskRequest) { r.SecretBindings[0].Name = "password" },
		func(r *TaskRequest) { r.Parameters = []byte(`{"system":"other"}`) },
		func(r *TaskRequest) { r.Parameters = []byte(`{"system":"linux","temp_pass":"plaintext"}`) },
		func(r *TaskRequest) { r.Action = ActionExitRescue },
	} {
		changed := request
		changed.SecretBindings = append([]SecretBinding(nil), request.SecretBindings...)
		mutate(&changed)
		if _, err = TaskRequestDigest(changed); err == nil {
			t.Fatal("invalid rescue accepted")
		}
	}
	request.Action = ActionExitRescue
	request.Parameters = []byte(`{}`)
	request.SecretBindings = nil
	request.RequestDigest, err = TaskRequestDigest(request)
	if err != nil || ValidateTaskRequest(request) != nil {
		t.Fatal("valid exit rejected", err)
	}
}
