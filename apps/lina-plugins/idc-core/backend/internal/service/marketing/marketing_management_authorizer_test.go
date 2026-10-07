package marketing

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type marketingManagementBizCtxStub struct{ current bizctxcap.CurrentContext }

func (s marketingManagementBizCtxStub) Current(context.Context) bizctxcap.CurrentContext {
	return s.current
}

type marketingManagementPrincipalStub struct{ audience *principal.Audience }

func (s marketingManagementPrincipalStub) Current(context.Context) (*principal.Audience, error) {
	return s.audience, nil
}

type marketingManagementAuthzStub struct {
	grants        map[authz.PermissionKey]bool
	platformAdmin bool
}

func (marketingManagementAuthzStub) BatchGetPermissions(
	context.Context,
	[]authz.PermissionKey,
) (*capmodel.BatchResult[*authz.PermissionInfo, authz.PermissionKey], error) {
	return nil, nil
}

func (s marketingManagementAuthzStub) BatchHasPermissions(
	context.Context,
	[]authz.PermissionKey,
) (map[authz.PermissionKey]bool, error) {
	return s.grants, nil
}

func (marketingManagementAuthzStub) HasPermission(context.Context, authz.PermissionKey) (bool, error) {
	return false, nil
}

func (s marketingManagementAuthzStub) IsPlatformAdmin(context.Context, authz.UserID) (bool, error) {
	return s.platformAdmin, nil
}

func (marketingManagementAuthzStub) ReplaceRolePermissions(
	context.Context,
	authz.RoleID,
	[]authz.PermissionKey,
) error {
	return nil
}

func TestMarketingManagementAuthorizerRequiresDirectStaffAndExactPermission(t *testing.T) {
	t.Parallel()
	current := bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2}
	staff := marketingManagementPrincipalStub{audience: &principal.Audience{
		UserID: 7, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}}
	authorized, err := NewManagementOverviewAuthorizer(
		marketingManagementBizCtxStub{current: current}, staff,
		marketingManagementAuthzStub{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionMarketingOverview): true,
		}},
	)
	if err != nil {
		t.Fatalf("create marketing overview authorizer: %v", err)
	}
	if actor, authorizeErr := authorized.AuthorizeMarketingOverview(context.Background()); authorizeErr != nil || actor != 7 {
		t.Fatalf("authorize marketing overview actor=%d err=%v", actor, authorizeErr)
	}

	cases := []struct {
		name      string
		current   bizctxcap.CurrentContext
		audience  *principal.Audience
		granted   bool
		platform  bool
		wantAllow bool
	}{
		{name: "missing exact permission", current: current, audience: staff.audience},
		{name: "platform administrator", current: current, audience: staff.audience, platform: true, wantAllow: true},
		{name: "customer audience", current: current, audience: &principal.Audience{UserID: 7, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive}, granted: true},
		{name: "impersonation", current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2, IsImpersonation: true}, audience: staff.audience, granted: true},
		{name: "unsupported scope", current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2, DataScopeUnsupported: true}, audience: staff.audience, granted: true},
		{name: "foreign tenant", current: bizctxcap.CurrentContext{UserID: 7, TenantID: 9, DataScope: 2}, audience: staff.audience, granted: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			grants := map[authz.PermissionKey]bool{}
			if test.granted {
				grants[authz.PermissionKey(permissionMarketingOverview)] = true
			}
			authorizer, createErr := NewManagementOverviewAuthorizer(
				marketingManagementBizCtxStub{current: test.current},
				marketingManagementPrincipalStub{audience: test.audience},
				marketingManagementAuthzStub{grants: grants, platformAdmin: test.platform},
			)
			if createErr != nil {
				t.Fatalf("create authorizer: %v", createErr)
			}
			actor, authorizeErr := authorizer.AuthorizeMarketingOverview(context.Background())
			if test.wantAllow {
				if authorizeErr != nil || actor != 7 {
					t.Fatalf("expected allow actor=%d err=%v", actor, authorizeErr)
				}
				return
			}
			if !bizerr.Is(authorizeErr, CodeManagementOverviewNotAccessible) {
				t.Fatalf("expected fail-closed denial, got actor=%d err=%v", actor, authorizeErr)
			}
		})
	}
}

func TestMarketingManagementAuthorizerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()
	current := marketingManagementBizCtxStub{}
	staff := marketingManagementPrincipalStub{}
	authorization := marketingManagementAuthzStub{}
	if _, err := NewManagementOverviewAuthorizer(nil, staff, authorization); err == nil {
		t.Fatal("nil business context must be rejected")
	}
	if _, err := NewManagementOverviewAuthorizer(current, nil, authorization); err == nil {
		t.Fatal("nil principal must be rejected")
	}
	if _, err := NewManagementOverviewAuthorizer(current, staff, nil); err == nil {
		t.Fatal("nil authorization must be rejected")
	}
	var typedNilCurrent *marketingManagementBizCtxStub
	if _, err := NewManagementOverviewAuthorizer(typedNilCurrent, staff, authorization); err == nil {
		t.Fatal("typed nil business context must be rejected")
	}
	var typedNilStaff *marketingManagementPrincipalStub
	if _, err := NewManagementOverviewAuthorizer(current, typedNilStaff, authorization); err == nil {
		t.Fatal("typed nil principal must be rejected")
	}
	var typedNilAuthorization *marketingManagementAuthzStub
	if _, err := NewManagementOverviewAuthorizer(current, staff, typedNilAuthorization); err == nil {
		t.Fatal("typed nil authorization must be rejected")
	}
}

