// Segment command tests reject malformed references and preserve explicit add/remove intent.

package customermanagement

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"testing"
)

func TestNormalizeSegmentMutation(t *testing.T) {
	base := SegmentMutationInput{CustomerNo: "CUS_0123456789abcdef0123456789abcdef", Kind: SegmentKindGroup, Name: " Priority ", IdempotencyKey: "segment-test-request-0001"}
	for _, tc := range []struct {
		name            string
		edit            func(*SegmentMutationInput)
		remove, invalid bool
	}{
		{"create", func(*SegmentMutationInput) {}, false, false},
		{"assign", func(i *SegmentMutationInput) { i.Name = ""; i.SegmentNo = "GRP_0123456789abcdef0123456789abcdef" }, false, false},
		{"remove", func(i *SegmentMutationInput) { i.Name = ""; i.SegmentNo = "GRP_0123456789abcdef0123456789abcdef" }, true, false},
		{"remove requires target", func(*SegmentMutationInput) {}, true, true},
		{"mixed inputs", func(i *SegmentMutationInput) { i.SegmentNo = "GRP_0123456789abcdef0123456789abcdef" }, false, true},
		{"wrong kind", func(i *SegmentMutationInput) {
			i.Kind = SegmentKindTag
			i.Name = ""
			i.SegmentNo = "GRP_0123456789abcdef0123456789abcdef"
		}, false, true},
		{"bad customer", func(i *SegmentMutationInput) { i.CustomerNo = "not-customer" }, false, true},
		{"empty name", func(i *SegmentMutationInput) { i.Name = " " }, false, true},
		{"bad key", func(i *SegmentMutationInput) { i.IdempotencyKey = "short" }, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			tc.edit(&input)
			result, err := normalizeSegmentMutation(input, tc.remove)
			if (err != nil) != tc.invalid {
				t.Fatalf("invalid=%v err=%v", tc.invalid, err)
			}
			if !tc.invalid && input.Name != "" && result.Name != "Priority" {
				t.Fatal("name not trimmed")
			}
		})
	}
}

// Authorization precedes even malformed input, so no target existence is revealed.
func TestSegmentMutationRequiresExactPermission(t *testing.T) {
	service, err := newMutationService(managementTestBizCtx{current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 2}}, &managementTestPrincipal{audience: activeStaffAudience()}, &managementTestAuthz{grants: map[authz.PermissionKey]bool{}}, &tagMutationTestAuditWriter{}, &tagMutationTestPersistence{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.(SegmentMutationService).AddSegment(context.Background(), SegmentMutationInput{})
	if !bizerr.Is(err, customer.CodeNotAccessible) {
		t.Fatalf("unexpected denied result: %v", err)
	}
}
