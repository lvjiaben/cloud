// Package managementruntime constructs the core-owned staff authorization and business-audit seam.
package managementruntime

import (
	"context"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/internal/runtimeconfig"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

// sensitiveAccessProofTTL is the reviewed upper bound for exact-target proofs.
const sensitiveAccessProofTTL = 5 * time.Minute

type authorizer struct {
	services  capability.Services
	principal principal.Service
}

var _ managementcap.Authorizer = (*authorizer)(nil)

type sensitiveAccessAuthenticator struct {
	service reauth.Service
}

var _ managementcap.SensitiveAccessAuthenticator = (*sensitiveAccessAuthenticator)(nil)

// New constructs one requester-scoped administration runtime from governed host capabilities.
func New(ctx context.Context, services capability.Services) (managementcap.Runtime, error) {
	if ctx == nil || ctx.Err() != nil || services == nil || services.BizCtx() == nil ||
		services.Sessions() == nil || services.Auth() == nil || services.Auth().Authz() == nil {
		return managementcap.Runtime{}, gerror.New("IDC management runtime host capabilities are unavailable")
	}
	principalReader, err := principal.NewReader(services.BizCtx())
	if err != nil {
		return managementcap.Runtime{}, err
	}
	auditConfig, err := runtimeconfig.LoadAuditConfig()
	if err != nil {
		return managementcap.Runtime{}, err
	}
	auditService, err := audit.New(
		services.BizCtx(), principalReader, services.Auth().Authz(), auditConfig,
	)
	if err != nil {
		return managementcap.Runtime{}, err
	}
	auditWriter, err := audit.NewBusinessWriterAdapter(auditService)
	if err != nil {
		return managementcap.Runtime{}, err
	}
	principalService, err := principal.New(
		services.BizCtx(), services.Auth().Authz(), auditWriter,
	)
	if err != nil {
		return managementcap.Runtime{}, err
	}
	recentAuthentication, err := newSensitiveAccessAuthenticator(services)
	if err != nil {
		return managementcap.Runtime{}, err
	}
	runtime := managementcap.Runtime{
		Authorizer: &authorizer{services: services, principal: principalService},
		Audit:      auditWriter, SensitiveAccess: recentAuthentication,
		Renewals: &staffRenewalManager{principal: principalService},
	}
	if err = managementcap.ValidateRuntime(runtime); err != nil {
		return managementcap.Runtime{}, err
	}
	return runtime, nil
}

func newSensitiveAccessAuthenticator(
	services capability.Services,
) (managementcap.SensitiveAccessAuthenticator, error) {
	keyProvider, err := runtimeconfig.LoadRecentAuthSigningKeyProvider()
	if err != nil {
		return nil, err
	}
	signer, err := reauth.NewHMACSigner(keyProvider)
	if err != nil {
		return nil, err
	}
	service, err := reauth.New(
		reauth.Config{ProofTTL: sensitiveAccessProofTTL}, services.BizCtx(), services.Sessions(),
		reauth.NewDAOReplayStore(), signer,
	)
	if err != nil {
		return nil, err
	}
	return &sensitiveAccessAuthenticator{service: service}, nil
}

func (a *sensitiveAccessAuthenticator) IssueSensitiveAccess(
	ctx context.Context,
	binding managementcap.SensitiveAccessBinding,
) (*managementcap.SensitiveAccessContext, error) {
	if a == nil || a.service == nil || managementcap.ValidateSensitiveAccessBinding(binding) != nil {
		return nil, gerror.New("IDC sensitive access authentication is unavailable")
	}
	issued, err := a.service.Issue(ctx, reauth.Binding{
		Purpose: reauth.PurposeSensitiveMaterial, TargetDigest: binding.TargetDigest,
	})
	if err != nil {
		return nil, err
	}
	return &managementcap.SensitiveAccessContext{
		Token: issued.Token, ExpiresAt: issued.ExpiresAt,
	}, nil
}

func (a *sensitiveAccessAuthenticator) ConsumeSensitiveAccess(
	ctx context.Context,
	token string,
	binding managementcap.SensitiveAccessBinding,
) (*managementcap.SensitiveAccessProof, error) {
	if a == nil || a.service == nil || managementcap.ValidateSensitiveAccessBinding(binding) != nil {
		return nil, gerror.New("IDC sensitive access authentication is unavailable")
	}
	proof, err := a.service.Consume(ctx, token, reauth.Binding{
		Purpose: reauth.PurposeSensitiveMaterial, TargetDigest: binding.TargetDigest,
	})
	if err != nil {
		return nil, err
	}
	return &managementcap.SensitiveAccessProof{
		ActorUserID: proof.UserID, ExpiresAt: proof.ExpiresAt,
	}, nil
}

func (a *authorizer) Authorize(
	ctx context.Context,
	request managementcap.AuthorizationRequest,
) (int, error) {
	if a == nil || ctx == nil || ctx.Err() != nil || a.services == nil || a.principal == nil ||
		managementcap.ValidateAuthorizationRequest(request) != nil {
		return 0, inaccessible()
	}
	current := a.services.BizCtx().Current(ctx)
	if !validCurrentContext(current) ||
		!a.hasPermission(ctx, current.UserID, request.Permission) ||
		!a.validStaffAudience(ctx, current.UserID) {
		return 0, inaccessible()
	}
	return current.UserID, nil
}

func validCurrentContext(current bizctxcap.CurrentContext) bool {
	return current.UserID > 0 && current.TenantID == idccap.DefaultTenantID &&
		current.ActingUserID == 0 && !current.IsImpersonation && !current.ActingAsTenant &&
		!current.DataScopeUnsupported && (current.DataScope == 1 || current.DataScope == 2)
}

func (a *authorizer) validStaffAudience(ctx context.Context, userID int) bool {
	audience, err := a.principal.EnsureCurrentStaff(ctx)
	return err == nil && audience != nil && audience.UserID == userID &&
		audience.Code == principal.AudienceStaff && audience.Status == principal.AudienceStatusActive
}

func (a *authorizer) hasPermission(ctx context.Context, userID int, permission string) bool {
	authorization := a.services.Auth().Authz()
	keys := []authz.PermissionKey{
		authz.PermissionKey(permission), authz.PermissionKey(idccap.HostPermissionWildcard),
	}
	grants, err := authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return false
	}
	platformAdmin, err := authorization.IsPlatformAdmin(
		ctx, authz.UserID(strconv.Itoa(userID)),
	)
	return err == nil && (grants[keys[0]] || grants[keys[1]] || platformAdmin)
}

func inaccessible() error {
	return gerror.New("IDC management operation is not accessible")
}
