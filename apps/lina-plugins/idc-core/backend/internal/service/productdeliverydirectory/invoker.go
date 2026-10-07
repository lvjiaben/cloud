package productdeliverydirectory

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capregistry"
	"lina-plugin-idc-core/backend/cap/productdeliverycap"
)

type CapabilityInvoker struct {
	mutex   sync.RWMutex
	service productdeliverycap.CurrentCustomerDeliveryService
}

var _ capregistry.Invoker = (*CapabilityInvoker)(nil)

func NewCapabilityInvoker() *CapabilityInvoker { return &CapabilityInvoker{} }

func (i *CapabilityInvoker) Bind(service productdeliverycap.CurrentCustomerDeliveryService) error {
	if i == nil || service == nil {
		return gerror.New("product delivery capability requires a service")
	}
	i.mutex.Lock()
	defer i.mutex.Unlock()
	if i.service != nil {
		return gerror.New("product delivery capability is already bound")
	}
	i.service = service
	return nil
}

func (i *CapabilityInvoker) Invoke(ctx context.Context, invocation capregistry.Invocation) (*capregistry.InvocationResult, error) {
	if i == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("product delivery capability is unavailable")
	}
	i.mutex.RLock()
	service := i.service
	i.mutex.RUnlock()
	caller := strings.TrimSpace(invocation.CallerPluginID)
	if service == nil || caller == "" || caller != invocation.CallerPluginID ||
		invocation.OwnerPluginID != productdeliverycap.OwnerPluginID || invocation.Service != productdeliverycap.ServiceName ||
		invocation.Version != productdeliverycap.ContractVersion || invocation.ResourceRef != "" || invocation.Table != "" ||
		len(invocation.Payload) == 0 || len(invocation.Payload) > productdeliverycap.MaximumPayloadBytes {
		return nil, gerror.New("product delivery capability invocation is not authorized")
	}
	var result any
	var err error
	switch invocation.Method {
	case productdeliverycap.MethodCurrentCustomerDeliveriesList:
		var request productdeliverycap.CurrentCustomerDeliveryListRequest
		if decodeExact(invocation.Payload, &request) != nil || productdeliverycap.ValidateListRequest(request) != nil {
			return nil, gerror.New("product delivery capability payload is invalid")
		}
		result, err = service.ListCurrentCustomerDeliveries(ctx, caller, request)
	case productdeliverycap.MethodCurrentCustomerDeliveryGet:
		var request productdeliverycap.CurrentCustomerDeliveryGetRequest
		if decodeExact(invocation.Payload, &request) != nil || productdeliverycap.ValidateGetRequest(request) != nil {
			return nil, gerror.New("product delivery capability payload is invalid")
		}
		result, err = service.GetCurrentCustomerDelivery(ctx, caller, request)
	default:
		return nil, gerror.New("product delivery capability invocation is not authorized")
	}
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(result)
	if err != nil || len(payload) == 0 || len(payload) > productdeliverycap.MaximumPayloadBytes {
		return nil, gerror.New("product delivery capability result is outside its size bound")
	}
	return &capregistry.InvocationResult{Payload: payload}, nil
}

func decodeExact(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return gerror.New("product delivery capability payload contains trailing values")
	}
	return nil
}
