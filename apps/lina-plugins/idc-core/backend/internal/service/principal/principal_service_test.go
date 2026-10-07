// This file verifies principal service guards and zero-write read behavior with isolated fakes.

package principal

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// testBizCtx returns one immutable host business-context snapshot.
type testBizCtx struct {
	current bizctxcap.CurrentContext
}

// Current returns the configured test snapshot.
func (s testBizCtx) Current(context.Context) bizctxcap.CurrentContext {
	return s.current
}

// testAuthorization records bounded host authorization reads.
type testAuthorization struct {
	platformAdmin bool
	platformErr   error
	batchErr      error
	grants        map[authz.PermissionKey]bool
	batches       [][]authz.PermissionKey
}

// BatchGetPermissions returns deterministic permission metadata for the
// complete host contract; principal tests do not otherwise consume it.
func (s *testAuthorization) BatchGetPermissions(
	_ context.Context,
	keys []authz.PermissionKey,
) (*capmodel.BatchResult[*authz.PermissionInfo, authz.PermissionKey], error) {
	result := &capmodel.BatchResult[*authz.PermissionInfo, authz.PermissionKey]{
		Items:      make(map[authz.PermissionKey]*authz.PermissionInfo, len(keys)),
		MissingIDs: []authz.PermissionKey{},
	}
	for _, key := range keys {
		result.Items[key] = &authz.PermissionInfo{Key: key}
	}
	return result, nil
}

// BatchHasPermissions returns configured exact-key grants.
func (s *testAuthorization) BatchHasPermissions(
	_ context.Context,
	keys []authz.PermissionKey,
) (map[authz.PermissionKey]bool, error) {
	s.batches = append(s.batches, append([]authz.PermissionKey(nil), keys...))
	if s.batchErr != nil {
		return nil, s.batchErr
	}
	result := make(map[authz.PermissionKey]bool, len(keys))
	for _, key := range keys {
		result[key] = s.grants[key]
	}
	return result, nil
}

// HasPermission returns one configured exact-key grant.
func (s *testAuthorization) HasPermission(_ context.Context, key authz.PermissionKey) (bool, error) {
	if s.batchErr != nil {
		return false, s.batchErr
	}
	return s.grants[key], nil
}

// IsPlatformAdmin returns the configured conservative platform-admin decision.
func (s *testAuthorization) IsPlatformAdmin(context.Context, authz.UserID) (bool, error) {
	if s.platformErr != nil {
		return false, s.platformErr
	}
	return s.platformAdmin, nil
}

// ReplaceRolePermissions is not used by the read-only principal authorization path.
func (s *testAuthorization) ReplaceRolePermissions(
	context.Context,
	authz.RoleID,
	[]authz.PermissionKey,
) error {
	return nil
}

// recordingPersistence records which persistence mutation path was selected.
type recordingPersistence struct {
	currentResult *Audience
	redeemResult  *CustomerBinding
	currentErr    error
	staffErr      error
	redeemErr     error
	currentInput  int
	staffInput    int
	redeemInput   redeemInput
	currentCalls  int
	staffCalls    int
	redeemCalls   int
}

// current records a read-only audience lookup.
func (s *recordingPersistence) current(_ context.Context, userID int) (*Audience, error) {
	s.currentCalls++
	s.currentInput = userID
	return s.currentResult, s.currentErr
}

// ensureStaff records a staff-binding mutation.
func (s *recordingPersistence) ensureStaff(_ context.Context, userID int) (*Audience, error) {
	s.staffCalls++
	s.staffInput = userID
	return &Audience{UserID: userID, Code: AudienceStaff, Status: AudienceStatusActive}, s.staffErr
}

// redeem records an onboarding redemption mutation.
func (s *recordingPersistence) redeem(_ context.Context, input redeemInput) (*CustomerBinding, error) {
	s.redeemCalls++
	s.redeemInput = input
	return s.redeemResult, s.redeemErr
}

