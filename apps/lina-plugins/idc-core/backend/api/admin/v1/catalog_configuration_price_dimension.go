// This file defines safe merchant-facing generic configuration price targets.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogConfigurationPriceCandidate struct {
	FieldToken         string `json:"fieldToken" dc:"Opaque configuration field identity shared by its price candidates, including fields with identical labels" eg:"candidate_0123456789abcdef"`
	Quantity           bool   `json:"quantity" dc:"Whether this candidate prices an integer quantity field" eg:"true"`
	PricingMode        string `json:"pricingMode" dc:"Frozen pricing algorithm: fixed option charge, per_unit quantity price, band_total whole-band price, or graduated marginal price; empty if not yet created" eg:"per_unit"`
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
	PricingMode    string `json:"pricingMode" v:"in:fixed,per_unit,band_total,graduated" dc:"Pricing algorithm: graduated sums marginal band charges with setup only in the first band; fixed for selectable options, per_unit quantity price, or band_total whole-band price; omitted defaults to fixed" eg:"per_unit"`
	g.Meta         `path:"/admin/catalog/product-versions/{versionNo}/configuration-price-dimensions" method:"post" tags:"IDC Catalog Management" summary:"Create configuration price dimension" dc:"Freeze one current bilingual candidate as an explicit core-owned price target." permission:"idc:admin:catalog:configuration:write"`
	VersionNo      string `json:"versionNo" in:"path" v:"required|regex:^ver_[0-9a-f]{32}$" dc:"Editable product version number"`
	CandidateToken string `json:"candidateToken" v:"required|max-length:128" dc:"Opaque token returned by the candidate list"`
}

type CatalogConfigurationPriceDimensionCreateRes struct {
	PricingMode        string `json:"pricingMode" dc:"Frozen pricing algorithm: fixed, per_unit, band_total, or graduated" eg:"per_unit"`
	DimensionNo        string `json:"dimensionNo" dc:"Stable configuration price dimension number"`
	FieldLabelZhCN     string `json:"fieldLabelZhCN" dc:"Frozen Chinese field label"`
	FieldLabelEnUS     string `json:"fieldLabelEnUS" dc:"Frozen English field label"`
	CandidateLabelZhCN string `json:"candidateLabelZhCN" dc:"Frozen Chinese candidate label"`
	CandidateLabelEnUS string `json:"candidateLabelEnUS" dc:"Frozen English candidate label"`
}
