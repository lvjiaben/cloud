// This file streams reserved raw enterprise evidence into the encrypted service.

package account

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
)

// EnterpriseVerificationMaterialContentUpdate encrypts and confirms one reserved original.
func (c *ControllerV1) EnterpriseVerificationMaterialContentUpdate(ctx context.Context, req *v1.EnterpriseVerificationMaterialContentUpdateReq) (res *v1.EnterpriseVerificationMaterialContentUpdateRes, err error) {
	request := g.RequestFromCtx(ctx)
	if req == nil || request == nil || request.Request == nil || request.Request.Body == nil {
		return nil, bizerr.NewCode(enterpriseverification.CodeInvalid)
	}
	item, err := c.enterprise.UploadMaterial(ctx, enterpriseverification.UploadMaterialInput{
		CaseNo: req.CaseNo, MaterialNo: req.MaterialNo,
		DeclaredMediaType: request.GetHeader("Content-Type"), Body: request.Request.Body,
	})
	if err != nil {
		return nil, err
	}
	return &v1.EnterpriseVerificationMaterialContentUpdateRes{Item: &v1.EnterpriseVerificationMaterialItem{
		MaterialNo: item.MaterialNo, MaterialTypeCode: string(item.MaterialType),
		SequenceNo: item.SequenceNo, MediaType: item.MediaType, ByteSize: item.ByteSize,
		StatusCode: item.StatusCode, Reused: item.Reused,
	}}, nil
}
