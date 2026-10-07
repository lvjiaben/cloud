package resourcecap

import (
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capregistry"
)

const (
	CapabilityServerCapabilitiesList     = "host:idc:resource.server-capabilities.list"
	CapabilityCurrentCustomerServersList = "host:idc:resource.current-customer-servers.list"
	CapabilityCurrentCustomerServerGet   = "host:idc:resource.current-customer-server.get"
)

func NewReadDescriptor(invoker capregistry.Invoker) (capregistry.Descriptor, error) {
	if invoker == nil {
		return capregistry.Descriptor{}, gerror.New("resourcecap: read descriptor invoker is required")
	}
	return capregistry.Descriptor{
		OwnerPluginID: OwnerPluginID, Service: ServiceName, Version: string(ReadVersion),
		SourceContract: SourceContractPackage, DynamicContract: DynamicContractPackage, Invoker: invoker,
		Methods: []capregistry.MethodDescriptor{
			{
				Method: string(MethodServerCapabilitiesList), Capability: CapabilityServerCapabilitiesList,
				Risk: capregistry.RiskLevelRead, ResourceKind: capregistry.ResourceKindNone,
				RequestPayload: "resourcecap.ServerCapabilitiesRequest", ResponsePayload: "resourcecap.ServerCapabilitiesResult",
			},
			{
				Method: string(MethodCurrentCustomerServersList), Capability: CapabilityCurrentCustomerServersList,
				Risk: capregistry.RiskLevelRead, ResourceKind: capregistry.ResourceKindNone,
				RequestPayload: "resourcecap.CurrentCustomerServerListRequest", ResponsePayload: "resourcecap.CurrentCustomerServerPage",
			},
			{
				Method: string(MethodCurrentCustomerServerGet), Capability: CapabilityCurrentCustomerServerGet,
				Risk: capregistry.RiskLevelRead, ResourceKind: capregistry.ResourceKindNone,
				RequestPayload: "resourcecap.CurrentCustomerServerGetRequest", ResponsePayload: "resourcecap.CurrentCustomerServerResult",
			},
		},
	}, nil
}
