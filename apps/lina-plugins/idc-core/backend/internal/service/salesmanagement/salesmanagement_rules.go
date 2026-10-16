// This file publishes immutable commission rule revisions. Financial accrual
// consumes the stored rule; the editor never accepts payment or income facts.
package salesmanagement

import (
	"context"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/util/guid"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// CommissionRule contains one immutable financial rule revision.
type CommissionRule struct {
	RuleNo, BeneficiaryType, OperationType, ProductNo, ProductName, CurrencyCode, CalculationType, FixedAmountMinor, Status string
	RateBasisPoints                                                                                                         *int
	MaturityDays                                                                                                            int
	Version                                                                                                                 int64
}

// CommissionRuleInput appends a new revision to one exact rule scope.
type CommissionRuleInput struct {
	RuleNo, BeneficiaryType, OperationType, ProductNo, CurrencyCode, CalculationType, FixedAmountMinor, Status string
	RateBasisPoints                                                                                            *int
	MaturityDays                                                                                               int
	ExpectedVersion                                                                                            int64
}

// CommissionRulePage is the current revision page.
type CommissionRulePage struct {
	Items []CommissionRule
	Total int
}

func (s *serviceImpl) ListRules(ctx context.Context, input PageInput) (CommissionRulePage, error) {
	result := CommissionRulePage{Items: []CommissionRule{}}
	if _, err := s.authorize(ctx, permissionOverview, false); err != nil {
		return result, err
	}
	input, err := normalizePage(input)
	if err != nil {
		return result, err
	}
	query := s.database.Model(dao.SalesCommissionRule.Table()+" r").Ctx(ctx).LeftJoin(dao.CatalogProduct.Table()+" p", "p.tenant_id=r.tenant_id AND p.product_no=r.product_no").Where("r.tenant_id", 0).Where("NOT EXISTS (SELECT 1 FROM plugin_idc_core_sales_commission_rule n WHERE n.tenant_id=r.tenant_id AND n.beneficiary_type_code=r.beneficiary_type_code AND n.operation_type_code=r.operation_type_code AND n.currency_code=r.currency_code AND COALESCE(n.product_no,'')=COALESCE(r.product_no,'') AND n.version>r.version)")
	if input.Search != "" {
		query = query.Where("p.name_zh_cn LIKE ? OR r.currency_code LIKE ?", "%"+input.Search+"%", "%"+input.Search+"%")
	}
	result.Total, err = query.Clone().Count()
	if err != nil {
		return result, managementError(err)
	}
	rows, err := query.Fields("r.*,p.name_zh_cn AS product_name").OrderDesc("r.id").Page(input.Page, input.PageSize).All()
	if err != nil {
		return result, managementError(err)
	}
	for _, row := range rows {
		var rule entity.SalesCommissionRule
		if err = row.Struct(&rule); err != nil {
			return result, managementError(err)
		}
		item := commissionRuleProjection(rule)
		item.ProductName = row["product_name"].String()
		result.Items = append(result.Items, item)
	}
	return result, nil
}
func (s *serviceImpl) SaveRule(ctx context.Context, input CommissionRuleInput) (CommissionRule, error) {
	if _, err := s.authorize(ctx, permissionCommission, false); err != nil {
		return CommissionRule{}, err
	}
	fixed, err := validateCommissionRuleInput(input)
	if err != nil {
		return CommissionRule{}, err
	}
	var result CommissionRule
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		// Serializing on the single merchant row also protects first-version creation.
		merchant, err := s.database.Model(dao.MerchantProfile.Table()).Ctx(ctx).Where(do.MerchantProfile{TenantId: 0}).LockUpdate().One()
		if err != nil {
			return err
		}
		if len(merchant) == 0 {
			return bizerr.NewCode(codeInvalid)
		}
		if input.ProductNo != "" {
			count, err := s.database.Model(dao.CatalogProduct.Table()).Ctx(ctx).Where(do.CatalogProduct{TenantId: 0, ProductNo: input.ProductNo}).Count()
			if err != nil {
				return err
			}
			if count != 1 {
				return bizerr.NewCode(codeInvalid)
			}
		}
		query := s.database.Model(dao.SalesCommissionRule.Table()).Ctx(ctx).Where(do.SalesCommissionRule{TenantId: 0, BeneficiaryTypeCode: input.BeneficiaryType, OperationTypeCode: input.OperationType, CurrencyCode: input.CurrencyCode})
		if input.ProductNo != "" {
			query = query.Where("product_no", input.ProductNo)
		} else {
			query = query.WhereNull("product_no")
		}
		var previous *entity.SalesCommissionRule
		if err := query.OrderDesc("version").Limit(1).Scan(&previous); err != nil {
			return err
		}
		version := int64(1)
		if previous != nil {
			if previous.Version != input.ExpectedVersion || previous.RuleNo != input.RuleNo {
				return bizerr.NewCode(codeConflict)
			}
			version = previous.Version + 1
		} else if input.ExpectedVersion != 0 || input.RuleNo != "" {
			return bizerr.NewCode(codeConflict)
		}
		now := time.Now().UTC()
		status := "published"
		if input.Status == statusInactive {
			status = "retired"
		}
		result = CommissionRule{RuleNo: "SCR_" + guid.S(), BeneficiaryType: input.BeneficiaryType, OperationType: input.OperationType, ProductNo: input.ProductNo, CurrencyCode: input.CurrencyCode, CalculationType: input.CalculationType, FixedAmountMinor: input.FixedAmountMinor, RateBasisPoints: input.RateBasisPoints, MaturityDays: input.MaturityDays, Status: input.Status, Version: version}
		digest, err := snapshotDigest(result)
		if err != nil {
			return err
		}
		data := do.SalesCommissionRule{TenantId: 0, RuleNo: result.RuleNo, BeneficiaryTypeCode: input.BeneficiaryType, OperationTypeCode: input.OperationType, CurrencyCode: input.CurrencyCode, CalculationTypeCode: input.CalculationType, FixedAmountMinor: fixed, RateBasisPoints: input.RateBasisPoints, MaturityDays: input.MaturityDays, StatusCode: status, EffectiveAt: &now, Version: version, RuleDigest: digest}
		if input.ProductNo != "" {
			data.ProductNo = &input.ProductNo
		}
		if _, err = s.database.Model(dao.SalesCommissionRule.Table()).Ctx(ctx).Data(data).Insert(); err != nil {
			return err
		}
		return s.auditChange(ctx, permissionCommission, "rule", result.RuleNo, "", version, previous, result)
	})
	return result, managementError(err)
}
func commissionRuleProjection(row entity.SalesCommissionRule) CommissionRule {
	result := CommissionRule{RuleNo: row.RuleNo, BeneficiaryType: row.BeneficiaryTypeCode, OperationType: row.OperationTypeCode, CurrencyCode: row.CurrencyCode, CalculationType: row.CalculationTypeCode, RateBasisPoints: row.RateBasisPoints, MaturityDays: row.MaturityDays, Version: row.Version, Status: statusActive}
	if row.StatusCode != "published" {
		result.Status = statusInactive
	}
	if row.ProductNo != nil {
		result.ProductNo = *row.ProductNo
	}
	if row.FixedAmountMinor != nil {
		result.FixedAmountMinor = strconv.FormatInt(*row.FixedAmountMinor, 10)
	}
	return result
}

func validateCommissionRuleInput(input CommissionRuleInput) (*int64, error) {
	if (input.BeneficiaryType != "affiliate" && input.BeneficiaryType != "sales") || (input.OperationType != "new_purchase" && input.OperationType != "renewal") || len(input.CurrencyCode) != 3 || input.MaturityDays < 0 || input.MaturityDays > 3650 || (input.Status != statusActive && input.Status != statusInactive) {
		return nil, bizerr.NewCode(codeInvalid)
	}
	var fixed *int64
	if input.CalculationType == "fixed" {
		value, err := strconv.ParseInt(input.FixedAmountMinor, 10, 64)
		if err != nil || value < 0 || input.RateBasisPoints != nil {
			return nil, bizerr.NewCode(codeInvalid)
		}
		fixed = &value
	} else if input.CalculationType != "rate" || input.RateBasisPoints == nil || *input.RateBasisPoints < 0 || *input.RateBasisPoints > 10000 || input.FixedAmountMinor != "" {
		return nil, bizerr.NewCode(codeInvalid)
	}
	return fixed, nil
}
