// This file defines the staff-only catalog publication readiness endpoint.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CatalogProductPublicationPreviewReq reads one draft version's readiness.
type CatalogProductPublicationPreviewReq struct {
	g.Meta    `path:"/admin/catalog/product-versions/{versionNo}/publication-preview" method:"get" tags:"IDC Catalog Management" summary:"Preview catalog product publication" dc:"Return bounded advisory readiness checks. The publication write always rechecks authoritative state." permission:"idc:admin:catalog:product:publish"`
	VersionNo string `json:"versionNo" v:"required|max-length:128" dc:"Stable catalog product version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
}

// CatalogProductPublicationCheck is one stable readiness dimension.
type CatalogProductPublicationCheck struct {
	Code   string `json:"code" dc:"Stable readiness check code localized by the client" eg:"pricing"`
	Passed bool   `json:"passed" dc:"Whether this readiness dimension currently passes" eg:"true"`
}

// CatalogProductPublicationPreviewRes is a bounded advisory readiness graph.
type CatalogProductPublicationPreviewRes struct {
	ProductNo string                            `json:"productNo" dc:"Catalog product business number" eg:"prd_0123456789abcdef0123456789abcdef"`
	VersionNo string                            `json:"versionNo" dc:"Catalog product version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
	Ready     bool                              `json:"ready" dc:"Whether every current readiness check passes" eg:"true"`
	Checks    []*CatalogProductPublicationCheck `json:"checks" dc:"Fixed bounded publication readiness checks"`
}
