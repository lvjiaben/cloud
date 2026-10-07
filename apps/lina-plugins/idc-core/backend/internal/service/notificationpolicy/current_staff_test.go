package notificationpolicy

import (
	"context"
	"errors"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type currentStaffBizCtxStub struct {
	current bizctxcap.CurrentContext
}

func (s currentStaffBizCtxStub) Current(context.Context) bizctxcap.CurrentContext {
	return s.current
}

type currentStaffPrincipalStub struct {
	audience *principal.Audience
	err      error
	calls    int
}

func (s *currentStaffPrincipalStub) Current(context.Context) (*principal.Audience, error) {
	s.calls++
	return s.audience, s.err
}

func TestCurrentStaffReturnsOnlyActiveDirectStaffProjection(t *testing.T) {
	reader, err := NewCurrentStaff(
		currentStaffBizCtxStub{current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 4}},
		&currentStaffPrincipalStub{audience: &principal.Audience{
			UserID: 7, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive, Version: 3,
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := reader.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if projection.HostUserID != 7 || projection.StaffVersion != 3 {
		t.Fatalf("unexpected projection: %+v", projection)
	}
}

func TestCurrentStaffFailsClosedBeforeAudienceReadForInvalidContext(t *testing.T) {
	tests := []struct {
		name    string
		current bizctxcap.CurrentContext
	}{
		{name: "missing user", current: bizctxcap.CurrentContext{TenantID: 0, DataScope: 2}},
		{name: "foreign tenant", current: bizctxcap.CurrentContext{UserID: 7, TenantID: 9, DataScope: 2}},
		{name: "acting user", current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2, ActingUserID: 8}},
		{name: "impersonation", current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2, IsImpersonation: true}},
		{name: "acting tenant", current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2, ActingAsTenant: true}},
		{name: "unsupported flag", current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2, DataScopeUnsupported: true}},
		{name: "unknown scope", current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 9}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			principalReader := &currentStaffPrincipalStub{}
			reader, err := NewCurrentStaff(currentStaffBizCtxStub{current: test.current}, principalReader)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = reader.Current(context.Background()); !bizerr.Is(err, codeCurrentStaffNotAccessible) {
				t.Fatalf("expected inaccessible error, got %v", err)
			}
			if principalReader.calls != 0 {
				t.Fatalf("invalid context reached principal read: %d", principalReader.calls)
			}
		})
	}
}

func TestCurrentStaffRejectsCustomerRevokedMismatchedAndUnavailableAudience(t *testing.T) {
	tests := []struct {
		name     string
		audience *principal.Audience
		err      error
	}{
		{name: "missing audience"},
		{name: "customer", audience: &principal.Audience{UserID: 7, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, Version: 1}},
		{name: "revoked staff", audience: &principal.Audience{UserID: 7, Code: principal.AudienceStaff, Status: principal.AudienceStatusRevoked, Version: 2}},
		{name: "other staff", audience: &principal.Audience{UserID: 8, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive, Version: 2}},
		{name: "missing version", audience: &principal.Audience{UserID: 7, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}},
		{name: "principal unavailable", err: errors.New("database unavailable")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader, err := NewCurrentStaff(
				currentStaffBizCtxStub{current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2}},
				&currentStaffPrincipalStub{audience: test.audience, err: test.err},
			)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = reader.Current(context.Background()); !bizerr.Is(err, codeCurrentStaffNotAccessible) {
				t.Fatalf("expected inaccessible error, got %v", err)
			}
		})
	}
}
