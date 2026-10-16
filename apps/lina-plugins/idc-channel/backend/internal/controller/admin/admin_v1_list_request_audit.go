// 本文件将后台分页读取指定授权的请求记录，不扩大可见范围。

package admin

import (
	"context"

	"lina-plugin-idc-channel/backend/api/admin/v1"
)

// ListRequestAudit 分页读取指定授权的请求记录，不扩大可见范围。
func (c *ControllerV1) ListRequestAudit(ctx context.Context, req *v1.ListRequestAuditReq) (*v1.ListRequestAuditRes, error) {
	return c.delegate.ListRequestAudit(ctx, req)
}
