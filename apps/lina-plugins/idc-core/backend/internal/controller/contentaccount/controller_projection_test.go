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
	view := toDetail(&contenthub.Item{ContentNo: "cnt_safe", Type: "announcement", Visibility: "specific_services", Title: "Notice", Body: "Body", PublicPath: "/pages/announcements/cnt_safe", Status: "published", Version: 4, TargetServiceNos: []string{"SVC_secret"}, PublishedAt: &now, UpdatedAt: &now})
	payload, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	for _, forbidden := range []string{"specific_services", "SVC_secret", `"status"`, `"version"`, "targetService"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("account content projection leaked %q: %s", forbidden, text)
		}
	}
}
