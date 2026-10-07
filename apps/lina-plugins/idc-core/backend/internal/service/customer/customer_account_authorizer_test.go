// This file verifies exact host authorization for customer account-state writes.

package customer

import (
	"context"
	"errors"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type accountAuthorizerTestAuthz struct {
	grants        map[authz.PermissionKey]bool
	platformAdmin bool
	err           error
	batchCalls    int
	platformCalls int
	userID        authz.UserID
}

func (*accountAuthorizerTestAuthz) BatchGetPermissions(
	context.Context,
	[]authz.PermissionKey,
) (*capmodel.BatchResult[*authz.PermissionInfo, authz.PermissionKey], error) {
	return nil, nil
}

func (a *accountAuthorizerTestAuthz) BatchHasPermissions(
	_ context.Context,
	_ []authz.PermissionKey,
) (map[authz.PermissionKey]bool, error) {
	a.batchCalls++
	if a.err != nil {
		return nil, a.err
	}
	return a.grants, nil
}

func (*accountAuthorizerTestAuthz) HasPermission(
	context.Context,
	authz.PermissionKey,
) (bool, error) {
	return false, nil
}

func (a *accountAuthorizerTestAuthz) IsPlatformAdmin(
	_ context.Context,
	userID authz.UserID,
) (bool, error) {
	a.platformCalls++
	a.userID = userID
	if a.err != nil {
		return false, a.err
	}
	return a.platformAdmin, nil
}

func (*accountAuthorizerTestAuthz) ReplaceRolePermissions(
	context.Context,
	authz.RoleID,
	[]authz.PermissionKey,
) error {
	return nil
}

func TestAccountStateAuthorizerAcceptsOnlyHostGovernedAuthority(t *testing.T) {
	tests := []struct {
		name     string
		grants   map[authz.PermissionKey]bool
		platform bool
	}{
		{
			name: "exact permission",
			grants: map[authz.PermissionKey]bool{
				authz.PermissionKey(permissionCustomerAccountUpdate): true,
			},
		},
		{
			name: "host wildcard",
			grants: map[authz.PermissionKey]bool{
				authz.PermissionKey(idccap.HostPermissionWildcard): true,
			},
		},
		{name: "platform administrator", platform: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorization := &accountAuthorizerTestAuthz{
				grants: test.grants, platformAdmin: test.platform,
			}
			authorizer, err := NewAccountStateAuthorizer(
				staticBizCtx{current: bizctxcap.CurrentContext{
					UserID: 91, TenantID: idccap.DefaultTenantID, DataScope: 2,
					PlatformBypass: true,
				}},
				authorization,
			)
			if err != nil {
				t.Fatalf("create authorizer: %v", err)
			}
			if err = authorizer.AuthorizeTransition(context.Background(), testCustomerNo); err != nil {
				t.Fatalf("authorize transition: %v", err)
			}
			if authorization.batchCalls != 1 || authorization.platformCalls != 1 ||
				authorization.userID != "91" {
				t.Fatalf("unexpected authorization evidence calls: %+v", authorization)
			}
		})
	}
}

func TestAccountStateAuthorizerFailsClosedBeforeTargetAccess(t *testing.T) {
	contexts := []bizctxcap.CurrentContext{
		{},
		{UserID: 91, TenantID: 7, DataScope: 2},
		{UserID: 91, DataScope: 0},
		{UserID: 91, DataScope: 3},
		{UserID: 91, DataScope: 4},
		{UserID: 91, DataScope: 2, DataScopeUnsupported: true},
		{UserID: 91, DataScope: 2, ActingUserID: 12},
		{UserID: 91, DataScope: 2, IsImpersonation: true},
		{UserID: 91, DataScope: 2, ActingAsTenant: true},
	}
	for index, current := range contexts {
		authorization := &accountAuthorizerTestAuthz{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionCustomerAccountUpdate): true,
		}}
		authorizer, err := NewAccountStateAuthorizer(staticBizCtx{current: current}, authorization)
		if err != nil {
			t.Fatalf("case %d create authorizer: %v", index, err)
		}
		if err = authorizer.AuthorizeTransition(context.Background(), testCustomerNo); !bizerr.Is(err, CodeNotAccessible) {
			t.Fatalf("case %d returned %v, want inaccessible", index, err)
		}
		if authorization.batchCalls != 0 || authorization.platformCalls != 0 {
			t.Fatalf("case %d crossed context and data-scope guard: %+v", index, authorization)
		}
	}

	for _, target := range []string{"", "malformed", "CUS_ffffffff"} {
		authorization := &accountAuthorizerTestAuthz{}
		authorizer, err := NewAccountStateAuthorizer(
			staticBizCtx{current: bizctxcap.CurrentContext{UserID: 91, DataScope: 2}},
			authorization,
		)
		if err != nil {
			t.Fatalf("target %q create authorizer: %v", target, err)
		}
		if err = authorizer.AuthorizeTransition(context.Background(), target); !bizerr.Is(err, CodeNotAccessible) {
			t.Fatalf("target %q returned %v, want inaccessible", target, err)
		}
		if authorization.batchCalls != 0 {
			t.Fatalf("target %q crossed identifier guard", target)
		}
	}
}

func TestAccountStateAuthorizerRejectsMissingGrantAndAuthorizationFailure(t *testing.T) {
	for _, authorization := range []*accountAuthorizerTestAuthz{
		{},
		{err: errors.New("authorization unavailable")},
	} {
		authorizer, err := NewAccountStateAuthorizer(
			staticBizCtx{current: bizctxcap.CurrentContext{UserID: 91, DataScope: 2}},
			authorization,
		)
		if err != nil {
			t.Fatalf("create authorizer: %v", err)
		}
		if err = authorizer.AuthorizeTransition(context.Background(), testCustomerNo); !bizerr.Is(err, CodeNotAccessible) {
			t.Fatalf("authorization failure returned %v", err)
		}
	}
}

func TestNewAccountStateAuthorizerRejectsMissingAndTypedNilDependencies(t *testing.T) {
	authorization := &accountAuthorizerTestAuthz{}
	if _, err := NewAccountStateAuthorizer(nil, authorization); err == nil {
		t.Fatal("missing bizctx service must fail")
	}
	if _, err := NewAccountStateAuthorizer(staticBizCtx{}, nil); err == nil {
		t.Fatal("missing authorization service must fail")
	}
	var typedNil *accountAuthorizerTestAuthz
	if _, err := NewAccountStateAuthorizer(staticBizCtx{}, typedNil); err == nil {
		t.Fatal("typed-nil authorization service must fail")
	}
}
