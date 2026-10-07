package v1

import "github.com/gogf/gf/v2/frame/g"

type PreferenceGetReq struct {
	g.Meta `path:"/account/notifications/preferences" method:"get" tags:"IDC Account Notifications" summary:"Get current customer notification preferences" dc:"Returns the authoritative contact preferences, marketing consents, and quiet hours for the currently authenticated customer."`
}

type PreferenceGetRes struct {
	CustomerNo        string                   `json:"customerNo" dc:"Current authenticated customer identifier" eg:"customer_1001"`
	PreferredLocale   string                   `json:"preferredLocale" dc:"Current customer preferred locale, such as zh-CN or en-US" eg:"zh-CN"`
	Version           int64                    `json:"version" dc:"Aggregate optimistic preference version" eg:"3"`
	Preferences       []*ContactPreferenceItem `json:"preferences" dc:"Explicit contact, topic, and channel choices" eg:"[]"`
	MarketingConsents []*MarketingConsentItem  `json:"marketingConsents" dc:"Explicit current marketing consent choices" eg:"[]"`
	QuietHours        *QuietHoursItem          `json:"quietHours" dc:"Current marketing quiet-hours setting when configured; null when absent" eg:"null"`
}
