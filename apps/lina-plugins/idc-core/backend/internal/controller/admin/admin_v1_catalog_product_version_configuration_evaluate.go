package admin

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (c *ControllerV1) CatalogProductVersionConfigurationEvaluate(ctx context.Context, req *v1.CatalogProductVersionConfigurationEvaluateReq) (*v1.CatalogProductVersionConfigurationEvaluateRes, error) {
	if req.MerchantValues == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
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
