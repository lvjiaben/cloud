package notificationpolicy

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type currentStaffReader struct {
	bizCtx    bizctxcap.Service
	principal principal.Reader
}

var _ notificationcap.CurrentStaff = (*currentStaffReader)(nil)

// NewCurrentStaff constructs a source-only, write-free view of the current
// persisted staff audience. It deliberately accepts a principal.Reader so the
// read path cannot call EnsureCurrentStaff.
func NewCurrentStaff(bizCtx bizctxcap.Service, principalReader principal.Reader) (notificationcap.CurrentStaff, error) {
	if bizCtx == nil || principalReader == nil {
		return nil, gerror.New("notification current staff requires current identity")
	}
	return &currentStaffReader{bizCtx: bizCtx, principal: principalReader}, nil
}

func (r *currentStaffReader) Current(ctx context.Context) (notificationcap.CurrentStaffProjection, error) {
	if ctx == nil || ctx.Err() != nil {
		return notificationcap.CurrentStaffProjection{}, bizerr.NewCode(codeCurrentStaffNotAccessible)
	}
	current := r.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID ||
		current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant ||
		!supportedCurrentStaffDataScope(current) {
		return notificationcap.CurrentStaffProjection{}, bizerr.NewCode(codeCurrentStaffNotAccessible)
	}
	audience, err := r.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive ||
		audience.Version <= 0 {
		return notificationcap.CurrentStaffProjection{}, bizerr.NewCode(codeCurrentStaffNotAccessible)
	}
	return notificationcap.CurrentStaffProjection{HostUserID: audience.UserID, StaffVersion: audience.Version}, nil
}

func supportedCurrentStaffDataScope(current bizctxcap.CurrentContext) bool {
	if current.DataScopeUnsupported {
		return false
	}
	switch current.DataScope {
	case 1, 2, 3, 4:
		return true
	default:
		return false
	}
}
