// Package source constructs a caller-bound staff-access source client.
package source

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-staff-access/backend/cap/staffaccesscap"
	"lina-plugin-staff-access/backend/internal/service/sourceauth"
	"lina-plugin-staff-access/backend/internal/service/staffaccess"
)

func New(ctx context.Context, services capability.Services, callerPluginID string) (staffaccesscap.Service, error) {
	caller, err := sourceauth.Authorize(ctx, services, callerPluginID)
	if err != nil {
		return nil, err
	}
	service, err := staffaccess.New(g.DB(), services.BizCtx(), services.Users(), services.Auth().Authz(), services.Sessions())
	if err != nil {
		return nil, err
	}
	return staffaccess.Bind(service, caller)
}
