// This file defines one recently authenticated Aliyun FACE credential replacement request.
package v1

import "github.com/gogf/gf/v2/frame/g"

// ProviderConfigurationUpdateReq carries one short-lived credential replacement submission.
type ProviderConfigurationUpdateReq struct {
	g.Meta          `path:"/admin/compliance/providers/aliyun-face" method:"put" tags:"IDC Compliance Provider" summary:"Configure Aliyun identity verification" dc:"Create, resume, or replace the installation-wide Aliyun FACE credential through a fail-closed audited transition. A full host sign-in within the last five minutes is required. Credentials are consumed and never returned." permission:"idc:admin:compliance:provider:write"`
	SceneID         string `json:"sceneId" v:"required|max-length:64" dc:"Merchant-owned numeric Aliyun verification scene identifier" eg:"100000001"`
	AccessKeyID     string `json:"accessKeyId" v:"required|max-length:256" dc:"RAM AccessKey ID consumed by secretcap and never returned" eg:"LTAI5tExampleAccessKey"`
	AccessKeySecret string `json:"accessKeySecret" v:"required|max-length:512" dc:"RAM AccessKey secret consumed by secretcap and never returned" eg:"exampleSecretValueNeverStoredInDTO"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:0" dc:"Last read optimistic version; use zero only for first configuration" eg:"3"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|max-length:128" dc:"Stable 8 to 128 character operation key replayed only for the exact same submission" eg:"aliyun-face-config-20260812-0001"`
}

// ProviderConfigurationUpdateRes returns only the committed secret-safe projection.
type ProviderConfigurationUpdateRes struct {
	Item *ProviderConfigurationItem `json:"item" dc:"Committed secret-safe provider configuration" eg:"{}"`
}
