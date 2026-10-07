package productautomation

import (
	"context"
	"errors"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type automationManagementBizCtx struct{ current bizctxcap.CurrentContext }

func (b automationManagementBizCtx) Current(context.Context) bizctxcap.CurrentContext {
	return b.current
}

type automationManagementPrincipal struct{ audience *principal.Audience }

func (p *automationManagementPrincipal) Current(context.Context) (*principal.Audience, error) {
	return p.audience, nil
}

type automationManagementAuthz struct {
	grants        map[authz.PermissionKey]bool
	platformAdmin bool
}

func (*automationManagementAuthz) BatchGetPermissions(context.Context, []authz.PermissionKey) (*capmodel.BatchResult[*authz.PermissionInfo, authz.PermissionKey], error) {
	return nil, nil
}
func (a *automationManagementAuthz) BatchHasPermissions(context.Context, []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	return a.grants, nil
}
func (*automationManagementAuthz) HasPermission(context.Context, authz.PermissionKey) (bool, error) {
	return false, nil
}
func (a *automationManagementAuthz) IsPlatformAdmin(context.Context, authz.UserID) (bool, error) {
	return a.platformAdmin, nil
}
func (*automationManagementAuthz) ReplaceRolePermissions(context.Context, authz.RoleID, []authz.PermissionKey) error {
	return nil
}

type publicationAuthorityStub struct {
	actor     int
	err       error
	versionNo string
	calls     int
}

func (s *publicationAuthorityStub) AuthorizeProductPublication(_ context.Context, versionNo string) (int, error) {
	s.calls++
	s.versionNo = versionNo
	return s.actor, s.err
}

func TestPublicationOnlyAuthorizerDelegatesExactEstablishedAuthority(t *testing.T) {
	stub := &publicationAuthorityStub{actor: 71}
	authorizer, err := NewPublicationOnlyAuthorizer(stub)
	if err != nil {
		t.Fatalf("construct publication authorizer: %v", err)
	}
	authorized, err := authorizer.AuthorizePublicationAdmission(context.Background(), "ver_0123456789abcdef0123456789abcdef")
	if err != nil || authorized.ActorUserID != 71 || authorized.RequiredPermission != catalog.PermissionCatalogProductPublish {
		t.Fatalf("authorization=%+v err=%v", authorized, err)
	}
	if stub.calls != 1 || stub.versionNo != "ver_0123456789abcdef0123456789abcdef" {
		t.Fatalf("delegation calls=%d version=%q", stub.calls, stub.versionNo)
	}
}

func TestManagementAuthorizerDelegatesPublicationToEstablishedAuthority(t *testing.T) {
	stub := &publicationAuthorityStub{actor: 73}
	authorizer, err := NewManagementAuthorizer(automationManagementBizCtx{}, &automationManagementPrincipal{}, &automationManagementAuthz{}, stub)
	if err != nil {
		t.Fatalf("construct management authorizer: %v", err)
	}
	authorized, err := authorizer.AuthorizePublicationAdmission(context.Background(), "ver_0123456789abcdef0123456789abcdef")
	if err != nil || authorized.ActorUserID != 73 || authorized.RequiredPermission != catalog.PermissionCatalogProductPublish {
		t.Fatalf("authorization=%+v err=%v", authorized, err)
	}
	if stub.calls != 1 || stub.versionNo != "ver_0123456789abcdef0123456789abcdef" {
		t.Fatalf("delegation calls=%d version=%q", stub.calls, stub.versionNo)
	}
}

func TestManagementAuthorizerPermissionMatrix(t *testing.T) {
	ctx := context.Background()
	newAuthorizer := func(grants map[authz.PermissionKey]bool) Authorizer {
		authorizer, err := NewManagementAuthorizer(
			automationManagementBizCtx{current: bizctxcap.CurrentContext{UserID: 19, TenantID: 0, DataScope: 2}},
			&automationManagementPrincipal{audience: &principal.Audience{UserID: 19, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}},
			&automationManagementAuthz{grants: grants}, &publicationAuthorityStub{actor: 19},
		)
		if err != nil {
			t.Fatalf("construct management authorizer: %v", err)
		}
		return authorizer
	}
	viewOnly := newAuthorizer(map[authz.PermissionKey]bool{authz.PermissionKey(PermissionInterfaceView): true})
	if _, err := viewOnly.AuthorizeInterfaceList(ctx); err != nil {
		t.Fatalf("view permission denied list: %v", err)
	}
	if _, err := viewOnly.AuthorizeInterfaceCreate(ctx); !bizerr.Is(err, catalog.CodeManagementNotAccessible) {
		t.Fatalf("view permission substituted for manage: %v", err)
	}
	if _, err := viewOnly.AuthorizeBindingWrite(ctx, "ver_0123456789abcdef0123456789abcdef"); !bizerr.Is(err, catalog.CodeManagementNotAccessible) {
		t.Fatalf("view permission substituted for binding write: %v", err)
	}

	manageOnly := newAuthorizer(map[authz.PermissionKey]bool{authz.PermissionKey(PermissionInterfaceManage): true})
	if _, err := manageOnly.AuthorizeInterfaceCreate(ctx); err != nil {
		t.Fatalf("manage permission denied create: %v", err)
	}
	if _, err := manageOnly.AuthorizeInterfaceList(ctx); !bizerr.Is(err, catalog.CodeManagementNotAccessible) {
		t.Fatalf("manage permission substituted for view: %v", err)
	}
	if _, err := manageOnly.AuthorizeBindingWrite(ctx, "ver_0123456789abcdef0123456789abcdef"); !bizerr.Is(err, catalog.CodeManagementNotAccessible) {
		t.Fatalf("manage permission substituted for binding write: %v", err)
	}

	writeOnly := newAuthorizer(map[authz.PermissionKey]bool{authz.PermissionKey(PermissionBindingWrite): true})
	if _, err := writeOnly.AuthorizeBindingWrite(ctx, "ver_0123456789abcdef0123456789abcdef"); !bizerr.Is(err, catalog.CodeManagementNotAccessible) {
		t.Fatalf("binding write without view was accepted: %v", err)
	}
	writeAndView := newAuthorizer(map[authz.PermissionKey]bool{authz.PermissionKey(PermissionBindingWrite): true, authz.PermissionKey(PermissionInterfaceView): true})
	authorized, err := writeAndView.AuthorizeBindingWrite(ctx, "ver_0123456789abcdef0123456789abcdef")
	if err != nil || authorized.RequiredPermission != PermissionBindingWrite || authorized.ActorUserID != 19 {
		t.Fatalf("view plus binding write authorization=%+v err=%v", authorized, err)
	}
}

func TestManagementAuthorizerRejectsUnsafeBusinessContexts(t *testing.T) {
	base := bizctxcap.CurrentContext{UserID: 23, TenantID: 0, DataScope: 2}
	tests := []struct {
		name   string
		mutate func(*bizctxcap.CurrentContext)
	}{
		{name: "cross tenant", mutate: func(value *bizctxcap.CurrentContext) { value.TenantID = 1 }},
		{name: "unsupported scope flag", mutate: func(value *bizctxcap.CurrentContext) { value.DataScopeUnsupported = true }},
		{name: "unsupported scope value", mutate: func(value *bizctxcap.CurrentContext) { value.DataScope = 3 }},
		{name: "acting user", mutate: func(value *bizctxcap.CurrentContext) { value.ActingUserID = 99 }},
		{name: "impersonation", mutate: func(value *bizctxcap.CurrentContext) { value.IsImpersonation = true }},
		{name: "acting tenant", mutate: func(value *bizctxcap.CurrentContext) { value.ActingAsTenant = true }},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			current := base
			testCase.mutate(&current)
			authorizer, err := NewManagementAuthorizer(automationManagementBizCtx{current: current}, &automationManagementPrincipal{audience: &principal.Audience{UserID: 23, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}}, &automationManagementAuthz{grants: map[authz.PermissionKey]bool{authz.PermissionKey(PermissionInterfaceView): true}}, &publicationAuthorityStub{actor: 23})
			if err != nil {
				t.Fatalf("construct management authorizer: %v", err)
			}
			if _, err = authorizer.AuthorizeInterfaceList(context.Background()); !bizerr.Is(err, catalog.CodeManagementNotAccessible) {
				t.Fatalf("unsafe context was authorized: %v", err)
			}
		})
	}
}

