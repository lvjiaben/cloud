// This file verifies secret-safe controller projection and constructor validation.
package admin

import (
	"context"
	"testing"
	"time"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
)

type enterpriseManagementServiceStub struct{}













func (*enterpriseManagementServiceStub) ListCases(context.Context, enterpriseverification.CaseListInput) (*enterpriseverification.CaseList, error) {
	return &enterpriseverification.CaseList{}, nil
}
func (*enterpriseManagementServiceStub) GetCase(context.Context, string) (*enterpriseverification.CaseDetail, error) {
	return &enterpriseverification.CaseDetail{}, nil
}
func (*enterpriseManagementServiceStub) IssueMaterialAccess(context.Context, string, string) (*enterpriseverification.MaterialAccessContext, error) {
	return &enterpriseverification.MaterialAccessContext{}, nil
}
func (*enterpriseManagementServiceStub) OpenMaterial(context.Context, string, string, string) (*enterpriseverification.MaterialContent, error) {
	return &enterpriseverification.MaterialContent{}, nil
}
func (*enterpriseManagementServiceStub) StartReview(context.Context, enterpriseverification.ReviewInput) (*enterpriseverification.ReviewResult, error) {
	return &enterpriseverification.ReviewResult{}, nil
}
func (*enterpriseManagementServiceStub) Approve(context.Context, enterpriseverification.DecisionInput) (*enterpriseverification.ReviewResult, error) {
	return &enterpriseverification.ReviewResult{}, nil
}
func (*enterpriseManagementServiceStub) Reject(context.Context, enterpriseverification.DecisionInput) (*enterpriseverification.ReviewResult, error) {
	return &enterpriseverification.ReviewResult{}, nil
}

type providerConfigurationServiceStub struct {
	getResult        *providerconfig.Configuration
	getError         error
	saveResult       *providerconfig.Configuration
	saveError        error
	saveInput        providerconfig.SaveInput
	recoveryResult   *providerconfig.Recovery
	recoveryError    error
	recoveryInput    providerconfig.RecoveryInput
	validationResult *providerconfig.Configuration
	validationError  error
}

func (s *providerConfigurationServiceStub) ValidateConnectivity(
	context.Context,
) (*providerconfig.Configuration, error) {
	return s.validationResult, s.validationError
}

func (s *providerConfigurationServiceStub) Recover(
	_ context.Context,
	input providerconfig.RecoveryInput,
) (*providerconfig.Recovery, error) {
	s.recoveryInput = input
	return s.recoveryResult, s.recoveryError
}

func (s *providerConfigurationServiceStub) Get(context.Context) (*providerconfig.Configuration, error) {
	return s.getResult, s.getError
}

func (s *providerConfigurationServiceStub) Save(
	_ context.Context,
	input providerconfig.SaveInput,
) (*providerconfig.Configuration, error) {
	s.saveInput = input
	return s.saveResult, s.saveError
}

// TestProviderConfigurationGetMapsUnixMilliseconds verifies the public time boundary.
func TestProviderConfigurationGetMapsUnixMilliseconds(t *testing.T) {
	updatedAt := time.Date(2026, 8, 12, 8, 30, 0, 123_000_000, time.UTC)
	stub := &providerConfigurationServiceStub{getResult: &providerconfig.Configuration{
		ProviderCode: "aliyun_face", Status: "configured", SceneID: "100000001",
		CredentialConfigured: true, CredentialMaskedTail: "K001",
		CredentialGeneration: 2, Version: 3, UpdatedAt: &updatedAt,
	}}
	controller, err := NewV1(stub, &enterpriseManagementServiceStub{}, &supplierConfigurationStub{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := controller.ProviderConfigurationGet(
		context.Background(), &v1.ProviderConfigurationGetReq{},
	)
	if err != nil || result == nil || result.Item == nil || result.Item.UpdatedAt == nil ||
		*result.Item.UpdatedAt != updatedAt.UnixMilli() ||
		result.Item.CredentialMaskedTail != "K001" || result.Item.Version != 3 {
		t.Fatalf("configuration result=%+v err=%v", result, err)
	}
}

// TestNewV1RejectsTypedNilService verifies startup fails closed on invalid DI.
func TestNewV1RejectsTypedNilService(t *testing.T) {
	var service *providerConfigurationServiceStub
	if _, err := NewV1(service, &enterpriseManagementServiceStub{}, &supplierConfigurationStub{}); err == nil {
		t.Fatal("typed-nil provider configuration service was accepted")
	}
}
