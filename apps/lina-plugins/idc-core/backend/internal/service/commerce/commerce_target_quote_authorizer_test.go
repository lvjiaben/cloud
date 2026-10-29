// Staff authorization must fail before touching quotation or customer facts.
package commerce

import (
	"context"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"testing"
)

func TestTargetQuoteAuthorizerRejectsUnsafeStaff(t *testing.T) {
	for _, test := range []struct {
		name     string
		current  bizctxcap.CurrentContext
		audience *principal.Audience
		grant    bool
		allow    bool
	}{
		{"exact", bizctxcap.CurrentContext{UserID: 81, DataScope: 2}, &principal.Audience{UserID: 81, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}, true, true},
		{"no-edit", bizctxcap.CurrentContext{UserID: 81, DataScope: 1}, &principal.Audience{UserID: 81, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}, false, false},
		{"impersonation", bizctxcap.CurrentContext{UserID: 81, DataScope: 1, IsImpersonation: true}, &principal.Audience{UserID: 81, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}, true, false},
		{"unsupported-scope", bizctxcap.CurrentContext{UserID: 81, DataScope: 3}, &principal.Audience{UserID: 81, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}, true, false},
		{"customer", bizctxcap.CurrentContext{UserID: 81, DataScope: 1}, &principal.Audience{UserID: 81, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: "customer-1"}, true, false},
		{"actor-mismatch", bizctxcap.CurrentContext{UserID: 81, DataScope: 1}, &principal.Audience{UserID: 82, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}, true, false},
		{"tenant", bizctxcap.CurrentContext{UserID: 81, DataScope: 1, TenantID: 2}, &principal.Audience{UserID: 81, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			authorizer, err := NewTargetQuoteAuthorizer(riskReviewBizCtxStub{current: test.current}, riskReviewPrincipalStub{audience: test.audience}, riskReviewAuthzStub{grants: map[authz.PermissionKey]bool{authz.PermissionKey(permissionTargetQuoteEdit): test.grant}})
			if err != nil {
				t.Fatal(err)
			}
			actor, err := authorizer.AuthorizeTargetQuote(context.Background())
			if test.allow {
				if err != nil || actor != 81 {
					t.Fatalf("actor=%d error=%+v", actor, err)
				}
			} else if err == nil || actor != 0 {
				t.Fatalf("unsafe staff accepted actor=%d error=%+v", actor, err)
			}
		})
	}
}
