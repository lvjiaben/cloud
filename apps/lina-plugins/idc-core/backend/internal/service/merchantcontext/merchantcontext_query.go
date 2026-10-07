// This file performs the single bounded merchant-profile projection query.

package merchantcontext

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
)

// CurrentMerchant reads exactly one default-tenant profile with a bounded projection.
func (s *serviceImpl) CurrentMerchant(ctx context.Context) (idccap.MerchantContext, error) {
	if s == nil || nilDatabase(s.database) || ctx == nil || ctx.Err() != nil {
		return idccap.MerchantContext{}, gerror.New("IDC merchant context is unavailable")
	}
	var row struct {
		TenantID        int    `orm:"tenant_id"`
		MerchantNo      string `orm:"merchant_no"`
		DisplayName     string `orm:"display_name"`
		LegalName       string `orm:"legal_name"`
		CountryCode     string `orm:"country_code"`
		DefaultLocale   string `orm:"default_locale"`
		DefaultCurrency string `orm:"default_currency"`
		Timezone        string `orm:"timezone"`
		Enabled         bool   `orm:"enabled"`
		Version         int64  `orm:"version"`
	}
	err := s.database.Model(merchantProfileTable).
		Ctx(ctx).
		Fields(
			"tenant_id", "merchant_no", "display_name", "legal_name", "country_code",
			"default_locale", "default_currency", "timezone", "enabled", "version",
		).
		Where("tenant_id", idccap.DefaultTenantID).
		Limit(1).
		Scan(&row)
	if err != nil {
		return idccap.MerchantContext{}, gerror.Wrap(err, "read IDC merchant context")
	}
	current := idccap.MerchantContext{
		TenantID: row.TenantID, MerchantNo: row.MerchantNo,
		DisplayName: row.DisplayName, LegalName: row.LegalName,
		CountryCode: row.CountryCode, DefaultLocale: row.DefaultLocale,
		DefaultCurrency: row.DefaultCurrency, Timezone: row.Timezone,
		Enabled: row.Enabled, Version: row.Version,
	}
	if err = idccap.ValidateMerchantContext(current); err != nil {
		return idccap.MerchantContext{}, err
	}
	return current, nil
}
