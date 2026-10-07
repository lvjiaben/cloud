package contractlifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

// Consume applies a bounded page before acknowledging it. A crash before the
// acknowledgement is safe because Apply is keyed by gateway route version.
func (s *serviceImpl) Consume(ctx context.Context, limit int) (ConsumeResult, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || limit < 1 || limit > MaximumBatchSize {
		return ConsumeResult{}, gerror.New("contract lifecycle batch is invalid")
	}
	items, err := s.gateway.ListTencentContractDeliveries(ctx, limit)
	if err != nil {
		return ConsumeResult{}, gerror.Wrap(err, "list exact Tencent contract deliveries")
	}
	result := ConsumeResult{Received: len(items)}
	if len(items) == 0 {
		return result, nil
	}
	numbers := make([]string, 0, len(items))
	for _, item := range items {
		_, err = s.Apply(ctx, observationFromDelivery(item), s.systemAudit)
		if err != nil {
			return result, err
		}
		numbers = append(numbers, item.DeliveryNo)
	}
	sort.Strings(numbers)
	if err = s.gateway.AcknowledgeTencentContractDeliveries(ctx, numbers, acknowledgementKey(numbers)); err != nil {
		return result, gerror.Wrap(err, "acknowledge exact Tencent contract deliveries")
	}
	result.Acknowledged = len(numbers)
	return result, nil
}

func observationFromDelivery(item processauth.TencentContractDeliveryItem) Observation {
	return Observation{
		DeliveryNo: item.DeliveryNo, RouteNo: item.RouteNo, RouteVersion: item.RouteVersion,
		ProviderStatus: item.ProviderStatus, SourceKind: item.SourceKind,
		GatewayDecisionCode: item.DecisionCode,
		SupplierObservedAt:  time.Unix(item.SupplierObservedUnix, 0).UTC(),
		ObservedAt:          time.Unix(item.ObservedUnix, 0).UTC(),
	}
}

func acknowledgementKey(numbers []string) string {
	digest := sha256.Sum256([]byte(strings.Join(numbers, "\n")))
	return "contract-status-delivery:" + hex.EncodeToString(digest[:])
}
