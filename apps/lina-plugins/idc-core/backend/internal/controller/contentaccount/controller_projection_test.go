package contentaccount

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/internal/service/contenthub"
)

func TestAccountProjectionDoesNotSerializeAudienceFacts(t *testing.T) {
	now := time.Date(2026, time.August, 15, 1, 0, 0, 0, time.UTC)
	view := toDetail(&contenthub.Item{ContentNo: "cnt_safe", Type: "knowledge", Visibility: "customer_group", Title: "Notice", Body: "Body", PublicPath: "/pages/announcements/cnt_safe", Status: "published", Version: 4, TargetCustomerGroupNos: []string{"GRP_secret"}, PublishedAt: &now, UpdatedAt: &now})
	payload, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	for _, forbidden := range []string{"customer_group", "GRP_secret", `"status"`, `"version"`, "targetCustomerGroup"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("account content projection leaked %q: %s", forbidden, text)
		}
	}
}
