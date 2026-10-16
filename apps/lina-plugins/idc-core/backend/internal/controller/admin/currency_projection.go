// This file projects the internal currency setting into its non-sensitive HTTP DTO.
package admin

import (
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/currency"
)

func projectCurrency(item currency.Item) v1.CurrencyItem {
	return v1.CurrencyItem{CurrencyCode: item.CurrencyCode, Enabled: item.Enabled, IsDefault: item.IsDefault, Version: item.Version}
}
