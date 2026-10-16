// This file serializes currency updates with the merchant lock and preserves all historical money.
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

const (
	auditScope     = "merchant.currency"
	auditOperation = "merchant.currency.configure"
	auditTarget    = "merchant_currency"
	auditRisk      = "medium"
)

func (s *serviceImpl) Create(ctx context.Context, code string) (Item, error) {
	actor, err := s.authorize(ctx, permissionWrite)
	if err != nil {
		return Item{}, err
	}
	if !validCode(code) {
		return Item{}, bizerr.NewCode(codeInvalid)
	}
	result := Item{CurrencyCode: code, Enabled: true, Version: 1}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, err := s.lockMerchant(ctx); err != nil {
			return err
		}
		var existing *entity.MerchantCurrency
		if err := s.database.Model(dao.MerchantCurrency.Table()).Ctx(ctx).Where(do.MerchantCurrency{TenantId: 0, CurrencyCode: code}).Scan(&existing); err != nil {
			return err
		}
		if existing != nil && !existing.Deleted {
			return bizerr.NewCode(codeConflict)
		}
		if existing != nil {
			result.Version = existing.Version + 1
		}
		count, err := s.database.Model(dao.MerchantCurrency.Table()).Ctx(ctx).Where(do.MerchantCurrency{TenantId: 0, Deleted: false}).Count()
		if err != nil {
			return err
		}
		if count >= maximumCurrencies {
			return bizerr.NewCode(codeInvalid)
		}
		if existing == nil {
			_, err = s.database.Model(dao.MerchantCurrency.Table()).Ctx(ctx).Data(do.MerchantCurrency{TenantId: 0, CurrencyCode: code, Enabled: true, Version: result.Version, CreatedBy: actor, UpdatedBy: actor}).Insert()
		} else {
			_, err = s.database.Model(dao.MerchantCurrency.Table()).Ctx(ctx).Where(do.MerchantCurrency{TenantId: 0, CurrencyCode: code}).Data(do.MerchantCurrency{Deleted: false, Enabled: true, Version: result.Version, UpdatedBy: actor}).Update()
		}
		if err != nil {
			return err
		}
		return s.auditChange(ctx, map[string]any{}, result)
	})
	return result, safeError(err)
}
func (s *serviceImpl) Update(ctx context.Context, code string, version int64, enabled, makeDefault bool) (Item, error) {
	actor, err := s.authorize(ctx, permissionWrite)
	if err != nil {
		return Item{}, err
	}
	if !validCode(code) || version < 1 || (makeDefault && !enabled) {
		return Item{}, bizerr.NewCode(codeInvalid)
	}
	var result Item
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
		if merchant.DefaultCurrency == code && !enabled {
			return bizerr.NewCode(codeDefault)
		}
		before := Item{CurrencyCode: code, Enabled: row.Enabled, IsDefault: merchant.DefaultCurrency == code, Version: version}
		result = Item{CurrencyCode: code, Enabled: enabled, IsDefault: makeDefault || before.IsDefault, Version: version + 1}
		if _, err = s.database.Model(dao.MerchantCurrency.Table()).Ctx(ctx).Where(do.MerchantCurrency{TenantId: 0, CurrencyCode: code, Version: version}).Data(do.MerchantCurrency{Enabled: enabled, Version: version + 1, UpdatedBy: actor}).Update(); err != nil {
			return err
		}
		if makeDefault && merchant.DefaultCurrency != code {
			if _, err = s.database.Model(dao.MerchantProfile.Table()).Ctx(ctx).Where(do.MerchantProfile{TenantId: 0, Version: merchant.Version}).Data(do.MerchantProfile{DefaultCurrency: code, Version: merchant.Version + 1, UpdatedBy: actor}).Update(); err != nil {
				return err
			}
		}
		return s.auditChange(ctx, before, result)
	})
	return result, safeError(err)
}
func (s *serviceImpl) lockMerchant(ctx context.Context) (*entity.MerchantProfile, error) {
	var row *entity.MerchantProfile
	err := s.database.Model(dao.MerchantProfile.Table()).Ctx(ctx).Where(do.MerchantProfile{TenantId: 0, Enabled: true}).LockUpdate().Scan(&row)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, bizerr.NewCode(codeUnavailable)
	}
	return row, nil
}
func (s *serviceImpl) auditChange(ctx context.Context, before any, after Item) error {
	return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: auditScope, IdempotencyKey: after.CurrencyCode + ":" + strconv.FormatInt(after.Version, 10), OperationCode: auditOperation, TargetType: auditTarget, TargetNo: after.CurrencyCode, ResultCode: idccap.BusinessAuditResultSucceeded, RiskCode: auditRisk, RequiredPermission: permissionWrite, BeforeSnapshot: before, AfterSnapshot: after})
}
