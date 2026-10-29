package customerlogin

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/extlogin/extidspi"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
)

type identityEngine struct{ directory customerlogincap.Directory }

// NewIdentityEngine exposes only the verified-contact provider owned by the
// notification plugin. It never provisions accounts or mints tokens itself.
func NewIdentityEngine(directory customerlogincap.Directory) (extidspi.Provider, error) {
	if directory == nil {
		return nil, gerror.New("customer identity engine directory is required")
	}
	return &identityEngine{directory: directory}, nil
}
func (e *identityEngine) Resolve(ctx context.Context, in extidspi.ResolveInput) (int, bool, error) {
	if in.Provider != customerlogincap.IdentityProviderID || e.directory == nil {
		return 0, false, nil
	}
	userID, err := e.directory.ResolveSubject(ctx, in.Subject)
	if err != nil {
		if bizerr.Is(err, CodeInvalid) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return userID, true, nil
}
func (*identityEngine) Provision(context.Context, extidspi.ProvisionInput) (int, error) {
	return 0, extidspi.ErrProviderUnavailable
}
func (*identityEngine) Bind(context.Context, extidspi.BindInput) error {
	return extidspi.ErrProviderUnavailable
}
func (*identityEngine) Unbind(context.Context, extidspi.UnbindInput) error {
	return extidspi.ErrProviderUnavailable
}
func (*identityEngine) List(context.Context, int) ([]extidspi.BoundIdentity, error) {
	return []extidspi.BoundIdentity{}, nil
}
