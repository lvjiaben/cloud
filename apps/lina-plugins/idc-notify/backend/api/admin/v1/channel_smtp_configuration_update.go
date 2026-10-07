package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelSMTPConfigurationUpdateReq struct {
	g.Meta          `path:"/admin/notifications/channels/email/smtp-configuration" method:"put" tags:"IDC Notification Channels" summary:"Configure SMTP email channel" dc:"Stores one installation-owned SMTP account. Port 465 requires implicit TLS and port 587 requires STARTTLS; the deployment allowlist must contain the exact host and port." permission:"idc:admin:notification:channel:manage"`
	Host            string `json:"host" v:"required|max-length:253" dc:"SMTP host; exact host and port must be allowed by deployment policy" eg:"smtp.example.com"`
	Port            int    `json:"port" v:"required|in:465,587" dc:"SMTP TLS port: 465 for implicit TLS or 587 for STARTTLS" eg:"465"`
	Security        string `json:"security" v:"required|in:implicit_tls,starttls" dc:"Mandatory transport security mode matching the selected port" eg:"implicit_tls"`
	Username        string `json:"username" v:"required|max-length:320" dc:"SMTP authentication username" eg:"notify@example.com"`
	Password        string `json:"password" v:"required|max-length:4096" dc:"SMTP password written only to the authoritative secret service" eg:"secret"`
	FromAddress     string `json:"fromAddress" v:"required|max-length:320" dc:"RFC mailbox used as the sender address" eg:"notify@example.com"`
	FromName        string `json:"fromName" v:"max-length:128" dc:"Optional sender display name" eg:"IDC Notifications"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Expected channel configuration version" eg:"1"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key" eg:"smtp-config-20260815-001"`
}

type ChannelSMTPConfigurationUpdateRes struct {
	Item *ChannelItem `json:"item" dc:"Email channel projection after configuration" eg:"{}"`
}
