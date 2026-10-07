// This file routes verified event envelopes by durable addon intent, without
// allowing failed addon deliveries to become new-purchase fulfillment.

package fulfillment

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

// routedSettlementConsumer delegates each settled order to its existing owner.
type routedSettlementConsumer struct {
	generic productfulfillment.SettlementConsumer
	addons  AddonSettlementConsumer
}

// claimedEventView retains all coordinates required by strict event decoding.
func claimedEventView(claimed outbox.ClaimedEvent) outboxEventView {
	return outboxEventView{
		EventNo: claimed.EventNo, EventType: claimed.EventType,
		EventVersion: claimed.EventVersion, Topic: claimed.Topic,
		AggregateType: claimed.AggregateType, AggregateNo: claimed.AggregateNo,
		CorrelationNo: claimed.CorrelationNo, Payload: claimed.Payload, Metadata: claimed.Metadata,
	}
}

// NewRoutedConsumer creates the only consumer registered for the settlement
// outbox. Persisted addon intents resume their original server operation;
// ordinary purchases retain their immutable product-version policy routing.
func NewRoutedConsumer(generic productfulfillment.SettlementConsumer, addons AddonSettlementConsumer) (Consumer, error) {
	if generic == nil || addons == nil {
		return nil, gerror.New("financial settlement routed consumer dependencies are unavailable")
	}
	return &routedSettlementConsumer{generic: generic, addons: addons}, nil
}

// Consume validates the envelope before choosing an owner; owner errors never fall through.
func (c *routedSettlementConsumer) Consume(ctx context.Context, claimed outbox.ClaimedEvent) (*Delivery, error) {
	payload, metadata, err := decodeSettlementEvent(claimedEventView(claimed))
	if err != nil {
		return nil, err
	}
	delivery, handled, err := c.addons.ConsumeAddonSettlement(ctx, SettlementEvent{
		EventNo: claimed.EventNo, SettlementNo: payload.SettlementNo, OrderNo: payload.OrderNo,
		InvoiceNo: payload.InvoiceNo, CustomerNo: payload.CustomerNo, CurrencyCode: payload.CurrencyCode,
		BasisCode: payload.BasisCode, TotalAmountMinor: payload.TotalAmountMinor,
		SettledAtUnixMS: payload.SettledAtUnixMS, SourceCount: metadata.SourceCount,
	})
	if err != nil || handled {
		return delivery, err
	}
	if _, err = c.generic.ConsumeFinanciallySettled(ctx, productfulfillment.SettlementInput{SettlementNo: payload.SettlementNo, OrderNo: payload.OrderNo}); err != nil {
		return nil, err
	}
	return &Delivery{EventNo: claimed.EventNo, SettlementNo: payload.SettlementNo, OrderNo: payload.OrderNo, Reused: false}, nil
}
