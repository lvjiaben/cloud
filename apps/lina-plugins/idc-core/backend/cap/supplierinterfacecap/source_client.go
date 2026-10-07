package supplierinterfacecap

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capregistry"
	"lina-core/pkg/plugin/pluginhost"
)

type sourceClient struct {
	invoker capregistry.Invoker
	secrets SourceSecretAccessInvoker
}

var _ Service = (*sourceClient)(nil)

// NewSourceClient binds one trusted source provider identity to the same owner
// invoker used by dynamic host-service calls.
func NewSourceClient(invoker capregistry.Invoker, secrets SourceSecretAccessInvoker) (Service, error) {
	if invoker == nil || secrets == nil {
		return nil, gerror.New("supplierinterfacecap source client requires configuration and secret invokers")
	}
	return &sourceClient{invoker: invoker, secrets: secrets}, nil
}

func (c *sourceClient) AccessSecret(ctx context.Context, contextID, fieldKey string, consume func([]byte) error) error {
	if c == nil || c.secrets == nil || ctx == nil || ctx.Err() != nil || contextID == "" || len(contextID) > 128 || fieldKey == "" || len(fieldKey) > 64 || consume == nil {
		return gerror.New("supplierinterfacecap source secret request is invalid")
	}
	if _, ok := pluginhost.SourceBusinessExtensionCallerPluginID(ctx); !ok {
		return gerror.New("supplierinterfacecap source invocation identity is unavailable")
	}
	return c.secrets.AccessSecret(ctx, contextID, fieldKey, consume)
}

func (c *sourceClient) ExecuteOutboundHTTP(ctx context.Context, contextID string, request OutboundHTTPRequest) (*OutboundHTTPResponse, error) {
	if err := ValidateOutboundHTTPRequest(request); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, gerror.New("supplierinterfacecap source outbound request is invalid")
	}
	result, err := c.invoke(ctx, contextID, MethodOutboundHTTP, payload, MaximumOutboundPayload)
	if err != nil {
		return nil, err
	}
	var response OutboundHTTPResponse
	if err = json.Unmarshal(result, &response); err != nil || ValidateOutboundHTTPResponse(&response) != nil {
		return nil, gerror.New("supplierinterfacecap source outbound response is invalid")
	}
	return &response, nil
}

// ExecuteOutboundSession runs one host-confidential authenticated session.
func (c *sourceClient) ExecuteOutboundSession(ctx context.Context, contextID string, request OutboundSessionRequest) (*OutboundSessionResponse, error) {
	if err := ValidateOutboundSessionRequest(request); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, gerror.New("supplierinterfacecap source outbound session request is invalid")
	}
	result, err := c.invoke(ctx, contextID, MethodOutboundSession, payload, MaximumOutboundPayload)
	if err != nil {
		return nil, err
	}
	var response OutboundSessionResponse
	if err = json.Unmarshal(result, &response); err != nil || ValidateOutboundSessionResponse(&response) != nil {
		return nil, gerror.New("supplierinterfacecap source outbound session response is invalid")
	}
	return &response, nil
}

func (c *sourceClient) ListConfigurationFiles(ctx context.Context, contextID, fieldKey string) (*ConfigurationFileList, error) {
	request := ConfigurationFileListRequest{FieldKey: fieldKey}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, gerror.New("supplierinterfacecap source file list request is invalid")
	}
	result, err := c.invoke(ctx, contextID, MethodFilesList, payload, MaximumPayloadBytes)
	if err != nil {
		return nil, err
	}
	var list ConfigurationFileList
	if err = json.Unmarshal(result, &list); err != nil || ValidateConfigurationFileList(&list) != nil {
		return nil, gerror.New("supplierinterfacecap source file list response is invalid")
	}
	return &list, nil
}

func (c *sourceClient) ReadConfigurationFile(ctx context.Context, contextID string, request ConfigurationFileReadRequest) (*ConfigurationFileChunk, error) {
	if err := ValidateConfigurationFileReadRequest(request); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, gerror.New("supplierinterfacecap source file request is invalid")
	}
	result, err := c.invoke(ctx, contextID, MethodFileRead, payload, MaximumFilePayloadBytes)
	if err != nil {
		return nil, err
	}
	var chunk ConfigurationFileChunk
	if err = json.Unmarshal(result, &chunk); err != nil || ValidateConfigurationFileChunk(request, &chunk) != nil {
		return nil, gerror.New("supplierinterfacecap source file response is invalid")
	}
	return &chunk, nil
}

func (c *sourceClient) GetConfiguration(ctx context.Context, contextID string) (*ConfigurationView, error) {
	result, err := c.invoke(ctx, contextID, MethodConfigurationGet, []byte("{}"), MaximumPayloadBytes)
	if err != nil {
		return nil, err
	}
	var view ConfigurationView
	if err = json.Unmarshal(result, &view); err != nil {
		return nil, gerror.Wrap(err, "decode supplier interface source response")
	}
	if err = ValidateConfigurationView(&view); err != nil {
		return nil, err
	}
	return &view, nil
}

func (c *sourceClient) invoke(ctx context.Context, contextID, method string, payload []byte, maximumResponse int) ([]byte, error) {
	if c == nil || c.invoker == nil || ctx == nil || ctx.Err() != nil || contextID == "" || len(contextID) > 128 {
		return nil, gerror.New("supplierinterfacecap source client request is invalid")
	}
	callerPluginID, ok := pluginhost.SourceBusinessExtensionCallerPluginID(ctx)
	if !ok {
		return nil, gerror.New("supplierinterfacecap source invocation identity is unavailable")
	}
	result, err := c.invoker.Invoke(ctx, capregistry.Invocation{CallerPluginID: callerPluginID, OwnerPluginID: OwnerPluginID, Service: ServiceName,
		Version: ContractVersion, Method: method, ResourceRef: contextID, Payload: payload})
	if err != nil {
		return nil, err
	}
	if result == nil || result.Status != 0 || len(result.Payload) == 0 || len(result.Payload) > maximumResponse {
		return nil, gerror.New("supplierinterfacecap source client response is invalid")
	}
	return result.Payload, nil
}
