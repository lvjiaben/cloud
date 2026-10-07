// This file applies authenticated tenant and permission guards before principal persistence operations.

package principal

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const (
	maximumOnboardingTokenLength = 2048
	maximumBrowserNonceLength    = 128
	maximumDisplayNameRunes      = 256
)

type hostDataScope int

const (
	hostDataScopeAll        hostDataScope = 1
	hostDataScopeTenant     hostDataScope = 2
	hostDataScopeDepartment hostDataScope = 3
	hostDataScopeSelf       hostDataScope = 4
)

// Current returns the current identity's audience without creating records.
func (s *serviceImpl) Current(ctx context.Context) (*Audience, error) {
	current, err := s.currentContext(ctx)
	if err != nil {
		return nil, err
	}
	audience, err := s.persistence.current(ctx, current.UserID)
	if err != nil {
		if bizerr.Is(err, CodeAudienceInconsistent) {
			return audience, err
		}
		return nil, bizerr.WrapCode(err, CodePrincipalUnavailable)
	}
	// A customer identity that later acquires effective IDC management authority
	// is an invalid mixed audience. Re-evaluate the complete host authority on
	// every bound-customer read so omitted source-plugin permission snapshots do
	// not reopen customer self-service or audit writes.
	if audience != nil && audience.Code == AudienceCustomer {
		backoffice, resolveErr := s.isBackofficeIdentity(ctx, current)
		if resolveErr != nil {
			return nil, bizerr.WrapCode(resolveErr, CodePrincipalUnavailable)
		}
		if backoffice {
			return nil, bizerr.NewCode(CodeAudienceConflict)
		}
	}
	return audience, nil
}

// EnsureCurrentStaff idempotently binds an authorized management identity as staff.
func (s *serviceImpl) EnsureCurrentStaff(ctx context.Context) (*Audience, error) {
	current, err := s.currentContext(ctx)
	if err != nil {
		return nil, err
	}
	if isDelegatedIdentity(current) || !hasSupportedStaffDataScope(current) {
		return nil, bizerr.NewCode(CodeAdminAudienceRequired)
	}
	backoffice, err := s.isBackofficeIdentity(ctx, current)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAdminAudienceRequired)
	}
	if !backoffice {
		return nil, bizerr.NewCode(CodeAdminAudienceRequired)
	}
	audience, err := s.persistence.ensureStaff(ctx, current.UserID)
	if err == nil || bizerr.Is(err, CodeAudienceConflict) {
		return audience, err
	}
	return nil, bizerr.WrapCode(err, CodePrincipalUnavailable)
}

func (s *serviceImpl) OnboardingStatus(ctx context.Context) (*OnboardingState, error) {
	current, err := s.currentContext(ctx)
	if err != nil || isDelegatedIdentity(current) {
		return nil, bizerr.NewCode(CodeOnboardingUnavailable)
	}
	backoffice, err := s.isBackofficeIdentity(ctx, current)
	if err != nil || backoffice {
		return nil, bizerr.NewCode(CodeOnboardingUnavailable)
	}
	audience, err := s.persistence.current(ctx, current.UserID)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeOnboardingUnavailable)
	}
	if audience == nil {
		return &OnboardingState{Required: true}, nil
	}
	if audience.Code != AudienceCustomer || audience.Status != AudienceStatusActive || audience.CustomerNo == "" {
		return nil, bizerr.NewCode(CodeOnboardingUnavailable)
	}
	return &OnboardingState{CustomerNo: audience.CustomerNo}, nil
}

// hasSupportedStaffDataScope requires evidence that the host permission
// middleware resolved one documented role data scope before permanent binding.
func hasSupportedStaffDataScope(current bizctxcap.CurrentContext) bool {
	if current.DataScopeUnsupported {
		return false
	}
	switch hostDataScope(current.DataScope) {
	case hostDataScopeAll, hostDataScopeTenant, hostDataScopeDepartment, hostDataScopeSelf:
		return true
	default:
		return false
	}
}

