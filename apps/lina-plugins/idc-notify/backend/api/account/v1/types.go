package v1

type QuietHoursInput struct {
	TimezoneName string `json:"timezoneName" v:"required|max-length:64" dc:"IANA timezone used to interpret the local quiet window" eg:"Asia/Shanghai"`
	StartMinute  int    `json:"startMinute" v:"between:0,1439" dc:"Quiet-window start minute counted from local 00:00, from 0 to 1439" eg:"1320"`
	EndMinute    int    `json:"endMinute" v:"between:0,1439" dc:"Quiet-window end minute counted from local 00:00, from 0 to 1439" eg:"480"`
	Enabled      bool   `json:"enabled" dc:"Whether marketing quiet hours are enabled" eg:"true"`
}

type ContactPreferenceItem struct {
	ContactNo   string `json:"contactNo" dc:"Opaque contact identifier owned by the current customer" eg:"contact_1001"`
	Topic       string `json:"topic" dc:"Notification topic: account, billing, service, security, support, or marketing" eg:"support"`
	ChannelCode string `json:"channelCode" dc:"Customer notification channel: in_app, email, sms, or wechat_official" eg:"in_app"`
	Enabled     bool   `json:"enabled" dc:"Whether the current customer explicitly enabled this combination" eg:"true"`
	Version     int64  `json:"version" dc:"Current preference row version" eg:"2"`
}

type MarketingConsentItem struct {
	ContactNo       string `json:"contactNo" dc:"Opaque contact identifier owned by the current customer" eg:"contact_1001"`
	ChannelCode     string `json:"channelCode" dc:"Customer marketing channel: email, sms, or wechat_official" eg:"email"`
	Status          string `json:"status" dc:"Current explicit marketing consent status: opted_in=consented, opted_out=withdrawn" eg:"opted_in"`
	PolicyReference string `json:"policyReference" dc:"Authoritative policy reference recorded with the current marketing decision; empty when legacy evidence is unavailable" eg:"privacy-policy"`
	PolicyVersion   string `json:"policyVersion" dc:"Authoritative policy version recorded with the current marketing decision; empty when legacy evidence is unavailable" eg:"2026-08"`
	Locale          string `json:"locale" dc:"Authoritative evidence locale recorded with the current marketing decision; empty when legacy evidence is unavailable" eg:"zh-CN"`
	Version         int64  `json:"version" dc:"Current consent row version" eg:"2"`
}

type QuietHoursItem struct {
	TimezoneName    string `json:"timezoneName" dc:"IANA timezone used by the customer" eg:"Asia/Shanghai"`
	StartMinute     int    `json:"startMinute" dc:"Local start minute counted from 00:00, from 0 to 1439" eg:"1320"`
	EndMinute       int    `json:"endMinute" dc:"Local end minute counted from 00:00, from 0 to 1439" eg:"480"`
	CrossesMidnight bool   `json:"crossesMidnight" dc:"Whether the quiet window crosses midnight" eg:"true"`
	Enabled         bool   `json:"enabled" dc:"Whether quiet hours are enabled" eg:"true"`
	Version         int64  `json:"version" dc:"Current quiet-hours row version" eg:"2"`
}

type DeliveryItem struct {
	TaskNo      string `json:"taskNo" dc:"Opaque notification delivery task number" eg:"ntask_01jnotify000000000000000001"`
	EventCode   string `json:"eventCode" dc:"Stable notification event code" eg:"support.ticket_updated"`
	Topic       string `json:"topic" dc:"Notification topic: account, billing, service, security, support, or marketing" eg:"support"`
	ChannelCode string `json:"channelCode" dc:"Customer delivery channel: in_app, email, sms, or wechat_official" eg:"in_app"`
	Status      string `json:"status" dc:"Delivery status: queued, sending, retry_wait, submitted, delivered, failed, cancelled, preference_blocked, result_unknown, manual_review, or manual_unknown_closed" eg:"delivered"`
	ResultClass string `json:"resultClass" dc:"Normalized delivery result without raw provider details" eg:"delivered"`
	QueuedAt    int64  `json:"queuedAt" dc:"Delivery queue time as Unix timestamp in milliseconds" eg:"1786752000000"`
	CompletedAt *int64 `json:"completedAt" dc:"Delivery completion time as Unix timestamp in milliseconds; null while unfinished" eg:"1786752005000"`
}
