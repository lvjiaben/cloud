// This file applies one bounded customer-tag mutation with sealed batch audit evidence.

package customermanagement

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

type daoTagMutationPersistence struct{}

func (*daoTagMutationPersistence) applyTagBatch(
	ctx context.Context,
	input normalizedTagBatch,
	auditWriter idccap.BusinessAuditWriter,
) (*TagBatchResult, error) {
	var output *TagBatchResult
	err := dao.CustomerTagAssignment.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		customers, err := lockTagBatchCustomers(txCtx, input.customerNos)
		if err != nil {
			return err
		}
		auditResult, err := auditWriter.AppendBusinessBatchTransition(
			txCtx,
			idccap.BusinessAuditBatchTransition{
				IdempotencyScope:    "customer.tag.batch." + string(input.action),
				IdempotencyKey:      input.idempotencyKey,
				OperationCode:       "customer.tag.batch." + string(input.action),
				TargetType:          "customer_tag_batch",
				TargetNo:            input.targetNo,
				AffectedCustomerNos: input.customerNos,
				ResultCode:          "succeeded",
				ReasonCode:          "manual_segment_update",
				RiskCode:            "medium",
				RequiredPermission:  permissionCustomerTagBatch,
				CorrelationNo:       input.targetNo,
				BeforeSnapshot: map[string]any{
					"action": string(input.action), "state": "requested",
					"customer_count": len(input.customerNos), "tag_nos": input.tagNos,
				},
				AfterSnapshot: map[string]any{
					"reason": input.reason, "state": "applied",
				},
			},
		)
		if err != nil {
			return err
		}
		output = &TagBatchResult{
			AuditNo: auditResult.AuditNo, Replayed: auditResult.Replayed,
			CustomerCount: len(input.customerNos), TagCount: len(input.tagNos),
		}
		if auditResult.Replayed {
			return nil
		}
		if err = validateTagBatchCustomers(customers); err != nil {
			return err
		}
		if err = lockTagBatchDefinitions(txCtx, input.tagNos); err != nil {
			return err
		}
		return executeTagBatchMutation(txCtx, input)
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}

func lockTagBatchCustomers(
	ctx context.Context,
	customerNos []string,
) ([]*entity.CustomerSubject, error) {
	columns := dao.CustomerSubject.Columns()
	var rows []*entity.CustomerSubject
	if err := dao.CustomerSubject.Ctx(ctx).
		Fields(columns.CustomerNo, columns.StatusCode).
		Where(do.CustomerSubject{TenantId: idccap.DefaultTenantID}).
		WhereIn(columns.CustomerNo, customerNos).
		OrderAsc(columns.CustomerNo).
		LockUpdate().
		Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "lock batch tag customers")
	}
	if len(rows) != len(customerNos) {
		return nil, gerror.New("batch tag customer projection is incomplete")
	}
	for index, row := range rows {
		if row == nil || row.CustomerNo != customerNos[index] {
			return nil, gerror.New("batch tag customer projection is invalid")
		}
	}
	return rows, nil
}

func validateTagBatchCustomers(rows []*entity.CustomerSubject) error {
	for _, row := range rows {
		if row == nil || row.StatusCode == "anonymized" {
			return gerror.New("batch tag customer projection is invalid")
		}
	}
	return nil
}

func lockTagBatchDefinitions(ctx context.Context, tagNos []string) error {
	columns := dao.CustomerTag.Columns()
	var rows []*entity.CustomerTag
	if err := dao.CustomerTag.Ctx(ctx).
		Fields(columns.TagNo, columns.StatusCode).
		Where(do.CustomerTag{TenantId: idccap.DefaultTenantID}).
		WhereIn(columns.TagNo, tagNos).
		OrderAsc(columns.TagNo).
		LockUpdate().
		Scan(&rows); err != nil {
		return gerror.Wrap(err, "lock batch tag definitions")
	}
	if len(rows) != len(tagNos) {
		return gerror.New("batch tag definition projection is incomplete")
	}
	for index, row := range rows {
		if row == nil || row.TagNo != tagNos[index] || row.StatusCode != "active" {
			return gerror.New("batch tag definition projection is invalid")
		}
	}
	return nil
}

func executeTagBatchMutation(ctx context.Context, input normalizedTagBatch) error {
	table := dao.CustomerTagAssignment.Table()
	customerJSON, err := json.Marshal(input.customerNos)
	if err != nil {
		return gerror.Wrap(err, "encode batch tag customers")
	}
	tagJSON, err := json.Marshal(input.tagNos)
	if err != nil {
		return gerror.Wrap(err, "encode batch tag definitions")
	}
	switch input.action {
	case TagBatchAssign:
		//nolint:dupword // Three distinct lifecycle columns intentionally share one database timestamp.
		_, err = g.DB().Exec(ctx, fmt.Sprintf(`
INSERT INTO %s (
    tenant_id, customer_no, tag_no, source_code, status_code, assigned_by,
    assigned_at, created_at, updated_at
)
SELECT ?, customer_no, tag_no, 'manual', 'active', ?,
       CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
  FROM jsonb_array_elements_text(?::jsonb) AS customer_no
 CROSS JOIN jsonb_array_elements_text(?::jsonb) AS tag_no
ON CONFLICT (customer_no, tag_no) WHERE status_code = 'active' DO NOTHING`, table),
			idccap.DefaultTenantID,
			input.actorUserID,
			string(customerJSON),
			string(tagJSON),
		)
		return gerror.Wrap(err, "assign batch customer tags")
	case TagBatchRevoke:
		_, err = g.DB().Exec(ctx, fmt.Sprintf(`
UPDATE %s
   SET status_code = 'revoked', revoked_by = ?, revoked_at = CURRENT_TIMESTAMP,
       updated_at = CURRENT_TIMESTAMP
 WHERE tenant_id = ?
   AND status_code = 'active'
   AND customer_no IN (SELECT jsonb_array_elements_text(?::jsonb))
   AND tag_no IN (SELECT jsonb_array_elements_text(?::jsonb))`, table),
			input.actorUserID,
			idccap.DefaultTenantID,
			string(customerJSON),
			string(tagJSON),
		)
		return gerror.Wrap(err, "revoke batch customer tags")
	default:
		return gerror.New("batch customer tag action is invalid")
	}
}
