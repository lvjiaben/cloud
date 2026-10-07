package contentadmin

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/contenthub"
)

type uploadPolicyServiceStub struct {
	contenthub.Service
	policy *contenthub.UploadPolicy
	err    error
	calls  int
}

func (s *uploadPolicyServiceStub) GetUploadPolicy(context.Context) (*contenthub.UploadPolicy, error) {
	s.calls++
	return s.policy, s.err
}

func TestContentUploadPolicyProjectsOnlyBusinessFields(t *testing.T) {
	stub := &uploadPolicyServiceStub{policy: &contenthub.UploadPolicy{
		OrdinaryMaxSizeBytes:           5000,
		InspectionRequiredMaxSizeBytes: 2000,
		InspectionRequiredAvailable:    true,
		OrdinaryExtensions:             []string{".pdf"},
		InspectionRequiredExtensions:   []string{".zip"},
	}}
	controller, err := NewV1(stub)
	if err != nil {
		t.Fatal(err)
	}
	response, err := controller.ContentUploadPolicy(context.Background(), &v1.ContentUploadPolicyReq{})
	if err != nil {
		t.Fatal(err)
	}
	if stub.calls != 1 || response.OrdinaryMaxSizeBytes != 5000 || response.InspectionRequiredMaxSizeBytes != 2000 || !response.InspectionRequiredAvailable || !reflect.DeepEqual(response.OrdinaryExtensions, []string{".pdf"}) || !reflect.DeepEqual(response.InspectionRequiredExtensions, []string{".zip"}) {
		t.Fatalf("unexpected upload policy response: %+v (calls=%d)", response, stub.calls)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	expectedFields := []string{"ordinaryMaxSizeBytes", "inspectionRequiredMaxSizeBytes", "inspectionRequiredAvailable", "ordinaryExtensions", "inspectionRequiredExtensions"}
	if len(fields) != len(expectedFields) {
		t.Fatalf("upload policy leaked unexpected fields: %s", encoded)
	}
	for _, field := range expectedFields {
		if _, found := fields[field]; !found {
			t.Fatalf("upload policy omitted %q: %s", field, encoded)
		}
	}
}
