package productfulfillment

import (
	"context"
	"sort"
)

func evaluateOwnerGates(ctx context.Context, validator OwnerGateValidator, orderNo string, itemNos []string) (map[string]bool, error) {
	if validator == nil || len(itemNos) == 0 || len(itemNos) > 100 {
		return nil, errorsNew("product_fulfillment_owner_gate_unavailable")
	}
	requested := append([]string(nil), itemNos...)
	sort.Strings(requested)
	for index, itemNo := range requested {
		if !managementReferencePattern.MatchString(itemNo) || (index > 0 && itemNo == requested[index-1]) {
			return nil, errorsNew("product_fulfillment_owner_gate_unavailable")
		}
	}
	result, err := validator.EvaluateFulfillmentRelease(ctx, orderNo, requested)
	if err != nil || len(result) != len(requested) {
		return nil, errorsNew("product_fulfillment_owner_gate_unavailable")
	}
	for _, itemNo := range requested {
		if _, ok := result[itemNo]; !ok {
			return nil, errorsNew("product_fulfillment_owner_gate_unavailable")
		}
	}
	for itemNo := range result {
		if !managementReferencePattern.MatchString(itemNo) {
			return nil, errorsNew("product_fulfillment_owner_gate_unavailable")
		}
	}
	return result, nil
}
