// This file resolves authenticated user actors and validates trusted resolver output.

package audit

import (
	"context"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	hostDataScopeAll        = 1
	hostDataScopeTenant     = 2
	hostDataScopeDepartment = 3
	hostDataScopeSelf       = 4

	// permissionAuditRead is checked as an exact effective permission. This
	// internal service constant does not register an HTTP permission or route.
	permissionAuditRead = "idc:admin:audit:list"
)

// userActorResolver accepts only the host-authenticated user represented by
// bizctx and its persisted principal audience.
type userActorResolver struct {
	bizCtxSvc    bizctxcap.Service
	principalSvc principal.Reader
}

// newUserActorResolver creates the production user-only actor resolver.
func newUserActorResolver(
	bizCtxSvc bizctxcap.Service,
	principalSvc principal.Reader,
) (*userActorResolver, error) {
	if isNilInterface(bizCtxSvc) {
		return nil, gerror.New("audit actor resolver requires host bizctx service")
	}
	if isNilInterface(principalSvc) {
		return nil, gerror.New("audit actor resolver requires principal service")
	}
	return &userActorResolver{bizCtxSvc: bizCtxSvc, principalSvc: principalSvc}, nil
}

// ResolveTrustedActor resolves one authenticated user without accepting any
// caller-selected actor type or identity field.
func (r *userActorResolver) ResolveTrustedActor(ctx context.Context) (TrustedActor, error) {
	current := r.bizCtxSvc.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID {
		return TrustedActor{}, bizerr.NewCode(CodeIdentityDenied)
	}
	if current.ActingUserID < 0 || current.ActingUserID == current.UserID {
		return TrustedActor{}, bizerr.NewCode(CodeIdentityDenied)
	}
	delegated := current.ActingUserID > 0 || current.IsImpersonation || current.ActingAsTenant
	if delegated && current.ActingUserID <= 0 {
		return TrustedActor{}, bizerr.NewCode(CodeIdentityDenied)
	}
	audience, err := r.principalSvc.Current(ctx)
	if err != nil {
		return TrustedActor{}, bizerr.WrapCode(err, CodeIdentityDenied)
	}
	if audience == nil ||
		audience.UserID != current.UserID ||
		audience.Status != principal.AudienceStatusActive {
		return TrustedActor{}, bizerr.NewCode(CodeIdentityDenied)
	}
	switch audience.Code {
	case principal.AudienceCustomer:
		if audience.CustomerNo != strings.TrimSpace(audience.CustomerNo) ||
			!validReferenceText(audience.CustomerNo, 40, true) {
			return TrustedActor{}, bizerr.NewCode(CodeIdentityDenied)
		}
	case principal.AudienceStaff:
		if audience.CustomerNo != "" {
			return TrustedActor{}, bizerr.NewCode(CodeIdentityDenied)
		}
	default:
		return TrustedActor{}, bizerr.NewCode(CodeIdentityDenied)
	}
	return TrustedActor{
		Type:                 ActorUser,
		UserID:               current.UserID,
		ActingUserID:         current.ActingUserID,
		AudienceCode:         audience.Code,
		CustomerNo:           audience.CustomerNo,
		DataScope:            current.DataScope,
		DataScopeUnsupported: current.DataScopeUnsupported,
		PlatformBypass:       current.PlatformBypass,
		Delegated:            delegated,
	}, nil
}

// resolveActor validates every trusted resolver output before it reaches persistence.
func (s *serviceImpl) resolveActor(ctx context.Context) (TrustedActor, error) {
	actor, err := s.actorResolver.ResolveTrustedActor(ctx)
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return TrustedActor{}, err
		}
		return TrustedActor{}, bizerr.WrapCode(err, CodeIdentityDenied)
	}
	switch actor.Type {
	case ActorUser:
		if actor.Reference != "" || actor.UserID <= 0 || actor.ActingUserID < 0 || actor.ActingUserID == actor.UserID ||
			actor.Delegated != (actor.ActingUserID > 0) {
			return TrustedActor{}, bizerr.NewCode(CodeIdentityDenied)
		}
		if actor.AudienceCode != principal.AudienceCustomer &&
			actor.AudienceCode != principal.AudienceStaff {
			return TrustedActor{}, bizerr.NewCode(CodeIdentityDenied)
		}
		if actor.AudienceCode == principal.AudienceCustomer &&
			(actor.CustomerNo != strings.TrimSpace(actor.CustomerNo) ||
				!validReferenceText(actor.CustomerNo, 40, true)) {
			return TrustedActor{}, bizerr.NewCode(CodeIdentityDenied)
		}
		if actor.AudienceCode == principal.AudienceStaff && actor.CustomerNo != "" {
			return TrustedActor{}, bizerr.NewCode(CodeIdentityDenied)
		}
	case ActorSystem, ActorExternal:
		if !validReferenceText(actor.Reference, maximumIdentifierRunes, true) ||
			actor.Reference != strings.TrimSpace(actor.Reference) ||
			actor.UserID != 0 || actor.ActingUserID != 0 || actor.AudienceCode != "" || actor.CustomerNo != "" ||
			actor.DataScope != 0 || actor.DataScopeUnsupported || actor.PlatformBypass || actor.Delegated {
			return TrustedActor{}, bizerr.NewCode(CodeIdentityDenied)
		}
	default:
		return TrustedActor{}, bizerr.NewCode(CodeIdentityDenied)
	}
	return actor, nil
}

