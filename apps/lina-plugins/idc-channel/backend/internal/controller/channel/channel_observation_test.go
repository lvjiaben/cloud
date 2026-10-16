// Authenticated public observations are stamped by the source HTTP runtime,
// never by the managed connector sandbox.
package channel

import (
	"context"
	"testing"
	"time"

	v1 "lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

type observationService struct{ channel.Service }

func (observationService) Capabilities(context.Context) channel.Capabilities {
	return channel.Capabilities{GenericAPI: true}
}
func (observationService) ReadOrderDeliveries(context.Context, string, int64) (*channelcommercecap.OrderDeliveries, error) {
	return &channelcommercecap.OrderDeliveries{}, nil
}
func (observationService) ReadRenewalByRequest(context.Context, string, string, string) (*channelcommercecap.RenewalRequestResult, error) {
	return &channelcommercecap.RenewalRequestResult{}, nil
}

func TestPublicObservationsUseRuntimeMilliseconds(t *testing.T) {
	c := &ControllerV1{delegate: New(observationService{})}
	ctx := context.Background()
	before := time.Now().UnixMilli()
	capabilities, err := c.Capabilities(ctx, &v1.CapabilitiesReq{})
	if err != nil {
		t.Fatal(err)
	}
	deliveries, err := c.OrderDeliveries(ctx, &v1.OrderDeliveriesReq{Id: 1})
	if err != nil {
		t.Fatal(err)
	}
	renewal, err := c.RenewalByRequest(ctx, &v1.RenewalByRequestReq{})
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now().UnixMilli()
	for _, value := range []int64{capabilities.ObservedAt, deliveries.ObservedAt, renewal.ObservedAt} {
		if value < before || value > after {
			t.Fatalf("observation is not runtime milliseconds: %d", value)
		}
	}
}
