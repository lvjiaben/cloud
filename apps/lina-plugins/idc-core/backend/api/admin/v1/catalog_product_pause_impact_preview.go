package v1

import "github.com/gogf/gf/v2/frame/g"

// CatalogProductPauseImpactPreviewReq reads the impact of stopping new sales
// for one product. The operation is advisory and does not pause anything.
type CatalogProductPauseImpactPreviewReq struct {
	g.Meta    `path:"/admin/catalog/products/{productNo}/pause-impact-preview" method:"get" tags:"IDC Catalog Management" summary:"Preview product pause impact" dc:"Show what new sales would be blocked and confirm that existing paid services remain unchanged." permission:"idc:admin:catalog:product:query"`
	ProductNo string `json:"productNo" in:"path" v:"required|max-length:64" dc:"Stable product business number" eg:"prd_0123456789abcdef0123456789abcdef"`
}

type CatalogProductPauseImpactPreview struct {
	ProductNo                 string `json:"productNo" dc:"Stable product business number" eg:"prd_0123456789abcdef0123456789abcdef"`
	Status                    string `json:"status" dc:"Current product status" eg:"active"`
	NewSalesBlocked           bool   `json:"newSalesBlocked" dc:"Whether new sales are blocked by the current status" eg:"false"`
	ActiveCartItems           int64  `json:"activeCartItems" dc:"Current cart items that would no longer be sellable" eg:"3"`
	UnpaidOrders              int64  `json:"unpaidOrders" dc:"Unpaid orders containing this product" eg:"1"`
	PaidOrdersInProgress      int64  `json:"paidOrdersInProgress" dc:"Paid orders still being fulfilled" eg:"1"`
	ExistingServices          int64  `json:"existingServices" dc:"Existing services retained after pausing new sales" eg:"12"`
	ExistingServicesProtected bool   `json:"existingServicesProtected" dc:"Whether existing services are protected from this action" eg:"true"`
}

type CatalogProductPauseImpactPreviewRes struct {
	Preview *CatalogProductPauseImpactPreview `json:"preview" dc:"Read-only product pause impact preview" eg:"{}"`
}
