// This file exposes editable revisions on existing products and explicit sale
// state changes, keeping published settings and historical charges immutable.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductRevisionCreateReq struct {
	g.Meta          `path:"/admin/catalog/products/{productNo}/draft-versions" method:"post" tags:"IDC Catalog Management" summary:"Start editing a published product" dc:"Create a new editable revision on the same product, preserving the current published sale and all historical transactions." permission:"idc:admin:catalog:product:update"`
	ProductNo       string `json:"productNo" in:"path" v:"required|length:36,36" dc:"Visible existing product number" eg:"prd_0123456789abcdef0123456789abcdef"`
	SourceVersionNo string `json:"sourceVersionNo" v:"required|length:36,36" dc:"Exact current published version to revise" eg:"ver_0123456789abcdef0123456789abcdef"`
}
type CatalogProductRevisionCreateRes struct {
	CatalogProductCreateRes
}

type CatalogProductSaleStateUpdateReq struct {
	g.Meta          `path:"/admin/catalog/products/{productNo}/sale-state" method:"put" tags:"IDC Catalog Management" summary:"Change product sale state" dc:"Pause new charges or resume a complete published product with exact provider admission, optimistic concurrency and transactional audit." permission:"idc:admin:catalog:product:publish"`
	ProductNo       string `json:"productNo" in:"path" v:"required|length:36,36" dc:"Visible product whose sales state changes" eg:"prd_0123456789abcdef0123456789abcdef"`
	VersionNo       string `json:"versionNo" v:"required|length:36,36" dc:"Exact current published version" eg:"ver_0123456789abcdef0123456789abcdef"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Current product revision for optimistic concurrency" eg:"2"`
	Active          bool   `json:"active" dc:"True resumes sales after admission checks; false stops new purchases, renewals and trial conversions" eg:"false"`
}
type CatalogProductSaleStateUpdateRes struct {
	Version int64 `json:"version" dc:"Committed product revision" eg:"3"`
}
