// This file delegates explicit tax policies to the transactional catalog service.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogTaxPolicyCreate(
	ctx context.Context,
	req *v1.CatalogTaxPolicyCreateReq,
) (*v1.CatalogTaxPolicyCreateRes, error) {
	roundingQuantumMinor, parseErr := parseCatalogAmount(req.RoundingQuantumMinor)
	if parseErr != nil || roundingQuantumMinor < 1 || roundingQuantumMinor > 1_000_000 {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	created, err := c.catalogWriter.CreateTaxPolicy(ctx, catalog.TaxPolicyCreateInput{
		VersionNo: req.VersionNo, CurrencyCode: req.CurrencyCode,
		Definition: catalog.TaxPolicyDefinition{
			PriceIncludesTax:     req.PriceIncludesTax,
			TaxTreatment:         catalog.TaxTreatment(req.TaxTreatment),
			TaxRateBasisPoints:   req.TaxRateBasisPoints,
			TaxCategoryCode:      req.TaxCategoryCode,
			RoundingScope:        catalog.TaxRoundingScope(req.RoundingScope),
			RoundingMode:         catalog.TaxRoundingMode(req.RoundingMode),
			RoundingQuantumMinor: roundingQuantumMinor,
		},
	})
	if err != nil {
		return nil, err
	}
	return &v1.CatalogTaxPolicyCreateRes{
		TaxPolicyNo: created.TaxPolicyNo, VersionNo: created.VersionNo,
		CurrencyCode:         created.CurrencyCode,
		PriceIncludesTax:     created.Definition.PriceIncludesTax,
		TaxTreatment:         string(created.Definition.TaxTreatment),
		TaxRateBasisPoints:   created.Definition.TaxRateBasisPoints,
		TaxCategoryCode:      created.Definition.TaxCategoryCode,
		RoundingScope:        string(created.Definition.RoundingScope),
		RoundingMode:         string(created.Definition.RoundingMode),
		RoundingQuantumMinor: formatCatalogAmount(created.Definition.RoundingQuantumMinor),
	}, nil
}
