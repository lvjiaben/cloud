package v1

import "github.com/gogf/gf/v2/frame/g"

type ContractPolicyTemplate struct {
	TemplateNo       string `json:"templateNo" dc:"Stable local template number"`
	VersionNo        int64  `json:"versionNo" dc:"Enabled immutable template version"`
	Name             string `json:"name" dc:"Template name"`
	ScopeCode        string `json:"scopeCode" dc:"Personal, enterprise, or all customers"`
	SigningOrderCode string `json:"signingOrderCode" dc:"Customer first, merchant first, or parallel"`
	LanguageCode     string `json:"languageCode" dc:"Explicit template language"`
}

type ContractPolicyRuleInput struct {
	PolicyCode      string `json:"policyCode" v:"required|in:inherit,none,required" dc:"Inherit, no contract, or contract required"`
	TemplateNo      string `json:"templateNo" v:"max-length:64" dc:"Enabled template required only for policyCode required"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Last displayed rule version"`
}

type ContractPolicyRule struct {
	PolicyCode string `json:"policyCode" dc:"Stored rule choice"`
	TemplateNo string `json:"templateNo,omitempty" dc:"Selected template when required"`
	Version    int64  `json:"version" dc:"Current rule version"`
}

type ContractPolicyOrderRule struct {
	ContractPolicyRule
	RequiresContract bool                    `json:"requiresContract" dc:"Final order contract requirement"`
	DecisionSource   string                  `json:"decisionSource" dc:"Winning rule level"`
	Template         *ContractPolicyTemplate `json:"template,omitempty" dc:"Frozen final template metadata"`
}

type ContractPolicyTemplateListReq struct {
	g.Meta     `path:"/admin/contract-policy/templates" method:"get" tags:"IDC Contract Policy" summary:"List selectable contract templates" dc:"Return only enabled templates allowed for the requested setting level and target customer type." permission:"idc:admin:contract-policy:template:list"`
	Level      string `json:"level" v:"required|in:product,enterprise,order" dc:"Business setting level"`
	TargetNo   string `json:"targetNo" v:"max-length:64" dc:"Enterprise customer or order number; empty only for product level"`
	CustomerNo string `json:"customerNo" v:"max-length:40" dc:"Customer number for an order being prepared before an order number exists"`
}

type ContractPolicyTemplateListRes struct {
	Templates []*ContractPolicyTemplate `json:"templates" dc:"Bounded selectable template list"`
}

type CustomerContractPolicyUpdateReq struct {
	g.Meta     `path:"/admin/customers/{customerNo}/contract-policy" method:"put" tags:"IDC Customer Management" summary:"Update enterprise contract rule" dc:"Apply the rule only to orders created after this update." permission:"idc:admin:customer:contract-policy:write"`
	CustomerNo string `json:"customerNo" in:"path" v:"required|max-length:40" dc:"Enterprise customer number"`
	ContractPolicyRuleInput
}

type CustomerContractPolicyUpdateRes struct {
	Rule *ContractPolicyRule `json:"rule" dc:"Committed enterprise rule"`
}

type CatalogContractPolicyUpdateReq struct {
	g.Meta    `path:"/admin/catalog/product-versions/{versionNo}/contract-policy" method:"put" tags:"IDC Catalog Management" summary:"Update product contract rule" dc:"A required product rule accepts only all-customer templates." permission:"idc:admin:catalog:contract-policy:write"`
	VersionNo string `json:"versionNo" in:"path" v:"required|max-length:40" dc:"Draft product version number"`
	ContractPolicyRuleInput
}

type CatalogContractPolicyUpdateRes struct {
	Rule *ContractPolicyRule `json:"rule" dc:"Committed product rule"`
}

type CommerceOrderContractPolicyUpdateReq struct {
	g.Meta  `path:"/admin/commerce/orders/{orderNo}/contract-policy" method:"put" tags:"IDC Commerce Management" summary:"Update order contract rule before payment" dc:"Reject changes after payment confirmation and recompute the final single-template decision." permission:"idc:admin:commerce:contract-policy:write"`
	OrderNo string `json:"orderNo" in:"path" v:"required|max-length:40" dc:"Order number"`
	ContractPolicyRuleInput
}

type CommerceOrderContractPolicyUpdateRes struct {
	Rule *ContractPolicyOrderRule `json:"rule" dc:"Committed order rule and final decision"`
}
