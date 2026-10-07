package saleschannel

import (
	"context"
	"sort"

	"github.com/gogf/gf/v2/errors/gerror"
)

// LoadOverview reads the small set of counters needed by the sales workbench.
// It deliberately uses one aggregate query, so the page does not perform a
// query for every card or currency.
func (s *serviceImpl) LoadOverview(ctx context.Context) (Overview, error) {
	if s.database == nil {
		return Overview{}, gerror.New("sales channel database is unavailable")
	}
	if ctx == nil {
		return Overview{}, gerror.New("sales channel overview context is required")
	}
	rows, err := s.database.GetAll(ctx, `
SELECT 'qualifications' AS code, COUNT(*)::bigint AS count, NULL::text AS currency_code, 0::bigint AS amount
  FROM plugin_idc_core_sales_agent_qualification WHERE tenant_id=0 AND status_code='active'
UNION ALL SELECT 'affiliate_links', COUNT(*)::bigint, NULL::text, 0::bigint
  FROM plugin_idc_core_sales_affiliate_link WHERE tenant_id=0 AND status_code='active'
UNION ALL SELECT 'attributions', COUNT(*)::bigint, NULL::text, 0::bigint
  FROM plugin_idc_core_sales_affiliate_attribution WHERE tenant_id=0 AND status_code='eligible'
UNION ALL SELECT 'pending_maturity', COUNT(*)::bigint, NULL::text, 0::bigint
  FROM plugin_idc_core_commission_accrual
 WHERE tenant_id=0 AND status_code IN ('pending_maturity','partially_reversed')
UNION ALL SELECT 'pending_settlement', COUNT(*)::bigint, NULL::text, 0::bigint
  FROM plugin_idc_core_sales_commission_settlement WHERE tenant_id=0 AND status_code IN ('draft','pending_review','approved','manual_review')
UNION ALL SELECT 'accrued', COUNT(*)::bigint, currency_code, COALESCE(SUM(delta_credited_minor),0)::bigint
  FROM plugin_idc_core_sales_commission_account_entry WHERE tenant_id=0 AND entry_type_code='accrual' GROUP BY currency_code
UNION ALL SELECT 'matured', COUNT(*)::bigint, currency_code, COALESCE(SUM(delta_available_minor),0)::bigint
  FROM plugin_idc_core_sales_commission_account_entry WHERE tenant_id=0 AND entry_type_code='maturity' GROUP BY currency_code`,
	)
	if err != nil {
		return Overview{}, gerror.Wrap(err, "load sales channel overview")
	}
	overview := Overview{Currencies: make([]CurrencyOverview, 0)}
	currencyIndex := make(map[string]int)
	for _, row := range rows {
		code := row["code"].String()
		count := row["count"].Int64()
		currency := row["currency_code"].String()
		amount := row["amount"].Int64()
		switch code {
		case "qualifications":
			overview.Qualifications = count
		case "affiliate_links":
			overview.AffiliateLinks = count
		case "attributions":
			overview.Attributions = count
		case "pending_maturity":
			overview.PendingMaturity = count
		case "pending_settlement":
			overview.PendingSettlement = count
		case "accrued", "matured":
			idx, ok := currencyIndex[currency]
			if !ok {
				idx = len(overview.Currencies)
				currencyIndex[currency] = idx
				overview.Currencies = append(overview.Currencies, CurrencyOverview{CurrencyCode: currency})
			}
			if code == "accrued" {
				overview.Currencies[idx].AccruedMinor = amount
			} else {
				overview.Currencies[idx].MaturedMinor = amount
			}
		}
	}
	sort.Slice(overview.Currencies, func(i, j int) bool {
		return overview.Currencies[i].CurrencyCode < overview.Currencies[j].CurrencyCode
	})
	return overview, nil
}
