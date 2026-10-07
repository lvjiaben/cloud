// Privacy retention controllers expose active policies and explicit replacement. They do not invent a duration when policy is missing.

package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/privacygovernance"
)

func (c *ControllerV1) PrivacyRetentionList(
	ctx context.Context,
	_ *v1.PrivacyRetentionListReq,
) (*v1.PrivacyRetentionListRes, error) {
	items, err := c.privacyGovernance.ListRetentionPolicies(ctx)
	if err != nil {
		return nil, err
	}
	result := &v1.PrivacyRetentionListRes{Items: make([]v1.PrivacyRetentionPolicyItem, 0, len(items))}
	for index := range items {
		result.Items = append(result.Items, privacyRetentionItem(&items[index]))
	}
	return result, nil
}

func (c *ControllerV1) PrivacyRetentionUpdate(
	ctx context.Context,
	req *v1.PrivacyRetentionUpdateReq,
) (*v1.PrivacyRetentionUpdateRes, error) {
	item, err := c.privacyGovernance.SetRetentionPolicy(ctx, privacygovernance.RetentionPolicyInput{
		DataType: privacygovernance.DataType(req.DataTypeCode), RetentionDays: req.RetentionDays,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.PrivacyRetentionUpdateRes{Item: privacyRetentionItem(item)}, nil
}

func privacyRetentionItem(item *privacygovernance.RetentionPolicy) v1.PrivacyRetentionPolicyItem {
	if item == nil {
		return v1.PrivacyRetentionPolicyItem{}
	}
	return v1.PrivacyRetentionPolicyItem{
		PolicyNo: item.PolicyNo, DataTypeCode: string(item.DataType), Version: item.Version,
		RetentionDays: item.RetentionDays, StatusCode: item.StatusCode,
		CreatedBy: item.CreatedBy, CreatedAt: item.CreatedAt.UnixMilli(),
	}
}
