package commerce

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type riskReviewBizCtxStub struct{ current bizctxcap.CurrentContext }

func (s riskReviewBizCtxStub) Current(context.Context) bizctxcap.CurrentContext { return s.current }

type riskReviewPrincipalStub struct{ audience *principal.Audience }

func (s riskReviewPrincipalStub) Current(context.Context) (*principal.Audience, error) {
	return s.audience, nil
}

type riskReviewAuthzStub struct {
	grants        map[authz.PermissionKey]bool
	platformAdmin bool
}

func (riskReviewAuthzStub) BatchGetPermissions(
	context.Context,
	[]authz.PermissionKey,
) (*capmodel.BatchResult[*authz.PermissionInfo, authz.PermissionKey], error) {
	return nil, nil
}

func (s riskReviewAuthzStub) BatchHasPermissions(
	context.Context,
	[]authz.PermissionKey,
) (map[authz.PermissionKey]bool, error) {
	return s.grants, nil
}

func (riskReviewAuthzStub) HasPermission(context.Context, authz.PermissionKey) (bool, error) {
	return false, nil
}

func (s riskReviewAuthzStub) IsPlatformAdmin(context.Context, authz.UserID) (bool, error) {
	return s.platformAdmin, nil
}

func (riskReviewAuthzStub) ReplaceRolePermissions(
	context.Context,
	authz.RoleID,
	[]authz.PermissionKey,
) error {
	return nil
}

func TestOrderRiskReviewAuthorizerKeepsApproveAndRejectIndependent(t *testing.T) {
	current := bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 2}
	staff := riskReviewPrincipalStub{audience: &principal.Audience{
		UserID: 71, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}}
	authorizer, err := NewOrderRiskReviewAuthorizer(
		riskReviewBizCtxStub{current: current}, staff,
		riskReviewAuthzStub{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionOrderRiskReviewApprove): true,
		}},
	)
	if err != nil {
		t.Fatalf("create risk review authorizer: %v", err)
	}
	if actor, authorizeErr := authorizer.AuthorizeRiskReviewApprove(
		context.Background(), "REV_0123456789abcdef0123456789abcdef",
	); authorizeErr != nil || actor != 71 {
		t.Fatalf("approve permission actor=%d err=%v", actor, authorizeErr)
	}
	if actor, authorizeErr := authorizer.AuthorizeRiskReviewReject(
		context.Background(), "REV_0123456789abcdef0123456789abcdef",
	); !bizerr.Is(authorizeErr, CodeOrderRiskReviewNotAccessible) || actor != 0 {
		t.Fatalf("approve grant leaked into rejection actor=%d err=%v", actor, authorizeErr)
	}
}

func TestOrderRiskReviewAuthorizerFailsClosedForUnsafeActors(t *testing.T) {
	staff := &principal.Audience{
		UserID: 71, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}
	cases := []struct {
		name     string
		current  bizctxcap.CurrentContext
		audience *principal.Audience
		platform bool
		allow    bool
	}{
		{name: "exact direct staff", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 1}, audience: staff, allow: true},
		{name: "platform administrator", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 2}, audience: staff, platform: true, allow: true},
		{name: "customer audience", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 1}, audience: &principal.Audience{UserID: 71, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive}},
		{name: "impersonation", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 1, IsImpersonation: true}, audience: staff},
		{name: "unsupported scope", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 2, DataScopeUnsupported: true}, audience: staff},
		{name: "foreign tenant", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 9, DataScope: 1}, audience: staff},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			authorizer, err := NewOrderRiskReviewAuthorizer(
				riskReviewBizCtxStub{current: testCase.current},
				riskReviewPrincipalStub{audience: testCase.audience},
				riskReviewAuthzStub{
					grants: map[authz.PermissionKey]bool{
						authz.PermissionKey(permissionOrderRiskReviewApprove): true,
					},
					platformAdmin: testCase.platform,
				},
			)
			if err != nil {
				t.Fatalf("create risk review authorizer: %v", err)
			}
			actor, authorizeErr := authorizer.AuthorizeRiskReviewApprove(
				context.Background(), "REV_0123456789abcdef0123456789abcdef",
			)
			if testCase.allow {
				if authorizeErr != nil || actor != 71 {
					t.Fatalf("expected allow actor=%d err=%v", actor, authorizeErr)
				}
				return
			}
			if !bizerr.Is(authorizeErr, CodeOrderRiskReviewNotAccessible) || actor != 0 {
				t.Fatalf("unsafe actor was not denied actor=%d err=%v", actor, authorizeErr)
			}
		})
	}
}

func TestOrderRiskReviewAuthorizerRejectsMissingDependencies(t *testing.T) {
	current := riskReviewBizCtxStub{}
	staff := riskReviewPrincipalStub{}
	authorization := riskReviewAuthzStub{}
	if _, err := NewOrderRiskReviewAuthorizer(nil, staff, authorization); err == nil {
		t.Fatal("nil business context must be rejected")
	}
	if _, err := NewOrderRiskReviewAuthorizer(current, nil, authorization); err == nil {
		t.Fatal("nil principal must be rejected")
	}
	if _, err := NewOrderRiskReviewAuthorizer(current, staff, nil); err == nil {
		t.Fatal("nil authorization must be rejected")
	}
}
