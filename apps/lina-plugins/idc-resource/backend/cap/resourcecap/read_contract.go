package resourcecap

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	OwnerPluginID           = "idc-resource"
	ServiceName             = "resource-read"
	SourceContractPackage   = "lina-plugin-idc-resource/backend/cap/resourcecap"
	DynamicContractPackage  = SourceContractPackage + "/bridge"
	MaximumReadRoutes       = 64
	MaximumReadPayloadBytes = 64 * 1024
)

type Version string
type Method string

const (
	ReadVersion                      Version = "v1"
	MethodServerCapabilitiesList     Method  = "server-capabilities.list"
	MethodCurrentCustomerServersList Method  = "current-customer-servers.list"
	MethodCurrentCustomerServerGet   Method  = "current-customer-server.get"
)

type Dependency struct {
	OwnerPluginID string   `json:"ownerPluginId"`
	Service       string   `json:"service"`
	Version       Version  `json:"version"`
	Methods       []Method `json:"methods"`
}

type ServerCapabilityRoute struct {
	ServiceNo string `json:"serviceNo"`
	ChannelNo string `json:"channelNo"`
	PoolNo    string `json:"poolNo"`
}

type ServerCapabilitiesRequest struct {
	Routes []ServerCapabilityRoute `json:"routes"`
}

type ServerCapabilityProjection struct {
	ServiceNo    string           `json:"serviceNo"`
	Capabilities []CapabilityCode `json:"capabilities"`
	ObservedAt   time.Time        `json:"observedAt"`
}

type ServerCapabilitiesResult struct {
	Items []ServerCapabilityProjection `json:"items"`
}

// ServerCapabilityService reads one bounded capability projection batch.
type ServerCapabilityService interface {
	// ListServerCapabilities returns one projection for each supplied route.
	ListServerCapabilities(context.Context, ServerCapabilitiesRequest) (*ServerCapabilitiesResult, error)
}

// ReadService combines all methods published by resource-read/v1.
type ReadService interface {
	ServerCapabilityService
	CurrentCustomerServerService
}

var readReferencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

func NewReadDependency(methods ...Method) (Dependency, error) {
	if len(methods) == 0 {
		methods = []Method{MethodServerCapabilitiesList}
	}
	dependency := Dependency{OwnerPluginID: OwnerPluginID, Service: ServiceName, Version: ReadVersion, Methods: append([]Method(nil), methods...)}
	if err := ValidateReadDependency(dependency); err != nil {
		return Dependency{}, err
	}
	return dependency, nil
}

func ValidateReadDependency(dependency Dependency) error {
	if dependency.OwnerPluginID != OwnerPluginID || dependency.OwnerPluginID != strings.TrimSpace(dependency.OwnerPluginID) ||
		dependency.Service != ServiceName || dependency.Service != strings.TrimSpace(dependency.Service) || dependency.Version != ReadVersion ||
		len(dependency.Methods) == 0 {
		return gerror.New("resourcecap: incompatible read dependency")
	}
	seen := make(map[Method]struct{}, len(dependency.Methods))
	for _, method := range dependency.Methods {
		if method != MethodServerCapabilitiesList && method != MethodCurrentCustomerServersList &&
			method != MethodCurrentCustomerServerGet {
			return gerror.New("resourcecap: read dependency contains an unsupported method")
		}
		if _, exists := seen[method]; exists {
			return gerror.New("resourcecap: read dependency contains a duplicate method")
		}
		seen[method] = struct{}{}
	}
	return nil
}

func ValidateServerCapabilitiesRequest(request ServerCapabilitiesRequest) error {
	if len(request.Routes) == 0 || len(request.Routes) > MaximumReadRoutes {
		return gerror.New("resourcecap: server capability routes are outside their size bound")
	}
	seen := make(map[string]struct{}, len(request.Routes))
	for _, route := range request.Routes {
		if !validReadReference(route.ServiceNo, 40) || !validReadReference(route.ChannelNo, 64) || !validReadReference(route.PoolNo, 64) {
			return gerror.New("resourcecap: server capability route is invalid")
		}
		if _, exists := seen[route.ServiceNo]; exists {
			return gerror.New("resourcecap: server capability route is duplicated")
		}
		seen[route.ServiceNo] = struct{}{}
	}
	return nil
}

func ValidateServerCapabilitiesResult(result ServerCapabilitiesResult, request ServerCapabilitiesRequest) error {
	if len(result.Items) != len(request.Routes) {
		return gerror.New("resourcecap: server capability result count is invalid")
	}
	for index, item := range result.Items {
		if item.ServiceNo != request.Routes[index].ServiceNo || len(item.Capabilities) > MaximumCapabilityCount {
			return gerror.New("resourcecap: server capability projection is invalid")
		}
		if len(item.Capabilities) == 0 {
			continue
		}
		if item.ObservedAt.IsZero() || !sort.SliceIsSorted(item.Capabilities, func(i, j int) bool { return item.Capabilities[i] < item.Capabilities[j] }) {
			return gerror.New("resourcecap: server capability projection is invalid")
		}
		seen := make(map[CapabilityCode]struct{}, len(item.Capabilities))
		for _, capability := range item.Capabilities {
			if _, exists := seen[capability]; exists {
				return gerror.New("resourcecap: server capability projection is duplicated")
			}
			seen[capability] = struct{}{}
		}
	}
	return nil
}

func validReadReference(value string, maximum int) bool {
	return len(value) <= maximum && value == strings.TrimSpace(value) &&
		readReferencePattern.MatchString(value) && !containsCredentialMaterial(value)
}
