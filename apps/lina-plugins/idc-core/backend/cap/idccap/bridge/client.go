// Package bridge provides the dynamic guest SDK for idccap v1.
package bridge

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginbridge"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const maximumMerchantContextPayloadBytes = 16 * 1024

// OwnerHostServiceInvoker is the explicit dynamic host-call transport dependency.
type OwnerHostServiceInvoker func(
	owner string,
	service string,
	version string,
	method string,
	resourceRef string,
	table string,
	payload []byte,
) ([]byte, error)

// Client implements the typed IDC base capability over owner-aware host calls.
type Client struct{ invoke OwnerHostServiceInvoker }

var _ idccap.Service = (*Client)(nil)

// NewClient creates a client with an explicit governed transport.
func NewClient(invoke OwnerHostServiceInvoker) (*Client, error) {
	if invoke == nil {
		return nil, gerror.New("idccap bridge: owner host-service invoker is required")
	}
	return &Client{invoke: invoke}, nil
}

// NewGuestClient creates a WASI guest client backed by the governed plugin bridge.
func NewGuestClient() (*Client, error) {
	return NewClient(pluginbridge.InvokeOwnerHostService)
}

// CurrentMerchant invokes the exact owner method and validates the returned projection.
func (c *Client) CurrentMerchant(ctx context.Context) (idccap.MerchantContext, error) {
	if c == nil || c.invoke == nil || ctx == nil {
		return idccap.MerchantContext{}, gerror.New("idccap bridge: client context is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return idccap.MerchantContext{}, gerror.Wrap(err, "idccap bridge context is closed")
	}
	response, err := c.invoke(
		idccap.OwnerPluginID,
		idccap.ServiceName,
		string(idccap.ContractVersion),
		string(idccap.MethodMerchantContextCurrent),
		"",
		"",
		nil,
	)
	if err != nil {
		return idccap.MerchantContext{}, gerror.Wrap(err, "invoke IDC merchant context")
	}
	if len(response) == 0 || len(response) > maximumMerchantContextPayloadBytes {
		return idccap.MerchantContext{}, gerror.New("idccap bridge: merchant response is outside its size bound")
	}
	var current idccap.MerchantContext
	if err = json.Unmarshal(response, &current); err != nil {
		return idccap.MerchantContext{}, gerror.Wrap(err, "decode IDC merchant context")
	}
	if err = idccap.ValidateMerchantContext(current); err != nil {
		return idccap.MerchantContext{}, err
	}
	return current, nil
}
