// This file defines one recently authenticated, read-only provider connectivity validation.
package v1

import "github.com/gogf/gf/v2/frame/g"

// ProviderConfigurationValidationCreateReq targets the fixed configured Aliyun FACE provider.
type ProviderConfigurationValidationCreateReq struct {
	g.Meta `path:"/admin/compliance/providers/aliyun-face/validations" method:"post" tags:"IDC Compliance Provider" summary:"Validate Aliyun identity verification connectivity" dc:"Perform one signed, synthetic read-only FACE query against the fixed Aliyun endpoint. The validation never initializes an identity flow, submits identity information, or returns provider messages or credentials. A full host sign-in within the last five minutes is required." permission:"idc:admin:compliance:provider:validate"`
}

// ProviderConfigurationValidationCreateRes returns the configuration with its latest bounded outcome.
type ProviderConfigurationValidationCreateRes struct {
	Item *ProviderConfigurationItem `json:"item" dc:"Secret-safe provider configuration with the latest bounded validation status" eg:"{}"`
}
