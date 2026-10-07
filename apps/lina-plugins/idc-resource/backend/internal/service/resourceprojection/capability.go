package resourceprojection

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capregistry"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type capabilityDirectory interface {
	ListCapabilities(context.Context, []serveroperationresourcecap.CapabilityRoute) ([]serveroperationresourcecap.CapabilityProjection, error)
}

type Service struct{ directory capabilityDirectory }

var _ resourcecap.ServerCapabilityService = (*Service)(nil)

func New(directory capabilityDirectory) (*Service, error) {
	if directory == nil {
		return nil, gerror.New("resource read projection requires a capability directory")
	}
	return &Service{directory: directory}, nil
}

func (s *Service) ListServerCapabilities(ctx context.Context, request resourcecap.ServerCapabilitiesRequest) (*resourcecap.ServerCapabilitiesResult, error) {
	if s == nil || s.directory == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("resource read projection is unavailable")
	}
	if err := resourcecap.ValidateServerCapabilitiesRequest(request); err != nil {
		return nil, err
	}
	routes := make([]serveroperationresourcecap.CapabilityRoute, len(request.Routes))
	for index, route := range request.Routes {
		routes[index] = serveroperationresourcecap.CapabilityRoute{ServiceNo: route.ServiceNo, ChannelNo: route.ChannelNo, PoolNo: route.PoolNo}
	}
	projections, err := s.directory.ListCapabilities(ctx, routes)
	if err != nil {
		return nil, err
	}
	result := &resourcecap.ServerCapabilitiesResult{Items: make([]resourcecap.ServerCapabilityProjection, len(projections))}
	for index, projection := range projections {
		item := resourcecap.ServerCapabilityProjection{ServiceNo: projection.ServiceNo, ObservedAt: projection.ObservedAt}
		seen := make(map[resourcecap.CapabilityCode]struct{}, len(projection.Actions))
		for _, action := range projection.Actions {
			capability, ok := capabilityForAction(action)
			if !ok {
				return nil, gerror.New("resource read projection contains an unsupported capability")
			}
			if _, exists := seen[capability]; exists {
				continue
			}
			seen[capability] = struct{}{}
			item.Capabilities = append(item.Capabilities, capability)
		}
		sort.Slice(item.Capabilities, func(i, j int) bool { return item.Capabilities[i] < item.Capabilities[j] })
		result.Items[index] = item
	}
	if err = resourcecap.ValidateServerCapabilitiesResult(*result, request); err != nil {
		return nil, err
	}
	return result, nil
}

type CapabilityInvoker struct {
	mutex        sync.RWMutex
	capabilities resourcecap.ServerCapabilityService
	servers      resourcecap.CurrentCustomerServerService
}

var _ capregistry.Invoker = (*CapabilityInvoker)(nil)

func NewCapabilityInvoker() *CapabilityInvoker { return &CapabilityInvoker{} }

func (i *CapabilityInvoker) Bind(capabilities resourcecap.ServerCapabilityService, servers resourcecap.CurrentCustomerServerService) error {
	if i == nil || capabilities == nil || servers == nil {
		return gerror.New("resource capability invoker requires read services")
	}
	i.mutex.Lock()
	defer i.mutex.Unlock()
	if i.capabilities != nil || i.servers != nil {
		return gerror.New("resource capability invoker is already bound")
	}
	i.capabilities = capabilities
	i.servers = servers
	return nil
}

func (i *CapabilityInvoker) Invoke(ctx context.Context, invocation capregistry.Invocation) (*capregistry.InvocationResult, error) {
	if i == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("resource capability invoker is unavailable")
	}
	i.mutex.RLock()
	capabilities := i.capabilities
	servers := i.servers
	i.mutex.RUnlock()
	caller := strings.TrimSpace(invocation.CallerPluginID)
	if capabilities == nil || servers == nil || caller == "" || caller != invocation.CallerPluginID ||
		invocation.OwnerPluginID != resourcecap.OwnerPluginID || invocation.Service != resourcecap.ServiceName ||
		invocation.Version != string(resourcecap.ReadVersion) || !supportedReadMethod(resourcecap.Method(invocation.Method)) ||
		invocation.ResourceRef != "" || invocation.Table != "" || len(invocation.Payload) == 0 || len(invocation.Payload) > resourcecap.MaximumReadPayloadBytes {
		return nil, gerror.New("resource capability invocation is not authorized")
	}
	var result any
	var err error
	switch resourcecap.Method(invocation.Method) {
	case resourcecap.MethodServerCapabilitiesList:
		result, err = invokeServerCapabilities(ctx, capabilities, invocation.Payload)
	case resourcecap.MethodCurrentCustomerServersList:
		result, err = invokeCurrentCustomerServers(ctx, servers, invocation.Payload)
	case resourcecap.MethodCurrentCustomerServerGet:
		result, err = invokeCurrentCustomerServer(ctx, servers, invocation.Payload)
	default:
		return nil, gerror.New("resource capability invocation is not authorized")
	}
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(result)
	if err != nil || len(payload) == 0 || len(payload) > resourcecap.MaximumReadPayloadBytes {
		return nil, gerror.New("resource capability result is outside its size bound")
	}
	return &capregistry.InvocationResult{Payload: payload}, nil
}

