// This file defines the staff-only catalog product detail endpoint.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductDetailReq struct {
	g.Meta    `path:"/admin/catalog/products/{productNo}" method:"get" tags:"IDC Catalog Management" summary:"Get catalog product detail" dc:"Return one bounded latest-version product graph containing general commercial identity, billing, explicit base prices, and tax policies." permission:"idc:admin:catalog:product:query"`
	ProductNo string `json:"productNo" v:"required|max-length:128" dc:"Stable catalog product business number" eg:"prd_0123456789abcdef0123456789abcdef"`
}

type CatalogProductDetailRes struct {
	Product            *CatalogProductDetailProduct       `json:"product" dc:"Stable product sales identity" eg:"{}"`
	Version            *CatalogProductDetailVersion       `json:"version" dc:"Latest management version" eg:"{}"`
	BillingRules       []*CatalogProductDetailBillingRule `json:"billingRules" dc:"Bounded billing rules" eg:"[]"`
	Prices             []*CatalogProductDetailPrice       `json:"prices" dc:"Bounded explicit minor-unit prices" eg:"[]"`
	TaxPolicies        []*CatalogProductDetailTaxPolicy   `json:"taxPolicies" dc:"Bounded explicit per-currency tax policies" eg:"[]"`
	Inventory          *CatalogProductVersionInventory    `json:"inventory" dc:"Simple inventory setting of the latest product version" eg:"{}"`
	FulfillmentBinding *CatalogFulfillmentBindingSummary  `json:"fulfillmentBinding" dc:"Safe frozen automation binding summary, or null when unbound" eg:"{}"`
	FulfillmentPolicy  *CatalogFulfillmentPolicySummary   `json:"fulfillmentPolicy" dc:"Safe fulfillment mode and user-readable plugin capability, or null until configured" eg:"{}"`
}
