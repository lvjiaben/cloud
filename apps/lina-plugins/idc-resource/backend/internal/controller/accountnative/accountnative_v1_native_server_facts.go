// This file maps current-customer local facts without any supplier calls.
package accountnative

import (
	"context"

	"lina-plugin-idc-resource/backend/api/accountnative/v1"
)

func (c *ControllerV1) NativeServerFacts(ctx context.Context, req *v1.NativeServerFactsReq) (res *v1.NativeServerFactsRes, err error) {
	items, err := c.service.Facts(ctx, req.EntitlementNos)
	if err != nil {
		return nil, publicError(err)
	}
	res = &v1.NativeServerFactsRes{Items: make([]v1.NativeServerFact, 0, len(items))}
	for _, item := range items {
		fact := v1.NativeServerFact{EntitlementNo: item.EntitlementNo, ServerNo: item.ServerNo, Hostname: item.Hostname, OsName: item.OSName, Addresses: item.Addresses, AddressesConfirmed: item.AddressesConfirmed, LastObservedState: string(item.LastObservedState), Stale: item.Stale}
		if item.ObservedAt != nil {
			stamp := item.ObservedAt.UnixMilli()
			fact.ObservedAt = &stamp
		}
		res.Items = append(res.Items, fact)
	}
	return res, nil
}
