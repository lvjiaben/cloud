// This file defines the staff-only catalog product-version publication endpoint.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CatalogProductPublishReq publishes one complete draft version.
type CatalogProductPublishReq struct {
	g.Meta    `path:"/admin/catalog/product-versions/{versionNo}/publish" method:"post" tags:"IDC Catalog Management" summary:"Apply saved catalog settings" dc:"Apply saved local settings immediately while preserving the sale switch and old order facts." permission:"idc:admin:catalog:product:update"`
	VersionNo string `json:"versionNo" v:"required|max-length:128" dc:"Stable catalog product version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
}

// CatalogProductPublishRes reports the committed publication transition.
type CatalogProductPublishRes struct {
	ProductNo           string `json:"productNo" dc:"Published catalog product business number" eg:"prd_0123456789abcdef0123456789abcdef"`
	VersionNo           string `json:"versionNo" dc:"Published catalog product version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
	Status              string `json:"status" dc:"Committed version status" eg:"published"`
	SupersededVersionNo string `json:"supersededVersionNo" dc:"Previously published version superseded by this transition, or empty" eg:"ver_fedcba9876543210fedcba9876543210"`
	PublishedAt         int64  `json:"publishedAt" dc:"Publication time as a Unix timestamp in milliseconds" eg:"1786248000000"`
}
