// This file streams one audited, one-time enterprise-material attachment.

package admin

import (
	"context"
	"strconv"

	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
)

// EnterpriseVerificationMaterialContentCreate consumes one proof and streams an attachment.
func (c *ControllerV1) EnterpriseVerificationMaterialContentCreate(
	ctx context.Context,
	req *v1.EnterpriseVerificationMaterialContentCreateReq,
) (*v1.EnterpriseVerificationMaterialContentCreateRes, error) {
	request := g.RequestFromCtx(ctx)
	if req == nil || request == nil {
		return nil, bizerr.NewCode(enterpriseverification.CodeInvalid)
	}
	content, err := c.enterpriseVerification.OpenMaterial(
		ctx, req.CaseNo, req.MaterialNo, req.AccessContext,
	)
	if err != nil {
		return nil, err
	}
	defer content.Clear()
	extension := materialExtension(content.MediaType)
	request.Response.Header().Set("Content-Type", content.MediaType)
	request.Response.Header().Set("Content-Length", strconv.Itoa(len(content.Content)))
	request.Response.Header().Set("Content-Disposition", "attachment; filename=enterprise-material."+extension)
	request.Response.Header().Set("Cache-Control", "no-store, max-age=0")
	request.Response.Header().Set("Pragma", "no-cache")
	request.Response.Header().Set("X-Content-Type-Options", "nosniff")
	request.Response.WriteOver(content.Content)
	request.ExitAll()
	return nil, nil
}

// materialExtension maps the closed content-sniffed media catalog to a safe suffix.
func materialExtension(mediaType string) string {
	switch mediaType {
	case "application/pdf":
		return "pdf"
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	default:
		return "bin"
	}
}
