// This file deletes a complete price schedule only from an editable draft.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogPriceScheduleDeleteReq struct {
	g.Meta    `path:"/admin/catalog/product-versions/{versionNo}/price-schedules/{priceNo}" method:"delete" tags:"IDC Catalog Management" summary:"Delete draft price schedule" dc:"Delete all tiers of one draft schedule under the price-write permission; published versions and stale row identifiers are rejected." permission:"idc:admin:catalog:price:write"`
	VersionNo string `json:"versionNo" in:"path" v:"required|max-length:40" dc:"Editable draft version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
	PriceNo   string `json:"priceNo" in:"path" v:"required|max-length:40" dc:"Existing row identifying the complete schedule and stale-delete fence" eg:"prc_0123456789abcdef0123456789abcdef"`
}
type CatalogPriceScheduleDeleteRes struct{}
