// This file verifies ownership-first self-service guards without database access.

package customer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

const testCustomerNo = "CUS_0123456789abcdef0123456789abcdef"

type staticBizCtx struct {
	current bizctxcap.CurrentContext
}

func (s staticBizCtx) Current(context.Context) bizctxcap.CurrentContext {
	return s.current
}

type staticPrincipal struct {
	audience      *principal.Audience
	err           error
	calls         int
	staffAudience *principal.Audience
	staffErr      error
	staffCalls    int
}

func (s *staticPrincipal) Current(context.Context) (*principal.Audience, error) {
	s.calls++
	return s.audience, s.err
}

func (s *staticPrincipal) EnsureCurrentStaff(context.Context) (*principal.Audience, error) {
	s.staffCalls++
	return s.staffAudience, s.staffErr
}

func (*staticPrincipal) RedeemCustomerOnboarding(
	context.Context,
	int,
	string,
	string,
	int,
) (*principal.CustomerBinding, error) {
	return nil, nil
}

func (*staticPrincipal) OnboardingStatus(context.Context) (*principal.OnboardingState, error) {
	return &principal.OnboardingState{}, nil
}

func testAccessGuard(current bizctxcap.CurrentContext, principalSvc *staticPrincipal) *AccessGuard {
	return &AccessGuard{bizCtxSvc: staticBizCtx{current: current}, principal: principalSvc}
}

type recordingPersistence struct {
	profile     *Profile
	loadErr     error
	updateErr   error
	loadCalls   int
	updateCalls int
	loadedNo    string
	actorUserID int
	update      normalizedUpdate
}

func (p *recordingPersistence) load(_ context.Context, customerNo string) (*Profile, error) {
	p.loadCalls++
	p.loadedNo = customerNo
	return p.profile, p.loadErr
}

func (p *recordingPersistence) updateOwnBasics(
	_ context.Context,
	actorUserID int,
	input normalizedUpdate,
) (*Profile, error) {
	p.updateCalls++
	p.actorUserID = actorUserID
	p.update = input
	return p.profile, p.updateErr
}

type stubAuditWriter struct{}

type stubReauth struct{}

func (*stubReauth) Issue(context.Context, reauth.Binding) (*reauth.ProofContext, error) {
	return &reauth.ProofContext{}, nil
}

func (*stubReauth) Consume(context.Context, string, reauth.Binding) (*reauth.Proof, error) {
	return &reauth.Proof{}, nil
}

type stubClosurePrecheck struct{}

func (*stubClosurePrecheck) BatchCheck(context.Context, []string) ([]ClosurePrecheckResult, error) {
	return nil, nil
}

func (*stubAuditWriter) AppendTransition(
	context.Context,
	audit.TransitionInput,
) (*audit.Event, error) {
	return &audit.Event{}, nil
}

func (*stubAuditWriter) RecordObservation(
	context.Context,
	audit.ObservationInput,
) (*audit.Event, error) {
	return &audit.Event{}, nil
}

func TestNewRejectsMissingOrTypedNilDependencies(t *testing.T) {
	principalSvc := &staticPrincipal{}
	auditWriter := &stubAuditWriter{}
	taskSvc, err := asynctask.New(asynctask.Config{})
	if err != nil {
		t.Fatalf("create async task service: %v", err)
	}
	reauthSvc := &stubReauth{}
	closure := &stubClosurePrecheck{}
	access := testAccessGuard(bizctxcap.CurrentContext{UserID: 81}, principalSvc)
	if _, err = New(nil, auditWriter, taskSvc, reauthSvc, closure); err == nil {
		t.Fatal("missing customer access guard must fail")
	}
	if _, err = New(access, nil, taskSvc, reauthSvc, closure); err == nil {
		t.Fatal("missing audit writer must fail")
	}
	var typedNil *stubAuditWriter
	if _, err = New(access, typedNil, taskSvc, reauthSvc, closure); err == nil {
		t.Fatal("typed-nil audit writer must fail")
	}
	if _, err = New(access, auditWriter, nil, reauthSvc, closure); err == nil {
		t.Fatal("missing async task service must fail")
	}
	if _, err = New(access, auditWriter, taskSvc, nil, closure); err == nil {
		t.Fatal("missing reauth service must fail")
	}
	if _, err = New(access, auditWriter, taskSvc, reauthSvc, nil); err == nil {
		t.Fatal("missing closure precheck must fail")
	}
}

