// This file keeps server accessories out of new-product cancellation projection.

package productfulfillment

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
)

// AddonCancellationConsumer owns addon cancellation; errors must never fall through.
type AddonCancellationConsumer interface {
	// ConsumeAddonCancellation returns handled=false only for a non-addon;
	// invalid or unavailable addon facts return an error without generic fallback.
	ConsumeAddonCancellation(context.Context, OrderCancellationInput) (*OrderCancellationResult, bool, error)
}

type routedCancellationConsumer struct {
	generic OrderCancellationConsumer
	addons  AddonCancellationConsumer
}

// NewRoutedCancellationConsumer retains the original worker and its bounded retry policy.
func NewRoutedCancellationConsumer(generic OrderCancellationConsumer, addons AddonCancellationConsumer) (OrderCancellationConsumer, error) {
	if generic == nil || addons == nil {
		return nil, gerror.New("order cancellation consumer dependencies are unavailable")
	}
	return &routedCancellationConsumer{generic: generic, addons: addons}, nil
}

func (s *routedCancellationConsumer) ConsumeOrderCancelled(ctx context.Context, input OrderCancellationInput) (*OrderCancellationResult, error) {
	result, handled, err := s.addons.ConsumeAddonCancellation(ctx, input)
	if err != nil || handled {
		return result, err
	}
	return s.generic.ConsumeOrderCancelled(ctx, input)
}
