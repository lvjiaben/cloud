// This file defines side-effect-free whole-form product configuration evaluation.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductVersionConfigurationEvaluateReq struct {
	g.Meta          `path:"/admin/catalog/product-versions/{versionNo}/configuration/evaluation" method:"post" tags:"IDC Catalog Management" summary:"Evaluate product version configuration" dc:"Batch-resolve safe dynamic candidates and evaluate one complete merchant product configuration form without saving it." permission:"idc:admin:catalog:configuration:write"`
	VersionNo       string                                 `json:"versionNo" in:"path" v:"required|regex:^ver_[0-9a-f]{32}$" dc:"Product version number" eg:"ver_0123456789abcdef0123456789abcdef"`
	MerchantValues  []*CatalogStructuredConfigurationValue `json:"merchantValues" dc:"Required current typed merchant configuration array; empty is valid when no merchant fields are declared"`
	ExpectedVersion int64                                  `json:"expectedVersion" v:"min:0" dc:"Expected product configuration version; zero means the first unsaved configuration" eg:"1"`
}

type CatalogProductVersionConfigurationEvaluateRes struct {
	Evaluation *CatalogConfigurationEvaluation `json:"evaluation" dc:"Authoritative whole-form evaluation without persistence"`
	Version    int64                           `json:"version" dc:"Current product configuration version" eg:"1"`
}
