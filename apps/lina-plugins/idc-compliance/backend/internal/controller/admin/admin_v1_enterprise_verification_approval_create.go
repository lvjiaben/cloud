// This file handles the server-authoritative enterprise-verification approval command.

package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
)

// EnterpriseVerificationApprovalCreate approves only server-authoritative snapshot values.
func (c *ControllerV1) EnterpriseVerificationApprovalCreate(
	ctx context.Context,
	req *v1.EnterpriseVerificationApprovalCreateReq,
) (*v1.EnterpriseVerificationApprovalCreateRes, error) {
	result, err := c.enterpriseVerification.Approve(ctx, enterpriseverification.DecisionInput{
		CaseNo: req.CaseNo, ExpectedVersion: req.ExpectedVersion,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.EnterpriseVerificationApprovalCreateRes{
		Item: enterpriseReviewResult(result),
	}, nil
}
