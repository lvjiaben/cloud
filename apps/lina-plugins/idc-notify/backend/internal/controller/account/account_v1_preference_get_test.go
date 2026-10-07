package account

import (
	"testing"

	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func TestProjectPreferenceSnapshotKeepsAuthoritativeMarketingEvidence(t *testing.T) {
	result := projectPreferenceSnapshot(&notification.PreferenceSnapshot{MarketingConsents: []notification.MarketingConsent{{
		ContactNo: "contact_1", ChannelCode: "sms", Status: "opted_out",
		PolicyReference: "privacy/marketing", PolicyVersion: "2026-08", Locale: "zh-CN", Version: 4,
	}}})
	if len(result.MarketingConsents) != 1 {
		t.Fatalf("unexpected marketing projection: %+v", result.MarketingConsents)
	}
	item := result.MarketingConsents[0]
	if item.PolicyReference != "privacy/marketing" || item.PolicyVersion != "2026-08" || item.Locale != "zh-CN" {
		t.Fatalf("controller dropped authoritative marketing evidence: %+v", item)
	}
}
