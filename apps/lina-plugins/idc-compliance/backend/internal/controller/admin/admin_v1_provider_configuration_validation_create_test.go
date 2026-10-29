// This file verifies connectivity status and Unix-millisecond projection.
package admin

import (
	"context"
	"testing"
	"time"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
)

// TestProviderConfigurationValidationCreateMapsBoundedOutcome verifies no raw
// provider details are needed to present the persisted validation result.
func TestProviderConfigurationValidationCreateMapsBoundedOutcome(t *testing.T) {
	validatedAt := time.Date(2026, 8, 12, 9, 0, 0, 456_000_000, time.UTC)
	stub := &providerConfigurationServiceStub{validationResult: &providerconfig.Configuration{
		ProviderCode: "aliyun_face", Status: "configured", SceneID: "100000001",
		ValidationStatus: providerconfig.ValidationStatusReachable,
		ValidatedAt:      &validatedAt, Version: 2,
	}}
	controller, err := NewV1(stub, &enterpriseManagementServiceStub{}, &supplierConfigurationStub{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := controller.ProviderConfigurationValidationCreate(
		context.Background(), &v1.ProviderConfigurationValidationCreateReq{},
	)
	if err != nil || result == nil || result.Item == nil ||
		result.Item.ValidationStatus != "reachable" || result.Item.ValidatedAt == nil ||
		*result.Item.ValidatedAt != validatedAt.UnixMilli() {
		t.Fatalf("validation result=%+v err=%v", result, err)
	}
}
