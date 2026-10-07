package notificationpolicy

import (
	"context"
	"strconv"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-core/pkg/statusflag"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type staffDirectory struct {
	bizCtx           bizctxcap.Service
	users            usercap.Service
	currentPrincipal principal.Reader
	principals       principal.StaffDirectoryReader
}

var _ notificationcap.StaffDirectory = (*staffDirectory)(nil)

// NewStaffDirectory constructs the source-only, write-free staff recipient
// validator. Host user lifecycle and core principal audience remain separate
// authorities and both must agree before a target is active.
func NewStaffDirectory(
	bizCtx bizctxcap.Service,
	users usercap.Service,
	currentPrincipal principal.Reader,
	principals principal.StaffDirectoryReader,
) (notificationcap.StaffDirectory, error) {
	if bizCtx == nil || users == nil || currentPrincipal == nil || principals == nil {
		return nil, gerror.New("notification staff directory requires authoritative identity readers")
	}
	return &staffDirectory{
		bizCtx: bizCtx, users: users,
		currentPrincipal: currentPrincipal, principals: principals,
	}, nil
}

func (r *staffDirectory) ResolveActiveBatch(
	ctx context.Context,
	requests []notificationcap.StaffDirectoryRequest,
) ([]notificationcap.StaffDirectoryProjection, error) {
	if err := r.authorizeCaller(ctx); err != nil {
		return nil, err
	}
	userIDs, capabilityIDs, err := normalizeStaffDirectoryRequests(requests)
	if err != nil {
		return nil, err
	}
	result := make([]notificationcap.StaffDirectoryProjection, len(requests))
	if len(requests) == 0 {
		return result, nil
	}
	hostUsers, err := r.users.BatchGet(ctx, capabilityIDs)
	if err != nil {
		return nil, gerror.Wrap(err, "resolve host staff batch")
	}
	if hostUsers == nil {
		return nil, gerror.New("resolve host staff batch returned no result")
	}
	principalStates, err := r.principals.ResolveBatch(ctx, userIDs)
	if err != nil {
		return nil, gerror.Wrap(err, "resolve principal staff batch")
	}
	for index, request := range requests {
		state, exists := principalStates[request.HostUserID]
		if !exists || state.UserID != request.HostUserID || state.Code != principal.AudienceStaff ||
			state.HasCustomerBinding || state.Version <= 0 {
			continue
		}
		result[index].CurrentVersion = state.Version
		hostID := usercap.UserID(strconv.Itoa(request.HostUserID))
		hostUser := hostUsers.Items[hostID]
		result[index].Active = hostUser != nil && hostUser.ID == hostID &&
			string(hostUser.TenantID) == strconv.Itoa(idccap.DefaultTenantID) &&
			hostUser.Status == statusflag.EnabledValue &&
			state.Status == principal.AudienceStatusActive &&
			state.Version == request.ExpectedVersion
	}
	return result, nil
}

func (r *staffDirectory) authorizeCaller(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil {
		return bizerr.NewCode(codeCurrentStaffNotAccessible)
	}
	current := r.bizCtx.Current(ctx)
	if current.TenantID != idccap.DefaultTenantID || current.ActingUserID != 0 ||
		current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported {
		return bizerr.NewCode(codeCurrentStaffNotAccessible)
	}
	// A host-owned background job has no represented user and carries the
	// explicit platform bypass bit. This is distinct from an unknown interactive
	// data scope and is required for queued staff delivery revalidation.
	if current.UserID == 0 {
		if current.PlatformBypass && current.DataScope == 0 {
			return nil
		}
		return bizerr.NewCode(codeCurrentStaffNotAccessible)
	}
	if !supportedCurrentStaffDataScope(current) {
		return bizerr.NewCode(codeCurrentStaffNotAccessible)
	}
	audience, err := r.currentPrincipal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive ||
		audience.Version <= 0 {
		return bizerr.NewCode(codeCurrentStaffNotAccessible)
	}
	return nil
}

func normalizeStaffDirectoryRequests(
	requests []notificationcap.StaffDirectoryRequest,
) ([]int, []usercap.UserID, error) {
	if len(requests) > notificationcap.MaximumStaffDirectoryBatch {
		return nil, nil, gerror.New("notification staff directory batch is invalid")
	}
	userIDs := make([]int, 0, len(requests))
	capabilityIDs := make([]usercap.UserID, 0, len(requests))
	seen := make(map[int]struct{}, len(requests))
	for _, request := range requests {
		if request.HostUserID <= 0 || request.ExpectedVersion <= 0 {
			return nil, nil, gerror.New("notification staff directory request is invalid")
		}
		if _, exists := seen[request.HostUserID]; exists {
			return nil, nil, gerror.New("notification staff directory request is duplicated")
		}
		seen[request.HostUserID] = struct{}{}
		userIDs = append(userIDs, request.HostUserID)
		capabilityIDs = append(capabilityIDs, usercap.UserID(strconv.Itoa(request.HostUserID)))
	}
	return userIDs, capabilityIDs, nil
}