func supportedReadMethod(method resourcecap.Method) bool {
	return method == resourcecap.MethodServerCapabilitiesList ||
		method == resourcecap.MethodCurrentCustomerServersList ||
		method == resourcecap.MethodCurrentCustomerServerGet
}

func invokeServerCapabilities(ctx context.Context, service resourcecap.ServerCapabilityService, payload []byte) (any, error) {
	var request resourcecap.ServerCapabilitiesRequest
	if json.Unmarshal(payload, &request) != nil || resourcecap.ValidateServerCapabilitiesRequest(request) != nil {
		return nil, gerror.New("resource capability invocation payload is invalid")
	}
	result, err := service.ListServerCapabilities(ctx, request)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func invokeCurrentCustomerServers(ctx context.Context, service resourcecap.CurrentCustomerServerService, payload []byte) (any, error) {
	var request resourcecap.CurrentCustomerServerListRequest
	if json.Unmarshal(payload, &request) != nil || resourcecap.ValidateCurrentCustomerServerListRequest(request) != nil {
		return nil, gerror.New("resource capability invocation payload is invalid")
	}
	result, err := service.ListCurrentCustomerServers(ctx, request)
	if err != nil {
		return nil, err
	}
	if err = resourcecap.ValidateCurrentCustomerServerPage(result, request); err != nil {
		return nil, err
	}
	return result, nil
}

func invokeCurrentCustomerServer(ctx context.Context, service resourcecap.CurrentCustomerServerService, payload []byte) (any, error) {
	var request resourcecap.CurrentCustomerServerGetRequest
	if json.Unmarshal(payload, &request) != nil || resourcecap.ValidateCurrentCustomerServerGetRequest(request) != nil {
		return nil, gerror.New("resource capability invocation payload is invalid")
	}
	result, err := service.GetCurrentCustomerServer(ctx, request)
	if err != nil {
		return nil, err
	}
	if err = resourcecap.ValidateCurrentCustomerServerResult(result, request); err != nil {
		return nil, err
	}
	return result, nil
}

func capabilityForAction(action serveroperationresourcecap.Action) (resourcecap.CapabilityCode, bool) {
	switch action {
	case serveroperationresourcecap.ActionStart:
		return resourcecap.CapabilityStart, true
	case serveroperationresourcecap.ActionStop:
		return resourcecap.CapabilityStop, true
	case serveroperationresourcecap.ActionReboot:
		return resourcecap.CapabilityReboot, true
	case serveroperationresourcecap.ActionResetPassword:
		return resourcecap.CapabilityResetPassword, true
	case serveroperationresourcecap.ActionConsole:
		return resourcecap.CapabilityConsole, true
	case serveroperationresourcecap.ActionReinstall:
		return resourcecap.CapabilityReinstall, true
	case serveroperationresourcecap.ActionResize:
		return resourcecap.CapabilityResize, true
	case serveroperationresourcecap.ActionChangeIP:
		return resourcecap.CapabilityChangeIP, true
	case serveroperationresourcecap.ActionSnapshotCreate,
		serveroperationresourcecap.ActionSnapshotDelete,
		serveroperationresourcecap.ActionSnapshotRestore:
		return resourcecap.CapabilityManageSnapshot, true
	case serveroperationresourcecap.ActionBackupCreate,
		serveroperationresourcecap.ActionBackupDelete,
		serveroperationresourcecap.ActionBackupRestore:
		return resourcecap.CapabilityBackup, true
	case serveroperationresourcecap.ActionSecurityGroupAttach,
		serveroperationresourcecap.ActionSecurityGroupDetach:
		return resourcecap.CapabilitySecurityGroup, true
	case serveroperationresourcecap.ActionNATCreate, serveroperationresourcecap.ActionNATDelete:
		return resourcecap.CapabilityNAT, true
	case serveroperationresourcecap.ActionVirtualMediaMount,
		serveroperationresourcecap.ActionVirtualMediaUnmount:
		return resourcecap.CapabilityVirtualMedia, true
	case serveroperationresourcecap.ActionTrafficAdd:
		return resourcecap.CapabilityTrafficPackage, true
	default:
		return "", false
	}
}
