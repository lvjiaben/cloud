package challenge

import (
	"context"
	"encoding/json"
	"regexp"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capregistry"
	"lina-plugin-shared-services/backend/cap/challengecap"
)

var dynamicCallerPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,127}$`)

type Invoker struct {
	mu      sync.RWMutex
	service *serviceImpl
}

func NewInvoker() *Invoker { return &Invoker{} }

func (i *Invoker) Bind(service *serviceImpl) error {
	if i == nil || service == nil {
		return gerror.New("shared challenge invoker requires a service")
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.service != nil {
		return gerror.New("shared challenge invoker is already bound")
	}
	i.service = service
	return nil
}

func (i *Invoker) Invoke(ctx context.Context, invocation capregistry.Invocation) (*capregistry.InvocationResult, error) {
	if i == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("shared challenge invoker is unavailable")
	}
	i.mu.RLock()
	service := i.service
	i.mu.RUnlock()
	if service == nil || invocation.OwnerPluginID != challengecap.OwnerPluginID ||
		invocation.Service != challengecap.ServiceName || invocation.Version != challengecap.ContractVersion ||
		!dynamicCallerPattern.MatchString(invocation.CallerPluginID) || invocation.CallerPluginID == challengecap.OwnerPluginID ||
		invocation.ResourceRef != "" || invocation.Table != "" || len(invocation.Payload) == 0 ||
		len(invocation.Payload) > challengecap.MaximumPayloadBytes {
		return nil, gerror.New("shared challenge invocation is not authorized")
	}
	client, err := Bind(service, invocation.CallerPluginID)
	if err != nil {
		return nil, err
	}
	var response any
	switch challengecap.Method(invocation.Method) {
	case challengecap.MethodCreate:
		var input challengecap.CreateRequest
		if err = json.Unmarshal(invocation.Payload, &input); err == nil {
			response, err = client.Create(ctx, input)
		}
	case challengecap.MethodComplete:
		var input challengecap.CompleteRequest
		if err = json.Unmarshal(invocation.Payload, &input); err == nil {
			response, err = client.Complete(ctx, input)
		}
	case challengecap.MethodConsume:
		var input challengecap.ConsumeRequest
		if err = json.Unmarshal(invocation.Payload, &input); err == nil {
			response, err = client.Consume(ctx, input)
		}
	default:
		return nil, gerror.New("shared challenge invocation method is unsupported")
	}
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(response)
	if err != nil {
		return nil, gerror.Wrap(err, "encode shared challenge response")
	}
	if len(payload) > challengecap.MaximumPayloadBytes {
		return nil, gerror.New("shared challenge response exceeds its size bound")
	}
	return &capregistry.InvocationResult{Payload: payload}, nil
}
