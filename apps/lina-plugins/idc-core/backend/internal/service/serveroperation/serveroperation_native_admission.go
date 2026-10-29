// Resource admission reads frozen intent before asking for confirmation and
// records proven pre-dispatch rejection without inventing provider evidence.
package serveroperation

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
)

func (s *nativeCommercialService) ReadCheckout(ctx context.Context, input servercommercialcap.CheckoutRequest) (*servercommercialcap.CheckoutIntent, error) {
	if ctx == nil || ctx.Err() != nil || !validReference(input.QuoteNo, 40) {
		return nil, PublicError(ErrInvalidInput)
	}
	intent, frozen, customerVersion, err := s.readNativeCheckoutIntent(ctx, input)
	if err != nil {
		return nil, PublicError(err)
	}
	if err = s.authorizeNativeTarget(ctx, input.Target.CustomerNo, intent.Action); err != nil {
		return nil, err
	}
	target, err := s.readNativeCommercialTarget(ctx, input.Target, false)
	if err != nil {
		return nil, PublicError(err)
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, PublicError(err)
	}
	if !intent.ExpiresAt.After(now) || customerVersion != target.CustomerVersion ||
		frozen.EntitlementNo != target.EntitlementNo || frozen.CustomerNo != target.CustomerNo ||
		frozen.EntitlementVersion != target.EntitlementVersion || intent.CurrencyCode != target.CurrencyCode ||
		frozen.LocalResourceNo != input.Target.LocalResourceNo || frozen.ChannelNo != input.Target.ChannelNo ||
		frozen.PoolNo != input.Target.PoolNo || !nativeSnapshotAllows(input.Target, intent.Action) {
		return nil, PublicError(ErrOperationConflict)
	}
	return intent, nil
}

func (s *nativeCommercialService) readNativeCheckoutIntent(ctx context.Context, input servercommercialcap.CheckoutRequest) (*servercommercialcap.CheckoutIntent, servercommercialcap.TargetSnapshot, int64, error) {
	var frozen servercommercialcap.TargetSnapshot
	priced, err := s.loadNativePricedQuote(ctx, input.QuoteNo, input.Target.CustomerNo, "")
	if err != nil {
		return nil, frozen, 0, err
	}
	if priced != nil {
		p := priced.Price
		return &servercommercialcap.CheckoutIntent{QuoteNo: priced.QuoteNo, EntitlementNo: priced.Target.EntitlementNo, LocalResourceNo: priced.Target.LocalResourceNo, CurrencyCode: p.Context.CurrencyCode, Action: p.Action, TotalAmountMinor: p.TotalAmountMinor, ExpiresAt: p.ExpiresAt}, priced.Target, p.Context.CustomerVersion, nil
	}
	q, err := s.readNativeAccessoryQuote(ctx, input.Target.CustomerNo, input.QuoteNo, "")
	if err != nil {
		return nil, frozen, 0, err
	}
	if q == nil || json.Unmarshal([]byte(q.NativeTargetSnapshot), &frozen) != nil || q.SubjectNo != frozen.EntitlementNo || q.ExpectedSubjectVersion != frozen.EntitlementVersion {
		return nil, frozen, 0, ErrOperationConflict
	}
	return &servercommercialcap.CheckoutIntent{QuoteNo: q.QuoteNo, EntitlementNo: q.SubjectNo, LocalResourceNo: frozen.LocalResourceNo, CurrencyCode: q.CurrencyCode, Action: q.Action, TotalAmountMinor: q.TotalAmountMinor, ExpiresAt: q.ExpiresAt}, frozen, q.CustomerSubjectVersion, nil
}

func (s *nativeCommercialService) RejectUnsent(ctx context.Context, input servercommercialcap.AdmissionFailure) error {
	if ctx == nil || ctx.Err() != nil || !validReference(input.OperationNo, 40) || !validReference(input.EntitlementNo, 40) || !validNativeLifecycleDigest(input.EvidenceDigest) {
		return PublicError(ErrInvalidInput)
	}
	switch input.ReasonCode {
	case "capability_unavailable", "target_conflict", "resource_unavailable", "object_identity_unknown":
	default:
		return PublicError(ErrInvalidInput)
	}
	reason := "server_commercial.ready_admission." + input.ReasonCode
	return PublicError(s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.lockNativeExecution(ctx, input.OperationNo)
		if err != nil {
			return err
		}
		var fence struct {
			LastErrorCode      string
			ExecutionStartedAt *time.Time
		}
		if err = s.database.GetScan(ctx, &fence, `SELECT last_error_code,execution_started_at FROM plugin_idc_core_business_operation WHERE tenant_id=0 AND operation_no=?`, input.OperationNo); err != nil {
			return err
		}
		if row.SubjectNo != input.EntitlementNo || row.ProviderTaskNo != "" || fence.ExecutionStartedAt != nil {
			return ErrOperationConflict
		}
		if row.StatusCode == "manual_review" && fence.LastErrorCode == reason {
			match, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_business_operation_event WHERE tenant_id=0 AND operation_no=? AND to_status_code='manual_review' AND actor_reference=? AND evidence_digest=?)`, input.OperationNo, input.OperationNo, input.EvidenceDigest)
			if err != nil {
				return err
			}
			if !match.Bool() {
				return ErrOperationConflict
			}
			return nil
		}
		if row.StatusCode != "ready" {
			return ErrOperationConflict
		}
		if err = execOne(ctx, s.database, `UPDATE plugin_idc_core_business_operation SET status_code='manual_review',last_error_code=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND operation_no=? AND version=? AND provider_task_no IS NULL AND execution_started_at IS NULL`, reason, input.OperationNo, row.Version); err != nil {
			return err
		}
		return appendNativeAccessoryEvent(ctx, s.database, input.OperationNo, row.SubjectNo, row.Version+1, row.StatusCode, "manual_review", reason, input.OperationNo, input.EvidenceDigest)
	}))
}
