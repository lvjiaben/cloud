package account

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) CreditEvidenceUploadCreate(ctx context.Context, req *v1.CreditEvidenceUploadCreateReq) (*v1.CreditEvidenceUploadCreateRes, error) {
	item, err := c.creditCommands.ReserveCreditEvidence(ctx, req.FileName, req.SizeBytes)
	if err != nil {
		return nil, err
	}
	return &v1.CreditEvidenceUploadCreateRes{UploadNo: item.UploadNo, ExpiresAt: item.ExpiresAt.UnixMilli()}, nil
}

func (c *ControllerV1) CreditEvidenceUploadContentUpdate(ctx context.Context, req *v1.CreditEvidenceUploadContentUpdateReq) (*v1.CreditEvidenceUploadContentUpdateRes, error) {
	request := g.RequestFromCtx(ctx)
	if request == nil || request.Request == nil || request.Request.Body == nil {
		return nil, bizerr.NewCode(billing.CodeCreditInvalid)
	}
	item, err := c.creditCommands.UploadCreditEvidence(ctx, req.UploadNo, request.GetHeader("Content-Type"), request.Request.Body)
	if err != nil {
		return nil, err
	}
	return &v1.CreditEvidenceUploadContentUpdateRes{UploadNo: item.UploadNo, FileName: item.FileName, MediaType: item.MediaType, SizeBytes: item.SizeBytes}, nil
}
