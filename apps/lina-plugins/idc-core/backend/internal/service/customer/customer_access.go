// This file provides the customer route and object-ownership access guard.

package customer

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/net/ghttp"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type customerAccessContextKey struct{}

// AccessSubject is the immutable customer identity made available after the
// active-customer route guard succeeds. It contains no profile or credential
// data and cannot be supplied through an HTTP request payload.
type AccessSubject struct {
	UserID     int
	CustomerNo string
}

// AccessGuard is the single customer-audience and object-ownership boundary
// shared by route middleware and customer application services.
type AccessGuard struct {
	bizCtxSvc bizctxcap.Service
	principal principal.Reader
}

// NewAccessGuard creates a fail-closed customer access guard from host context
// and the persisted, mutually exclusive audience service.
func NewAccessGuard(
	bizCtxSvc bizctxcap.Service,
	principalSvc principal.Reader,
) (*AccessGuard, error) {
	if nilInterface(bizCtxSvc) || nilInterface(principalSvc) {
		return nil, gerror.New("customer access guard requires host bizctx and principal services")
	}
	return &AccessGuard{bizCtxSvc: bizCtxSvc, principal: principalSvc}, nil
}

// Middleware admits only a direct active customer identity and stores its
// immutable subject in the request context for downstream ownership checks.
// It must be installed after the host authentication and tenancy middleware;
// customer ownership does not reuse a backoffice menu permission as authority.
func (g *AccessGuard) Middleware(request *ghttp.Request) {
	if request == nil {
		return
	}
	audience, err := g.resolveDirect(request.Context())
	if err != nil {
		request.SetError(err)
		return
	}
	request.SetCtx(withAccessSubject(request.Context(), audience))
	request.Middleware.Next()
}

// Resolve returns the trusted active customer subject. A subject already
// injected by Middleware is reused, avoiding a second principal lookup in the
// controller and service path of the same request.
func (g *AccessGuard) Resolve(ctx context.Context) (*principal.Audience, error) {
	if subject, ok := AccessSubjectFromContext(ctx); ok {
		return &principal.Audience{
			UserID:     subject.UserID,
			Code:       principal.AudienceCustomer,
			Status:     principal.AudienceStatusActive,
			CustomerNo: subject.CustomerNo,
		}, nil
	}
	return g.resolveDirect(ctx)
}

// RequireOwned checks a target customer number before syntax validation or
// persistence access. Malformed, missing, and foreign identifiers deliberately
// collapse to the same inaccessible result.
func (g *AccessGuard) RequireOwned(
	ctx context.Context,
	targetCustomerNo string,
) (AccessSubject, error) {
	audience, err := g.Resolve(ctx)
	if err != nil {
		return AccessSubject{}, err
	}
	if targetCustomerNo != audience.CustomerNo || !validCustomerNo(targetCustomerNo) {
		return AccessSubject{}, bizerr.NewCode(CodeNotAccessible)
	}
	return AccessSubject{UserID: audience.UserID, CustomerNo: audience.CustomerNo}, nil
}

// AccessSubjectFromContext returns the customer subject installed by
// Middleware. The private typed key prevents request parameters and unrelated
// packages from fabricating the ownership projection.
func AccessSubjectFromContext(ctx context.Context) (AccessSubject, bool) {
	if ctx == nil {
		return AccessSubject{}, false
	}
	subject, ok := ctx.Value(customerAccessContextKey{}).(AccessSubject)
	if !ok || subject.UserID <= 0 || !validCustomerNo(subject.CustomerNo) {
		return AccessSubject{}, false
	}
	return subject, true
}

func withAccessSubject(ctx context.Context, audience *principal.Audience) context.Context {
	return context.WithValue(ctx, customerAccessContextKey{}, AccessSubject{
		UserID:     audience.UserID,
		CustomerNo: audience.CustomerNo,
	})
}

func (g *AccessGuard) resolveDirect(ctx context.Context) (*principal.Audience, error) {
	if g == nil || nilInterface(g.bizCtxSvc) || nilInterface(g.principal) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	current := g.bizCtxSvc.Current(ctx)
	directCustomer := current.ActingUserID == 0 && !current.IsImpersonation && !current.ActingAsTenant
	delegatedCustomer := current.IsImpersonation && current.ActingUserID > 0 &&
		current.ActingUserID != current.UserID && !current.ActingAsTenant
	if current.UserID <= 0 || (!directCustomer && !delegatedCustomer) ||
		current.IsSuperAdmin || current.PlatformBypass || idccap.HasAdminPermission(current.Permissions) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	audience, err := g.principal.Current(ctx)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	if audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive ||
		!validCustomerNo(audience.CustomerNo) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return audience, nil
}
