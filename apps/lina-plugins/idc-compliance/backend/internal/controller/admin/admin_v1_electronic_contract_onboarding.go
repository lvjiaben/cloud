// This file adapts Tencent onboarding services to management API DTOs.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontractonboarding"
	"lina-plugin-idc-compliance/backend/internal/service/supplierconfig"
)

// ElectronicContractOnboardingGet returns the current secret-free onboarding state.
func (c *ControllerV1) ElectronicContractOnboardingGet(
	ctx context.Context,
	_ *v1.ElectronicContractOnboardingGetReq,
) (*v1.ElectronicContractOnboardingGetRes, error) {
	if c == nil || c.electronicContract == nil {
		return nil, bizerr.NewCode(supplierconfig.CodeUnavailable)
	}
	item, err := c.electronicContract.Get(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.ElectronicContractOnboardingGetRes{Item: mapElectronicContractOnboarding(item)}, nil
}

// ElectronicContractOnboardingLinkCreate creates one official short-lived Tencent URL.
func (c *ControllerV1) ElectronicContractOnboardingLinkCreate(
	ctx context.Context,
	req *v1.ElectronicContractOnboardingLinkCreateReq,
) (*v1.ElectronicContractOnboardingLinkCreateRes, error) {
	if c == nil || c.electronicContract == nil || req == nil {
		return nil, bizerr.NewCode(supplierconfig.CodeUnavailable)
	}
	item, err := c.electronicContract.CreateLink(ctx, req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	return &v1.ElectronicContractOnboardingLinkCreateRes{Item: mapElectronicContractOnboarding(item)}, nil
}

// ElectronicContractOnboardingRecheckCreate performs an explicit authoritative recheck.
func (c *ControllerV1) ElectronicContractOnboardingRecheckCreate(
	ctx context.Context,
	req *v1.ElectronicContractOnboardingRecheckCreateReq,
) (*v1.ElectronicContractOnboardingRecheckCreateRes, error) {
	if c == nil || c.electronicContract == nil || req == nil {
		return nil, bizerr.NewCode(supplierconfig.CodeUnavailable)
	}
	item, err := c.electronicContract.Recheck(ctx, req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	return &v1.ElectronicContractOnboardingRecheckCreateRes{Item: mapElectronicContractOnboarding(item)}, nil
}

func mapElectronicContractOnboarding(
	item *electroniccontractonboarding.View,
) *v1.ElectronicContractOnboardingItem {
	if item == nil {
		return nil
	}
	result := &v1.ElectronicContractOnboardingItem{
		Merchant: v1.ElectronicContractMerchantProfile{
			LegalName: item.Merchant.LegalName, CountryCode: item.Merchant.CountryCode,
			MerchantNo: item.Merchant.MerchantNo,
		},
		State: item.State, ReasonCode: item.ReasonCode, ConsoleURL: item.ConsoleURL,
		OrganizationReady: item.OrganizationReady, OperatorReady: item.OperatorReady,
		BindingNo: item.BindingNo, SubjectStatus: item.SubjectStatus,
		Capabilities: make([]v1.ElectronicContractCapability, 0, len(item.Capabilities)),
	}
	for _, capability := range item.Capabilities {
		result.Capabilities = append(result.Capabilities, v1.ElectronicContractCapability{
			Code: capability.Code, State: capability.State,
			ReasonCode: capability.ReasonCode, CheckedAt: capability.CheckedAt,
		})
	}
	return result
}
