package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelSignedWebhookConfigurationUpdateReq struct {
	g.Meta          `path:"/admin/notifications/channels/signed-webhook/configuration" method:"put" tags:"IDC Notification Channels" summary:"Configure signed Webhook channel" dc:"Stores one public HTTPS endpoint and HMAC-SHA256 signing secret. Redirects and non-public destinations are rejected, and the endpoint is resolved and checked again before every connection." permission:"idc:admin:notification:channel:manage"`
	EndpointURL     string `json:"endpointUrl" v:"required|max-length:2048" dc:"Public HTTPS Webhook endpoint; private, loopback, link-local, shared, reserved, and redirect targets are rejected" eg:"https://hooks.example.com/lina/events"`
	SigningSecret   string `json:"signingSecret" v:"required|min-length:32|max-length:512" dc:"HMAC-SHA256 signing secret written only to the authoritative secret service" eg:"0123456789abcdef0123456789abcdef"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Expected channel configuration version" eg:"1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key" eg:"webhook-config-20260815-001"`
}

type ChannelSignedWebhookConfigurationUpdateRes struct {
	Item *ChannelItem `json:"item" dc:"Signed Webhook channel projection after configuration" eg:"{}"`
}
