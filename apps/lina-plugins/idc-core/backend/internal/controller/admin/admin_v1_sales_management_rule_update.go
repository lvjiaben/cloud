// This generated controller delegates scoped sales operations to its owner.
package admin

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func (c *ControllerV1) SalesManagementRuleUpdate(ctx context.Context, req *v1.SalesManagementRuleUpdateReq) (*v1.SalesManagementRuleUpdateRes, error) {
	item, err := c.salesManagement.SaveRule(ctx, salesmanagement.CommissionRuleInput{RuleNo: req.RuleNo, BeneficiaryType: req.BeneficiaryType, OperationType: req.OperationType, ProductNo: req.ProductNo, CurrencyCode: req.CurrencyCode, CalculationType: req.CalculationType, FixedAmountMinor: req.FixedAmountMinor, Status: req.Status, RateBasisPoints: req.RateBasisPoints, MaturityDays: req.MaturityDays, ExpectedVersion: req.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	return &v1.SalesManagementRuleUpdateRes{Item: salesManagementRule(item)}, nil
}
