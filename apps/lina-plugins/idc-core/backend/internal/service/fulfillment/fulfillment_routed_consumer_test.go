// This file verifies strict event routing and prevents addon failures from
// falling through to generic new-purchase fulfillment.

package fulfillment

import (
	"context"
	"errors"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

type routingGenericStub struct{ calls int }

func (s *routingGenericStub) ConsumeFinanciallySettled(context.Context, productfulfillment.SettlementInput) (*productfulfillment.SettlementResult, error) {
	s.calls++
	return &productfulfillment.SettlementResult{}, nil
}

type routingAddonStub struct {
	handled bool
	err     error
	calls   int
}

func (s *routingAddonStub) ConsumeAddonSettlement(context.Context, SettlementEvent) (*Delivery, bool, error) {
	s.calls++
	if s.err != nil {
		return nil, s.handled, s.err
	}
	if s.handled {
		return &Delivery{DeliveryNo: "addon-receipt", Reused: true}, true, nil
	}
	return nil, false, nil
}

func TestSettlementRoutingDoesNotTurnAddonIntoNewPurchase(t *testing.T) {
	for _, test := range []struct {
		name          string
		handled, fail bool
		genericCalls  int
	}{
		{"ordinary purchase", false, false, 1}, {"addon replay", true, false, 0},
		{"invalid addon", true, true, 0}, {"classification unavailable", false, true, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			generic := &routingGenericStub{}
			addon := &routingAddonStub{handled: test.handled}
			if test.fail {
				addon.err = errors.New("evidence unavailable")
			}
			consumer, err := NewRoutedConsumer(generic, addon)
			if err != nil {
				t.Fatal(err)
			}
			result, err := consumer.Consume(context.Background(), routingSettlementEvent())
			if (err != nil) != test.fail || generic.calls != test.genericCalls || addon.calls != 1 {
				t.Fatalf("unexpected routing: result=%+v err=%v generic=%d addon=%d", result, err, generic.calls, addon.calls)
			}
			if test.handled && !test.fail && (!result.Reused || result.FulfillmentCount != 0) {
				t.Fatal("addon receipt was replaced")
			}
		})
	}
	if _, err := NewRoutedConsumer(&routingGenericStub{}, nil); err == nil {
		t.Fatal("missing addon consumer accepted")
	}
}

func TestSettlementRoutingRejectsInvalidEnvelopeBeforeEitherConsumer(t *testing.T) {
	generic, addon := &routingGenericStub{}, &routingAddonStub{}
	consumer, err := NewRoutedConsumer(generic, addon)
	if err != nil {
		t.Fatal(err)
	}
	event := routingSettlementEvent()
	event.Payload = []byte(`{"orderNo":"ord_1"}`)
	if _, err = consumer.Consume(context.Background(), event); err == nil || generic.calls != 0 || addon.calls != 0 {
		t.Fatal("invalid event reached consumer")
	}
}

func routingSettlementEvent() outbox.ClaimedEvent {
	return outbox.ClaimedEvent{Event: outbox.Event{
		EventNo: "evt_1", EventType: FinanciallySettledEventType, EventVersion: 1, Topic: FinanciallySettledEventType,
		AggregateType: "commerce_order", AggregateNo: "ord_1", CorrelationNo: "fst_1",
		Payload:  []byte(`{"settlementNo":"fst_1","orderNo":"ord_1","invoiceNo":"inv_1","customerNo":"cus_1","currencyCode":"CNY","basisCode":"funded_invoice","totalAmountMinor":100,"settledAtUnixMs":1780000000000}`),
		Metadata: []byte(`{"sourceCount":1}`),
	}}
}
