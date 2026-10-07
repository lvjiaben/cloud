package contentpublic

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/internal/service/contenthub"
)

func TestPublicProjectionDoesNotSerializeManagementFacts(t *testing.T) {
	now := time.Date(2026, time.August, 15, 1, 0, 0, 0, time.UTC)
	view := toDetail(&contenthub.Item{ContentNo: "cnt_safe", Type: "download", Visibility: "specific_customers", Title: "Public", Summary: "Summary", Body: "Body", PublicPath: "/pages/downloads/cnt_safe", Status: "published", CurrentFileNo: "FIL_secret", Version: 9, ValidityVersion: 8, TargetCustomerNos: []string{"CUS_secret"}, CandidateFile: &contenthub.CandidateFile{FileNo: "FIL_candidate"}, CurrentFile: &contenthub.CurrentFile{FileNo: "FIL_current", FileName: "guide.pdf", MediaType: "application/pdf", SizeBytes: 20, SHA256: strings.Repeat("a", 64)}, PublishedAt: &now, UpdatedAt: &now})
	payload, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	for _, forbidden := range []string{"specific_customers", "CUS_secret", "FIL_secret", "FIL_candidate", "FIL_current", "validityVersion", `"version"`, "candidate", "upload"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("public content projection leaked %q: %s", forbidden, text)
		}
	}
}
