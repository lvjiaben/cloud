// This file composes the source-only customer contact directory and the host's
// published verified-identity engine. Host credentials stay host-owned.
package backend

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/plugin/capability"
	"lina-core/pkg/plugin/capability/authcap/extlogin/extidspi"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-core/backend/internal/runtimeconfig"
	"lina-plugin-idc-core/backend/internal/service/customerlogin"
)

func buildCustomerLoginDirectory(bizctx bizctxcap.Service, users usercap.Service) (customerlogin.Service, error) {
	indexKey, sealKey, err := runtimeconfig.LoadCustomerLoginKeys()
	if err != nil {
		return nil, err
	}
	defer clear(indexKey)
	defer clear(sealKey)
	block, err := aes.NewCipher(sealKey)
	if err != nil {
		return nil, err
	}
	seal, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return customerlogin.New(g.DB(), bizctx, users, indexKey, seal)
}
func newSourceCustomerLoginDirectory(ctx context.Context, services capability.Services, callerPluginID string) (customerlogincap.Directory, error) {
	if ctx == nil || ctx.Err() != nil || services == nil || callerPluginID != customerlogincap.AllowedCallerPluginID {
		return nil, gerror.New("customer login directory source caller is not allowed")
	}
	return buildCustomerLoginDirectory(services.BizCtx(), services.Users())
}
func newSourceCustomerIdentityEngine(ctx context.Context, env extidspi.ProviderEnv) (extidspi.Provider, error) {
	if ctx == nil || ctx.Err() != nil || env.PluginID != pluginID {
		return nil, gerror.New("customer identity engine owner is invalid")
	}
	directory, err := buildCustomerLoginDirectory(env.BizCtx, env.Users)
	if err != nil {
		return nil, err
	}
	return customerlogin.NewIdentityEngine(directory)
}
