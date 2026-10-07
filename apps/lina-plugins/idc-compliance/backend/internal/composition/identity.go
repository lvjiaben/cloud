// This file binds the core-owned source-only identity projection capability.
package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/identitycap"
	identityspi "lina-plugin-idc-core/backend/cap/identitycap/spi"
)

// LoadIdentityRuntime freezes the owner declaration and constructs the
// compliance-only in-process identity and audit boundaries.
func LoadIdentityRuntime(
	ctx context.Context,
	services capability.Services,
) (identitycap.Runtime, error) {
	if ctx == nil || ctx.Err() != nil || services == nil {
		return identitycap.Runtime{}, gerror.New("compliance identity runtime construction context is invalid")
	}
	snapshot, err := identityspi.Freeze()
	if err != nil {
		return identitycap.Runtime{}, gerror.Wrap(err, "freeze identity runtime source declaration")
	}
	factory := snapshot.Factory()
	if factory == nil {
		return identitycap.Runtime{}, gerror.New("identity runtime source factory is unavailable")
	}
	runtime, err := factory(ctx, services, identitycap.AllowedCallerPluginID)
	if err != nil {
		return identitycap.Runtime{}, gerror.Wrap(err, "construct compliance identity runtime source")
	}
	if err = identitycap.ValidateRuntime(runtime); err != nil {
		return identitycap.Runtime{}, gerror.Wrap(err, "validate compliance identity runtime source")
	}
	return runtime, nil
}
