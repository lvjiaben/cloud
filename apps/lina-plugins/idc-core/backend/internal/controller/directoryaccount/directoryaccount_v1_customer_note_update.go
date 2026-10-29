// This file projects the exact-customer note through the shared directory owner.
package directoryaccount

import (
	"context"
	"lina-plugin-idc-core/backend/api/directoryaccount/v1"
	"lina-plugin-idc-core/backend/internal/service/productdirectory"
)

func (c *ControllerV1) CustomerNoteUpdate(ctx context.Context, req *v1.CustomerNoteUpdateReq) (*v1.CustomerNoteUpdateRes, error) {
	note, err := c.service.SaveNote(ctx, productdirectory.CustomerNote{EntitlementNo: req.EntitlementNo, Remark: req.Remark, Revision: req.ExpectedRevision})
	if err != nil {
		return nil, err
	}
	return &v1.CustomerNoteUpdateRes{Note: v1.CustomerNote{EntitlementNo: note.EntitlementNo, Remark: note.Remark, Revision: note.Revision}}, nil
}