func TestPublicationOnlyAuthorizerPropagatesPublicationDenialAndFailsOtherActionsClosed(t *testing.T) {
	denied := errors.New("publication denied")
	authorizer, err := NewPublicationOnlyAuthorizer(&publicationAuthorityStub{err: denied})
	if err != nil {
		t.Fatalf("construct publication authorizer: %v", err)
	}
	if _, err = authorizer.AuthorizePublicationAdmission(context.Background(), "ver_0123456789abcdef0123456789abcdef"); !errors.Is(err, denied) {
		t.Fatalf("publication denial=%v", err)
	}
	checks := []func() error{
		func() error { _, err := authorizer.AuthorizeProviderCandidates(context.Background()); return err },
		func() error { _, err := authorizer.AuthorizeInterfaceList(context.Background()); return err },
		func() error { _, err := authorizer.AuthorizeInterfaceCreate(context.Background()); return err },
		func() error { _, err := authorizer.AuthorizeInterfaceStop(context.Background(), "pfi_x"); return err },
		func() error { _, err := authorizer.AuthorizeInterfaceResume(context.Background(), "pfi_x"); return err },
		func() error { _, err := authorizer.AuthorizeBindingRead(context.Background(), "ver_x"); return err },
		func() error { _, err := authorizer.AuthorizeBindingWrite(context.Background(), "ver_x"); return err },
	}
	for index, check := range checks {
		if err = check(); !bizerr.Is(err, catalog.CodeManagementNotAccessible) {
			t.Fatalf("management action %d did not fail closed: %v", index, err)
		}
	}
}

func TestNewPublicationOnlyAuthorizerRejectsNil(t *testing.T) {
	if _, err := NewPublicationOnlyAuthorizer(nil); err == nil {
		t.Fatal("nil publication authorizer accepted")
	}
	var typedNil *publicationAuthorityStub
	if _, err := NewPublicationOnlyAuthorizer(typedNil); err == nil {
		t.Fatal("typed-nil publication authorizer accepted")
	}
}
