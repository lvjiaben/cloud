// Package bridge provides the dynamic guest SDK for challengecap v1.
package bridge

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginbridge"
	"lina-plugin-shared-services/backend/cap/challengecap"
)

type OwnerHostServiceInvoker func(string, string, string, string, string, string, []byte) ([]byte, error)

type Client struct{ invoke OwnerHostServiceInvoker }

var _ challengecap.Service = (*Client)(nil)

func NewClient(invoke OwnerHostServiceInvoker) (*Client, error) {
	if invoke == nil {
		return nil, gerror.New("challengecap bridge invoker is required")
	}
	return &Client{invoke: invoke}, nil
}

func NewGuestClient() (*Client, error) { return NewClient(pluginbridge.InvokeOwnerHostService) }

func (c *Client) Create(ctx context.Context, input challengecap.CreateRequest) (*challengecap.Presentation, error) {
	var output challengecap.Presentation
	if err := c.call(ctx, string(challengecap.MethodCreate), input, &output); err != nil {
		return nil, err
	}
	return &output, nil
}

func (c *Client) Complete(ctx context.Context, input challengecap.CompleteRequest) (*challengecap.Proof, error) {
	var output challengecap.Proof
	if err := c.call(ctx, string(challengecap.MethodComplete), input, &output); err != nil {
		return nil, err
	}
	return &output, nil
}

func (c *Client) Consume(ctx context.Context, input challengecap.ConsumeRequest) (*challengecap.ConsumeResult, error) {
	var output challengecap.ConsumeResult
	if err := c.call(ctx, string(challengecap.MethodConsume), input, &output); err != nil {
		return nil, err
	}
	return &output, nil
}

func (c *Client) call(ctx context.Context, method string, input any, output any) error {
	if c == nil || c.invoke == nil || ctx == nil || ctx.Err() != nil {
		return gerror.New("challengecap bridge is unavailable")
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return gerror.Wrap(err, "encode challengecap request")
	}
	if len(payload) > challengecap.MaximumPayloadBytes {
		return gerror.New("challengecap request exceeds its size bound")
	}
	response, err := c.invoke(challengecap.OwnerPluginID, challengecap.ServiceName, challengecap.ContractVersion, method, "", "", payload)
	if err != nil {
		return err
	}
	if len(response) == 0 || len(response) > challengecap.MaximumPayloadBytes {
		return gerror.New("challengecap bridge response is invalid")
	}
	return json.Unmarshal(response, output)
}
