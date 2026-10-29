// This file verifies credential request consumption and safe update projection.
package admin

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
)

// TestProviderConfigurationUpdateConsumesRequestCredentials verifies request fields and copies are cleared.
func TestProviderConfigurationUpdateConsumesRequestCredentials(t *testing.T) {
	stub := &providerConfigurationServiceStub{saveResult: &providerconfig.Configuration{
		ProviderCode: "aliyun_face", Status: "configured", SceneID: "100000001", Version: 2,
	}}
	controller, err := NewV1(stub, &enterpriseManagementServiceStub{}, &supplierConfigurationStub{})
	if err != nil {
		t.Fatal(err)
	}
	request := &v1.ProviderConfigurationUpdateReq{
		SceneID: "100000001", AccessKeyID: "merchant-access-key-id",
		AccessKeySecret: "merchant-access-key-secret-0001",
		IdempotencyKey:  "credential-controller-0001",
	}
	result, err := controller.ProviderConfigurationUpdate(context.Background(), request)
	if err != nil || result == nil || result.Item == nil || result.Item.Version != 2 {
		t.Fatalf("configuration update result=%+v err=%v", result, err)
	}
	if request.AccessKeyID != "" || request.AccessKeySecret != "" {
		t.Fatal("controller retained credential request strings")
	}
	if stub.saveInput.SceneID != "100000001" ||
		stub.saveInput.IdempotencyKey != "credential-controller-0001" {
		t.Fatalf("save input=%+v", stub.saveInput)
	}
	for _, value := range append(stub.saveInput.AccessKeyID, stub.saveInput.AccessKeySecret...) {
		if value != 0 {
			t.Fatal("controller-owned credential copy was not cleared")
		}
	}
}

// TestProviderConfigurationUpdateRejectsMissingRequest verifies nil input is a stable business error.
func TestProviderConfigurationUpdateRejectsMissingRequest(t *testing.T) {
	controller, err := NewV1(&providerConfigurationServiceStub{}, &enterpriseManagementServiceStub{}, &supplierConfigurationStub{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = controller.ProviderConfigurationUpdate(context.Background(), nil); !bizerr.Is(err, providerconfig.CodeInvalid) {
		t.Fatalf("nil request error=%v", err)
	}
}
