// This file projects core-owned channel order deliveries with the same payer
// and application isolation as order reads, using two bounded set queries.

package backend

import (
	"context"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

// ReadOrderDeliveries includes one-time deliveries without requiring a service.
func (runtime *channelCommerceRuntime) ReadOrderDeliveries(ctx context.Context, channelID, orderNo string) (*channelcommercecap.OrderDeliveries, error) {
	order, err := runtime.loadOrder(ctx, channelID, orderNo)
	if err != nil {
		return nil, err
	}
	if _, err = runtime.requirePayer(ctx, order.PayerCustomerNo); err != nil {
		return nil, err
	}
	var rows []struct {
		DeliveryNo, StatusCode string
		Quantity               int
		OngoingManagement      bool
		StartedAt, CompletedAt *time.Time
	}
	err = runtime.database.GetScan(ctx, &rows, `SELECT delivery_no,status_code,quantity,ongoing_management,started_at,completed_at
FROM plugin_idc_core_product_delivery_record WHERE tenant_id=0 AND order_no=? AND customer_no=?
ORDER BY delivery_no LIMIT 1001`, order.OrderNo, order.ServiceOwnerCustomerNo)
	if err != nil || len(rows) > 1000 {
		return nil, bizerr.NewCode(commerce.CodeQuoteNotAvailable)
	}
	var services []struct {
		DeliveryNo, EntitlementNo, StatusCode, RenewalModeCode string
		CurrencyCode, PeriodUnitCode                           string
		PeriodCount                                            int
		StartedAt, ExpiresAt                                   *time.Time
	}
	err = runtime.database.GetScan(ctx, &services, `SELECT delivery_no,entitlement_no,status_code,renewal_mode_code,started_at,expires_at,currency_code,period_unit_code,period_count
FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND order_no=? AND customer_no=?
ORDER BY entitlement_no LIMIT 1001`, order.OrderNo, order.ServiceOwnerCustomerNo)
	if err != nil || len(services) > 1000 {
		return nil, bizerr.NewCode(commerce.CodeQuoteNotAvailable)
	}
	byDelivery := make(map[string][]channelcommercecap.DeliveryService, len(rows))
	for _, service := range services {
		byDelivery[service.DeliveryNo] = append(byDelivery[service.DeliveryNo], channelcommercecap.DeliveryService{ServiceNo: service.EntitlementNo, Status: service.StatusCode, RenewalMode: service.RenewalModeCode, StartedAt: service.StartedAt, ExpiresAt: service.ExpiresAt, CurrencyCode: service.CurrencyCode, PeriodUnit: service.PeriodUnitCode, PeriodCount: service.PeriodCount})
	}
	result := &channelcommercecap.OrderDeliveries{OrderNo: order.OrderNo, Items: make([]channelcommercecap.OrderDelivery, len(rows))}
	for index, row := range rows {
		linked := byDelivery[row.DeliveryNo]
		if linked == nil {
			linked = []channelcommercecap.DeliveryService{}
		}
		result.Items[index] = channelcommercecap.OrderDelivery{DeliveryNo: row.DeliveryNo, Status: row.StatusCode, Quantity: row.Quantity, Ongoing: row.OngoingManagement, StartedAt: row.StartedAt, CompletedAt: row.CompletedAt, Services: linked}
	}
	return result, nil
}

var _ channelcommercecap.OrderDeliveryRuntime = (*channelCommerceRuntime)(nil)
