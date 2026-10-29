package inbox

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capregistry"
	"lina-plugin-shared-services/backend/cap/inboxcap"
)

type Invoker struct {
	mu      sync.RWMutex
	service *serviceImpl
}

func NewInvoker() *Invoker { return &Invoker{} }

func (i *Invoker) Bind(service *serviceImpl) error {
	if i == nil || service == nil {
		return gerror.New("shared inbox invoker requires a service")
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.service != nil {
		return gerror.New("shared inbox invoker is already bound")
	}
	i.service = service
	return nil
}

func (i *Invoker) Invoke(ctx context.Context, invocation capregistry.Invocation) (*capregistry.InvocationResult, error) {
	if i == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("shared inbox invoker is unavailable")
	}
	i.mu.RLock()
	service := i.service
	i.mu.RUnlock()
	if service == nil || invocation.OwnerPluginID != inboxcap.OwnerPluginID ||
		invocation.Service != inboxcap.ServiceName || invocation.Version != inboxcap.ContractVersion ||
		(invocation.Method != inboxcap.MethodPublish && invocation.Method != inboxcap.MethodCurrentSync) || !callerPattern.MatchString(invocation.CallerPluginID) ||
		invocation.CallerPluginID == inboxcap.OwnerPluginID || invocation.ResourceRef != "" || invocation.Table != "" ||
		len(invocation.Payload) == 0 || len(invocation.Payload) > inboxcap.MaximumPayloadBytes {
		return nil, gerror.New("shared inbox invocation is not authorized")
	}
	var result any
	var err error
	if invocation.Method == inboxcap.MethodCurrentSync {
		var input inboxcap.CurrentSyncRequest
		decoder := json.NewDecoder(bytes.NewReader(invocation.Payload))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&input); err != nil {
			return nil, gerror.Wrap(err, "decode inbox current sync request")
		}
		var trailing any
		if trailingErr := decoder.Decode(&trailing); trailingErr != io.EOF {
			return nil, gerror.New("inbox current sync request has trailing data")
		}
		result, err = service.CurrentSync(ctx, input.Limit)
	} else {
		var input inboxcap.PublishRequest
		if err = json.Unmarshal(invocation.Payload, &input); err != nil {
			return nil, gerror.Wrap(err, "decode shared inbox request")
		}
		client, bindErr := Bind(service, invocation.CallerPluginID)
		if bindErr != nil {
			return nil, bindErr
		}
		result, err = client.Publish(ctx, input)
	}
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return nil, gerror.Wrap(err, "encode shared inbox response")
	}
	if len(payload) > inboxcap.MaximumPayloadBytes {
		return nil, gerror.New("shared inbox response exceeds its size bound")
	}
	return &capregistry.InvocationResult{Payload: payload}, nil
}
