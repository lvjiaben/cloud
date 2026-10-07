// Package bridge provides the dynamic guest SDK for inboxcap v1.
package bridge

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginbridge"
	"lina-plugin-shared-services/backend/cap/inboxcap"
)

type OwnerHostServiceInvoker func(string, string, string, string, string, string, []byte) ([]byte, error)

type Client struct{ invoke OwnerHostServiceInvoker }

var _ inboxcap.Service = (*Client)(nil)

func NewClient(invoke OwnerHostServiceInvoker) (*Client, error) {
	if invoke == nil {
		return nil, gerror.New("inboxcap bridge invoker is required")
	}
	return &Client{invoke: invoke}, nil
}

func NewGuestClient() (*Client, error) { return NewClient(pluginbridge.InvokeOwnerHostService) }

func (c *Client) Publish(ctx context.Context, input inboxcap.PublishRequest) (*inboxcap.PublishResult, error) {
	if c == nil || c.invoke == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("inboxcap bridge is unavailable")
	}
	if err := inboxcap.ValidatePublishRequest(input); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, gerror.Wrap(err, "encode inboxcap request")
	}
	if len(payload) > inboxcap.MaximumPayloadBytes {
		return nil, gerror.New("inboxcap request exceeds its size bound")
	}
	response, err := c.invoke(inboxcap.OwnerPluginID, inboxcap.ServiceName, inboxcap.ContractVersion, inboxcap.MethodPublish, "", "", payload)
	if err != nil {
		return nil, err
	}
	if len(response) == 0 || len(response) > inboxcap.MaximumPayloadBytes {
		return nil, gerror.New("inboxcap bridge response is invalid")
	}
	var result inboxcap.PublishResult
	if err = json.Unmarshal(response, &result); err != nil {
		return nil, gerror.Wrap(err, "decode inboxcap response")
	}
	return &result, nil
}
