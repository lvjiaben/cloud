// Native IP/resize quotes freeze real entitlement ownership, shared core prices
// and resource reservations. They never invent channel identity or paid evidence.

package serveroperation

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type nativePricedQuote struct {
	QuoteNo, TargetConfigurationNo, IdempotencyDigest, RequestDigest, ContentDigest string
	Target                                                                          servercommercialcap.TargetSnapshot
	Price                                                                           *PreparedNativePrice
}

func nativeHasPricedQuote(ctx context.Context, database gdb.DB, quoteNo string) (bool, error) {
	value, err := database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_channel_service_quote WHERE tenant_id=0 AND quote_no=? AND source_code='native')`, quoteNo)
	if err != nil {
		return false, err
	}
	return value.Bool(), nil
}

func (s *nativeCommercialService) nativeQuotePriced(ctx context.Context, input servercommercialcap.QuoteRequest) (*servercommercialcap.Quote, error) {
	if input.PackageNo != "" || len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 256 || strings.TrimSpace(input.IdempotencyKey) != input.IdempotencyKey {
		return nil, PublicError(ErrInvalidInput)
	}
	if err := s.authorizeNativeTarget(ctx, input.Target.CustomerNo, input.Action); err != nil {
		return nil, err
	}
	var result *servercommercialcap.Quote
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		target, err := s.readNativeCommercialTarget(ctx, input.Target, true)
		if err != nil {
			return err
		}
		if !nativeSnapshotAllows(input.Target, input.Action) {
			return ErrActionUnavailable
		}
		request, err := digestValue(struct {
			EntitlementNo string
			Action        serveroperationresourcecap.Action
			Parameters    json.RawMessage
			Resize        any
		}{target.EntitlementNo, input.Action, input.Parameters, input.Resize})
		if err != nil {
			return err
		}
		idempotency, err := digestValue(struct{ CustomerNo, Key string }{target.CustomerNo, "native-priced:" + input.IdempotencyKey})
		if err != nil {
			return err
		}
		stored, err := s.loadNativePricedQuote(ctx, "", target.CustomerNo, idempotency)
		if err != nil {
			return err
		}
		if stored != nil {
			if stored.RequestDigest != request {
				return ErrOperationConflict
			}
			result = projectNativePricedQuote(stored, true)
			return nil
		}
		busy, err := s.nativeCommercialBusy(ctx, target.EntitlementNo, "")
		if err != nil {
			return err
		}
		if busy {
			return ErrOperationConflict
		}
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		var price *PreparedNativePrice
		configuration := target.PriceConfigurationNo
		switch input.Action {
		case serveroperationresourcecap.ActionResize:
			if input.Resize == nil || len(input.Parameters) > 0 && strings.TrimSpace(string(input.Parameters)) != "{}" {
				return ErrInvalidInput
			}
			price, configuration, err = s.nativeStructuredResize(ctx, target, input.Target, input.Resize, now)
		case serveroperationresourcecap.ActionChangeIP:
			if input.Resize != nil || len(input.Parameters) > 0 && strings.TrimSpace(string(input.Parameters)) != "{}" {
				return ErrInvalidInput
			}
			price, err = s.prices.PrepareIPChange(ctx, target.NativePriceContext, "native-ip:"+idempotency, now)
		default:
			return ErrInvalidInput
		}
		if err != nil {
			return PublicError(err)
		}
		quoteNo, err := deterministicReference("npq_", idempotency)
		if err != nil {
			return err
		}
		quote := &nativePricedQuote{QuoteNo: quoteNo, TargetConfigurationNo: configuration, IdempotencyDigest: idempotency, RequestDigest: request, Target: input.Target, Price: price}
		quote.ContentDigest, err = nativePricedContent(quote)
		if err != nil {
			return err
		}
		if err = s.persistNativePricedQuote(ctx, quote, target); err != nil {
			if releaseErr := s.prices.ReleaseIP(ctx, price, "core_quote_failed"); releaseErr != nil {
				return releaseErr
			}
			return err
		}
		result = projectNativePricedQuote(quote, false)
		return nil
	})
	return result, PublicError(err)
}

func nativePricedContent(quote *nativePricedQuote) (string, error) {
	value := struct {
		QuoteNo, Configuration, Request string
		Target                          servercommercialcap.TargetSnapshot
		Price                           *PreparedNativePrice
	}{quote.QuoteNo, quote.TargetConfigurationNo, quote.RequestDigest, quote.Target, quote.Price}
	// JSONB normalizes object key order. Hash the semantic JSON document with
	// exact integer tokens so multi-key resource projections survive round trips.
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var canonical any
	if err = decoder.Decode(&canonical); err != nil {
		return "", err
	}
	return digestValue(canonical)
}

func (s *nativeCommercialService) loadNativePricedQuote(ctx context.Context, quoteNo, customer, idempotency string) (*nativePricedQuote, error) {
	var row *struct {
		QuoteNo, TargetConfigurationNo, IdempotencyDigest, RequestDigest, ContentDigest, NativeTargetSnapshot, PreparedPrice, TargetQuoteNo, ResourceReservationNo, ResourceSnapshotNo string
		IPQuantity                                                                                                                                                                     int
	}
	err := s.database.GetScan(ctx, &row, `SELECT quote_no,target_configuration_snapshot_no AS target_configuration_no,idempotency_digest,request_digest,content_digest,native_target_snapshot::TEXT AS native_target_snapshot,prepared_price::TEXT AS prepared_price,COALESCE(native_target_quote_no,'') AS target_quote_no,COALESCE(resource_reservation_no,'') AS resource_reservation_no,resource_snapshot_no,ip_quantity FROM plugin_idc_core_channel_service_quote WHERE tenant_id=0 AND source_code='native' AND service_owner_customer_no=? AND ((?<>'' AND quote_no=?) OR (?<>'' AND idempotency_digest=?))`, customer, quoteNo, quoteNo, idempotency, idempotency)
	if err != nil || row == nil {
		return nil, err
	}
	result := &nativePricedQuote{QuoteNo: row.QuoteNo, TargetConfigurationNo: row.TargetConfigurationNo, IdempotencyDigest: row.IdempotencyDigest, RequestDigest: row.RequestDigest, ContentDigest: row.ContentDigest}
	if json.Unmarshal([]byte(row.NativeTargetSnapshot), &result.Target) != nil || json.Unmarshal([]byte(row.PreparedPrice), &result.Price) != nil || result.Price == nil {
		return nil, ErrOperationConflict
	}
	c := result.Price.Context
	if row.TargetQuoteNo != result.Price.TargetQuoteNo {
		return nil, ErrOperationConflict
	}
	if result.Price.Action == serveroperationresourcecap.ActionChangeIP && !validNativeFrozenIP(result.Price, row.ResourceReservationNo, row.ResourceSnapshotNo, row.IPQuantity) {
		return nil, ErrOperationConflict
	}
	if c.EntitlementNo != result.Target.EntitlementNo || c.CustomerNo != result.Target.CustomerNo || c.EntitlementVersion != result.Target.EntitlementVersion || c.LocalResourceNo != result.Target.LocalResourceNo || c.ChannelNo != result.Target.ChannelNo || c.PoolNo != result.Target.PoolNo {
		return nil, ErrOperationConflict
	}
	content, err := nativePricedContent(result)
	if err != nil || content != result.ContentDigest {
		return nil, ErrOperationConflict
	}
	return result, nil
}

func validNativeFrozenIP(price *PreparedNativePrice, reservationNo, snapshotNo string, quantity int) bool {
	request := price.ReservationRequest
	reservation := price.Reservation
	return reservation != nil && serveroperationresourcecap.ValidateIPReservation(*reservation) == nil && serveroperationresourcecap.ValidateIPReservationRequest(request) == nil &&
		reservation.ReservationNo == reservationNo && reservation.CapacityObservationNo == snapshotNo && reservation.Quantity == quantity &&
		price.ResourceSnapshotNo == snapshotNo && price.IPQuantity == quantity && request.Quantity == quantity && request.Route.ServiceNo == price.Context.LocalResourceNo &&
		request.Route.ChannelNo == price.Context.ChannelNo && request.Route.PoolNo == price.Context.PoolNo && request.ExpiresAt.Equal(price.ExpiresAt)
}

func projectNativePricedQuote(quote *nativePricedQuote, reused bool) *servercommercialcap.Quote {
	p := quote.Price
	return &servercommercialcap.Quote{QuoteNo: quote.QuoteNo, EntitlementNo: p.Context.EntitlementNo, CurrencyCode: p.Context.CurrencyCode, PolicyNo: p.PolicyNo, Action: p.Action, TotalAmountMinor: p.TotalAmountMinor, TaxAmountMinor: p.TaxAmountMinor, PolicyVersion: p.PolicyVersion, ExpiresAt: p.ExpiresAt, Reused: reused, Direction: p.Direction, EffectiveAt: p.EffectiveAt, CurrentRecurringMinor: p.CurrentRecurringMinor, TargetRecurringMinor: p.TargetRecurringMinor}
}