// TestNewRejectsMissingBizCtx verifies initialization does not construct a partial service graph.
func TestNewRejectsMissingBizCtx(t *testing.T) {
	if _, err := New(nil, &testAuthorization{}, &principalTestAuditWriter{}); err == nil {
		t.Fatal("expected missing bizctx service to be rejected")
	}
	if _, err := New(testBizCtx{}, nil, &principalTestAuditWriter{}); err == nil {
		t.Fatal("expected missing authorization service to be rejected")
	}
	if _, err := New(testBizCtx{}, &testAuthorization{}, nil); err == nil {
		t.Fatal("expected missing audit writer to be rejected")
	}
	if _, err := NewReader(nil); err == nil {
		t.Fatal("expected missing reader bizctx service to be rejected")
	}
	if _, err := NewAuthorizedReader(nil, &testAuthorization{}); err == nil {
		t.Fatal("expected missing authorized-reader bizctx service to be rejected")
	}
	if _, err := NewAuthorizedReader(testBizCtx{}, nil); err == nil {
		t.Fatal("expected missing authorized-reader authorization service to be rejected")
	}
}

// TestCurrentIsReadOnly verifies ordinary audience inspection performs only one read.
func TestCurrentIsReadOnly(t *testing.T) {
	store := &recordingPersistence{currentResult: &Audience{
		UserID: 41,
		Code:   AudienceCustomer,
		Status: AudienceStatusActive,
	}}
	svc := &serviceImpl{
		bizCtxSvc:     testBizCtx{current: bizctxcap.CurrentContext{UserID: 41}},
		authorization: &testAuthorization{},
		persistence:   store,
	}
	result, err := svc.Current(context.Background())
	if err != nil {
		t.Fatalf("Current returned error: %v", err)
	}
	if result == nil || result.UserID != 41 || store.currentCalls != 1 || store.staffCalls != 0 || store.redeemCalls != 0 {
		t.Fatalf("expected one read and zero writes, result=%+v store=%+v", result, store)
	}
}

// TestCurrentRejectsCustomerThatLaterReceivesManagementAuthority verifies a
// persisted customer binding cannot be combined with newly granted backoffice
// authority, including grants omitted from the source-plugin context snapshot.
func TestCurrentRejectsCustomerThatLaterReceivesManagementAuthority(t *testing.T) {
	permission := idccap.HostPermissionWildcard
	store := &recordingPersistence{currentResult: &Audience{
		UserID: 41, Code: AudienceCustomer, Status: AudienceStatusActive, CustomerNo: "CUS_test",
	}}
	authorization := &testAuthorization{grants: map[authz.PermissionKey]bool{
		authz.PermissionKey(permission): true,
	}}
	svc := &serviceImpl{
		bizCtxSvc:     testBizCtx{current: bizctxcap.CurrentContext{UserID: 41}},
		authorization: authorization,
		persistence:   store,
	}
	if _, err := svc.Current(context.Background()); !bizerr.Is(err, CodeAudienceConflict) {
		t.Fatalf("expected mixed-audience conflict, got %v", err)
	}
	if store.currentCalls != 1 || len(authorization.batches) == 0 {
		t.Fatalf("expected one audience read and effective authority lookup: store=%+v auth=%+v", store, authorization)
	}
}

// TestCurrentRejectsNonDefaultTenantBeforePersistence verifies tenant isolation is fail-closed.
func TestCurrentRejectsNonDefaultTenantBeforePersistence(t *testing.T) {
	store := &recordingPersistence{}
	svc := &serviceImpl{
		bizCtxSvc:   testBizCtx{current: bizctxcap.CurrentContext{UserID: 41, TenantID: 9}},
		persistence: store,
	}
	_, err := svc.Current(context.Background())
	if !bizerr.Is(err, CodeDefaultTenantRequired) {
		t.Fatalf("expected default-tenant error, got %v", err)
	}
	if store.currentCalls != 0 || store.staffCalls != 0 || store.redeemCalls != 0 {
		t.Fatalf("expected no persistence access, got %+v", store)
	}
}

