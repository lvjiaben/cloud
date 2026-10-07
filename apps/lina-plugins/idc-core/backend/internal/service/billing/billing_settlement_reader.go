// This file provides read-only access to the billing owner's verified facts.

package billing

import (
	"context"
	"strings"

	"lina-core/pkg/bizerr"
)

// GetSettledOrderEvidence validates persisted evidence without creating financial facts.
func (s *settlementEvidenceReader) GetSettledOrderEvidence(ctx context.Context, orderNo string) (*FinancialSettlement, error) {
	if ctx == nil || ctx.Err() != nil || orderNo != strings.TrimSpace(orderNo) || !validBusinessReference(orderNo, 40) {
		return nil, bizerr.NewCode(CodeSettlementInvalid)
	}
	header, err := loadFinancialSettlementByOrder(ctx, s.database, orderNo)
	if err != nil {
		return nil, err
	}
	if header == nil {
		return nil, bizerr.NewCode(CodeSettlementNotEligible)
	}
	return projectPersistedFinancialSettlement(ctx, s.database, header, true)
}
