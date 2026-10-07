package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelAliyunSMSConfigurationUpdateReq struct {
	g.Meta          `path:"/admin/notifications/channels/sms/aliyun-configuration" method:"put" tags:"IDC Notification Channels" summary:"Configure Aliyun SMS" dc:"Selects one installation-owned Aliyun SMS account and the fixed SendSms 2017-05-25 adapter." permission:"idc:admin:notification:channel:manage"`
	SignName        string `json:"signName" v:"required|max-length:64" dc:"Approved Aliyun SMS signature" eg:"LinaIDC"`
	AccessKeyID     string `json:"accessKeyId" v:"required|max-length:256" dc:"Aliyun AccessKey ID written only to the authoritative secret service" eg:"LTAIexample"`
	AccessKeySecret string `json:"accessKeySecret" v:"required|max-length:256" dc:"Aliyun AccessKey secret written only to the authoritative secret service" eg:"secret"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Expected channel configuration version" eg:"1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key" eg:"aliyun-sms-config-20260816-001"`
}

type ChannelAliyunSMSConfigurationUpdateRes struct {
	Item *ChannelItem `json:"item" dc:"SMS channel projection after configuration" eg:"{}"`
}
