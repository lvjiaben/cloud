package composition

import (
	"context"
	"errors"

	"lina-plugin-idc-core/backend/cap/secretcap"
	secretspi "lina-plugin-idc-core/backend/cap/secretcap/spi"
)

const paymentCredentialScope = "payment.channel-credential"

func LoadSecretSource(ctx context.Context) (secretcap.Source, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, errors.New("payment secret context is invalid")
	}
	snapshot, err := secretspi.Freeze()
	if err != nil {
		return nil, err
	}
	factory := snapshot.Factory()
	if factory == nil {
		return nil, errors.New("payment secret source is unavailable")
	}
	return factory(ctx)
}
func BindPaymentCredentials(source secretcap.Source) (secretcap.Accessor, secretcap.Manager, error) {
	if source == nil {
		return nil, nil, errors.New("payment secret source is required")
	}
	scope := secretcap.Scope{TenantID: 0, OwnerPluginID: "idc-payment", CallerScope: paymentCredentialScope}
	accessor, err := source.Accessor(scope)
	if err != nil {
		return nil, nil, err
	}
	manager, err := source.Manager(scope)
	if err != nil {
		return nil, nil, err
	}
	if accessor == nil || manager == nil {
		return nil, nil, errors.New("payment credential boundary is unavailable")
	}
	return accessor, manager, nil
}
