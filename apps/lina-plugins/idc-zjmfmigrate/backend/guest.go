package backend

import (
	"lina-core/pkg/plugin/pluginbridge"
	"lina-plugin-idc-zjmfmigrate/backend/internal/controller/admin"
)

// NewGuestHandler composes the migration controller with host-governed data
// and request-context services. The WASM module never receives a database
// handle, login token, or source-system credential.
func NewGuestHandler() pluginbridge.GuestHandler {
	dispatcher := pluginbridge.MustNewGuestControllerRouteDispatcher(
		admin.New(),
	)
	return dispatcher.HandleRequest
}
