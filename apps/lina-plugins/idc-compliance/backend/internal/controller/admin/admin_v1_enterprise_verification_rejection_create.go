// This file handles actionable enterprise-verification rejection commands.

package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
)

// EnterpriseVerificationRejectionCreate rejects with one actionable reason code.
func (c *ControllerV1) EnterpriseVerificationRejectionCreate(
	ctx context.Context,
	req *v1.EnterpriseVerificationRejectionCreateReq,
) (*v1.EnterpriseVerificationRejectionCreateRes, error) {
	result, err := c.enterpriseVerification.Reject(ctx, enterpriseverification.DecisionInput{
		CaseNo: req.CaseNo, ExpectedVersion: req.ExpectedVersion,
		ReasonCode: req.ReasonCode, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.EnterpriseVerificationRejectionCreateRes{
		Item: enterpriseReviewResult(result),
	}, nil
}
