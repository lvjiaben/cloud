package serveroperation

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

// previewIPChangeForStaff reuses the customer pricing and inventory contract
// while taking an already-authorized exact customer/service pair. It has no
// separate permission path and is reachable only through BatchManagement.
func (s *batchManagementImpl) previewIPChangeForStaff(ctx context.Context, customerNo string,
	input IPChangeQuoteInput) (*IPChangeQuote, error) {
	if !validReference(customerNo, 40) || !validReference(input.ServiceNo, 40) ||
		len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 256 {
		return nil, bizerr.NewCode(CodeIPChangeInvalid)
	}
	idempotencyDigest, _ := digestValue(struct{ CustomerNo, Key string }{customerNo, input.IdempotencyKey})
	requestDigest, _ := digestValue(struct{ CustomerNo, ServiceNo string }{customerNo, input.ServiceNo})
	if replay, replayErr := s.pricing.loadIPChangeQuoteReplay(ctx, customerNo, idempotencyDigest, requestDigest); replayErr != nil || replay != nil {
		return replay, replayErr
	}
	row, err := s.pricing.loadIPChangePreview(ctx, customerNo, input.ServiceNo)
	if err != nil || row == nil || !row.PolicyEnabled || row.ChannelNo == "" || row.PoolNo == "" ||
		!allowedAction(row.service(), serveroperationresourcecap.ActionChangeIP) {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	directory, err := s.pricing.directory.GetAccessories(ctx, serveroperationresourcecap.CapabilityRoute{
		ServiceNo: row.ServiceNo, ChannelNo: row.ChannelNo, PoolNo: row.PoolNo})
	if err != nil || directory == nil || directory.ServiceNo != row.ServiceNo || directory.SnapshotNo == "" ||
		directory.ExpiresAt.Before(time.Now().UTC()) || !containsAction(directory.Actions, serveroperationresourcecap.ActionChangeIP) {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	addresses, err := currentIPAddresses(directory.Items)
	if err != nil {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	oldDigest, _ := digestValue(struct {
		Domain, ServiceNo string
		Addresses         []string
	}{
		"server-ip-change-old-address-set/v1", row.ServiceNo, addresses})
	windowCount, dayCount, err := s.pricing.ipChangeFrequency(ctx, row.ServiceNo, row.WindowDays)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeIPChangeUnavailable)
	}
	if dayCount >= row.MaxChangesPerDay {
		return nil, bizerr.NewCode(CodeIPChangeFrequencyExceeded)
	}
	amount := row.AmountMinor
	if windowCount < row.FreeChangesPerWindow {
		amount = 0
	}
	now, err := databaseNow(ctx, s.pricing.database)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeIPChangeUnavailable)
	}
	expiresAt := now.Add(ipChangeQuoteTTL)
	reservationRequest := serveroperationresourcecap.IPReservationRequest{
		Route: serveroperationresourcecap.CapabilityRoute{ServiceNo: row.ServiceNo,
			ChannelNo: row.ChannelNo, PoolNo: row.PoolNo}, Quantity: len(addresses),
		MaximumCapacityAgeSeconds: row.MaximumSnapshotAgeSeconds, ExpiresAt: expiresAt,
		IdempotencyKey: "ip-change:" + idempotencyDigest,
	}
	reservationRequest.RequestDigest, err = serveroperationresourcecap.IPReservationRequestDigest(reservationRequest)
	if err != nil {
		return nil, bizerr.NewCode(CodeIPChangeInvalid)
	}
	reservation, err := s.pricing.ipCapacity.ReserveIP(ctx, reservationRequest)
	if err != nil || reservation == nil || reservation.Status != serveroperationresourcecap.IPReservationReserved ||
		reservation.Quantity != len(addresses) || reservation.ExpiresAt.Before(expiresAt) {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	quoteNo, _ := deterministicReference("ipq_", struct{ CustomerNo, Key string }{customerNo, input.IdempotencyKey})
	quote := &IPChangeQuote{QuoteNo: quoteNo, ServiceNo: row.ServiceNo, CurrencyCode: row.CurrencyCode,
		IPQuantity: len(addresses), FreeChange: amount == 0, AmountMinor: amount,
		PolicyNo: row.PolicyNo, PolicyVersion: row.PolicyVersion,
		ResourceReservationNo: reservation.ReservationNo, ResourceSnapshotNo: reservation.CapacityObservationNo,
		MaximumSnapshotAgeSeconds: row.MaximumSnapshotAgeSeconds, ExpiresAt: expiresAt}
	if err = s.pricing.persistIPChangeQuote(ctx, customerNo, input.IdempotencyKey,
		idempotencyDigest, requestDigest, oldDigest, row, directory, quote, now); err != nil {
		_, _ = s.pricing.ipCapacity.ReleaseIP(ctx, serveroperationresourcecap.IPReservationReleaseRequest{
			ReservationNo: reservation.ReservationNo, ReasonCode: "core_quote_failed"})
		if replay, replayErr := s.pricing.loadIPChangeQuoteReplay(ctx, customerNo, idempotencyDigest, requestDigest); replayErr == nil && replay != nil {
			return replay, nil
		}
		return nil, err
	}
	return quote, nil
}

