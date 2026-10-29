// This file prepares only the documented ZJMF login before guest dispatch.
// Resource reads and writes remain behind the host's unchanged execution guard.
package composition

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"net/http"
	"time"
)

const authenticationPreparationTimeout = 30 * time.Second

// prepareAuthentication uses the existing shared owner session. The frozen
// ZJMF contract specifies POST login; other connectors retain their own flow.
// Explicit probes use a fresh login, never a cached session. No resource action runs here.
func (h *confidentialHTTPHandler) prepareAuthentication(ctx context.Context) error {
	if h.configuration.PluginID != "idc-zjmfcloud" || h.providerMethod == resourcecap.DynamicProviderMethodDiscoverCapabilities {
		return nil
	}
	settings, err := h.authenticationSettings()
	if err != nil {
		return bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	if settings.Strategy != "login_session" {
		return nil
	}
	authCtx, cancel := context.WithTimeout(ctx, authenticationPreparationTimeout)
	defer cancel()
	request := resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost, Path: settings.LoginPath}
	if h.forceFreshAuthentication {
		token, loginErr := h.performLogin(authCtx, request)
		clear(token)
		if loginErr == nil {
			h.preparedProbeAuthentication = true
		}
		return loginErr
	}
	return h.authenticate(authCtx, request)
}
