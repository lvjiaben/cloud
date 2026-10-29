// This file verifies recovery credential consumption and the replay outcome projection.
package admin

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
)

// TestProviderConfigurationRecoveryCreateConsumesCredentials verifies both
// request strings and controller-owned mutable copies are cleared.
func TestProviderConfigurationRecoveryCreateConsumesCredentials(t *testing.T) {
	stub := &providerConfigurationServiceStub{recoveryResult: &providerconfig.Recovery{
		Configuration: &providerconfig.Configuration{
			ProviderCode: "aliyun_face", Status: "configured", SceneID: "100000001", Version: 2,
		},
		ReusedExistingMutation: true,
	}}
	controller, err := NewV1(stub, &enterpriseManagementServiceStub{}, &supplierConfigurationStub{})
	if err != nil {
		t.Fatal(err)
	}
	request := &v1.ProviderConfigurationRecoveryCreateReq{
		AccessKeyID: "merchant-access-key-id", AccessKeySecret: "merchant-access-key-secret-0001",
	}
	result, err := controller.ProviderConfigurationRecoveryCreate(context.Background(), request)
	if err != nil || result == nil || result.Item == nil || result.Item.Version != 2 ||
		!result.ReusedExistingMutation {
		t.Fatalf("configuration recovery result=%+v err=%v", result, err)
	}
	if request.AccessKeyID != "" || request.AccessKeySecret != "" {
		t.Fatal("controller retained recovery credential request strings")
	}
	for _, value := range append(stub.recoveryInput.AccessKeyID, stub.recoveryInput.AccessKeySecret...) {
		if value != 0 {
			t.Fatal("controller-owned recovery credential copy was not cleared")
		}
	}
}

// TestProviderConfigurationRecoveryCreateRejectsMissingRequest verifies nil
// input returns the stable validation business error.
func TestProviderConfigurationRecoveryCreateRejectsMissingRequest(t *testing.T) {
	controller, err := NewV1(&providerConfigurationServiceStub{}, &enterpriseManagementServiceStub{}, &supplierConfigurationStub{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = controller.ProviderConfigurationRecoveryCreate(context.Background(), nil); !bizerr.Is(err, providerconfig.CodeInvalid) {
		t.Fatalf("nil recovery request error=%v", err)
	}
}