// RedeemCustomerOnboarding validates the authenticated caller and passes only
// digested onboarding material into the atomic persistence boundary.
func (s *serviceImpl) RedeemCustomerOnboarding(
	ctx context.Context,
	currentUserID int,
	intentToken string,
	browserNonce string,
	hostRegistrationUserID int,
) (*CustomerBinding, error) {
	current, err := s.currentContext(ctx)
	if err != nil {
		return nil, err
	}
	if currentUserID <= 0 ||
		current.UserID != currentUserID ||
		isDelegatedIdentity(current) ||
		current.DataScopeUnsupported {
		return nil, bizerr.NewCode(CodeOnboardingUnavailable)
	}
	backoffice, err := s.isBackofficeIdentity(ctx, current)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeOnboardingUnavailable)
	}
	if backoffice {
		return nil, bizerr.NewCode(CodeOnboardingUnavailable)
	}

	var (
		token       = strings.TrimSpace(intentToken)
		browser     = strings.TrimSpace(browserNonce)
		displayName = strings.Join(strings.Fields(current.Username), " ")
	)
	if token == "" ||
		len(token) > maximumOnboardingTokenLength ||
		len(browser) < 16 || len(browser) > maximumBrowserNonceLength ||
		hostRegistrationUserID != current.UserID ||
		displayName == "" ||
		!utf8.ValidString(displayName) ||
		utf8.RuneCountInString(displayName) > maximumDisplayNameRunes {
		return nil, bizerr.NewCode(CodeOnboardingUnavailable)
	}
	binding, err := s.persistence.redeem(ctx, redeemInput{
		UserID:                 current.UserID,
		HostRegistrationUserID: hostRegistrationUserID,
		DisplayName:            displayName,
		UsernameDigest:         digestValue(normalizeUsername(current.Username)),
		TokenDigest:            digestValue(token),
		BrowserDigest:          digestValue(browser),
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeOnboardingUnavailable)
	}
	return binding, nil
}

// isDelegatedIdentity prevents an acting administrator or impersonation token
// from binding the represented identity to a permanent business audience.
func isDelegatedIdentity(current bizctxcap.CurrentContext) bool {
	return current.IsImpersonation || current.ActingAsTenant || current.ActingUserID > 0
}

// currentContext returns the authenticated single-merchant request snapshot.
func (s *serviceImpl) currentContext(ctx context.Context) (bizctxcap.CurrentContext, error) {
	current := s.bizCtxSvc.Current(ctx)
	if current.UserID <= 0 {
		return bizctxcap.CurrentContext{}, bizerr.NewCode(CodeIdentityRequired)
	}
	if current.TenantID != idccap.DefaultTenantID {
		return bizctxcap.CurrentContext{}, bizerr.NewCode(CodeDefaultTenantRequired)
	}
	return current, nil
}

// isBackofficeIdentity blocks every super administrator or effective IDC
// management permission from entering the customer onboarding path. The host
// authorization capability completes source-plugin contexts that omit the
// declared bizctx permission fields; errors fail closed at the caller.
func (s *serviceImpl) isBackofficeIdentity(
	ctx context.Context,
	current bizctxcap.CurrentContext,
) (bool, error) {
	if current.IsSuperAdmin || current.PlatformBypass || idccap.HasAdminPermission(current.Permissions) {
		return true, nil
	}
	if s.authorization == nil {
		return false, gerror.New("principal authorization service is unavailable")
	}
	platformAdmin, err := s.authorization.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil {
		return false, gerror.Wrap(err, "resolve platform administrator authority")
	}
	if platformAdmin {
		return true, nil
	}

	keys := make([]authz.PermissionKey, 0, len(s.adminPermissionKey)+1)
	keys = append(keys, authz.PermissionKey(idccap.HostPermissionWildcard))
	keys = append(keys, s.adminPermissionKey...)
	for start := 0; start < len(keys); start += authz.MaxBatchHasPermissions {
		end := min(start+authz.MaxBatchHasPermissions, len(keys))
		grants, err := s.authorization.BatchHasPermissions(ctx, keys[start:end])
		if err != nil {
			return false, gerror.Wrap(err, "resolve IDC management authority")
		}
		for _, key := range keys[start:end] {
			if grants[key] {
				return true, nil
			}
		}
	}
	return false, nil
}
