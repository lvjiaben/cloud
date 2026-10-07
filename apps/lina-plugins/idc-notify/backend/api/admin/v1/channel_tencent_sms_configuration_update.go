package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelTencentSMSConfigurationUpdateReq struct {
	g.Meta          `path:"/admin/notifications/channels/sms/tencent-cloud-configuration" method:"put" tags:"IDC Notification Channels" summary:"Configure Tencent Cloud SMS" dc:"Stores one installation-owned Tencent Cloud SMS account for API version 2021-01-11." permission:"idc:admin:notification:channel:manage"`
	Region          string `json:"region" v:"required|max-length:32" dc:"Tencent Cloud SMS API region" eg:"ap-guangzhou"`
	SmsSdkAppID     string `json:"smsSdkAppId" v:"required|max-length:20" dc:"Tencent Cloud SMS application ID" eg:"1400000000"`
	SignName        string `json:"signName" v:"required|max-length:64" dc:"Approved domestic SMS signature" eg:"LinaIDC"`
	SecretID        string `json:"secretId" v:"required|max-length:256" dc:"Tencent Cloud API SecretId written only to the authoritative secret service" eg:"AKIDexample"`
	SecretKey       string `json:"secretKey" v:"required|max-length:256" dc:"Tencent Cloud API SecretKey written only to the authoritative secret service" eg:"secret"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Expected channel configuration version" eg:"1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key" eg:"sms-config-20260815-001"`
}

type ChannelTencentSMSConfigurationUpdateRes struct {
	Item *ChannelItem `json:"item" dc:"SMS channel projection after configuration" eg:"{}"`
}