// authorizationEvidence is derived only from the host authorization capability.
type authorizationEvidence struct {
	RequiredPermission string `json:"required_permission"`
	RequiredGranted    bool   `json:"required_granted"`
	WildcardGranted    bool   `json:"wildcard_granted"`
	PlatformAdmin      bool   `json:"platform_admin"`
}

// granted reports whether exact, wildcard, or platform-administrator authority exists.
func (e authorizationEvidence) granted() bool {
	return e.RequiredGranted || e.WildcardGranted || e.PlatformAdmin
}

// resolveAuthorization queries host-governed permission and platform-role evidence.
func (s *serviceImpl) resolveAuthorization(
	ctx context.Context,
	actor TrustedActor,
	permission string,
) (authorizationEvidence, error) {
	if actor.Type != ActorUser || actor.UserID <= 0 || !idccap.IsAdminPermission(permission) {
		return authorizationEvidence{}, bizerr.NewCode(CodePermissionDenied)
	}
	keys := []authz.PermissionKey{
		authz.PermissionKey(permission),
		authz.PermissionKey(idccap.HostPermissionWildcard),
	}
	grants, err := s.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return authorizationEvidence{}, bizerr.WrapCode(err, CodePermissionDenied)
	}
	platformAdmin, err := s.authorization.IsPlatformAdmin(
		ctx,
		authz.UserID(strconv.Itoa(actor.UserID)),
	)
	if err != nil {
		return authorizationEvidence{}, bizerr.WrapCode(err, CodePermissionDenied)
	}
	return authorizationEvidence{
		RequiredPermission: permission,
		RequiredGranted:    grants[authz.PermissionKey(permission)],
		WildcardGranted:    grants[authz.PermissionKey(idccap.HostPermissionWildcard)],
		PlatformAdmin:      platformAdmin,
	}, nil
}

// validateWriteAuthority binds customer writes to their own subject and checks
// exact staff authority through authz without granting the underlying business action.
func (s *serviceImpl) validateWriteAuthority(
	ctx context.Context,
	actor TrustedActor,
	fields *Fields,
) (authorizationEvidence, error) {
	if actor.DataScopeUnsupported {
		return authorizationEvidence{}, bizerr.NewCode(CodePermissionDenied)
	}
	switch actor.Type {
	case ActorSystem, ActorExternal:
		if fields.RequiredPermission != "" {
			return authorizationEvidence{}, bizerr.NewCode(CodeInvalid)
		}
		return authorizationEvidence{}, nil
	case ActorUser:
	default:
		return authorizationEvidence{}, bizerr.NewCode(CodeIdentityDenied)
	}
	switch actor.AudienceCode {
	case principal.AudienceCustomer:
		if fields.RequiredPermission != "" {
			return authorizationEvidence{}, bizerr.NewCode(CodePermissionDenied)
		}
		if fields.CustomerNo == "" {
			fields.CustomerNo = actor.CustomerNo
		}
		if fields.CustomerNo != actor.CustomerNo {
			return authorizationEvidence{}, bizerr.NewCode(CodePermissionDenied)
		}
		return authorizationEvidence{}, nil
	case principal.AudienceStaff:
		if !idccap.IsAdminPermission(fields.RequiredPermission) {
			return authorizationEvidence{}, bizerr.NewCode(CodePermissionDenied)
		}
		evidence, err := s.resolveAuthorization(ctx, actor, fields.RequiredPermission)
		if err != nil {
			return authorizationEvidence{}, err
		}
		if !isSupportedStaffDataScope(actor, evidence) ||
			(fields.Result != ResultDenied && !evidence.granted()) {
			return authorizationEvidence{}, bizerr.NewCode(CodePermissionDenied)
		}
		return evidence, nil
	default:
		return authorizationEvidence{}, bizerr.NewCode(CodeIdentityDenied)
	}
}

// requireReadAuthority permits only staff with exact read authority and an
// all/tenant scope representable by the current audit storage model.
func (s *serviceImpl) requireReadAuthority(ctx context.Context, actor TrustedActor) error {
	if actor.Type != ActorUser || actor.AudienceCode != principal.AudienceStaff || actor.Delegated {
		return bizerr.NewCode(CodePermissionDenied)
	}
	if actor.DataScopeUnsupported {
		return bizerr.NewCode(CodePermissionDenied)
	}
	evidence, err := s.resolveAuthorization(ctx, actor, permissionAuditRead)
	if err != nil {
		return err
	}
	if !evidence.granted() || !isBroadStaffDataScope(actor, evidence) {
		return bizerr.NewCode(CodePermissionDenied)
	}
	return nil
}

// isSupportedStaffDataScope rejects unknown persisted role scope values.
func isSupportedStaffDataScope(actor TrustedActor, evidence authorizationEvidence) bool {
	if evidence.PlatformAdmin {
		return true
	}
	switch actor.DataScope {
	case hostDataScopeAll, hostDataScopeTenant, hostDataScopeDepartment, hostDataScopeSelf:
		return true
	default:
		return false
	}
}

// isBroadStaffDataScope rejects department/self audit searches because the
// append-only audit row has no safe organization ownership projection yet.
func isBroadStaffDataScope(actor TrustedActor, evidence authorizationEvidence) bool {
	if evidence.PlatformAdmin {
		return true
	}
	return actor.DataScope == hostDataScopeAll || actor.DataScope == hostDataScopeTenant
}
