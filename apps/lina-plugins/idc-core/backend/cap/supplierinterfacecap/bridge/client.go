package bridge

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginbridge"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
)

type OwnerHostServiceInvoker func(owner, service, version, method, resourceRef, table string, payload []byte) ([]byte, error)

type Client struct{ invoke OwnerHostServiceInvoker }

var _ supplierinterfacecap.Service = (*Client)(nil)

func NewClient(invoke OwnerHostServiceInvoker) (*Client, error) {
	if invoke == nil {
		return nil, gerror.New("supplierinterfacecap bridge: owner host-service invoker is required")
	}
	return &Client{invoke: invoke}, nil
}

func NewGuestClient() (*Client, error) { return NewClient(pluginbridge.InvokeOwnerHostService) }

func (c *Client) GetConfiguration(ctx context.Context, contextID string) (*supplierinterfacecap.ConfigurationView, error) {
	payload, err := c.call(ctx, contextID, supplierinterfacecap.MethodConfigurationGet, []byte("{}"), supplierinterfacecap.MaximumPayloadBytes)
	if err != nil {
		return nil, err
	}
	var result supplierinterfacecap.ConfigurationView
	if err = json.Unmarshal(payload, &result); err != nil {
		return nil, gerror.Wrap(err, "decode supplier interface configuration")
	}
	if err = supplierinterfacecap.ValidateConfigurationView(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ExecuteOutboundHTTP(ctx context.Context, contextID string, request supplierinterfacecap.OutboundHTTPRequest) (*supplierinterfacecap.OutboundHTTPResponse, error) {
	if err := supplierinterfacecap.ValidateOutboundHTTPRequest(request); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, gerror.New("supplierinterfacecap bridge: outbound request is invalid")
	}
	response, err := c.call(ctx, contextID, supplierinterfacecap.MethodOutboundHTTP, payload, supplierinterfacecap.MaximumOutboundPayload)
	if err != nil {
		return nil, err
	}
	var result supplierinterfacecap.OutboundHTTPResponse
	if err = json.Unmarshal(response, &result); err != nil || supplierinterfacecap.ValidateOutboundHTTPResponse(&result) != nil {
		return nil, gerror.New("supplierinterfacecap bridge: outbound response is invalid")
	}
	return &result, nil
}

// ExecuteOutboundSession runs one host-confidential authenticated session.
func (c *Client) ExecuteOutboundSession(ctx context.Context, contextID string, request supplierinterfacecap.OutboundSessionRequest) (*supplierinterfacecap.OutboundSessionResponse, error) {
	if err := supplierinterfacecap.ValidateOutboundSessionRequest(request); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, gerror.New("supplierinterfacecap bridge: outbound session request is invalid")
	}
	response, err := c.call(ctx, contextID, supplierinterfacecap.MethodOutboundSession, payload, supplierinterfacecap.MaximumOutboundPayload)
	if err != nil {
		return nil, err
	}
	var result supplierinterfacecap.OutboundSessionResponse
	if err = json.Unmarshal(response, &result); err != nil || supplierinterfacecap.ValidateOutboundSessionResponse(&result) != nil {
		return nil, gerror.New("supplierinterfacecap bridge: outbound session response is invalid")
	}
	return &result, nil
}

func (c *Client) ListConfigurationFiles(ctx context.Context, contextID, fieldKey string) (*supplierinterfacecap.ConfigurationFileList, error) {
	payload, err := json.Marshal(supplierinterfacecap.ConfigurationFileListRequest{FieldKey: fieldKey})
	if err != nil {
		return nil, gerror.New("supplierinterfacecap bridge: file list request is invalid")
	}
	response, err := c.call(ctx, contextID, supplierinterfacecap.MethodFilesList, payload, supplierinterfacecap.MaximumPayloadBytes)
	if err != nil {
		return nil, err
	}
	var list supplierinterfacecap.ConfigurationFileList
	if err = json.Unmarshal(response, &list); err != nil || supplierinterfacecap.ValidateConfigurationFileList(&list) != nil {
		return nil, gerror.New("supplierinterfacecap bridge: file list response is invalid")
	}
	return &list, nil
}

func (c *Client) ReadConfigurationFile(ctx context.Context, contextID string, request supplierinterfacecap.ConfigurationFileReadRequest) (*supplierinterfacecap.ConfigurationFileChunk, error) {
	if err := supplierinterfacecap.ValidateConfigurationFileReadRequest(request); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, gerror.New("supplierinterfacecap bridge: file request is invalid")
	}
	response, err := c.call(ctx, contextID, supplierinterfacecap.MethodFileRead, payload, supplierinterfacecap.MaximumFilePayloadBytes)
	if err != nil {
		return nil, err
	}
	var chunk supplierinterfacecap.ConfigurationFileChunk
	if err = json.Unmarshal(response, &chunk); err != nil || supplierinterfacecap.ValidateConfigurationFileChunk(request, &chunk) != nil {
		return nil, gerror.New("supplierinterfacecap bridge: file response is invalid")
	}
	return &chunk, nil
}

func (c *Client) call(ctx context.Context, contextID, method string, payload []byte, maximumResponse int) ([]byte, error) {
	if c == nil || c.invoke == nil || ctx == nil || ctx.Err() != nil || contextID == "" || len(contextID) > 128 {
		return nil, gerror.New("supplierinterfacecap bridge: request is invalid")
	}
	response, err := c.invoke(supplierinterfacecap.OwnerPluginID, supplierinterfacecap.ServiceName,
		supplierinterfacecap.ContractVersion, method, contextID, "", payload)
	if err != nil {
		return nil, gerror.Wrap(err, "read supplier interface runtime data")
	}
	if len(response) == 0 || len(response) > maximumResponse {
		return nil, gerror.New("supplierinterfacecap bridge: response is invalid")
	}
	return response, nil
}

func (*Client) AccessSecret(context.Context, string, string, func([]byte) error) error {
	return gerror.New("supplierinterfacecap bridge: dynamic secret access is unavailable")
}
