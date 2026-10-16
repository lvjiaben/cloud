// This file defines explicit staff copying into an unpublished product draft.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CatalogProductCopyReq selects the source and the new product's sales identity.
type CatalogProductCopyReq struct {
	g.Meta          `path:"/admin/catalog/products/{productNo}/copies" method:"post" tags:"IDC Catalog Management" summary:"Copy catalog product draft" dc:"Copy commercial and fulfillment settings atomically into a new unpublished zero-stock product. Source transactions and publication remain unchanged." permission:"idc:admin:catalog:product:create"`
	ProductNo       string `json:"productNo" v:"required|length:36,36" dc:"Visible source product business number" eg:"prd_0123456789abcdef0123456789abcdef"`
	SourceVersionNo string `json:"sourceVersionNo" v:"required|length:36,36" dc:"Exact source version belonging to the source product" eg:"ver_0123456789abcdef0123456789abcdef"`
	PublicSlug      string `json:"publicSlug" v:"required|max-length:128" dc:"Unused lowercase URL slug for the new product" eg:"cloud-server-copy"`
	NameZhCn        string `json:"nameZhCn" v:"required|max-length:256" dc:"Simplified Chinese name of the new product" eg:"Cloud server copy zh-CN"`
	NameEnUs        string `json:"nameEnUs" v:"required|max-length:256" dc:"English name of the new product" eg:"Cloud server copy"`
}

// CatalogProductCopyRes returns the newly committed unsaleable draft identity.
type CatalogProductCopyRes struct {
	CatalogProductCreateRes
}
