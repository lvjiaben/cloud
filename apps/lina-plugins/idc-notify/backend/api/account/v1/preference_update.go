package v1

import "github.com/gogf/gf/v2/frame/g"

type PreferenceUpdateReq struct {
	g.Meta                 `path:"/account/notifications/preferences" method:"put" tags:"IDC Account Notifications" summary:"Update current customer notification preference" dc:"Applies one authoritative notification preference command for the currently authenticated customer with optimistic version and idempotency protection."`
	ContactNo              string           `json:"contactNo" v:"max-length:80" dc:"Opaque contact identifier owned by the current customer; required for a contact preference or marketing consent change and omitted for a quiet-hours-only change" eg:"contact_1001"`
	Topic                  string           `json:"topic" v:"in:account,billing,service,security,support,marketing" dc:"Notification topic required for a contact preference or marketing consent change; omit for a quiet-hours-only change" eg:"support"`
	ChannelCode            string           `json:"channelCode" v:"in:in_app,email,sms,wechat_official" dc:"Customer notification channel required for a contact preference or marketing consent change; omit for a quiet-hours-only change; WeCom is internal staff only" eg:"in_app"`
	Enabled                bool             `json:"enabled" dc:"Whether this contact, topic, and channel combination is enabled; security notifications cannot be disabled" eg:"true"`
	MarketingConsentAction string           `json:"marketingConsentAction" v:"in:granted,withdrawn" dc:"Optional marketing consent action: granted=opt in, withdrawn=opt out; omit for non-marketing changes" eg:"granted"`
	PolicyReference        string           `json:"policyReference" v:"max-length:128" dc:"Policy reference required only when granting marketing consent; withdrawal reuses the authoritative current evidence" eg:"privacy-policy"`
	PolicyVersion          string           `json:"policyVersion" v:"max-length:64" dc:"Policy version required only when granting marketing consent; withdrawal reuses the authoritative current evidence" eg:"2026-08"`
	Locale                 string           `json:"locale" v:"in:zh-CN,en-US" dc:"Evidence locale required only when granting marketing consent; withdrawal reuses the authoritative current evidence" eg:"zh-CN"`
	QuietHours             *QuietHoursInput `json:"quietHours" dc:"Optional marketing quiet-hours replacement; omit to keep the current setting" eg:"null"`
	ExpectedVersion        int64            `json:"expectedVersion" v:"min:0" dc:"Expected aggregate preference version; use 0 only when no preference aggregate exists" eg:"3"`
	IdempotencyKey         string           `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key for safely replaying the same command" eg:"pref-update-20260815-001"`
}
type PreferenceUpdateRes struct {
	Version        int64  `json:"version" dc:"Authoritative aggregate preference version after the command" eg:"4"`
	EvidenceDigest string `json:"evidenceDigest" dc:"Lowercase SHA-256 digest of the stored preference evidence" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	AppliedAt      int64  `json:"appliedAt" dc:"Command application time as Unix timestamp in milliseconds" eg:"1786752000000"`
	Reused         bool   `json:"reused" dc:"Whether this response reused a previously applied idempotent command" eg:"false"`
}