func TestMarketingManagementDirectoryAuthorizerKeepsPermissionsIndependent(t *testing.T) {
	t.Parallel()
	current := bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2}
	staff := marketingManagementPrincipalStub{audience: &principal.Audience{
		UserID: 7, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}}
	permissions := []struct {
		permission string
		authorize  func(ManagementAuthorizer) (int, error)
	}{
		{permissionMarketingSegmentList, func(a ManagementAuthorizer) (int, error) { return a.AuthorizeSegmentList(context.Background()) }},
		{permissionMarketingSegmentQuery, func(a ManagementAuthorizer) (int, error) { return a.AuthorizeSegmentQuery(context.Background()) }},
		{permissionMarketingCampaignList, func(a ManagementAuthorizer) (int, error) { return a.AuthorizeCampaignList(context.Background()) }},
		{permissionMarketingCampaignQuery, func(a ManagementAuthorizer) (int, error) { return a.AuthorizeCampaignQuery(context.Background()) }},
		{permissionMarketingAssetList, func(a ManagementAuthorizer) (int, error) { return a.AuthorizeAssetList(context.Background()) }},
		{permissionMarketingAssetQuery, func(a ManagementAuthorizer) (int, error) { return a.AuthorizeAssetQuery(context.Background()) }},
		{permissionMarketingPointsList, func(a ManagementAuthorizer) (int, error) { return a.AuthorizePointsList(context.Background()) }},
		{permissionMarketingPointsQuery, func(a ManagementAuthorizer) (int, error) { return a.AuthorizePointsQuery(context.Background()) }},
		{permissionMarketingGrantList, func(a ManagementAuthorizer) (int, error) { return a.AuthorizeGrantList(context.Background()) }},
		{permissionMarketingGrantQuery, func(a ManagementAuthorizer) (int, error) { return a.AuthorizeGrantQuery(context.Background()) }},
	}
	for _, test := range permissions {
		t.Run(test.permission, func(t *testing.T) {
			authorizer, err := NewManagementOverviewAuthorizer(
				marketingManagementBizCtxStub{current: current}, staff,
				marketingManagementAuthzStub{grants: map[authz.PermissionKey]bool{authz.PermissionKey(test.permission): true}},
			)
			if err != nil {
				t.Fatalf("create authorizer: %v", err)
			}
			if actor, authorizeErr := test.authorize(authorizer); authorizeErr != nil || actor != 7 {
				t.Fatalf("authorize exact permission actor=%d err=%v", actor, authorizeErr)
			}
			missing, err := NewManagementOverviewAuthorizer(
				marketingManagementBizCtxStub{current: current}, staff,
				marketingManagementAuthzStub{grants: map[authz.PermissionKey]bool{authz.PermissionKey(permissionMarketingOverview): true}},
			)
			if err != nil {
				t.Fatalf("create missing-permission authorizer: %v", err)
			}
			if actor, authorizeErr := test.authorize(missing); actor != 0 || !bizerr.Is(authorizeErr, CodeManagementDirectoryNotAccessible) {
				t.Fatalf("overview permission implied directory access actor=%d err=%v", actor, authorizeErr)
			}
		})
	}
}

func TestMarketingManagementActionAuthorizerKeepsPermissionsIndependent(t *testing.T) {
	t.Parallel()
	current := bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2}
	staff := marketingManagementPrincipalStub{audience: &principal.Audience{
		UserID: 7, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}}
	permissions := []struct {
		permission string
		authorize  func(ManagementAuthorizer) (int, error)
	}{
		{permissionMarketingAssetIssue, func(a ManagementAuthorizer) (int, error) { return a.AuthorizeAssetIssue(context.Background()) }},
		{permissionMarketingAssetRevoke, func(a ManagementAuthorizer) (int, error) { return a.AuthorizeAssetRevoke(context.Background()) }},
		{permissionMarketingPointsCredit, func(a ManagementAuthorizer) (int, error) { return a.AuthorizePointsCredit(context.Background()) }},
		{permissionMarketingPointsReverse, func(a ManagementAuthorizer) (int, error) { return a.AuthorizePointsReverse(context.Background()) }},
	}
	for _, test := range permissions {
		t.Run(test.permission, func(t *testing.T) {
			authorizer, err := NewManagementOverviewAuthorizer(
				marketingManagementBizCtxStub{current: current}, staff,
				marketingManagementAuthzStub{grants: map[authz.PermissionKey]bool{authz.PermissionKey(test.permission): true}},
			)
			if err != nil {
				t.Fatalf("create authorizer: %v", err)
			}
			if actor, authorizeErr := test.authorize(authorizer); authorizeErr != nil || actor != 7 {
				t.Fatalf("authorize exact permission actor=%d err=%v", actor, authorizeErr)
			}
			missing, err := NewManagementOverviewAuthorizer(
				marketingManagementBizCtxStub{current: current}, staff,
				marketingManagementAuthzStub{grants: map[authz.PermissionKey]bool{authz.PermissionKey(permissionMarketingOverview): true}},
			)
			if err != nil {
				t.Fatalf("create missing-permission authorizer: %v", err)
			}
			if actor, authorizeErr := test.authorize(missing); actor != 0 || !bizerr.Is(authorizeErr, CodeManagementActionNotAccessible) {
				t.Fatalf("read permission implied action access actor=%d err=%v", actor, authorizeErr)
			}
		})
	}
}