// TestServiceWrapsPersistenceFailures verifies public service methods expose
// stable business codes while retaining low-level causes for diagnostics.
func TestServiceWrapsPersistenceFailures(t *testing.T) {
	cause := errors.New("database unavailable")

	currentStore := &recordingPersistence{currentErr: cause}
	currentService := &serviceImpl{
		bizCtxSvc:   testBizCtx{current: bizctxcap.CurrentContext{UserID: 41}},
		persistence: currentStore,
	}
	if _, err := currentService.Current(context.Background()); !bizerr.Is(err, CodePrincipalUnavailable) || !errors.Is(err, cause) {
		t.Fatalf("expected wrapped principal failure, got %v", err)
	}

	staffStore := &recordingPersistence{staffErr: cause}
	staffService := &serviceImpl{
		bizCtxSvc: testBizCtx{current: bizctxcap.CurrentContext{
			UserID:       41,
			IsSuperAdmin: true,
			DataScope:    1,
		}},
		authorization: &testAuthorization{},
		persistence:   staffStore,
	}
	if _, err := staffService.EnsureCurrentStaff(context.Background()); !bizerr.Is(err, CodePrincipalUnavailable) || !errors.Is(err, cause) {
		t.Fatalf("expected wrapped staff persistence failure, got %v", err)
	}

	redeemStore := &recordingPersistence{redeemErr: bizerr.NewCode(CodeAudienceConflict)}
	redeemService := &serviceImpl{
		bizCtxSvc:     testBizCtx{current: bizctxcap.CurrentContext{UserID: 41, Username: "customer"}},
		authorization: &testAuthorization{},
		persistence:   redeemStore,
	}
	if _, err := redeemService.RedeemCustomerOnboarding(context.Background(), 41, "opaque-token", "browser-nonce-0001", 41); !bizerr.Is(err, CodeOnboardingUnavailable) || !bizerr.Is(errors.Unwrap(err), CodeAudienceConflict) {
		t.Fatalf("expected non-enumerating onboarding wrapper with retained cause, got %v", err)
	}
}

// TestEnsureCurrentStaffUsesOnlyEffectiveAuthority verifies role-like strings
// and unsupported data scopes cannot create a staff audience.
func TestEnsureCurrentStaffUsesOnlyEffectiveAuthority(t *testing.T) {
	tests := []struct {
		name    string
		current bizctxcap.CurrentContext
		allowed bool
	}{
		{name: "super administrator", current: bizctxcap.CurrentContext{UserID: 7, IsSuperAdmin: true, DataScope: 1}, allowed: true},
		{name: "IDC permission", current: bizctxcap.CurrentContext{UserID: 7, Permissions: []string{"idc:admin:customer:list"}, DataScope: 2}, allowed: true},
		{name: "missing data scope", current: bizctxcap.CurrentContext{UserID: 7, Permissions: []string{"idc:admin:customer:list"}}},
		{name: "unsupported data scope", current: bizctxcap.CurrentContext{UserID: 7, Permissions: []string{"idc:admin:customer:list"}, DataScope: 1, DataScopeUnsupported: true}},
		{name: "impersonated administrator", current: bizctxcap.CurrentContext{UserID: 7, Permissions: []string{"idc:admin:customer:list"}, DataScope: 1, IsImpersonation: true}},
		{name: "acting administrator", current: bizctxcap.CurrentContext{UserID: 7, Permissions: []string{"idc:admin:customer:list"}, DataScope: 1, ActingUserID: 9}},
		{name: "role-like value", current: bizctxcap.CurrentContext{UserID: 7, Permissions: []string{"admin"}, DataScope: 1}},
		{name: "unrelated permission", current: bizctxcap.CurrentContext{UserID: 7, Permissions: []string{"system:user:list"}, DataScope: 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &recordingPersistence{}
			svc := &serviceImpl{
				bizCtxSvc:     testBizCtx{current: test.current},
				authorization: &testAuthorization{},
				persistence:   store,
			}
			_, err := svc.EnsureCurrentStaff(context.Background())
			if test.allowed {
				if err != nil || store.staffCalls != 1 || store.staffInput != test.current.UserID {
					t.Fatalf("expected one staff bind, err=%v store=%+v", err, store)
				}
				return
			}
			if !bizerr.Is(err, CodeAdminAudienceRequired) || store.staffCalls != 0 {
				t.Fatalf("expected authority rejection without writes, err=%v store=%+v", err, store)
			}
		})
	}
}

