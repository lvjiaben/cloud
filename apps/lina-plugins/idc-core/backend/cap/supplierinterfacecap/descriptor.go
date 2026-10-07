package supplierinterfacecap

import (
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capregistry"
)

const (
	CapabilityConfigurationGet = "host:idc:supplier-interface.configuration.get"
	CapabilityFilesList        = "host:idc:supplier-interface.configuration.files.list"
	CapabilityFileRead         = "host:idc:supplier-interface.configuration.file.read"
	CapabilityOutboundHTTP     = "host:idc:supplier-interface.outbound.http.execute"
	CapabilityOutboundSession  = "host:idc:supplier-interface.outbound.session.execute"
)

func NewDescriptor(invoker capregistry.Invoker) (capregistry.Descriptor, error) {
	if invoker == nil {
		return capregistry.Descriptor{}, gerror.New("supplierinterfacecap: descriptor invoker is required")
	}
	return capregistry.Descriptor{
		OwnerPluginID: OwnerPluginID, Service: ServiceName, Version: ContractVersion,
		SourceContract: SourceContractPackage, DynamicContract: DynamicContractPackage, Invoker: invoker,
		Methods: []capregistry.MethodDescriptor{{
			Method: MethodConfigurationGet, Capability: CapabilityConfigurationGet,
			Risk: capregistry.RiskLevelRead, ResourceKind: capregistry.ResourceKindRef,
			ResponsePayload: "supplierinterfacecap.ConfigurationView",
		}, {
			Method: MethodFilesList, Capability: CapabilityFilesList,
			Risk: capregistry.RiskLevelRead, ResourceKind: capregistry.ResourceKindRef,
			RequestPayload:  "supplierinterfacecap.ConfigurationFileListRequest",
			ResponsePayload: "supplierinterfacecap.ConfigurationFileList",
		}, {
			Method: MethodFileRead, Capability: CapabilityFileRead,
			Risk: capregistry.RiskLevelRead, ResourceKind: capregistry.ResourceKindRef,
			RequestPayload:  "supplierinterfacecap.ConfigurationFileReadRequest",
			ResponsePayload: "supplierinterfacecap.ConfigurationFileChunk",
		}, {
			Method: MethodOutboundHTTP, Capability: CapabilityOutboundHTTP,
			Risk: capregistry.RiskLevelExecute, ResourceKind: capregistry.ResourceKindRef,
			RequestPayload:  "supplierinterfacecap.OutboundHTTPRequest",
			ResponsePayload: "supplierinterfacecap.OutboundHTTPResponse",
		}, {
			Method: MethodOutboundSession, Capability: CapabilityOutboundSession,
			Risk: capregistry.RiskLevelExecute, ResourceKind: capregistry.ResourceKindRef,
			RequestPayload:  "supplierinterfacecap.OutboundSessionRequest",
			ResponsePayload: "supplierinterfacecap.OutboundSessionResponse",
		}},
	}, nil
}
