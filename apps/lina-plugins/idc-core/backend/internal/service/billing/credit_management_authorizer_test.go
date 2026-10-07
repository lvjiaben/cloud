package billing

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func newManagementCreditAuthorizerForTest(
	t *testing.T,
	current bizctxcap.CurrentContext,
	audience *principal.Audience,
	grants map[authz.PermissionKey]bool,
) ManagementCreditAuthorizer {
	t.Helper()
	authorizer, err := NewManagementCreditAuthorizer(
		managementFinanceBizCtxStub{current: current},
		managementFinancePrincipalStub{audience: audience},
		managementFinanceAuthzStub{grants: grants},
	)
	if err != nil {
		t.Fatalf("create management credit authorizer: %v", err)
	}
	return authorizer
}

func TestManagementCreditAuthorizerKeepsEveryPermissionIndependent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	current := bizctxcap.CurrentContext{UserID: 81, TenantID: 0, DataScope: 2}
	staff := &principal.Audience{
		UserID: 81, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}
	tests := []struct {
		permission string
		authorize  func(ManagementCreditAuthorizer) (int, error)
	}{
		{permissionCreditApplicationList, func(a ManagementCreditAuthorizer) (int, error) { return a.AuthorizeApplicationList(ctx) }},
		{permissionCreditApplicationQuery, func(a ManagementCreditAuthorizer) (int, error) {
			return a.AuthorizeApplicationDetail(ctx, "CAP_AUTH_0001")
		}},
		{permissionCreditApplicationReview, func(a ManagementCreditAuthorizer) (int, error) {
			return a.AuthorizeApplicationReview(ctx, "CAP_AUTH_0001")
		}},
		{permissionCreditAccountList, func(a ManagementCreditAuthorizer) (int, error) { return a.AuthorizeAccountList(ctx) }},
		{permissionCreditAccountQuery, func(a ManagementCreditAuthorizer) (int, error) { return a.AuthorizeAccountDetail(ctx, "CRA_AUTH_0001") }},
		{permissionCreditStatementList, func(a ManagementCreditAuthorizer) (int, error) { return a.AuthorizeStatementList(ctx) }},
		{permissionCreditStatementQuery, func(a ManagementCreditAuthorizer) (int, error) {
			return a.AuthorizeStatementDetail(ctx, "CST_AUTH_0001")
		}},
		{permissionCreditPolicyWrite, func(a ManagementCreditAuthorizer) (int, error) { return a.AuthorizePolicyWrite(ctx) }},
	}
	for _, test := range tests {
		t.Run(test.permission, func(t *testing.T) {
			authorizer := newManagementCreditAuthorizerForTest(t, current, staff, map[authz.PermissionKey]bool{
				authz.PermissionKey(test.permission): true,
			})
			if actor, err := test.authorize(authorizer); err != nil || actor != 81 {
				t.Fatalf("authorize exact credit permission actor=%d err=%v", actor, err)
			}
			missing := newManagementCreditAuthorizerForTest(t, current, staff, map[authz.PermissionKey]bool{
				authz.PermissionKey(permissionBillingOverview): true,
			})
			if actor, err := test.authorize(missing); actor != 0 || !bizerr.Is(err, CodeDirectoryNotAccessible) {
				t.Fatalf("another billing permission implied credit access actor=%d err=%v", actor, err)
			}
		})
	}
}

func TestManagementCreditAuthorizerRejectsDelegationAndInvalidReferences(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	staff := &principal.Audience{
		UserID: 81, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}
	authorizer := newManagementCreditAuthorizerForTest(
		t,
		bizctxcap.CurrentContext{UserID: 81, TenantID: 0, DataScope: 2, IsImpersonation: true},
		staff,
		map[authz.PermissionKey]bool{authz.PermissionKey(permissionCreditPolicyWrite): true},
	)
	if actor, err := authorizer.AuthorizePolicyWrite(ctx); actor != 0 || !bizerr.Is(err, CodeDirectoryNotAccessible) {
		t.Fatalf("impersonated credit actor was accepted actor=%d err=%v", actor, err)
	}
	authorizer = newManagementCreditAuthorizerForTest(
		t,
		bizctxcap.CurrentContext{UserID: 81, TenantID: 0, DataScope: 2},
		staff,
		map[authz.PermissionKey]bool{authz.PermissionKey(permissionCreditApplicationReview): true},
	)
	if actor, err := authorizer.AuthorizeApplicationReview(ctx, "CAP_AUTH_\x01"); actor != 0 || !bizerr.Is(err, CodeDirectoryNotAccessible) {
		t.Fatalf("invalid credit application reference was accepted actor=%d err=%v", actor, err)
	}
}