func TestCurrentProfileLoadsOnlyPersistedOwner(t *testing.T) {
	profile := validTestProfile()
	store := &recordingPersistence{profile: profile}
	principalSvc := activeTestPrincipal()
	service := &serviceImpl{
		access:      testAccessGuard(bizctxcap.CurrentContext{UserID: 81}, principalSvc),
		persistence: store,
	}
	result, err := service.CurrentProfile(context.Background())
	if err != nil || result != profile {
		t.Fatalf("CurrentProfile result=%+v err=%v", result, err)
	}
	if store.loadCalls != 1 || store.loadedNo != testCustomerNo || store.updateCalls != 0 {
		t.Fatalf("unexpected persistence calls: %+v", store)
	}
}

func TestGetOwnedProfileRejectsForeignAndMalformedBeforePersistence(t *testing.T) {
	requests := []string{
		"CUS_ffffffffffffffffffffffffffffffff",
		"not-a-customer-number",
		"",
	}
	for _, requested := range requests {
		t.Run(requested, func(t *testing.T) {
			store := &recordingPersistence{}
			service := &serviceImpl{
				access:      testAccessGuard(bizctxcap.CurrentContext{UserID: 81}, activeTestPrincipal()),
				persistence: store,
			}
			_, err := service.GetOwnedProfile(context.Background(), requested)
			if !bizerr.Is(err, CodeNotAccessible) {
				t.Fatalf("request %q returned %v, want inaccessible", requested, err)
			}
			if store.loadCalls != 0 || store.updateCalls != 0 {
				t.Fatalf("foreign request reached persistence: %+v", store)
			}
		})
	}
}

func TestDirectCustomerGuardRejectsDelegationAndAdminContext(t *testing.T) {
	tests := []bizctxcap.CurrentContext{
		{UserID: 81, ActingUserID: 7},
		{UserID: 81, IsImpersonation: true},
		{UserID: 81, ActingAsTenant: true},
		{UserID: 81, IsSuperAdmin: true},
		{UserID: 81, PlatformBypass: true},
		{UserID: 81, Permissions: []string{"idc:admin:customer:list"}},
	}
	for index, current := range tests {
		principalSvc := activeTestPrincipal()
		store := &recordingPersistence{}
		service := &serviceImpl{
			access:      testAccessGuard(current, principalSvc),
			persistence: store,
		}
		if _, err := service.CurrentProfile(context.Background()); !bizerr.Is(err, CodeNotAccessible) {
			t.Fatalf("case %d returned %v, want inaccessible", index, err)
		}
		if principalSvc.calls != 0 || store.loadCalls != 0 {
			t.Fatalf("case %d crossed direct-context guard", index)
		}
	}
}

func TestCustomerGuardAcceptsRepresentedCustomerWithoutActorPermissions(t *testing.T) {
	principalSvc := activeTestPrincipal()
	store := &recordingPersistence{profile: validTestProfile()}
	service := &serviceImpl{
		access: testAccessGuard(bizctxcap.CurrentContext{
			UserID: 81, ActingUserID: 7, IsImpersonation: true,
		}, principalSvc),
		persistence: store,
	}
	if _, err := service.CurrentProfile(context.Background()); err != nil {
		t.Fatalf("represented customer was rejected: %v", err)
	}
	if principalSvc.calls != 1 || store.loadCalls != 1 {
		t.Fatalf("represented customer did not use ordinary ownership path: principal=%d load=%d", principalSvc.calls, store.loadCalls)
	}
}

func TestUpdateOwnBasicsNormalizesOnlyAllowedFields(t *testing.T) {
	profile := validTestProfile()
	store := &recordingPersistence{profile: profile}
	service := &serviceImpl{
		access:      testAccessGuard(bizctxcap.CurrentContext{UserID: 81}, activeTestPrincipal()),
		persistence: store,
	}
	result, err := service.UpdateOwnBasics(context.Background(), UpdateOwnBasicsInput{
		CustomerNo:      testCustomerNo,
		DisplayName:     "  Example   Customer  ",
		CountryCode:     " us ",
		PreferredLocale: "en-US",
		ExpectedVersion: 1,
		IdempotencyKey:  " profile-request-1 ",
	})
	if err != nil || result != profile {
		t.Fatalf("UpdateOwnBasics result=%+v err=%v", result, err)
	}
	if store.updateCalls != 1 || store.loadCalls != 0 || store.actorUserID != 81 {
		t.Fatalf("unexpected update calls: %+v", store)
	}
	if store.update.displayName != "Example Customer" ||
		store.update.searchName != "example customer" ||
		store.update.countryCode != "US" ||
		store.update.preferredLocale != "en-US" ||
		store.update.idempotencyKey != "profile-request-1" {
		t.Fatalf("unexpected normalized update: %+v", store.update)
	}
}

