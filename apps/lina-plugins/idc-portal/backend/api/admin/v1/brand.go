package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-portal/backend/internal/service/brand"
)

type BrandGetReq struct {
	g.Meta `path:"/admin/site/brand" method:"get" tags:"Website Brand" summary:"Get published website brand" permission:"idc:admin:portal:brand"`
}
type BrandGetRes struct {
	Published bool          `json:"published"`
	Brand     *brand.Config `json:"brand,omitempty"`
}
type BrandSaveReq struct {
	g.Meta `path:"/admin/site/brand/revisions" method:"post" tags:"Website Brand" summary:"Stage a website brand revision" permission:"idc:admin:portal:brand"`
	Brand  brand.Config `json:"brand" v:"required"`
}
type BrandSaveRes struct {
	Brand brand.Config `json:"brand"`
}
type BrandPreviewReq struct {
	g.Meta   `path:"/admin/site/brand/revisions/{revision}" method:"get" tags:"Website Brand" summary:"Preview a website brand revision" permission:"idc:admin:portal:brand"`
	Revision string `json:"revision" v:"required|length:1,64"`
}
type BrandPreviewRes struct {
	Brand brand.Config `json:"brand"`
}
type BrandPublishReq struct {
	g.Meta   `path:"/admin/site/brand/publish" method:"post" tags:"Website Brand" summary:"Publish a website brand revision" permission:"idc:admin:portal:brand"`
	Revision string `json:"revision" v:"required|length:1,64"`
}
type BrandPublishRes struct {
	Published bool `json:"published"`
}
type BrandRollbackReq struct {
	g.Meta `path:"/admin/site/brand/rollback" method:"post" tags:"Website Brand" summary:"Roll back website brand" permission:"idc:admin:portal:brand"`
}
type BrandRollbackRes struct {
	RolledBack bool `json:"rolledBack"`
}
