// This file adapts the neutral core capability to the sealed audit owner.

package audit

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
)

type businessWriterAdapter struct {
	writer Writer
}

// NewBusinessWriterAdapter returns the narrow transaction writer consumed by
// business services without exposing audit persistence internals.
func NewBusinessWriterAdapter(writer Writer) (idccap.BusinessAuditWriter, error) {
	if isNilInterface(writer) {
		return nil, gerror.New("business audit adapter requires audit writer")
	}
	return &businessWriterAdapter{writer: writer}, nil
}

func (a *businessWriterAdapter) AppendBusinessTransition(
	ctx context.Context,
	input idccap.BusinessAuditTransition,
) error {
	_, err := a.writer.AppendTransition(ctx, TransitionInput{
		Fields: Fields{
			IdempotencyScope:   input.IdempotencyScope,
			IdempotencyKey:     input.IdempotencyKey,
			OperationCode:      input.OperationCode,
			TargetType:         input.TargetType,
			TargetNo:           input.TargetNo,
			CustomerNo:         input.CustomerNo,
			Result:             ResultCode(input.ResultCode),
			ReasonCode:         input.ReasonCode,
			Risk:               RiskCode(input.RiskCode),
			RequiredPermission: input.RequiredPermission,
			CorrelationNo:      input.CorrelationNo,
		},
		BeforeSnapshot: input.BeforeSnapshot,
		AfterSnapshot:  input.AfterSnapshot,
	})
	return err
}

func (a *businessWriterAdapter) AppendBusinessBatchTransition(
	ctx context.Context,
	input idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	result, err := a.writer.AppendBatchTransition(ctx, BatchTransitionInput{
		Fields: Fields{
			IdempotencyScope:   input.IdempotencyScope,
			IdempotencyKey:     input.IdempotencyKey,
			OperationCode:      input.OperationCode,
			TargetType:         input.TargetType,
			TargetNo:           input.TargetNo,
			Result:             ResultCode(input.ResultCode),
			ReasonCode:         input.ReasonCode,
			Risk:               RiskCode(input.RiskCode),
			RequiredPermission: input.RequiredPermission,
			CorrelationNo:      input.CorrelationNo,
		},
		AffectedCustomerNos: input.AffectedCustomerNos,
		BeforeSnapshot:      input.BeforeSnapshot,
		AfterSnapshot:       input.AfterSnapshot,
	})
	if err != nil {
		return idccap.BusinessAuditAppendResult{}, err
	}
	return idccap.BusinessAuditAppendResult{
		AuditNo: result.AuditNo, Replayed: result.Replayed,
	}, nil
}

var _ idccap.BusinessAuditWriter = (*businessWriterAdapter)(nil)
