// This file verifies production audience, scope and owner identity boundaries.
package productdirectory

import (
	"context"
	"errors"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"testing"
)

type accessContext struct{ current bizctxcap.CurrentContext }

func (a *accessContext) Current(context.Context) bizctxcap.CurrentContext { return a.current }

type accessPrincipal struct{ value *principal.Audience }

func (a *accessPrincipal) Current(context.Context) (*principal.Audience, error) { return a.value, nil }

type accessAuthorization struct {
	authz.Service
	granted bool
}

func (a *accessAuthorization) BatchHasPermissions(_ context.Context, keys []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	return map[authz.PermissionKey]bool{keys[0]: a.granted}, nil
}
func (a *accessAuthorization) IsPlatformAdmin(context.Context, authz.UserID) (bool, error) {
	return false, nil
}

type accessCustomer struct {
	value *identitycap.CustomerSubject
	err   error
	calls int
}

func (a *accessCustomer) CurrentCustomer(context.Context) (*identitycap.CustomerSubject, error) {
	a.calls++
	return a.value, a.err
}
func TestProductionDirectoryAccessRequiresCurrentOwnerAudience(t *testing.T) {
	ctx := context.Background()
	state := &accessContext{current: bizctxcap.CurrentContext{UserID: 7, DataScope: 2}}
	actor := &accessPrincipal{value: &principal.Audience{UserID: 7, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}}
	grants := &accessAuthorization{granted: true}
	customer := &accessCustomer{err: errors.New("staff has no customer audience")}
	access, err := NewAccess(state, actor, grants, customer)
	if err != nil {
		t.Fatal(err)
	}
	if id, err := access.Admin(ctx, PermissionList); err != nil || id != 7 {
		t.Fatalf("staff permission: %d %v", id, err)
	}
	if _, err := access.Customer(ctx); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("staff customer access: %v", err)
	}
	actor.value.Code = principal.AudienceCustomer
	if _, err := access.Admin(ctx, PermissionList); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("customer admin access: %v", err)
	}
	customer.err = nil
	customer.value = &identitycap.CustomerSubject{ActorUserID: 7, CustomerNo: "customer-a"}
	if id, err := access.Customer(ctx); err != nil || id != "customer-a" {
		t.Fatalf("customer owner: %s %v", id, err)
	}
	customer.value.CustomerNo = "customer-b"
	if id, err := access.Customer(ctx); err != nil || id != "customer-b" || customer.calls != 3 {
		t.Fatalf("owner must re-resolve: %s %v calls=%d", id, err, customer.calls)
	}
	actor.value.Code = principal.AudienceStaff
	grants.granted = false
	if _, err := access.Admin(ctx, PermissionUpdate); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("missing permission: %v", err)
	}
	grants.granted = true
	for _, scope := range []bizctxcap.CurrentContext{{UserID: 7, TenantID: 3, DataScope: 2}, {UserID: 7, DataScope: 3}, {UserID: 7, DataScope: 2, IsImpersonation: true}, {UserID: 7, DataScope: 2, ActingAsTenant: true}} {
		state.current = scope
		if _, err := access.Admin(ctx, PermissionCreate); !bizerr.Is(err, CodeNotAccessible) {
			t.Fatalf("unsafe scope %+v: %v", scope, err)
		}
	}
	if !bizerr.Is(directoryError(errors.New("database details")), CodeUnavailable) {
		t.Fatal("internal cause not wrapped")
	}
}
