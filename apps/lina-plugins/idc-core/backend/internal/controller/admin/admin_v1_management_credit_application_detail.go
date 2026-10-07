// This file serves one authorized credit application management aggregate.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

// ManagementCreditApplicationDetail returns one authorized application aggregate.
func (c *ControllerV1) ManagementCreditApplicationDetail(ctx context.Context, req *v1.ManagementCreditApplicationDetailReq) (res *v1.ManagementCreditApplicationDetailRes, err error) {
	detail, err := c.credit.GetManagementCreditApplication(ctx, req.ApplicationNo)
	if err != nil {
		return nil, err
	}
	evidence, err := c.credit.ListManagementCreditEvidence(ctx, req.ApplicationNo)
	if err != nil {
		return nil, err
	}
	mapped := make([]*v1.ManagementCreditEvidence, 0, len(evidence))
	for _, item := range evidence {
		mapped = append(mapped, &v1.ManagementCreditEvidence{UploadNo: item.UploadNo, FileName: item.FileName, MediaType: item.MediaType, SizeBytes: item.SizeBytes})
	}
	return &v1.ManagementCreditApplicationDetailRes{Detail: toAPIManagementCreditApplicationDetail(detail), Evidence: mapped}, nil
}
