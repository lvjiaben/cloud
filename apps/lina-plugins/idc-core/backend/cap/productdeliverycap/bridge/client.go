package bridge

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginbridge"
	"lina-plugin-idc-core/backend/cap/productdeliverycap"
)

type OwnerHostServiceInvoker func(owner, service, version, method, resourceRef, table string, payload []byte) ([]byte, error)

type Client struct{ invoke OwnerHostServiceInvoker }

var _ productdeliverycap.CurrentCustomerDeliveryService = (*Client)(nil)

func NewClient(invoke OwnerHostServiceInvoker) (*Client, error) {
	if invoke == nil {
		return nil, gerror.New("productdeliverycap bridge: invoker is required")
	}
	return &Client{invoke: invoke}, nil
}

func NewGuestClient() (*Client, error) { return NewClient(pluginbridge.InvokeOwnerHostService) }

func (c *Client) ListCurrentCustomerDeliveries(ctx context.Context, _ string, request productdeliverycap.CurrentCustomerDeliveryListRequest) (*productdeliverycap.CurrentCustomerDeliveryPage, error) {
	if c == nil || c.invoke == nil || ctx == nil || ctx.Err() != nil || productdeliverycap.ValidateListRequest(request) != nil {
		return nil, gerror.New("productdeliverycap bridge: list request is invalid")
	}
	response, err := c.invokeJSON(productdeliverycap.MethodCurrentCustomerDeliveriesList, request)
	if err != nil {
		return nil, err
	}
	var result productdeliverycap.CurrentCustomerDeliveryPage
	if json.Unmarshal(response, &result) != nil || productdeliverycap.ValidatePage(&result, request) != nil {
		return nil, gerror.New("productdeliverycap bridge: list response is invalid")
	}
	return &result, nil
}

func (c *Client) GetCurrentCustomerDelivery(ctx context.Context, _ string, request productdeliverycap.CurrentCustomerDeliveryGetRequest) (*productdeliverycap.CurrentCustomerDeliveryResult, error) {
	if c == nil || c.invoke == nil || ctx == nil || ctx.Err() != nil || productdeliverycap.ValidateGetRequest(request) != nil {
		return nil, gerror.New("productdeliverycap bridge: detail request is invalid")
	}
	response, err := c.invokeJSON(productdeliverycap.MethodCurrentCustomerDeliveryGet, request)
	if err != nil {
		return nil, err
	}
	var result productdeliverycap.CurrentCustomerDeliveryResult
	if json.Unmarshal(response, &result) != nil || productdeliverycap.ValidateResult(&result, request) != nil {
		return nil, gerror.New("productdeliverycap bridge: detail response is invalid")
	}
	return &result, nil
}

func (c *Client) invokeJSON(method string, request any) ([]byte, error) {
	payload, err := json.Marshal(request)
	if err != nil || len(payload) == 0 || len(payload) > productdeliverycap.MaximumPayloadBytes {
		return nil, gerror.New("productdeliverycap bridge: request is outside its size bound")
	}
	response, err := c.invoke(productdeliverycap.OwnerPluginID, productdeliverycap.ServiceName, productdeliverycap.ContractVersion, method, "", "", payload)
	if err != nil {
		return nil, gerror.Wrap(err, "invoke product delivery read capability")
	}
	if len(response) == 0 || len(response) > productdeliverycap.MaximumPayloadBytes {
		return nil, gerror.New("productdeliverycap bridge: response is outside its size bound")
	}
	return response, nil
}
