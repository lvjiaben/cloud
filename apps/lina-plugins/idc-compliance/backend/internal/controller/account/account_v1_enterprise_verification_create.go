// This file maps customer-owned enterprise draft creation to the service.

package account

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
)

// EnterpriseVerificationCreate creates or safely replays one owned draft.
func (c *ControllerV1) EnterpriseVerificationCreate(ctx context.Context, req *v1.EnterpriseVerificationCreateReq) (res *v1.EnterpriseVerificationCreateRes, err error) {
	if req == nil {
		return nil, bizerr.NewCode(enterpriseverification.CodeInvalid)
	}
	item, err := c.enterprise.CreateDraft(ctx, enterpriseverification.CreateDraftInput{
		ConsentNo: req.ConsentNo, LegalChangeRequestNo: req.LegalChangeRequestNo,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.EnterpriseVerificationCreateRes{Item: &v1.EnterpriseVerificationDraftItem{
		CaseNo: item.CaseNo, StatusCode: item.StatusCode,
		LegalChangeRequestNo: item.LegalChangeRequestNo,
		Reverification:       item.Reverification, Reused: item.Reused,
	}}, nil
}
