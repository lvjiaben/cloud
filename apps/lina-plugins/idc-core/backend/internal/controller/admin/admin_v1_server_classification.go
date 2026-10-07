package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerClassificationList(ctx context.Context,
	_ *v1.ServerClassificationListReq) (*v1.ServerClassificationListRes, error) {
	items, err := c.serverBatches.ListClassifications(ctx)
	if err != nil {
		return nil, err
	}
	result := &v1.ServerClassificationListRes{Items: make([]*v1.ServerClassification, 0, len(items))}
	for _, item := range items {
		result.Items = append(result.Items, projectServerClassification(&item))
	}
	return result, nil
}

func (c *ControllerV1) ServerClassificationCreate(ctx context.Context,
	req *v1.ServerClassificationCreateReq) (*v1.ServerClassificationSaveRes, error) {
	item, err := c.serverBatches.SaveClassification(ctx, serveroperation.ServerClassificationInput{
		NameZhCN: req.NameZhCN, NameEnUS: req.NameEnUS, RequestNo: req.RequestNo,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ServerClassificationSaveRes{Item: projectServerClassification(item)}, nil
}

func (c *ControllerV1) ServerClassificationUpdate(ctx context.Context,
	req *v1.ServerClassificationUpdateReq) (*v1.ServerClassificationSaveRes, error) {
	item, err := c.serverBatches.SaveClassification(ctx, serveroperation.ServerClassificationInput{
		ClassificationNo: req.ClassificationNo, NameZhCN: req.NameZhCN, NameEnUS: req.NameEnUS,
		RequestNo: req.RequestNo, ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ServerClassificationSaveRes{Item: projectServerClassification(item)}, nil
}

func (c *ControllerV1) ServerClassificationDisable(ctx context.Context,
	req *v1.ServerClassificationDisableReq) (*v1.ServerClassificationSaveRes, error) {
	item, err := c.serverBatches.DisableClassification(ctx, req.ClassificationNo, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.ServerClassificationSaveRes{Item: projectServerClassification(item)}, nil
}

func projectServerClassification(item *serveroperation.ServerClassification) *v1.ServerClassification {
	if item == nil {
		return nil
	}
	return &v1.ServerClassification{ClassificationNo: item.ClassificationNo, NameZhCN: item.NameZhCN,
		NameEnUS: item.NameEnUS, Status: item.Status, Version: item.Version}
}
