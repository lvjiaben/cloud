// This file verifies the split supplier configuration administration contracts.

package admin

import (
	"context"
	"testing"

	v1 "lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/supplierconfig"
)

type supplierConfigurationStub struct {
	supplierconfig.Service
	input         supplierconfig.CredentialSaveInput
	sawCredential bool
}

func (s *supplierConfigurationStub) Get(
	context.Context,
	string,
) (*supplierconfig.Configuration, error) {
	return &supplierconfig.Configuration{}, nil
}

func (s *supplierConfigurationStub) SaveCredential(
	_ context.Context,
	input supplierconfig.CredentialSaveInput,
) (*supplierconfig.Configuration, error) {
	s.sawCredential = string(input.CredentialEnvelope) == "opaque-local-envelope"
	input.CredentialEnvelope = nil
	s.input = input
	return &supplierconfig.Configuration{
		ProviderCode:     input.ProviderCode,
		CredentialStatus: supplierconfig.CredentialConfigured,
		SubjectStatus:    supplierconfig.SubjectUnbound, CredentialConfigured: true,
		CapabilityEvidenceCode: supplierconfig.EvidenceExternalConfirmationRequired,
		AdapterEnabled:         false, Version: 2,
	}, nil
}

func TestSupplierConfigurationUpdateConsumesCredentialAndStaysBlocked(t *testing.T) {
	stub := &supplierConfigurationStub{}
	controller := &ControllerV1{supplierConfiguration: stub}
	request := &v1.SupplierCredentialUpdateReq{
		ProviderCode:       supplierconfig.ProviderTencentESSBasic,
		CredentialEnvelope: "opaque-local-envelope", IdempotencyKey: "supplier-config-0001",
	}
	result, err := controller.SupplierCredentialUpdate(context.Background(), request)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if request.CredentialEnvelope != "" || !stub.sawCredential || stub.input.CredentialEnvelope != nil {
		t.Fatalf("credential request=%q observed=%v retained=%v", request.CredentialEnvelope, stub.sawCredential, stub.input.CredentialEnvelope)
	}
	if result.Item.AdapterEnabled ||
		result.Item.CapabilityEvidenceCode != supplierconfig.EvidenceExternalConfirmationRequired {
		t.Fatalf("supplier gate projection=%+v", result.Item)
	}
}