func (s *batchManagementImpl) checkoutIPChangeForStaff(ctx context.Context, customerNo string,
	parameters json.RawMessage, idempotencyKey string, actor int) (*IPChangeCheckout, error) {
	var payload struct {
		QuoteNo string `json:"quoteNo"`
	}
	if json.Unmarshal(parameters, &payload) != nil || !validReference(customerNo, 40) ||
		!validReference(payload.QuoteNo, 40) || len(idempotencyKey) < 8 || len(idempotencyKey) > 256 {
		return nil, bizerr.NewCode(CodeIPChangeInvalid)
	}
	idempotencyDigest, _ := digestValue(struct{ CustomerNo, Key string }{customerNo, idempotencyKey})
	requestDigest, _ := digestValue(struct{ CustomerNo, QuoteNo string }{customerNo, payload.QuoteNo})
	if replay, err := s.pricing.loadIPChangeCheckoutReplay(ctx, payload.QuoteNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeIPChangeUnavailable)
	} else if replay != nil {
		if replay.IdempotencyDigest != idempotencyDigest || replay.RequestDigest != requestDigest || replay.ServiceNo == "" {
			return nil, bizerr.NewCode(CodeIPChangeConflict)
		}
		var quoteAmount struct {
			AmountMinor int64
			ExpiresAt   time.Time
		}
		if err = s.pricing.database.GetScan(ctx, &quoteAmount, `SELECT amount_minor,expires_at
FROM plugin_idc_core_server_ip_change_quote
WHERE tenant_id=0 AND customer_no=? AND quote_no=?`, customerNo, payload.QuoteNo); err != nil {
			return nil, bizerr.WrapCode(err, CodeIPChangeUnavailable)
		}
		return projectIPChangeCheckout(replay, quoteAmount.AmountMinor, quoteAmount.ExpiresAt, true), nil
	}
	preflight, err := s.pricing.loadIPChangeCheckout(ctx, customerNo, payload.QuoteNo, false)
	if err != nil || preflight == nil {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	reservationRequest := serveroperationresourcecap.IPReservationRequest{
		Route: serveroperationresourcecap.CapabilityRoute{ServiceNo: preflight.ServiceNo,
			ChannelNo: preflight.ChannelNo, PoolNo: preflight.PoolNo},
		Quantity: preflight.IPQuantity, MaximumCapacityAgeSeconds: preflight.MaximumSnapshotAgeSeconds,
		ExpiresAt: preflight.ExpiresAt,
	}
	quoteIDValue, err := s.pricing.database.GetValue(ctx, `SELECT idempotency_digest
FROM plugin_idc_core_server_ip_change_quote WHERE tenant_id=0 AND quote_no=?`, payload.QuoteNo)
	if err != nil || quoteIDValue.String() == "" {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	reservationRequest.IdempotencyKey = "ip-change:" + quoteIDValue.String()
	reservationRequest.RequestDigest, err = serveroperationresourcecap.IPReservationRequestDigest(reservationRequest)
	if err != nil {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	reservation, err := s.pricing.ipCapacity.ReserveIP(ctx, reservationRequest)
	if err != nil || reservation == nil || reservation.ReservationNo != preflight.ReservationNo ||
		reservation.Status != serveroperationresourcecap.IPReservationReserved || reservation.Quantity != preflight.IPQuantity ||
		reservation.CapacityObservationNo != preflight.SnapshotNo {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	result, err := s.pricing.checkoutIPChangeTransaction(ctx, customerNo, payload.QuoteNo,
		idempotencyKey, idempotencyDigest, requestDigest, ipChangeActor{Type: "staff",
			Reference: strconv.Itoa(actor), ReasonCode: "staff_batch_ip_change",
			Permission: PermissionServerBatchDangerous})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeIPChangeUnavailable)
	}
	return result, nil
}
