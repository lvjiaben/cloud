// This file adapts the merchant projection to the governed owner-aware dispatcher.
package merchantcontext

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capregistry"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const maximumMerchantContextPayloadBytes = 16 * 1024

// CapabilityInvoker dispatches only the exact merchant-context read method.
type CapabilityInvoker struct {
	mutex   sync.RWMutex
	service idccap.Service
}

var _ capregistry.Invoker = (*CapabilityInvoker)(nil)

// NewCapabilityInvoker creates an unbound dispatcher that rejects traffic until startup binding.
func NewCapabilityInvoker() *CapabilityInvoker { return &CapabilityInvoker{} }

// Bind installs the single owner service before traffic is accepted.
func (i *CapabilityInvoker) Bind(service idccap.Service) error {
	if i == nil || service == nil {
		return gerror.New("IDC capability invoker requires a service")
	}
	i.mutex.Lock()
	defer i.mutex.Unlock()
	if i.service != nil {
		return gerror.New("IDC capability invoker is already bound")
	}
	i.service = service
	return nil
}

// Invoke rejects malformed envelopes before reading the owner projection.
func (i *CapabilityInvoker) Invoke(ctx context.Context, invocation capregistry.Invocation) (*capregistry.InvocationResult, error) {
	if i == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("IDC capability invoker is unavailable")
	}
	i.mutex.RLock()
	service := i.service
	i.mutex.RUnlock()
	if service == nil {
		return nil, gerror.New("IDC capability invoker is unavailable")
	}
	caller := strings.TrimSpace(invocation.CallerPluginID)
	if caller == "" || caller != invocation.CallerPluginID ||
		invocation.OwnerPluginID != idccap.OwnerPluginID ||
		invocation.Service != idccap.ServiceName ||
		invocation.Version != string(idccap.ContractVersion) ||
		invocation.Method != string(idccap.MethodMerchantContextCurrent) ||
		invocation.ResourceRef != "" || invocation.Table != "" || len(invocation.Payload) != 0 {
		return nil, gerror.New("IDC capability invocation is not authorized")
	}
	current, err := service.CurrentMerchant(ctx)
	if err != nil {
		return nil, err
	}
	if err = idccap.ValidateMerchantContext(current); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(current)
	if err != nil {
		return nil, gerror.Wrap(err, "encode IDC merchant context")
	}
	if len(payload) == 0 || len(payload) > maximumMerchantContextPayloadBytes {
		return nil, gerror.New("IDC merchant context exceeds its payload bound")
	}
	return &capregistry.InvocationResult{Payload: payload}, nil
}
