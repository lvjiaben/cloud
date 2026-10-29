// This file projects the exact-customer note through the shared directory owner.
package directoryaccount

import (
	"context"
	"lina-plugin-idc-core/backend/api/directoryaccount/v1"
)

func (c *ControllerV1) CustomerNoteGet(ctx context.Context, req *v1.CustomerNoteGetReq) (*v1.CustomerNoteGetRes, error) {
	note, err := c.service.GetNote(ctx, req.EntitlementNo)
	if err != nil {
		return nil, err
	}
	return &v1.CustomerNoteGetRes{SupportAssociationReference: note.SupportAssociationReference, Note: v1.CustomerNote{EntitlementNo: note.EntitlementNo, Remark: note.Remark, Revision: note.Revision}}, nil
}
