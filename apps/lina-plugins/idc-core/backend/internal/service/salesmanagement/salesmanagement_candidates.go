// This file loads bounded product-period and staff candidates without exposing
// host storage or requiring operators to enter internal identifiers.
package salesmanagement

import (
	"context"
	"strconv"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

// ProductTerm is one explicitly priced period and currency. Pricing preparation
// may read drafts; sales overview remains limited to published products.
type ProductTerm struct {
	CurrencyCode, ChargeTypeCode, PeriodUnitCode string
	PeriodCount                                  int
}

// ProductCandidate includes its bounded billing choices, avoiding row-by-row HTTP.
type ProductCandidate struct {
	ProductNo, Name, EnglishName string
	Terms                        []ProductTerm
}

// ProductPage is a bounded catalog selector page.
type ProductPage struct {
	Currencies []string
	Items      []ProductCandidate
	Total      int
}

// StaffCandidate contains only public staff display data.
type StaffCandidate struct {
	UserID     int
	Name       string
	Selectable bool
}

// StaffPage is a bounded host-governed candidate page.
type StaffPage struct {
	Items []StaffCandidate
	Total int
}

func (s *serviceImpl) ProductCandidates(ctx context.Context, input PageInput) (ProductPage, error) {
	return s.productCandidates(ctx, input, permissionOverview)
}

// PriceProductCandidates provides exact product lookup without requiring sales overview access.
func (s *serviceImpl) PriceProductCandidates(ctx context.Context, input PageInput) (ProductPage, error) {
	return s.productCandidates(ctx, input, permissionPricing)
}

func (s *serviceImpl) productCandidates(ctx context.Context, input PageInput, permission string) (ProductPage, error) {
	result := ProductPage{Items: []ProductCandidate{}, Currencies: []string{}}
	if _, err := s.authorize(ctx, permission, false); err != nil {
		return result, err
	}
	input, err := normalizePage(input)
	if err != nil {
		return result, err
	}
	currencyRows, currencyErr := s.database.GetAll(ctx, `SELECT default_currency AS currency_code FROM plugin_idc_core_merchant_profile WHERE tenant_id=0 AND enabled=true UNION SELECT currency_code FROM plugin_idc_core_catalog_price WHERE tenant_id=0 AND status_code='active' ORDER BY currency_code LIMIT 100`)
	if currencyErr != nil {
		return result, managementError(currencyErr)
	}
	for _, row := range currencyRows {
		result.Currencies = append(result.Currencies, row["currency_code"].String())
	}
	query := s.database.Model(dao.CatalogProduct.Table()).Ctx(ctx).Where(do.CatalogProduct{TenantId: 0})
	versionProjection := "current_version_no"
	if permission == permissionPricing {
		query = query.WhereIn("status_code", []string{"draft", statusActive})
		versionProjection = "COALESCE(current_version_no,(SELECT v.version_no FROM plugin_idc_core_catalog_product_version v WHERE v.tenant_id=0 AND v.product_no=plugin_idc_core_catalog_product.product_no AND v.status_code='draft' ORDER BY v.version_number DESC LIMIT 1))"
		query = query.Where(versionProjection + " IS NOT NULL")
	} else {
		query = query.Where(do.CatalogProduct{StatusCode: statusActive}).WhereNotNull("current_version_no")
	}
	if input.ProductNo != "" {
		query = query.Where(do.CatalogProduct{ProductNo: input.ProductNo})
	}
	if input.Search != "" {
		query = query.Where("name_zh_cn LIKE ? OR name_en_us LIKE ?", "%"+input.Search+"%", "%"+input.Search+"%")
	}
	result.Total, err = query.Clone().Count()
	if err != nil {
		return result, managementError(err)
	}
	rows, err := query.Fields("product_no,name_zh_cn,name_en_us,"+versionProjection+" AS current_version_no").OrderAsc("sort_order").OrderDesc("id").Page(input.Page, input.PageSize).All()
	if err != nil {
		return result, managementError(err)
	}
	versions := []string{}
	for _, row := range rows {
		versions = append(versions, row["current_version_no"].String())
	}
	terms := map[string][]ProductTerm{}
	if len(versions) > 0 {
		choices, err := s.database.Model(dao.CatalogBillingRule.Table()+" b").Ctx(ctx).InnerJoin(dao.CatalogPrice.Table()+" p", "p.tenant_id=b.tenant_id AND p.version_no=b.version_no AND p.billing_rule_no=b.billing_rule_no").Where("b.tenant_id", 0).Where("b.status_code", statusActive).Where("p.status_code", statusActive).WhereNull("p.option_no").WhereNull("p.configuration_price_dimension_no").WhereIn("b.version_no", versions).Fields("DISTINCT b.version_no,b.charge_type_code,b.period_unit_code,b.period_count,p.currency_code").Limit(4000).All()
		if err != nil {
			return result, managementError(err)
		}
		if len(choices) >= 4000 {
			return result, bizerr.NewCode(codeInvalid)
		}
		for _, row := range choices {
			key := row["version_no"].String()
			terms[key] = append(terms[key], ProductTerm{CurrencyCode: row["currency_code"].String(), ChargeTypeCode: row["charge_type_code"].String(), PeriodUnitCode: row["period_unit_code"].String(), PeriodCount: row["period_count"].Int()})
		}
	}
	for _, row := range rows {
		values := terms[row["current_version_no"].String()]
		if values == nil {
			values = []ProductTerm{}
		}
		result.Items = append(result.Items, ProductCandidate{ProductNo: row["product_no"].String(), Name: row["name_zh_cn"].String(), EnglishName: row["name_en_us"].String(), Terms: values})
	}
	return result, nil
}
func (s *serviceImpl) StaffCandidates(ctx context.Context, input PageInput) (StaffPage, error) {
	result := StaffPage{Items: []StaffCandidate{}}
	if _, err := s.authorize(ctx, permissionAssignment, false); err != nil {
		return result, err
	}
	input, err := normalizePage(input)
	if err != nil {
		return result, err
	}
	page, err := s.users.List(ctx, usercap.ListInput{Keyword: input.Search, EnabledOnly: true, TenantID: capmodel.DomainID("0"), Page: capmodel.PageRequest{PageNum: input.Page, PageSize: input.PageSize}})
	if err != nil {
		return result, managementError(err)
	}
	if page == nil {
		return result, nil
	}
	result.Total = page.Total
	users := page.Items
	ids := []int{}
	for _, user := range users {
		if user != nil {
			id, err := strconv.Atoi(string(user.ID))
			if err != nil {
				return result, managementError(err)
			}
			ids = append(ids, id)
		}
	}
	valid := map[int]bool{}
	if len(ids) > 0 {
		rows, err := s.database.Model(dao.PrincipalAudience.Table()).Ctx(ctx).Where(do.PrincipalAudience{TenantId: 0, AudienceCode: principal.AudienceStaff, StatusCode: statusActive}).WhereIn("sys_user_id", ids).Fields("sys_user_id").All()
		if err != nil {
			return result, managementError(err)
		}
		for _, row := range rows {
			valid[row["sys_user_id"].Int()] = true
		}
	}
	for _, user := range users {
		if user == nil {
			continue
		}
		id, err := strconv.Atoi(string(user.ID))
		if err != nil {
			return result, managementError(err)
		}
		name := user.Nickname
		if name == "" {
			name = user.Username
		}
		result.Items = append(result.Items, StaffCandidate{UserID: id, Name: name, Selectable: valid[id]})
	}
	return result, nil
}
