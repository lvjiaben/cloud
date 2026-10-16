package serveroperation

import (
	"context"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

func (s *batchManagementImpl) ListClassifications(ctx context.Context) ([]ServerClassification, error) {
	actor, grants, err := s.management.authorize(ctx, []string{PermissionServerBatchView})
	if err != nil || actor <= 0 || !grants[PermissionServerBatchView] {
		return nil, bizerr.NewCode(CodeServerBatchNotAccessible)
	}
	var rows []ServerClassification
	if err = s.management.directory.database.GetScan(ctx, &rows, `SELECT classification_no,
name_zh_cn,name_en_us,status_code AS status,version
FROM plugin_idc_core_server_classification
WHERE tenant_id=0 ORDER BY status_code,name_zh_cn,id LIMIT 501`); err != nil || len(rows) > 500 {
		return nil, bizerr.NewCode(CodeServerBatchUnavailable)
	}
	return rows, nil
}

func (s *batchManagementImpl) SaveClassification(ctx context.Context,
	input ServerClassificationInput) (*ServerClassification, error) {
	actor, grants, err := s.management.authorize(ctx, []string{PermissionServerBatchSafe})
	input.NameZhCN, input.NameEnUS = strings.TrimSpace(input.NameZhCN), strings.TrimSpace(input.NameEnUS)
	if err != nil || actor <= 0 || !grants[PermissionServerBatchSafe] ||
		len([]rune(input.NameZhCN)) < 1 || len([]rune(input.NameZhCN)) > 80 ||
		len([]rune(input.NameEnUS)) < 1 || len([]rune(input.NameEnUS)) > 80 {
		return nil, bizerr.NewCode(CodeServerBatchInvalid)
	}
	now, err := databaseNow(ctx, s.management.directory.database)
	if err != nil {
		return nil, wrapBatchUnavailable(err)
	}
	classificationNo := strings.TrimSpace(input.ClassificationNo)
	input.ClassificationNo = classificationNo
	auditKey := input.RequestNo
	if classificationNo == "" {
		if len(input.RequestNo) < 16 || len(input.RequestNo) > 128 {
			return nil, bizerr.NewCode(CodeServerBatchInvalid)
		}
		classificationNo, _ = deterministicReference("scl_", struct {
			Actor int
			Key   string
		}{actor, input.RequestNo})
	} else {
		if !validReference(classificationNo, 40) || input.ExpectedVersion <= 0 {
			return nil, bizerr.NewCode(CodeServerBatchInvalid)
		}
		auditKey = classificationNo + ":" + strconv.FormatInt(input.ExpectedVersion, 10)
	}
	err = s.management.directory.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var before *ServerClassification
		if input.ClassificationNo != "" {
			var loadErr error
			before, loadErr = s.loadClassification(ctx, classificationNo)
			if loadErr != nil {
				return loadErr
			}
		}
		if input.ClassificationNo == "" {
			_, err = s.management.directory.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_classification
(tenant_id,classification_no,name_zh_cn,name_en_us,status_code,version,created_by,updated_by,created_at,updated_at)
VALUES (0,?,?,?,'active',1,?,?,?,?) ON CONFLICT (classification_no) DO NOTHING`, classificationNo,
				input.NameZhCN, input.NameEnUS, actor, actor, now, now)
			if err == nil {
				var persisted *struct{ NameZhCN, NameEnUS string }
				err = s.management.directory.database.GetScan(ctx, &persisted, `SELECT name_zh_cn,name_en_us
FROM plugin_idc_core_server_classification WHERE tenant_id=0 AND classification_no=?`, classificationNo)
				if err == nil && (persisted == nil || persisted.NameZhCN != input.NameZhCN || persisted.NameEnUS != input.NameEnUS) {
					return bizerr.NewCode(CodeServerBatchConflict)
				}
			}
		} else {
			result, updateErr := s.management.directory.database.Exec(ctx, `UPDATE plugin_idc_core_server_classification
SET name_zh_cn=?,name_en_us=?,version=version+1,updated_by=?,updated_at=?
WHERE tenant_id=0 AND classification_no=? AND version=?`, input.NameZhCN, input.NameEnUS,
				actor, now, classificationNo, input.ExpectedVersion)
			if updateErr != nil {
				return updateErr
			}
			affected, countErr := result.RowsAffected()
			if countErr != nil || affected != 1 {
				return bizerr.NewCode(CodeServerBatchConflict)
			}
		}
		if err != nil {
			return err
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"classification": before},
			IdempotencyScope: "server.classification.save", IdempotencyKey: auditKey,
			OperationCode: "server.classification.save", TargetType: "server_classification",
			TargetNo: classificationNo, ResultCode: "succeeded", ReasonCode: "staff_server_classification",
			RiskCode: "low", RequiredPermission: PermissionServerBatchSafe,
			AfterSnapshot: map[string]any{"name_zh_cn": input.NameZhCN, "name_en_us": input.NameEnUS},
		})
	})
	if err != nil {
		return nil, wrapBatchUnavailable(err)
	}
	return s.loadClassification(ctx, classificationNo)
}

func (s *batchManagementImpl) DisableClassification(ctx context.Context,
	classificationNo string, expectedVersion int64) (*ServerClassification, error) {
	actor, grants, err := s.management.authorize(ctx, []string{PermissionServerBatchSafe})
	if err != nil || actor <= 0 || !grants[PermissionServerBatchSafe] ||
		!validReference(classificationNo, 40) || expectedVersion <= 0 {
		return nil, bizerr.NewCode(CodeServerBatchNotAccessible)
	}
	err = s.management.directory.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, updateErr := s.management.directory.database.Exec(ctx, `UPDATE plugin_idc_core_server_classification
SET status_code='disabled',version=version+1,updated_by=?,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND classification_no=? AND version=? AND status_code='active'`, actor,
			classificationNo, expectedVersion)
		if updateErr != nil {
			return updateErr
		}
		affected, countErr := result.RowsAffected()
		if countErr != nil || affected != 1 {
			return bizerr.NewCode(CodeServerBatchConflict)
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"status": "active", "version": expectedVersion},
			IdempotencyScope: "server.classification.disable",
			IdempotencyKey:   classificationNo + ":" + strconv.FormatInt(expectedVersion, 10),
			OperationCode:    "server.classification.disable", TargetType: "server_classification",
			TargetNo: classificationNo, ResultCode: "succeeded", ReasonCode: "staff_server_classification",
			RiskCode: "low", RequiredPermission: PermissionServerBatchSafe,
			AfterSnapshot: map[string]any{"status": "disabled"},
		})
	})
	if err != nil {
		return nil, wrapBatchUnavailable(err)
	}
	return s.loadClassification(ctx, classificationNo)
}

func (s *batchManagementImpl) loadClassification(ctx context.Context, classificationNo string) (*ServerClassification, error) {
	var value *ServerClassification
	if err := s.management.directory.database.GetScan(ctx, &value, `SELECT classification_no,
name_zh_cn,name_en_us,status_code AS status,version FROM plugin_idc_core_server_classification
WHERE tenant_id=0 AND classification_no=?`, classificationNo); err != nil || value == nil {
		return nil, bizerr.NewCode(CodeServerBatchUnavailable)
	}
	return value, nil
}
