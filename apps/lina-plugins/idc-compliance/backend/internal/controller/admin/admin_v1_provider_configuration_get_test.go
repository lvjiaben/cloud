// This file verifies secret-safe controller projection and constructor validation.
package admin

import (
	"context"
	"testing"
	"time"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontractonboarding"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontractpreparation"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontracttemplate"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
)

type enterpriseManagementServiceStub struct{}

type electronicContractOnboardingStub struct{}

type electronicContractPreparationStub struct {
	electroniccontractpreparation.Service
	result              *electroniccontractpreparation.PreparedContract
	templates           []electroniccontractpreparation.ManualOptionalTemplate
	err                 error
	input               electroniccontractpreparation.ManualOptionalInput
	templateListOrderNo string
	options             *electroniccontractpreparation.ManualOptionalOptions
	submitted           *electroniccontractpreparation.SubmittedSigning
	submitInput         electroniccontractpreparation.SubmitSigningInput
	replacementInput    electroniccontractpreparation.PrepareReplacementInput
	replacementOptions  *electroniccontractpreparation.ReplacementOptions
	replacementOptionNo string
	reissueInput        electroniccontractpreparation.ReissueSigningInput
	terminationInput    electroniccontractpreparation.TerminateContractInput
}

func (s *electronicContractPreparationStub) GetReplacementOptions(
	_ context.Context,
	contractNo string,
) (*electroniccontractpreparation.ReplacementOptions, error) {
	s.replacementOptionNo = contractNo
	return s.replacementOptions, s.err
}

func (s *electronicContractPreparationStub) SubmitSigning(
	_ context.Context,
	input electroniccontractpreparation.SubmitSigningInput,
) (*electroniccontractpreparation.SubmittedSigning, error) {
	s.submitInput = input
	return s.submitted, s.err
}

func (s *electronicContractPreparationStub) PrepareReplacement(
	_ context.Context,
	input electroniccontractpreparation.PrepareReplacementInput,
) (*electroniccontractpreparation.PreparedContract, error) {
	s.replacementInput = input
	return s.result, s.err
}

func (s *electronicContractPreparationStub) SubmitReissue(
	_ context.Context,
	input electroniccontractpreparation.ReissueSigningInput,
) (*electroniccontractpreparation.SubmittedSigning, error) {
	s.reissueInput = input
	return s.submitted, s.err
}

func (s *electronicContractPreparationStub) TerminateContract(
	_ context.Context,
	input electroniccontractpreparation.TerminateContractInput,
) (*electroniccontractpreparation.SubmittedSigning, error) {
	s.terminationInput = input
	return s.submitted, s.err
}

func (s *electronicContractPreparationStub) ListManualOptionalTemplates(
	_ context.Context,
	orderNo string,
) ([]electroniccontractpreparation.ManualOptionalTemplate, error) {
	s.templateListOrderNo = orderNo
	return s.templates, s.err
}

func (s *electronicContractPreparationStub) GetManualOptionalOptions(
	_ context.Context,
	orderNo string,
) (*electroniccontractpreparation.ManualOptionalOptions, error) {
	s.templateListOrderNo = orderNo
	if s.options != nil {
		return s.options, s.err
	}
	return &electroniccontractpreparation.ManualOptionalOptions{
		OrderNo: orderNo, Templates: s.templates,
	}, s.err
}

func (s *electronicContractPreparationStub) PrepareManualOptional(
	_ context.Context,
	input electroniccontractpreparation.ManualOptionalInput,
) (*electroniccontractpreparation.PreparedContract, error) {
	s.input = input
	return s.result, s.err
}

type electronicContractTemplateStub struct {
	electroniccontracttemplate.Service
}

func (*electronicContractOnboardingStub) Get(context.Context) (*electroniccontractonboarding.View, error) {
	return &electroniccontractonboarding.View{}, nil
}
func (*electronicContractOnboardingStub) CreateLink(context.Context, string) (*electroniccontractonboarding.View, error) {
	return &electroniccontractonboarding.View{}, nil
}
func (*electronicContractOnboardingStub) Recheck(context.Context, string) (*electroniccontractonboarding.View, error) {
	return &electroniccontractonboarding.View{}, nil
}

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
	controller, err := NewV1(stub, &enterpriseManagementServiceStub{}, &supplierConfigurationStub{}, &electronicContractOnboardingStub{}, &electronicContractPreparationStub{}, &electronicContractTemplateStub{})
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
	if _, err := NewV1(service, &enterpriseManagementServiceStub{}, &supplierConfigurationStub{}, &electronicContractOnboardingStub{}, &electronicContractPreparationStub{}, &electronicContractTemplateStub{}); err == nil {
		t.Fatal("typed-nil provider configuration service was accepted")
	}
}
