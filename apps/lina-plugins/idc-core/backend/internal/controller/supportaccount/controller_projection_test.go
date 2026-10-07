package supportaccount

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

func TestSafeDetailDoesNotSerializeInternalTicketFacts(t *testing.T) {
	now := time.Date(2026, time.August, 15, 1, 0, 0, 0, time.UTC)
	view := ToSafeDetail(&supportdesk.Detail{
		Ticket:           &supportdesk.Ticket{TicketNo: "TKT_safe", CustomerNo: "CUS_secret", DepartmentNo: "DEP_secret", AssigneeUserID: 91, Visibility: "customer", BaseStage: "waiting_upstream", CustomerStage: "in_progress", ClosedReason: "forced", Subject: "Help", Priority: "normal", CurrentRound: 3, Version: 2, LastActivityAt: now, CreatedAt: now, UpdatedAt: now, RelatedEntitlementNo: "ent_internal_secret", EntitlementAssociationRef: "sar_0123456789abcdef0123456789abcdef", RelatedEntitlementNameZhCN: "云服务器", RelatedEntitlementNameEnUS: "Cloud server", RelatedEntitlementStatus: "active", RelatedEntitlementVersion: 1},
		Messages:         []supportdesk.Message{{MessageNo: "MSG_safe", Type: "public_reply", Visibility: "customer_visible", AuthorType: "staff", AuthorUserID: 91, Body: "safe", CreatedAt: now}},
		CustomFields:     []supportdesk.CustomFieldValue{{FieldNo: "SCF_public", Visibility: "customer_visible", NameZhCN: "公开", NameEnUS: "Public", NormalizedValue: "visible"}, {FieldNo: "SCF_internal", Visibility: "internal_only", NameZhCN: "内部", NameEnUS: "Internal", NormalizedValue: "secret-field-value"}},
		AvailableActions: []string{"reply"},
	})
	payload, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	for _, forbidden := range []string{"CUS_secret", "DEP_secret", "assigneeUser", "waiting_upstream", "forced", "authorUser", "visibility", "SCF_internal", "secret-field-value", "ent_internal_secret", "entitlementNo"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("safe ticket projection leaked %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"status":"in_progress"`) {
		t.Fatalf("customer status projection missing: %s", text)
	}
	if !strings.Contains(text, `"currentRound":3`) {
		t.Fatalf("customer rating round projection missing: %s", text)
	}
	if !strings.Contains(text, `"SCF_public"`) || !strings.Contains(text, `"visible"`) {
		t.Fatalf("customer-visible custom field missing: %s", text)
	}
	if !strings.Contains(text, `"associationReference":"sar_0123456789abcdef0123456789abcdef"`) {
		t.Fatalf("controlled support association reference missing: %s", text)
	}
}
