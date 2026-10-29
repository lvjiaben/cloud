package billing

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type managementFinanceBizCtxStub struct{ current bizctxcap.CurrentContext }

func (s managementFinanceBizCtxStub) Current(context.Context) bizctxcap.CurrentContext {
	return s.current
}

type managementFinancePrincipalStub struct{ audience *principal.Audience }

func (s managementFinancePrincipalStub) Current(context.Context) (*principal.Audience, error) {
	return s.audience, nil
}

type managementFinanceAuthzStub struct {
	grants        map[authz.PermissionKey]bool
	platformAdmin bool
}

func (managementFinanceAuthzStub) BatchGetPermissions(
	context.Context,
	[]authz.PermissionKey,
) (*capmodel.BatchResult[*authz.PermissionInfo, authz.PermissionKey], error) {
	return nil, nil
}

func (s managementFinanceAuthzStub) BatchHasPermissions(
	context.Context,
	[]authz.PermissionKey,
) (map[authz.PermissionKey]bool, error) {
	return s.grants, nil
}

func (managementFinanceAuthzStub) HasPermission(context.Context, authz.PermissionKey) (bool, error) {
	return false, nil
}

func (s managementFinanceAuthzStub) IsPlatformAdmin(context.Context, authz.UserID) (bool, error) {
	return s.platformAdmin, nil
}

func (managementFinanceAuthzStub) ReplaceRolePermissions(
	context.Context,
	authz.RoleID,
	[]authz.PermissionKey,
) error {
	return nil
}

func newManagementFinanceAuthorizerForTest(
	t *testing.T,
	current bizctxcap.CurrentContext,
	audience *principal.Audience,
	grants map[authz.PermissionKey]bool,
) ManagementFinanceAuthorizer {
	t.Helper()
	authorizer, err := NewManagementFinanceAuthorizer(
		managementFinanceBizCtxStub{current: current},
		managementFinancePrincipalStub{audience: audience},
		managementFinanceAuthzStub{grants: grants},
	)
	if err != nil {
		t.Fatalf("create management financial authorizer: %v", err)
	}
	return authorizer
}

func TestManagementFinanceAuthorizerKeepsEveryPermissionIndependent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	current := bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 2}
	staff := &principal.Audience{
		UserID: 71, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}
	tests := []struct {
		permission string
		authorize  func(ManagementFinanceAuthorizer) (int, error)
	}{
		{permissionBillingOverview, func(a ManagementFinanceAuthorizer) (int, error) { return a.AuthorizeFinancialOverview(ctx) }},
		{permissionBillingInvoiceList, func(a ManagementFinanceAuthorizer) (int, error) { return a.AuthorizeInvoiceList(ctx) }},
		{permissionBillingInvoiceQuery, func(a ManagementFinanceAuthorizer) (int, error) {
			return a.AuthorizeInvoiceDetail(ctx, "INV_AUTH_0001")
		}},
		{permissionBillingTransactionList, func(a ManagementFinanceAuthorizer) (int, error) { return a.AuthorizeTransactionList(ctx) }},
		{permissionBillingTransactionQuery, func(a ManagementFinanceAuthorizer) (int, error) {
			return a.AuthorizeTransactionDetail(ctx, "BTX_AUTH_0001")
		}},
		{permissionBillingAdjustmentList, func(a ManagementFinanceAuthorizer) (int, error) { return a.AuthorizeAdjustmentList(ctx) }},
		{permissionBillingAdjustmentQuery, func(a ManagementFinanceAuthorizer) (int, error) {
			return a.AuthorizeAdjustmentDetail(ctx, "BAR_AUTH_0001")
		}},
		{permissionBillingBalanceAdjustmentCreate, func(a ManagementFinanceAuthorizer) (int, error) { return a.AuthorizeBalanceAdjustmentCreate(ctx) }},
		{permissionBillingManualReceiptCreate, func(a ManagementFinanceAuthorizer) (int, error) { return a.AuthorizeManualReceiptCreate(ctx) }},
		{permissionBillingInvoiceWaiverCreate, func(a ManagementFinanceAuthorizer) (int, error) { return a.AuthorizeInvoiceWaiverCreate(ctx) }},
		{permissionBillingAdjustmentReview, func(a ManagementFinanceAuthorizer) (int, error) {
			return a.AuthorizeAdjustmentReview(ctx, "BAR_AUTH_0001")
		}},
		{permissionBillingAdjustmentPolicyWrite, func(a ManagementFinanceAuthorizer) (int, error) { return a.AuthorizeAdjustmentPolicyWrite(ctx) }},
	}
	for _, test := range tests {
		t.Run(test.permission, func(t *testing.T) {
			authorizer := newManagementFinanceAuthorizerForTest(t, current, staff, map[authz.PermissionKey]bool{
				authz.PermissionKey(test.permission): true,
			})
			if actor, err := test.authorize(authorizer); err != nil || actor != 71 {
				t.Fatalf("authorize exact permission actor=%d err=%v", actor, err)
			}
			missing := newManagementFinanceAuthorizerForTest(t, current, staff, map[authz.PermissionKey]bool{
				authz.PermissionKey(permissionBillingOverview): true,
			})
			if actor, err := test.authorize(missing); test.permission != permissionBillingOverview &&
				(actor != 0 || !bizerr.Is(err, CodeDirectoryNotAccessible)) {
				t.Fatalf("another financial permission implied access actor=%d err=%v", actor, err)
			}
		})
	}
}

