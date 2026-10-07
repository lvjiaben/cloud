// This file maps the typed idccap contract to generic owner-aware descriptor metadata.

package idccap

import (
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capregistry"
)

const (
	// CapabilityMerchantContextRead is the method-level authorization identity
	// used by an owner-aware dynamic bridge.
	CapabilityMerchantContextRead = "host:idc:merchant.context.current"
)

// NewDescriptor creates the generic owner-aware descriptor for an explicitly
// supplied invoker. It performs no registration and exposes no service lookup.
func NewDescriptor(invoker capregistry.Invoker) (capregistry.Descriptor, error) {
	if invoker == nil {
		return capregistry.Descriptor{}, gerror.New("idccap: descriptor invoker is required")
	}
	return capregistry.Descriptor{
		OwnerPluginID:   OwnerPluginID,
		Service:         ServiceName,
		Version:         string(ContractVersion),
		SourceContract:  SourceContractPackage,
		DynamicContract: DynamicContractPackage,
		Invoker:         invoker,
		Methods: []capregistry.MethodDescriptor{
			{
				Method:          string(MethodMerchantContextCurrent),
				Capability:      CapabilityMerchantContextRead,
				Risk:            capregistry.RiskLevelRead,
				ResourceKind:    capregistry.ResourceKindNone,
				ResponsePayload: "idccap.MerchantContext",
			},
		},
	}, nil
}
