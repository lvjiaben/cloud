package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (c *ControllerV1) CatalogAutomationInterfaceConfigurationEvaluate(ctx context.Context, req *v1.CatalogAutomationInterfaceConfigurationEvaluateReq) (*v1.CatalogAutomationInterfaceConfigurationEvaluateRes, error) {
	evaluation, err := c.productAutomation.EvaluateInterfaceConfiguration(ctx, productautomation.EvaluateInterfaceConfigurationInput{
		InterfaceInstanceNo: req.InterfaceInstanceNo,
		Values:              mapConfigurationInput(req.Values),
		ExpectedRevision:    req.ExpectedRevision,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogAutomationInterfaceConfigurationEvaluateRes{
		Evaluation: mapConfigurationEvaluation(evaluation), Revision: evaluation.Revision,
	}, nil
}
