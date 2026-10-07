// This file defines authoritative interactive configuration evaluation.

package v1

import "github.com/gogf/gf/v2/frame/g"

type PurchaseConfigurationEvaluateReq struct {
	g.Meta                `path:"/public/catalog/products/{publicSlug}/purchase-configuration/evaluate" method:"post" tags:"IDC Public Catalog" summary:"Evaluate governed purchase configuration" dc:"Re-evaluate typed values against the current frozen declaration without exposing conditions."`
	PublicSlug            string                        `json:"publicSlug" in:"path" v:"required|length:1,128" dc:"Public browser slug"`
	ConfigurationRevision string                        `json:"configurationRevision" v:"required" dc:"Opaque revision returned by the initial endpoint"`
	Values                []*PurchaseConfigurationValue `json:"values" dc:"Bounded explicit typed value rows"`
}

type PurchaseConfigurationEvaluateRes struct {
	PurchaseConfigurationEvaluation
}