// TestRedeemCustomerOnboardingHashesSensitiveInput verifies the raw token is
// not passed into persistence and role names are not treated as authority.
func TestRedeemCustomerOnboardingHashesSensitiveInput(t *testing.T) {
	store := &recordingPersistence{redeemResult: &CustomerBinding{UserID: 12, CustomerNo: "CUS_test"}}
	svc := &serviceImpl{
		bizCtxSvc: testBizCtx{current: bizctxcap.CurrentContext{
			UserID:      12,
			Username:    "  ExampleUser  ",
			Permissions: []string{"admin"},
		}},
		authorization: &testAuthorization{},
		persistence:   store,
	}
	result, err := svc.RedeemCustomerOnboarding(context.Background(), 12, "  opaque-secret-token  ", "browser-nonce-0001", 12)
	if err != nil {
		t.Fatalf("RedeemCustomerOnboarding returned error: %v", err)
	}
	if result == nil || result.CustomerNo != "CUS_test" || store.redeemCalls != 1 {
		t.Fatalf("expected one redemption, result=%+v store=%+v", result, store)
	}
	if store.redeemInput.TokenDigest != digestValue("opaque-secret-token") ||
		store.redeemInput.UsernameDigest != digestValue("exampleuser") ||
		store.redeemInput.DisplayName != "ExampleUser" {
		t.Fatalf("unexpected normalized redemption input: %+v", store.redeemInput)
	}
}

// TestRedeemCustomerOnboardingRejectsBackofficeAndIdentityMismatch verifies
// denied paths do not inspect or mutate onboarding persistence.
func TestRedeemCustomerOnboardingRejectsBackofficeAndIdentityMismatch(t *testing.T) {
	tests := []struct {
		name          string
		current       bizctxcap.CurrentContext
		currentUserID int
	}{
		{name: "identity mismatch", current: bizctxcap.CurrentContext{UserID: 12, Username: "user"}, currentUserID: 13},
		{name: "super administrator", current: bizctxcap.CurrentContext{UserID: 12, Username: "user", IsSuperAdmin: true}, currentUserID: 12},
		{name: "IDC administrator", current: bizctxcap.CurrentContext{UserID: 12, Username: "user", Permissions: []string{"idc:admin:server:list"}}, currentUserID: 12},
		{name: "platform bypass", current: bizctxcap.CurrentContext{UserID: 12, Username: "user", PlatformBypass: true}, currentUserID: 12},
		{name: "unsupported data scope", current: bizctxcap.CurrentContext{UserID: 12, Username: "user", DataScopeUnsupported: true}, currentUserID: 12},
		{name: "impersonation", current: bizctxcap.CurrentContext{UserID: 12, Username: "user", IsImpersonation: true}, currentUserID: 12},
		{name: "acting identity", current: bizctxcap.CurrentContext{UserID: 12, Username: "user", ActingUserID: 99}, currentUserID: 12},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &recordingPersistence{}
			svc := &serviceImpl{
				bizCtxSvc:     testBizCtx{current: test.current},
				authorization: &testAuthorization{},
				persistence:   store,
			}
			_, err := svc.RedeemCustomerOnboarding(context.Background(), test.currentUserID, "opaque-secret-token", "browser-nonce-0001", test.currentUserID)
			if !bizerr.Is(err, CodeOnboardingUnavailable) {
				t.Fatalf("expected non-enumerating onboarding error, got %v", err)
			}
			if store.redeemCalls != 0 || store.staffCalls != 0 || store.currentCalls != 0 {
				t.Fatalf("expected denied path to avoid persistence, got %+v", store)
			}
		})
	}
}

// TestRedeemCustomerOnboardingRejectsInvalidDisplayName verifies malformed or
// unbounded host identity data cannot cross the persistence boundary.
func TestRedeemCustomerOnboardingRejectsInvalidDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		username string
	}{
		{name: "blank", username: "  "},
		{name: "invalid UTF-8", username: string([]byte{0xff})},
		{name: "too long", username: strings.Repeat("客", maximumDisplayNameRunes+1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &recordingPersistence{}
			svc := &serviceImpl{
				bizCtxSvc: testBizCtx{current: bizctxcap.CurrentContext{
					UserID:   12,
					Username: test.username,
				}},
				authorization: &testAuthorization{},
				persistence:   store,
			}
			_, err := svc.RedeemCustomerOnboarding(context.Background(), 12, "opaque-token", "browser-nonce-0001", 12)
			if !bizerr.Is(err, CodeOnboardingUnavailable) || store.redeemCalls != 0 {
				t.Fatalf("expected invalid display name to reject without writes, err=%v store=%+v", err, store)
			}
		})
	}
}

