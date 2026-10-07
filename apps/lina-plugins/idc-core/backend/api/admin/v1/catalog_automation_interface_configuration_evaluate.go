// This file defines side-effect-free whole-form interface configuration evaluation.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogAutomationInterfaceConfigurationEvaluateReq struct {
	g.Meta              `path:"/admin/catalog/automation/interfaces/{interfaceInstanceNo}/configuration/evaluation" method:"post" tags:"IDC Catalog Management" summary:"Evaluate automation interface configuration" dc:"Batch-resolve safe dynamic candidates and evaluate one complete interface configuration form without saving it." permission:"idc:admin:catalog:automation-interface:manage"`
	InterfaceInstanceNo string                                 `json:"interfaceInstanceNo" in:"path" v:"required|regex:^pfi_[0-9a-f]{32}$" dc:"Automation interface number" eg:"pfi_0123456789abcdef0123456789abcdef"`
	Values              []*CatalogStructuredConfigurationValue `json:"values" v:"required" dc:"Current complete typed configuration values"`
	ExpectedRevision    int64                                  `json:"expectedRevision" v:"required|min:1" dc:"Expected latest immutable configuration revision" eg:"2"`
}

type CatalogAutomationInterfaceConfigurationEvaluateRes struct {
	Evaluation *CatalogConfigurationEvaluation `json:"evaluation" dc:"Authoritative whole-form evaluation without persistence"`
	Revision   int64                           `json:"revision" dc:"Current immutable interface configuration revision" eg:"2"`
}
