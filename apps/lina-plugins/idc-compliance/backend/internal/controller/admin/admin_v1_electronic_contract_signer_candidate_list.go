package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/contractsigner"
)

func (c *ControllerV1) ElectronicContractSignerCandidateList(ctx context.Context, req *v1.ElectronicContractSignerCandidateListReq) (*v1.ElectronicContractSignerCandidateListRes, error) {
	if c == nil || c.contractSigners == nil || req == nil {
		return nil, bizerr.NewCode(contractsigner.CodeUnavailable)
	}
	items, err := c.contractSigners.ListCandidates(ctx, req.CustomerNo)
	if err != nil {
		return nil, err
	}
	result := make([]v1.ElectronicContractSignerCandidateItem, len(items))
	for index, item := range items {
		result[index] = v1.ElectronicContractSignerCandidateItem{SignerNo: item.SignerNo, MaskedName: item.MaskedName, MaskedPhone: item.MaskedPhone, IdentityVerificationVersion: item.IdentityVerificationVersion, IdentityVerifiedAt: item.IdentityVerifiedAt.UnixMilli(), PhoneVerificationVersion: item.PhoneVerificationVersion, PhoneVerifiedAt: item.PhoneVerifiedAt.UnixMilli(), AuthorizationVersion: item.AuthorizationVersion, Version: item.Version}
	}
	return &v1.ElectronicContractSignerCandidateListRes{Items: result}, nil
}