// TestSourcePluginAuthorizationFallback verifies missing bizctx permission
// fields are completed through exact host authorization reads and wildcard checks.
func TestSourcePluginAuthorizationFallback(t *testing.T) {
	tests := []struct {
		name          string
		authorization *testAuthorization
		permissionKey []authz.PermissionKey
		wantBlocked   bool
	}{
		{name: "ordinary customer", authorization: &testAuthorization{}},
		{name: "platform administrator", authorization: &testAuthorization{platformAdmin: true}, wantBlocked: true},
		{
			name:          "cataloged IDC permission",
			authorization: &testAuthorization{grants: map[authz.PermissionKey]bool{"idc:admin:customer:list": true}},
			permissionKey: []authz.PermissionKey{"idc:admin:customer:list"},
			wantBlocked:   true,
		},
		{
			name:          "host wildcard",
			authorization: &testAuthorization{grants: map[authz.PermissionKey]bool{authz.PermissionKey(idccap.HostPermissionWildcard): true}},
			wantBlocked:   true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &recordingPersistence{redeemResult: &CustomerBinding{UserID: 12, CustomerNo: "CUS_test"}}
			svc := &serviceImpl{
				bizCtxSvc:          testBizCtx{current: bizctxcap.CurrentContext{UserID: 12, Username: "customer"}},
				authorization:      test.authorization,
				adminPermissionKey: test.permissionKey,
				persistence:        store,
			}
			_, err := svc.RedeemCustomerOnboarding(context.Background(), 12, "opaque-token", "browser-nonce-0001", 12)
			if test.wantBlocked {
				if !bizerr.Is(err, CodeOnboardingUnavailable) || store.redeemCalls != 0 {
					t.Fatalf("expected fail-closed rejection, err=%v store=%+v", err, store)
				}
				return
			}
			if err != nil || store.redeemCalls != 1 {
				t.Fatalf("expected ordinary customer redemption, err=%v store=%+v", err, store)
			}
		})
	}
}

// TestAuthorizationFallbackFailsClosedAndBatches verifies dependency failures
// reject onboarding and permission catalogs are queried within the host limit.
func TestAuthorizationFallbackFailsClosedAndBatches(t *testing.T) {
	for _, authorization := range []*testAuthorization{
		{platformErr: errors.New("platform authority unavailable")},
		{batchErr: errors.New("permission authority unavailable")},
	} {
		failedStore := &recordingPersistence{}
		failedService := &serviceImpl{
			bizCtxSvc:     testBizCtx{current: bizctxcap.CurrentContext{UserID: 12, Username: "customer", DataScope: 1}},
			authorization: authorization,
			persistence:   failedStore,
		}
		if _, err := failedService.EnsureCurrentStaff(context.Background()); !bizerr.Is(err, CodeAdminAudienceRequired) || failedStore.staffCalls != 0 {
			t.Fatalf("expected staff authorization failure to reject without writes, err=%v store=%+v", err, failedStore)
		}
		if _, err := failedService.RedeemCustomerOnboarding(context.Background(), 12, "opaque-token", "browser-nonce-0001", 12); !bizerr.Is(err, CodeOnboardingUnavailable) || failedStore.redeemCalls != 0 {
			t.Fatalf("expected authorization failure to reject without writes, err=%v store=%+v", err, failedStore)
		}
	}

	keys := make([]authz.PermissionKey, authz.MaxBatchHasPermissions+1)
	for index := range keys {
		keys[index] = authz.PermissionKey("idc:admin:test:" + strconv.Itoa(index+1))
	}
	authorization := &testAuthorization{grants: map[authz.PermissionKey]bool{keys[len(keys)-1]: true}}
	svc := &serviceImpl{authorization: authorization, adminPermissionKey: keys}
	blocked, err := svc.isBackofficeIdentity(context.Background(), bizctxcap.CurrentContext{UserID: 12})
	if err != nil || !blocked {
		t.Fatalf("expected final catalog grant to qualify, blocked=%v err=%v", blocked, err)
	}
	if len(authorization.batches) != 2 ||
		len(authorization.batches[0]) != authz.MaxBatchHasPermissions ||
		len(authorization.batches[1]) != 2 {
		t.Fatalf("unexpected authorization batch sizes: %+v", authorization.batches)
	}
}
