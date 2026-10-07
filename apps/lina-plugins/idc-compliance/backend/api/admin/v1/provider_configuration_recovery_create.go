// This file defines one recently authenticated interrupted-mutation recovery request.
package v1

import "github.com/gogf/gf/v2/frame/g"

// ProviderConfigurationRecoveryCreateReq carries fresh credentials for the
// single server-tracked Aliyun FACE mutation currently left in updating.
type ProviderConfigurationRecoveryCreateReq struct {
	g.Meta          `path:"/admin/compliance/providers/aliyun-face/recoveries" method:"post" tags:"IDC Compliance Provider" summary:"Recover interrupted Aliyun identity verification configuration" dc:"Resume only the installation-wide Aliyun FACE credential mutation persisted in updating. A full host sign-in within the last five minutes and a fresh credential pair are required. The server-owned operation key determines whether secretcap safely creates, replaces, or reuses the exact mutation; the API never forces success or returns credentials." permission:"idc:admin:compliance:provider:write"`
	AccessKeyID     string `json:"accessKeyId" v:"required|max-length:256" dc:"Fresh RAM AccessKey ID consumed by secretcap and never returned" eg:"LTAI5tExampleAccessKey"`
	AccessKeySecret string `json:"accessKeySecret" v:"required|max-length:512" dc:"Fresh RAM AccessKey secret consumed by secretcap and never returned" eg:"exampleSecretValueNeverStoredInDTO"`
}

// ProviderConfigurationRecoveryCreateRes returns only the committed safe
// projection and whether secretcap replayed an already completed mutation.
type ProviderConfigurationRecoveryCreateRes struct {
	Item                   *ProviderConfigurationItem `json:"item" dc:"Recovered secret-safe provider configuration" eg:"{}"`
	ReusedExistingMutation bool                       `json:"reusedExistingMutation" dc:"Whether secretcap safely reused the exact mutation completed before the interruption" eg:"true"`
}
