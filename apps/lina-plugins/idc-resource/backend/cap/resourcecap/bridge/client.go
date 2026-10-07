package bridge

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginbridge"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type OwnerHostServiceInvoker func(owner, service, version, method, resourceRef, table string, payload []byte) ([]byte, error)

type Client struct{ invoke OwnerHostServiceInvoker }

var _ resourcecap.ReadService = (*Client)(nil)

func NewClient(invoke OwnerHostServiceInvoker) (*Client, error) {
	if invoke == nil {
		return nil, gerror.New("resourcecap bridge: owner host-service invoker is required")
	}
	return &Client{invoke: invoke}, nil
}

func NewGuestClient() (*Client, error) { return NewClient(pluginbridge.InvokeOwnerHostService) }

func (c *Client) ListServerCapabilities(ctx context.Context, request resourcecap.ServerCapabilitiesRequest) (*resourcecap.ServerCapabilitiesResult, error) {
	if c == nil || c.invoke == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("resourcecap bridge: client is unavailable")
	}
	if err := resourcecap.ValidateServerCapabilitiesRequest(request); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(request)
	if err != nil || len(payload) > resourcecap.MaximumReadPayloadBytes {
		return nil, gerror.New("resourcecap bridge: request is outside its size bound")
	}
	response, err := c.invokeRead(resourcecap.MethodServerCapabilitiesList, payload)
	if err != nil {
		return nil, err
	}
	var result resourcecap.ServerCapabilitiesResult
	if json.Unmarshal(response, &result) != nil || resourcecap.ValidateServerCapabilitiesResult(result, request) != nil {
		return nil, gerror.New("resourcecap bridge: response is invalid")
	}
	return &result, nil
}

// ListCurrentCustomerServers returns one bounded current-customer page without
// accepting a customer number or returning hidden resource references.
func (c *Client) ListCurrentCustomerServers(ctx context.Context, request resourcecap.CurrentCustomerServerListRequest) (*resourcecap.CurrentCustomerServerPage, error) {
	if c == nil || c.invoke == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("resourcecap bridge: client is unavailable")
	}
	if err := resourcecap.ValidateCurrentCustomerServerListRequest(request); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(request)
	if err != nil || len(payload) > resourcecap.MaximumReadPayloadBytes {
		return nil, gerror.New("resourcecap bridge: request is outside its size bound")
	}
	response, err := c.invokeRead(resourcecap.MethodCurrentCustomerServersList, payload)
	if err != nil {
		return nil, err
	}
	var result resourcecap.CurrentCustomerServerPage
	if json.Unmarshal(response, &result) != nil || resourcecap.ValidateCurrentCustomerServerPage(&result, request) != nil {
		return nil, gerror.New("resourcecap bridge: response is invalid")
	}
	return &result, nil
}

// GetCurrentCustomerServer returns one server selected by its public number.
func (c *Client) GetCurrentCustomerServer(ctx context.Context, request resourcecap.CurrentCustomerServerGetRequest) (*resourcecap.CurrentCustomerServerResult, error) {
	if c == nil || c.invoke == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("resourcecap bridge: client is unavailable")
	}
	if err := resourcecap.ValidateCurrentCustomerServerGetRequest(request); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(request)
	if err != nil || len(payload) > resourcecap.MaximumReadPayloadBytes {
		return nil, gerror.New("resourcecap bridge: request is outside its size bound")
	}
	response, err := c.invokeRead(resourcecap.MethodCurrentCustomerServerGet, payload)
	if err != nil {
		return nil, err
	}
	var result resourcecap.CurrentCustomerServerResult
	if json.Unmarshal(response, &result) != nil || resourcecap.ValidateCurrentCustomerServerResult(&result, request) != nil {
		return nil, gerror.New("resourcecap bridge: response is invalid")
	}
	return &result, nil
}

func (c *Client) invokeRead(method resourcecap.Method, payload []byte) ([]byte, error) {
	response, err := c.invoke(resourcecap.OwnerPluginID, resourcecap.ServiceName, string(resourcecap.ReadVersion), string(method), "", "", payload)
	if err != nil {
		return nil, gerror.Wrap(err, "invoke resource capability read")
	}
	if len(response) == 0 || len(response) > resourcecap.MaximumReadPayloadBytes {
		return nil, gerror.New("resourcecap bridge: response is outside its size bound")
	}
	return response, nil
}
