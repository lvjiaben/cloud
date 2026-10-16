// This file injects HTTP Basic credentials only inside the resource owner's
// confidential boundary. Managed adapters never receive AppID or AppSecret.
package composition

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// withBasicAuthentication borrows both credential views synchronously and
// removes the derived Authorization header before returning to the adapter.
func (h *confidentialHTTPHandler) withBasicAuthentication(ctx context.Context, request *resourcecap.ConfidentialHTTPRequest, settings channelAuthenticationSettings, action func() error) error {
	if len(settings.Fields) != 2 || settings.Fields["username"] == "" || settings.Fields["password"] == "" || h.environment.Secrets == nil {
		return gerror.New("resource channel Basic authentication policy is invalid")
	}
	for name := range request.Headers {
		if strings.EqualFold(name, "Authorization") {
			return gerror.New("resource channel authentication header conflicts")
		}
	}
	user, ok := confidentialBinding(h.configuration.Config.SecretBindings, settings.Fields["username"])
	if !ok {
		return gerror.New("resource channel application identity is unavailable")
	}
	password, ok := confidentialBinding(h.configuration.Config.SecretBindings, settings.Fields["password"])
	if !ok {
		return gerror.New("resource channel application secret is unavailable")
	}
	purpose, err := secretcap.NewPurpose("provider.channel-authentication")
	if err != nil {
		return err
	}
	userAccess, err := secretcap.NewAccessRequest(user, purpose)
	if err != nil {
		return err
	}
	passwordAccess, err := secretcap.NewAccessRequest(password, purpose)
	if err != nil {
		return err
	}
	return h.environment.Secrets.Access(ctx, userAccess, func(username []byte) error {
		if len(username) == 0 || strings.ContainsAny(string(username), ":\r\n") {
			return gerror.New("resource channel application identity is invalid")
		}
		return h.environment.Secrets.Access(ctx, passwordAccess, func(secret []byte) error {
			if len(secret) == 0 || strings.ContainsAny(string(secret), "\r\n") {
				return gerror.New("resource channel application secret is invalid")
			}
			combined := make([]byte, 0, len(username)+len(secret)+1)
			combined = append(combined, username...)
			combined = append(combined, ':')
			combined = append(combined, secret...)
			defer clear(combined)
			encoded := make([]byte, base64.StdEncoding.EncodedLen(len(combined)))
			base64.StdEncoding.Encode(encoded, combined)
			defer clear(encoded)
			if request.Headers == nil {
				request.Headers = make(map[string]string)
			}
			request.Headers["Authorization"] = "Basic " + string(encoded)
			defer delete(request.Headers, "Authorization")
			return action()
		})
	})
}
