// This file projects bounded currency configuration without exposing accounts or clients.
package currency

import (
	"context"
	"golang.org/x/text/currency"
	"lina-core/pkg/bizerr"
	"strings"
)

const (
	permissionRead    = "idc:admin:currency:read"
	permissionWrite   = "idc:admin:currency:write"
	maximumCurrencies = 300
)

func validCode(code string) bool {
	if len(code) != 3 || code != strings.ToUpper(code) {
		return false
	}
	unit, err := currency.ParseISO(code)
	return err == nil && unit != currency.XXX
}
func (s *reader) Enabled(ctx context.Context) ([]string, error) {
	rows, err := s.database.GetAll(ctx, `SELECT c.currency_code FROM plugin_idc_core_merchant_currency c JOIN plugin_idc_core_merchant_profile m ON m.tenant_id=c.tenant_id AND m.enabled WHERE c.tenant_id=0 AND c.enabled AND NOT c.deleted ORDER BY (c.currency_code=m.default_currency) DESC,c.currency_code LIMIT 300`)
	if err != nil {
		return nil, safeError(err)
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, row["currency_code"].String())
	}
	return result, nil
}
func (s *reader) IsEnabled(ctx context.Context, code string) (bool, error) {
	if !validCode(code) {
		return false, nil
	}
	result, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_merchant_currency c JOIN plugin_idc_core_merchant_profile m ON m.tenant_id=c.tenant_id AND m.enabled WHERE c.tenant_id=0 AND c.currency_code=? AND c.enabled AND NOT c.deleted)`, code)
	if err != nil {
		return false, safeError(err)
	}
	return result.Bool(), nil
}
func (s *serviceImpl) List(ctx context.Context, page, size int) (Page, error) {
	if _, err := s.authorize(ctx, permissionRead); err != nil {
		return Page{}, err
	}
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 20
	}
	if page < 1 || page > 1000 || size < 1 || size > 100 {
		return Page{}, bizerr.NewCode(codeInvalid)
	}
	rows, err := s.database.GetAll(ctx, `SELECT c.currency_code,c.enabled,c.version,(c.currency_code=m.default_currency) AS is_default,COUNT(*) OVER() AS total FROM plugin_idc_core_merchant_currency c JOIN plugin_idc_core_merchant_profile m ON m.tenant_id=c.tenant_id WHERE c.tenant_id=0 AND NOT c.deleted ORDER BY (c.currency_code=m.default_currency) DESC,c.currency_code LIMIT ? OFFSET ?`, size, (page-1)*size)
	result := Page{Items: []Item{}}
	if err != nil {
		return result, safeError(err)
	}
	for _, row := range rows {
		result.Total = row["total"].Int()
		result.Items = append(result.Items, Item{CurrencyCode: row["currency_code"].String(), Enabled: row["enabled"].Bool(), IsDefault: row["is_default"].Bool(), Version: row["version"].Int64()})
	}
	if len(rows) == 0 {
		v, countErr := s.database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_merchant_currency WHERE tenant_id=0 AND NOT deleted`)
		if countErr != nil {
			return result, safeError(countErr)
		}
		result.Total = v.Int()
	}
	return result, nil
}

func (s *reader) IsRemoved(ctx context.Context, code string) (bool, error) {
	value, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_merchant_currency WHERE tenant_id=0 AND currency_code=? AND deleted)`, code)
	if err != nil {
		return false, safeError(err)
	}
	return value.Bool(), nil
}
