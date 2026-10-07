// Privacy deletion controllers list, scan, and confirm bounded deletion tasks. They keep scanning and destructive confirmation as separate operations.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/privacygovernance"
)

func (c *ControllerV1) PrivacyDeletionList(ctx context.Context, req *v1.PrivacyDeletionListReq) (*v1.PrivacyDeletionListRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(privacygovernance.CodeInvalid)
	}
	items, err := c.privacyGovernance.ListDeletionTasks(ctx, req.Limit)
	if err != nil {
		return nil, err
	}
	result := &v1.PrivacyDeletionListRes{Items: make([]v1.PrivacyDeletionItem, 0, len(items))}
	for index := range items {
		result.Items = append(result.Items, privacyDeletionItem(&items[index]))
	}
	return result, nil
}

func (c *ControllerV1) PrivacyDeletionScanCreate(ctx context.Context, req *v1.PrivacyDeletionScanCreateReq) (*v1.PrivacyDeletionScanCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(privacygovernance.CodeInvalid)
	}
	created, err := c.privacyGovernance.GenerateDueDeletionTasks(ctx, req.Limit)
	if err != nil {
		return nil, err
	}
	return &v1.PrivacyDeletionScanCreateRes{Created: created}, nil
}

func (c *ControllerV1) PrivacyDeletionConfirmCreate(ctx context.Context, req *v1.PrivacyDeletionConfirmCreateReq) (*v1.PrivacyDeletionConfirmCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(privacygovernance.CodeInvalid)
	}
	item, err := c.privacyGovernance.ConfirmDeletion(ctx, privacygovernance.DeletionConfirmInput{TaskNo: req.TaskNo, ExpectedVersion: req.ExpectedVersion, Confirmed: req.Confirmed, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.PrivacyDeletionConfirmCreateRes{Item: privacyDeletionItem(item)}, nil
}

func privacyDeletionItem(item *privacygovernance.DeletionTask) v1.PrivacyDeletionItem {
	if item == nil {
		return v1.PrivacyDeletionItem{}
	}
	result := v1.PrivacyDeletionItem{TaskNo: item.TaskNo, DataTypeCode: item.DataTypeCode, TargetTypeCode: item.TargetTypeCode, TargetNo: item.TargetNo, StatusCode: item.StatusCode, DueAt: item.DueAt.UnixMilli(), HoldReasonCode: item.HoldReasonCode, ConfirmedByUserID: item.ConfirmedByUserID, Version: item.Version}
	if item.ConfirmedAt != nil {
		result.ConfirmedAt = item.ConfirmedAt.UnixMilli()
	}
	if item.CompletedAt != nil {
		result.CompletedAt = item.CompletedAt.UnixMilli()
	}
	return result
}
