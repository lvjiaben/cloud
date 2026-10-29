package public

import (
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/internal/service/currency"
	"lina-plugin-idc-core/backend/internal/service/customeravatar"

	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

// ControllerV1 exposes anonymous public catalog and campaign projections.
type ControllerV1 struct {
	avatars    customeravatar.Service
	currencies currency.Reader
	catalog    catalog.PublicCatalogReader
	campaigns  marketing.PublicCampaignService
	commerce   commerce.Service
}

// NewV1 builds the public controller with catalog, campaign, and anonymous cart
// dependencies. The cart service remains server authoritative.
func NewV1(catalogReader catalog.PublicCatalogReader, campaigns marketing.PublicCampaignService, commerceService commerce.Service, avatars customeravatar.Service) (*ControllerV1, error) {
	if catalogReader == nil || campaigns == nil || commerceService == nil || avatars == nil {
		return nil, gerror.New("public controller requires catalog, campaign, and commerce services")
	}
	return &ControllerV1{catalog: catalogReader, campaigns: campaigns, commerce: commerceService, avatars: avatars}, nil
}

// BindCurrencies binds the explicit public, non-sensitive currency admission projection.
func BindCurrencies(c *ControllerV1, svc currency.Reader) error {
	if c == nil || svc == nil || c.currencies != nil {
		return gerror.New("public currencies require one reader binding")
	}
	c.currencies = svc
	return nil
}
