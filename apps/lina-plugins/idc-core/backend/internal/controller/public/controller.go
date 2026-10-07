package public

import (
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

// ControllerV1 exposes anonymous public catalog and campaign projections.
type ControllerV1 struct {
	catalog   catalog.PublicCatalogReader
	campaigns marketing.PublicCampaignService
	commerce  commerce.Service
}

// NewV1 builds the public controller with catalog, campaign, and anonymous cart
// dependencies. The cart service remains server authoritative.
func NewV1(catalogReader catalog.PublicCatalogReader, campaigns marketing.PublicCampaignService, commerceService commerce.Service) (*ControllerV1, error) {
	if catalogReader == nil || campaigns == nil || commerceService == nil {
		return nil, gerror.New("public controller requires catalog, campaign, and commerce services")
	}
	return &ControllerV1{catalog: catalogReader, campaigns: campaigns, commerce: commerceService}, nil
}
