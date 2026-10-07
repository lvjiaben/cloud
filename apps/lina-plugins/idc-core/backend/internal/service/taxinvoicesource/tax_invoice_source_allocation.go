// This file performs deterministic integer item and tax allocation.
package taxinvoicesource

import (
	"math"
	"math/bits"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

// allocateInvoiceItems distributes a requested gross amount over remaining
// item capacities using only server-owned source tax facts.
func allocateInvoiceItems(
	rows []itemRow,
	usage taxinvoicecap.InvoiceUsage,
	expectedRemaining int64,
	requested int64,
) ([]taxinvoicecap.ItemSnapshot, int64, int64, error) {
	usageByItem := make(map[string]taxinvoicecap.ItemUsage, len(usage.Items))
	for _, item := range usage.Items {
		usageByItem[item.BillingInvoiceItemNo] = item
	}
	capacities := make([]int64, len(rows))
	remainingNets := make([]int64, len(rows))
	var totalCapacity int64
	for index, row := range rows {
		prior, ok := usageByItem[row.InvoiceItemNo]
		if !ok {
			prior = taxinvoicecap.ItemUsage{BillingInvoiceItemNo: row.InvoiceItemNo}
		}
		delete(usageByItem, row.InvoiceItemNo)
		sourceGross := row.TotalAmountMinor - row.RefundedAmountMinor
		if sourceGross < 0 {
			return nil, 0, 0, gerror.New("tax invoice item source capacity is inconsistent")
		}
		sourceNet, allocationErr := multiplyDivideFloor(
			sourceGross, row.NetAmountMinor, row.TotalAmountMinor,
		)
		if allocationErr != nil {
			return nil, 0, 0, allocationErr
		}
		sourceTax := sourceGross - sourceNet
		if prior.AllocatedGrossMinor > sourceGross ||
			prior.AllocatedNetMinor > sourceNet || prior.AllocatedTaxMinor > sourceTax {
			return nil, 0, 0, gerror.New("tax invoice item source capacity is inconsistent")
		}
		capacity := sourceGross - prior.AllocatedGrossMinor
		remainingNet := sourceNet - prior.AllocatedNetMinor
		remainingTax := sourceTax - prior.AllocatedTaxMinor
		if capacity != remainingNet+remainingTax || addOverflow(totalCapacity, capacity) {
			return nil, 0, 0, gerror.New("tax invoice item remaining tax amounts are inconsistent")
		}
		capacities[index] = capacity
		remainingNets[index] = remainingNet
		totalCapacity += capacity
	}
	if len(usageByItem) != 0 || expectedRemaining != totalCapacity ||
		requested <= 0 || requested > totalCapacity {
		return nil, 0, 0, gerror.New("requested tax invoice item allocation is unavailable")
	}
	grossShares := make([]int64, len(rows))
	var assigned int64
	for index, capacity := range capacities {
		share, err := multiplyDivideFloor(requested, capacity, totalCapacity)
		if err != nil {
			return nil, 0, 0, err
		}
		grossShares[index] = share
		assigned += share
	}
	for remainder := requested - assigned; remainder > 0; {
		progress := false
		for index := len(grossShares) - 1; index >= 0 && remainder > 0; index-- {
			available := capacities[index] - grossShares[index]
			if available <= 0 {
				continue
			}
			step := available
			if step > remainder {
				step = remainder
			}
			grossShares[index] += step
			remainder -= step
			progress = true
		}
		if !progress {
			return nil, 0, 0, gerror.New("tax invoice item rounding remainder cannot be allocated")
		}
	}
	result := make([]taxinvoicecap.ItemSnapshot, 0, len(rows))
	var totalNet, totalTax int64
	for index, gross := range grossShares {
		if gross == 0 {
			continue
		}
		net, err := multiplyDivideFloor(gross, remainingNets[index], capacities[index])
		if err != nil {
			return nil, 0, 0, err
		}
		tax := gross - net
		row := rows[index]
		ratePPM := 0
		if row.TaxRateBasisPoints != nil {
			ratePPM = *row.TaxRateBasisPoints * 100
		}
		result = append(result, taxinvoicecap.ItemSnapshot{
			BillingInvoiceItemNo: row.InvoiceItemNo, SourceReference: row.SourceReference,
			DescriptionZhCN: row.DescriptionZhCN, DescriptionEnUS: row.DescriptionEnUS,
			TaxTreatmentCode: row.TaxTreatmentCode, TaxRatePPM: ratePPM,
			TaxClassificationCode: row.TaxCategoryCode,
			GrossAmountMinor:      gross, NetAmountMinor: net, TaxAmountMinor: tax,
			SourceContentDigest: row.SourceContentDigest,
		})
		totalNet += net
		totalTax += tax
	}
	if len(result) == 0 || totalNet+totalTax != requested {
		return nil, 0, 0, gerror.New("tax invoice item allocation total is inconsistent")
	}
	return result, totalNet, totalTax, nil
}

// multiplyDivideFloor calculates value*numerator/denominator without overflow.
func multiplyDivideFloor(value, numerator, denominator int64) (int64, error) {
	if value < 0 || numerator < 0 || denominator <= 0 || numerator > denominator {
		return 0, gerror.New("tax invoice integer allocation input is invalid")
	}
	high, low := bits.Mul64(uint64(value), uint64(numerator))
	if high >= uint64(denominator) {
		return 0, gerror.New("tax invoice integer allocation overflows")
	}
	quotient, _ := bits.Div64(high, low, uint64(denominator))
	if quotient > math.MaxInt64 {
		return 0, gerror.New("tax invoice integer allocation overflows")
	}
	return int64(quotient), nil
}

// addOverflow reports whether adding two non-negative amounts would overflow.
func addOverflow(left, right int64) bool {
	return right > 0 && left > math.MaxInt64-right
}
