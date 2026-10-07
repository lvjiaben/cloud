// This file handles recent-login-bound enterprise-material access issuance.

package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
)

// EnterpriseVerificationMaterialAccessCreate issues one short-lived access context.
func (c *ControllerV1) EnterpriseVerificationMaterialAccessCreate(
	ctx context.Context,
	req *v1.EnterpriseVerificationMaterialAccessCreateReq,
) (*v1.EnterpriseVerificationMaterialAccessCreateRes, error) {
	issued, err := c.enterpriseVerification.IssueMaterialAccess(ctx, req.CaseNo, req.MaterialNo)
	if err != nil {
		return nil, err
	}
	return &v1.EnterpriseVerificationMaterialAccessCreateRes{
		AccessContext: issued.AccessContext, ExpiresAt: issued.ExpiresAt.UnixMilli(),
	}, nil
}
