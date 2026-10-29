package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (c *ControllerV1) CatalogConfigurationPriceCandidateList(ctx context.Context, req *v1.CatalogConfigurationPriceCandidateListReq) (*v1.CatalogConfigurationPriceCandidateListRes, error) {
	items, err := c.productAutomation.ListConfigurationPriceCandidates(ctx, req.VersionNo)
	if err != nil {
		return nil, err
	}
	result := make([]*v1.CatalogConfigurationPriceCandidate, 0, len(items))
	for _, item := range items {
		result = append(result, &v1.CatalogConfigurationPriceCandidate{
			FieldToken: item.FieldToken, CandidateToken: item.CandidateToken, DimensionNo: item.DimensionNo, Quantity: item.Quantity, PricingMode: string(item.PricingMode),
			FieldLabelZhCN: item.FieldLabel.ZhCN, FieldLabelEnUS: item.FieldLabel.EnUS,
			CandidateLabelZhCN: item.CandidateLabel.ZhCN, CandidateLabelEnUS: item.CandidateLabel.EnUS,
		})
	}
	return &v1.CatalogConfigurationPriceCandidateListRes{Candidates: result}, nil
}

func (c *ControllerV1) CatalogConfigurationPriceDimensionCreate(ctx context.Context, req *v1.CatalogConfigurationPriceDimensionCreateReq) (*v1.CatalogConfigurationPriceDimensionCreateRes, error) {
	item, err := c.productAutomation.CreateConfigurationPriceDimension(ctx, productautomation.CreateConfigurationPriceDimensionInput{
		VersionNo: req.VersionNo, CandidateToken: req.CandidateToken, PricingMode: catalog.ConfigurationPriceMode(req.PricingMode),
	})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogConfigurationPriceDimensionCreateRes{
		DimensionNo: item.DimensionNo, PricingMode: string(item.PricingMode), FieldLabelZhCN: item.FieldLabel.ZhCN, FieldLabelEnUS: item.FieldLabel.EnUS,
		CandidateLabelZhCN: item.CandidateLabel.ZhCN, CandidateLabelEnUS: item.CandidateLabel.EnUS,
	}, nil
}
