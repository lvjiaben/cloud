package admin

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) ManagementCreditEvidenceOpen(ctx context.Context, req *v1.ManagementCreditEvidenceOpenReq) (*v1.ManagementCreditEvidenceOpenRes, error) {
	file, err := c.credit.OpenManagementCreditEvidence(ctx, req.ApplicationNo, req.UploadNo)
	if err != nil {
		return nil, err
	}
	defer file.Body.Close()
	request := g.RequestFromCtx(ctx)
	if request == nil {
		return nil, bizerr.NewCode(billing.CodeCreditUnavailable)
	}
	request.Response.Header().Set("Content-Type", file.MediaType)
	request.Response.Header().Set("Content-Disposition", "inline; filename*=UTF-8''"+url.PathEscape(file.FileName))
	request.Response.Header().Set("Cache-Control", "private, no-store")
	request.Response.Header().Set("Content-Length", fmt.Sprint(file.SizeBytes))
	if _, err = io.Copy(request.Response.Writer, file.Body); err != nil {
		return nil, err
	}
	return &v1.ManagementCreditEvidenceOpenRes{}, nil
}
