// This file converts private time values to the public Unix-millisecond API
// contract without changing the core-owned delivery representation.
package channel

import (
	"lina-plugin-idc-channel/backend/api/channel/v1"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"time"
)

func channelMilliseconds(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	milliseconds := value.UnixMilli()
	return &milliseconds
}

func projectChannelDeliveries(value *channelcommercecap.OrderDeliveries) *v1.OrderDeliveriesRes {
	result := &v1.OrderDeliveriesRes{OrderNo: value.OrderNo, Items: make([]v1.OrderDelivery, len(value.Items))}
	for index, delivery := range value.Items {
		row := v1.OrderDelivery{DeliveryNo: delivery.DeliveryNo, Status: delivery.Status, Quantity: delivery.Quantity, Ongoing: delivery.Ongoing, StartedAt: channelMilliseconds(delivery.StartedAt), CompletedAt: channelMilliseconds(delivery.CompletedAt), Services: make([]v1.DeliveryService, len(delivery.Services))}
		for serviceIndex, service := range delivery.Services {
			row.Services[serviceIndex] = v1.DeliveryService{ServiceNo: service.ServiceNo, Status: service.Status, CurrencyCode: service.CurrencyCode, PeriodUnit: service.PeriodUnit, PeriodCount: service.PeriodCount, RenewalMode: service.RenewalMode, StartedAt: channelMilliseconds(service.StartedAt), ExpiresAt: channelMilliseconds(service.ExpiresAt)}
		}
		result.Items[index] = row
	}
	return result
}
