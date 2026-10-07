// This file defines safe merchant-facing generic configuration price targets.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogConfigurationPriceCandidate struct {
	CandidateToken     string `json:"candidateToken" dc:"Opaque current configuration candidate token"`
	FieldLabelZhCN     string `json:"fieldLabelZhCN" dc:"Chinese field label"`
	FieldLabelEnUS     string `json:"fieldLabelEnUS" dc:"English field label"`
	CandidateLabelZhCN string `json:"candidateLabelZhCN" dc:"Chinese candidate label"`
	CandidateLabelEnUS string `json:"candidateLabelEnUS" dc:"English candidate label"`
	DimensionNo        string `json:"dimensionNo" dc:"Existing price dimension number, or empty"`
}

type CatalogConfigurationPriceCandidateListReq struct {
	g.Meta    `path:"/admin/catalog/product-versions/{versionNo}/configuration-price-candidates" method:"get" tags:"IDC Catalog Management" summary:"List safe configuration price candidates" dc:"Return one bounded bilingual candidate list without plugin keys or raw values." permission:"idc:admin:catalog:configuration:write"`
	VersionNo string `json:"versionNo" in:"path" v:"required|regex:^ver_[0-9a-f]{32}$" dc:"Editable product version number"`
}

type CatalogConfigurationPriceCandidateListRes struct {
	Candidates []*CatalogConfigurationPriceCandidate `json:"candidates" dc:"Bounded safe price candidate list"`
}

type CatalogConfigurationPriceDimensionCreateReq struct {
	g.Meta         `path:"/admin/catalog/product-versions/{versionNo}/configuration-price-dimensions" method:"post" tags:"IDC Catalog Management" summary:"Create configuration price dimension" dc:"Freeze one current bilingual candidate as an explicit core-owned price target." permission:"idc:admin:catalog:configuration:write"`
	VersionNo      string `json:"versionNo" in:"path" v:"required|regex:^ver_[0-9a-f]{32}$" dc:"Editable product version number"`
	CandidateToken string `json:"candidateToken" v:"required|max-length:128" dc:"Opaque token returned by the candidate list"`
}

type CatalogConfigurationPriceDimensionCreateRes struct {
	DimensionNo        string `json:"dimensionNo" dc:"Stable configuration price dimension number"`
	FieldLabelZhCN     string `json:"fieldLabelZhCN" dc:"Frozen Chinese field label"`
	FieldLabelEnUS     string `json:"fieldLabelEnUS" dc:"Frozen English field label"`
	CandidateLabelZhCN string `json:"candidateLabelZhCN" dc:"Frozen Chinese candidate label"`
	CandidateLabelEnUS string `json:"candidateLabelEnUS" dc:"Frozen English candidate label"`
}
