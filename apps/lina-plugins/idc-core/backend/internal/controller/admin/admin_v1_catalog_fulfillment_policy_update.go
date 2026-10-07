// This file delegates fulfillment-policy writes after exact plugin admission.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogFulfillmentPolicyUpdate(
	ctx context.Context,
	req *v1.CatalogFulfillmentPolicyUpdateReq,
) (*v1.CatalogFulfillmentPolicyUpdateRes, error) {
	authorization, err := c.productAutomation.AuthorizeFulfillmentPolicyWrite(ctx, req.VersionNo)
	if err != nil {
		return nil, err
	}
	evidence, err := catalog.SaveFulfillmentPolicy(ctx, catalog.FulfillmentPolicyUpdateInput{
		VersionNo: req.VersionNo, StrategyCode: req.StrategyCode,
	}, authorization)
	if err != nil {
		return nil, err
	}
	return &v1.CatalogFulfillmentPolicyUpdateRes{
		Policy: mapFulfillmentPolicySummary(evidence, authorization.Contract),
	}, nil
}
