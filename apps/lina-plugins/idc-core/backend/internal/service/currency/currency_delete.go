// This file removes currency admission without deleting any product or financial record.
package currency

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"strconv"
)

const auditDeleteOperation = "merchant.currency.delete"

func (s *serviceImpl) Delete(ctx context.Context, code string, version int64) error {
	actor, err := s.authorize(ctx, permissionWrite)
	if err != nil {
		return err
	}
	if !validCode(code) || version < 1 {
		return bizerr.NewCode(codeInvalid)
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		merchant, err := s.lockMerchant(ctx)
		if err != nil {
			return err
		}
		var row *entity.MerchantCurrency
		if err = s.database.Model(dao.MerchantCurrency.Table()).Ctx(ctx).Where(do.MerchantCurrency{TenantId: 0, CurrencyCode: code}).LockUpdate().Scan(&row); err != nil {
			return err
		}
		if row == nil || row.Deleted || row.Version != version {
			return bizerr.NewCode(codeConflict)
		}
		if merchant.DefaultCurrency == code {
			return bizerr.NewCode(codeDefault)
		}
		if _, err = s.database.Model(dao.MerchantCurrency.Table()).Ctx(ctx).Where(do.MerchantCurrency{TenantId: 0, CurrencyCode: code, Version: version}).Data(do.MerchantCurrency{Deleted: true, Enabled: false, Version: version + 1, UpdatedBy: actor}).Update(); err != nil {
			return err
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: auditScope, IdempotencyKey: code + ":" + strconv.FormatInt(version+1, 10), OperationCode: auditDeleteOperation, TargetType: auditTarget, TargetNo: code, ResultCode: idccap.BusinessAuditResultSucceeded, RiskCode: auditRisk, RequiredPermission: permissionWrite, BeforeSnapshot: Item{CurrencyCode: code, Enabled: row.Enabled, Version: version}, AfterSnapshot: map[string]any{"currencyCode": code, "deleted": true, "version": version + 1}})
	})
	return safeError(err)
}