func TestUpdateOwnBasicsPreservesStableBusinessErrorsAndWrapsFailures(t *testing.T) {
	input := UpdateOwnBasicsInput{
		CustomerNo: testCustomerNo, DisplayName: "Updated", CountryCode: "CN",
		PreferredLocale: "zh-CN", ExpectedVersion: 1, IdempotencyKey: "request-1",
	}
	for _, test := range []struct {
		err  error
		code *bizerr.Code
	}{
		{err: bizerr.NewCode(CodeProfileInvalid), code: CodeProfileInvalid},
		{err: bizerr.NewCode(CodeStateDenied), code: CodeStateDenied},
		{err: bizerr.NewCode(CodeVersionConflict), code: CodeVersionConflict},
		{err: bizerr.NewCode(CodeIdempotencyConflict), code: CodeIdempotencyConflict},
	} {
		store := &recordingPersistence{updateErr: test.err}
		service := &serviceImpl{
			access: testAccessGuard(bizctxcap.CurrentContext{UserID: 81}, activeTestPrincipal()), persistence: store,
		}
		_, err := service.UpdateOwnBasics(context.Background(), input)
		if !bizerr.Is(err, test.code) {
			t.Fatalf("expected stable error %v, got %v", test.err, err)
		}
	}

	cause := errors.New("database unavailable")
	store := &recordingPersistence{updateErr: cause}
	service := &serviceImpl{
		access: testAccessGuard(bizctxcap.CurrentContext{UserID: 81}, activeTestPrincipal()), persistence: store,
	}
	_, err := service.UpdateOwnBasics(context.Background(), input)
	if !bizerr.Is(err, CodeUnavailable) || !errors.Is(err, cause) {
		t.Fatalf("expected wrapped unavailable error, got %v", err)
	}
}

func TestNormalizeUpdateRejectsUnboundedAndControlInputs(t *testing.T) {
	valid := UpdateOwnBasicsInput{
		CustomerNo: testCustomerNo, DisplayName: "Customer", CountryCode: "CN",
		PreferredLocale: "zh-CN", ExpectedVersion: 1, IdempotencyKey: "request-1",
	}
	tests := []UpdateOwnBasicsInput{
		func() UpdateOwnBasicsInput { value := valid; value.CustomerNo = "CUS_bad"; return value }(),
		func() UpdateOwnBasicsInput {
			value := valid
			value.DisplayName = strings.Repeat("界", maximumDisplayNameRunes+1)
			return value
		}(),
		func() UpdateOwnBasicsInput { value := valid; value.CountryCode = "CHN"; return value }(),
		func() UpdateOwnBasicsInput { value := valid; value.PreferredLocale = "fr-FR"; return value }(),
		func() UpdateOwnBasicsInput { value := valid; value.ExpectedVersion = 0; return value }(),
		func() UpdateOwnBasicsInput { value := valid; value.IdempotencyKey = "bad\nkey"; return value }(),
	}
	for index, input := range tests {
		if _, err := normalizeUpdate(input); !bizerr.Is(err, CodeProfileInvalid) {
			t.Fatalf("case %d returned %v, want invalid", index, err)
		}
	}
}

func activeTestPrincipal() *staticPrincipal {
	return &staticPrincipal{audience: &principal.Audience{
		UserID: 81, Code: principal.AudienceCustomer,
		Status: principal.AudienceStatusActive, CustomerNo: testCustomerNo,
	}}
}

func validTestProfile() *Profile {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	return &Profile{
		Subject: Subject{
			CustomerNo: testCustomerNo, Type: TypeIndividual, Status: StatusActive,
			DisplayName: "Customer", CountryCode: "CN", PreferredLocale: "zh-CN",
			Version: 1, CreatedAt: now, UpdatedAt: now,
		},
		Individual: &IndividualProfile{
			IdentityStatus: VerificationUnverified, Version: 1, UpdatedAt: now,
		},
	}
}
