// This file shares published IP rules, frequency counting and local capacity
// reservation. Reservations never submit a provider operation or prove payment.
package serveroperation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

type ipPolicy struct {
	PolicyNo                                    string
	PolicyVersion                               int64
	Enabled                                     bool
	FreeChangesPerWindow, WindowDays, MaxPerDay int
	AmountMinor                                 int64
	MaximumSnapshotAgeSeconds                   int
}

// PrepareIPChange freezes only the current server's active IP count, merchant
// frequency/price policy and an idempotent capacity reservation. A failure after
// reservation releases it; a caller persistence failure must call ReleaseIP.
func (s *NativePriceBuilder) PrepareIPChange(ctx context.Context, c NativePriceContext, idempotencyKey string, now time.Time) (result *PreparedNativePrice, err error) {
	if !validNativePriceContext(ctx, c, now) || len(idempotencyKey) < 8 || len(idempotencyKey) > 256 {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	directory, ok := s.source.(serveroperationresourcecap.AccessoryDirectory)
	capacity, hasCapacity := s.source.(serveroperationresourcecap.IPCapacity)
	if !ok || !hasCapacity {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	route := serveroperationresourcecap.CapabilityRoute{ServiceNo: c.LocalResourceNo, ChannelNo: c.ChannelNo, PoolNo: c.PoolNo}
	projection, err := directory.GetAccessories(ctx, route)
	if err != nil || !validIPProjection(projection, c.LocalResourceNo, now) {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	addresses, err := currentIPAddresses(projection.Items)
	if err != nil {
		return nil, err
	}
	var policy *ipPolicy
	if err = s.database.GetScan(ctx, &policy, `SELECT policy.policy_no,version.policy_version,version.enabled,
version.free_changes_per_window,version.window_days,version.max_changes_per_day AS max_per_day,
version.amount_minor,version.maximum_snapshot_age_seconds FROM plugin_idc_core_server_ip_change_policy policy
JOIN plugin_idc_core_server_ip_change_policy_version version ON version.tenant_id=policy.tenant_id
AND version.policy_no=policy.policy_no AND version.policy_version=policy.current_version
WHERE policy.tenant_id=0 AND policy.currency_code=?`, c.CurrencyCode); err != nil {
		return nil, err
	}
	if !validIPPolicy(policy) {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	var frequency struct{ WindowCount, DayCount int }
	if err = s.database.GetScan(ctx, &frequency, `SELECT
COUNT(*) FILTER (WHERE created_at>=CURRENT_TIMESTAMP-(?*INTERVAL '1 day'))::INT AS window_count,
COUNT(*) FILTER (WHERE created_at>=date_trunc('day',CURRENT_TIMESTAMP))::INT AS day_count
FROM plugin_idc_core_business_operation WHERE tenant_id=0 AND subject_type_code='entitlement'
AND subject_no=? AND operation_type_code='ip_change'`, policy.WindowDays, c.EntitlementNo); err != nil {
		return nil, err
	}
	if frequency.DayCount >= policy.MaxPerDay {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	amount := policy.AmountMinor
	if frequency.WindowCount < policy.FreeChangesPerWindow {
		amount = 0
	}
	tax, err := s.LoadTax(ctx, c.VersionNo, c.CurrencyCode)
	if err != nil {
		return nil, err
	}
	// IP merchant fees are published final inclusive amounts, independently of
	// the catalog product's base-price tax display mode.
	tax.PriceIncludesTax = true
	amounts, err := commerce.CalculatePricingAmounts(catalog.PurchaseQuoteLine{PeriodUnitCode: string(catalog.PeriodUnitOnce), PriceIncludesTax: true, TaxTreatmentCode: tax.Treatment, TaxRateBasisPoints: tax.Rate, TaxCategoryCode: tax.Category, RoundingScopeCode: tax.Scope, RoundingModeCode: tax.Mode, RoundingQuantumMinor: tax.Quantum, Charges: []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 1, ChargeAmountMinor: amount, PricedAmountMinor: amount}}}, 0)
	if err != nil || amounts.TotalAmountMinor != amount {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	parameters, err := json.Marshal(map[string]int{"ip_num": len(addresses)})
	if err != nil {
		return nil, err
	}
	validator, ok := s.source.(serveroperationresourcecap.ParameterValidator)
	if !ok || validator.ValidateParameters(ctx, route, serveroperationresourcecap.ActionChangeIP, parameters) != nil {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	expires := now.UTC().Add(NativePriceQuoteTTL)
	request := serveroperationresourcecap.IPReservationRequest{Route: route, Quantity: len(addresses), MaximumCapacityAgeSeconds: policy.MaximumSnapshotAgeSeconds, ExpiresAt: expires, IdempotencyKey: idempotencyKey}
	request.RequestDigest, err = serveroperationresourcecap.IPReservationRequestDigest(request)
	if err != nil {
		return nil, err
	}
	reservation, err := capacity.ReserveIP(ctx, request)
	if err != nil {
		return nil, err
	}
	if !validIPReservation(reservation, len(addresses), expires) {
		// A malformed result is not trusted as a releasable reservation identity.
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return &PreparedNativePrice{Context: c, Action: serveroperationresourcecap.ActionChangeIP, Parameters: parameters, CatalogChargeMinor: amount, SubtotalMinor: amounts.SubtotalMinor, NetAmountMinor: amounts.NetAmountMinor, TaxAmountMinor: amounts.TaxAmountMinor, TotalAmountMinor: amount, TargetVersionNo: c.VersionNo, TargetBillingRuleNo: c.BillingRuleNo, Tax: *tax, PolicyNo: policy.PolicyNo, PolicyVersion: policy.PolicyVersion, Reservation: reservation, ReservationRequest: request, ResourceSnapshotNo: reservation.CapacityObservationNo, OldIPSetDigest: nativePriceHash("channel-ip-old/v1", c.EntitlementNo, strings.Join(addresses, ",")), IPQuantity: len(addresses), MaximumSnapshotAgeSeconds: policy.MaximumSnapshotAgeSeconds, EffectiveAt: now.UTC(), IssuedAt: now.UTC(), ExpiresAt: expires}, nil
}

// ReleaseIP releases only a valid reservation returned by this preparation.
// Unknown provider outcomes must retain occupancy rather than use this method.
func (s *NativePriceBuilder) ReleaseIP(ctx context.Context, quote *PreparedNativePrice, reason string) error {
	if quote == nil || quote.Reservation == nil {
		return nil
	}
	capacity, ok := s.source.(serveroperationresourcecap.IPCapacity)
	if !ok || serveroperationresourcecap.ValidateIPReservation(*quote.Reservation) != nil {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	_, err := capacity.ReleaseIP(ctx, serveroperationresourcecap.IPReservationReleaseRequest{ReservationNo: quote.Reservation.ReservationNo, ReasonCode: reason})
	return err
}

// RevalidateIP checks exactly the frozen request and snapshot; a different pool
// or observation cannot silently replace a quote's reserved capacity.
func (s *NativePriceBuilder) RevalidateIP(ctx context.Context, quote *PreparedNativePrice) error {
	if quote == nil || quote.Reservation == nil || serveroperationresourcecap.ValidateIPReservationRequest(quote.ReservationRequest) != nil {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	capacity, ok := s.source.(serveroperationresourcecap.IPCapacity)
	if !ok {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	reservation, err := capacity.ReserveIP(ctx, quote.ReservationRequest)
	if err != nil || !validIPReservation(reservation, quote.IPQuantity, quote.ExpiresAt) || reservation.ReservationNo != quote.Reservation.ReservationNo || reservation.CapacityObservationNo != quote.ResourceSnapshotNo {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return nil
}

func validIPPolicy(policy *ipPolicy) bool {
	return policy != nil && policy.Enabled && policy.PolicyNo != "" && policy.PolicyVersion > 0 && policy.AmountMinor >= 0 && policy.FreeChangesPerWindow >= 0 && policy.WindowDays > 0 && policy.MaxPerDay > 0 && policy.MaximumSnapshotAgeSeconds >= 60 && policy.MaximumSnapshotAgeSeconds <= 86400
}
func validIPProjection(projection *serveroperationresourcecap.AccessoryProjection, entitlement string, now time.Time) bool {
	if projection == nil || projection.ServiceNo != entitlement || projection.SnapshotNo == "" || !projection.ExpiresAt.After(now) {
		return false
	}
	for _, action := range projection.Actions {
		if action == serveroperationresourcecap.ActionChangeIP {
			return true
		}
	}
	return false
}
func validIPReservation(reservation *serveroperationresourcecap.IPReservation, quantity int, expires time.Time) bool {
	return reservation != nil && serveroperationresourcecap.ValidateIPReservation(*reservation) == nil && reservation.Status == serveroperationresourcecap.IPReservationReserved && reservation.Quantity == quantity && !reservation.ExpiresAt.Before(expires)
}
func nativePriceHash(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}
