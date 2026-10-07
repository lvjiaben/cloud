// This file registers ticket-required customer server routes independently of
// direct account APIs. Customer proof always uses the original request context;
// neither the proof result nor an assistance identity is cached here.

package backend

import (
	"context"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-resource/backend/api/serverpage"
)

// bindCustomerServerPageRoutes keeps the ticket verifier on the dedicated group
// only. The owner proof returns zero for missing identities, failing host checks.
func bindCustomerServerPageRoutes(group pluginhost.RouteGroup, middlewares pluginhost.RouteMiddlewares, customers customerauthcap.Reader, controller serverpage.IServerpageV1) {
	group.Group("", func(pageGroup pluginhost.RouteGroup) {
		pageGroup.Middleware(
			middlewares.NeverDoneCtx(), middlewares.HandlerResponse(), middlewares.CORS(),
			middlewares.RequestBodyLimit(), middlewares.Ctx(), middlewares.Auth(),
			middlewares.Tenancy(), middlewares.PageBridge(),
		)
		pageGroup.BindCustomerPageBridge(func(ctx context.Context) (int, error) {
			subject, err := customers.CurrentCustomer(ctx)
			if err != nil || subject == nil {
				return 0, err
			}
			return subject.ActorUserID, nil
		}, controller)
	})
}
