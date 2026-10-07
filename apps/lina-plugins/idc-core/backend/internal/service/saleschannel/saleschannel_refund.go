package saleschannel

import (
	"context"
	"math/big"
	"sort"

	"github.com/gogf/gf/v2/errors/gerror"
)

// CommissionRefundRequest is the immutable base and refund projection for a
// single commission accrual. Amounts are integer minor units of one currency.
type CommissionRefundRequest struct {
	OriginalBaseMinor   int64
	CommissionMinor     int64
	AlreadyRefundedBase int64
	RefundBaseMinor     int64
}

type CommissionRefundAllocation struct {
	ReversalMinor       int64
	RemainingCommission int64
}

// AllocateCommissionRefund calculates the incremental reversal. Replaying a
// prior refund amount returns the same result; cumulative reversal can never
// exceed the original commission.
func AllocateCommissionRefund(ctx context.Context, request CommissionRefundRequest) (CommissionRefundAllocation, error) {
	if ctx == nil {
		return CommissionRefundAllocation{}, gerror.New("commission refund context is required")
	}
	if err := ctx.Err(); err != nil {
		return CommissionRefundAllocation{}, gerror.Wrap(err, "commission refund context is unavailable")
	}
	if request.OriginalBaseMinor <= 0 || request.CommissionMinor < 0 || request.AlreadyRefundedBase < 0 || request.RefundBaseMinor < 0 ||
		request.AlreadyRefundedBase > request.OriginalBaseMinor || request.RefundBaseMinor > request.OriginalBaseMinor-request.AlreadyRefundedBase {
		return CommissionRefundAllocation{}, gerror.New("commission refund request is invalid")
	}
	previousReversal, ok := proportionalMinor(request.CommissionMinor, request.AlreadyRefundedBase, request.OriginalBaseMinor)
	if !ok {
		return CommissionRefundAllocation{}, gerror.New("commission refund allocation overflow")
	}
	totalReversal, ok := proportionalMinor(request.CommissionMinor, request.AlreadyRefundedBase+request.RefundBaseMinor, request.OriginalBaseMinor)
	if !ok {
		return CommissionRefundAllocation{}, gerror.New("commission refund allocation overflow")
	}
	if totalReversal < previousReversal {
		return CommissionRefundAllocation{}, gerror.New("commission refund allocation is not monotonic")
	}
	increment := totalReversal - previousReversal
	if increment < 0 {
		increment = 0
	}
	if totalReversal > request.CommissionMinor {
		totalReversal = request.CommissionMinor
		increment = totalReversal - previousReversal
	}
	return CommissionRefundAllocation{ReversalMinor: increment, RemainingCommission: request.CommissionMinor - totalReversal}, nil
}

type CurrencyAmount struct {
	CurrencyCode string
	AmountMinor  int64
}

// AggregateCommissionByCurrency groups only identical currencies and returns
// stable lexical order. No exchange rate or cross-currency total is invented.
func AggregateCommissionByCurrency(ctx context.Context, values []CurrencyAmount) ([]CurrencyAmount, error) {
	if ctx == nil {
		return nil, gerror.New("currency aggregation context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, gerror.Wrap(err, "currency aggregation context is unavailable")
	}
	grouped := make(map[string]int64, len(values))
	for _, value := range values {
		if len(value.CurrencyCode) != 3 || value.CurrencyCode[0] < 'A' || value.CurrencyCode[0] > 'Z' || value.CurrencyCode[1] < 'A' || value.CurrencyCode[1] > 'Z' || value.CurrencyCode[2] < 'A' || value.CurrencyCode[2] > 'Z' || value.AmountMinor < 0 {
			return nil, gerror.New("currency amount is invalid")
		}
		current := grouped[value.CurrencyCode]
		if current > int64(^uint64(0)>>1)-value.AmountMinor {
			return nil, gerror.New("currency amount overflow")
		}
		grouped[value.CurrencyCode] = current + value.AmountMinor
	}
	result := make([]CurrencyAmount, 0, len(grouped))
	for currency, amount := range grouped {
		result = append(result, CurrencyAmount{CurrencyCode: currency, AmountMinor: amount})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CurrencyCode < result[j].CurrencyCode })
	return result, nil
}

func proportionalMinor(amount, base, denominator int64) (int64, bool) {
	product := new(big.Int).Mul(big.NewInt(amount), big.NewInt(base))
	product.Quo(product, big.NewInt(denominator))
	if !product.IsInt64() {
		return 0, false
	}
	return product.Int64(), true
}
