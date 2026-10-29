// This file composes the core-owned renewal queue and one transactional outbox sink.
// It runs only at plugin startup and uses the supplied host identity services.

package backend

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/internal/controller/renewalstaff"
	"lina-plugin-idc-core/backend/internal/service/notificationpolicy"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/renewalnotice"
	inboxsource "lina-plugin-shared-services/backend/cap/inboxcap/source"
)

func newRenewalStaffController(ctx context.Context, database gdb.DB, services capability.Services, audience principal.Reader) (*renewalstaff.ControllerV1, error) {
	if services == nil || services.BizCtx() == nil || services.Auth() == nil || services.Auth().Authz() == nil {
		return nil, gerror.New("renewal notifications require startup host services")
	}
	inbox, err := inboxsource.NewCurrentSync(ctx, services, pluginID)
	if err != nil {
		return nil, err
	}
	writer, err := outbox.NewEnqueuer(database, outbox.Config{})
	if err != nil {
		return nil, err
	}
	sink, err := notificationpolicy.NewEventSink(pluginID, writer)
	if err != nil {
		return nil, err
	}
	service, err := renewalnotice.New(database, services.BizCtx(), audience, services.Auth().Authz(), sink, inbox)
	if err != nil {
		return nil, err
	}
	return renewalstaff.NewV1(service)
}
