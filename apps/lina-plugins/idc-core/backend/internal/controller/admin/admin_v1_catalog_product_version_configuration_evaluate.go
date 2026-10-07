package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (c *ControllerV1) CatalogProductVersionConfigurationEvaluate(ctx context.Context, req *v1.CatalogProductVersionConfigurationEvaluateReq) (*v1.CatalogProductVersionConfigurationEvaluateRes, error) {
	evaluation, err := c.productAutomation.EvaluateVersionConfiguration(ctx, productautomation.EvaluateVersionConfigurationInput{
		VersionNo: req.VersionNo, MerchantValues: mapConfigurationInput(req.MerchantValues),
		ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogProductVersionConfigurationEvaluateRes{
		Evaluation: mapConfigurationEvaluation(evaluation), Version: evaluation.Revision,
	}, nil
}