func TestManagementFinanceAuthorizerRejectsUnsafeActorsAndInvalidReferences(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	staff := &principal.Audience{
		UserID: 71, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}
	tests := []struct {
		name     string
		current  bizctxcap.CurrentContext
		audience *principal.Audience
	}{
		{name: "impersonation", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 2, IsImpersonation: true}, audience: staff},
		{name: "acting user", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 2, ActingUserID: 90}, audience: staff},
		{name: "acting tenant", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 2, ActingAsTenant: true}, audience: staff},
		{name: "foreign tenant", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 9, DataScope: 2}, audience: staff},
		{name: "unsupported scope", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 2, DataScopeUnsupported: true}, audience: staff},
		{name: "unrepresentable scope", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 3}, audience: staff},
		{name: "customer audience", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 2}, audience: &principal.Audience{UserID: 71, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive}},
		{name: "mismatched principal", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 2}, audience: &principal.Audience{UserID: 72, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorizer := newManagementFinanceAuthorizerForTest(t, test.current, test.audience, map[authz.PermissionKey]bool{
				authz.PermissionKey(permissionBillingManualReceiptCreate): true,
			})
			if actor, err := authorizer.AuthorizeManualReceiptCreate(ctx); actor != 0 || !bizerr.Is(err, CodeDirectoryNotAccessible) {
				t.Fatalf("unsafe financial actor was accepted actor=%d err=%v", actor, err)
			}
		})
	}
	authorizer := newManagementFinanceAuthorizerForTest(
		t,
		bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 2},
		staff,
		map[authz.PermissionKey]bool{authz.PermissionKey(permissionBillingAdjustmentReview): true},
	)
	if actor, err := authorizer.AuthorizeAdjustmentReview(ctx, "BAR_AUTH_\x01"); actor != 0 || !bizerr.Is(err, CodeDirectoryNotAccessible) {
		t.Fatalf("invalid adjustment reference was accepted actor=%d err=%v", actor, err)
	}
}

func TestNewManagementFinanceAuthorizerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()
	current := managementFinanceBizCtxStub{}
	staff := managementFinancePrincipalStub{}
	authorization := managementFinanceAuthzStub{}
	if _, err := NewManagementFinanceAuthorizer(nil, staff, authorization); err == nil {
		t.Fatal("nil business context must be rejected")
	}
	if _, err := NewManagementFinanceAuthorizer(current, nil, authorization); err == nil {
		t.Fatal("nil principal must be rejected")
	}
	if _, err := NewManagementFinanceAuthorizer(current, staff, nil); err == nil {
		t.Fatal("nil authorization must be rejected")
	}
	var typedNilCurrent *managementFinanceBizCtxStub
	if _, err := NewManagementFinanceAuthorizer(typedNilCurrent, staff, authorization); err == nil {
		t.Fatal("typed nil business context must be rejected")
	}
	var typedNilStaff *managementFinancePrincipalStub
	if _, err := NewManagementFinanceAuthorizer(current, typedNilStaff, authorization); err == nil {
		t.Fatal("typed nil principal must be rejected")
	}
	var typedNilAuthorization *managementFinanceAuthzStub
	if _, err := NewManagementFinanceAuthorizer(current, staff, typedNilAuthorization); err == nil {
		t.Fatal("typed nil authorization must be rejected")
	}
}
