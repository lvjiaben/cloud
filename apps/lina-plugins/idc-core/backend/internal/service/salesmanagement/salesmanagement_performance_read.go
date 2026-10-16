// This file aggregates immutable payment ownership in SQL before pagination.
// Refunds are collapsed per invoice before joining owners, preventing fan-out.
package salesmanagement

import (
	"context"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
)

func (s *serviceImpl) ListPerformance(ctx context.Context, input PerformanceInput) (PerformancePage, error) {
	result := PerformancePage{Items: []Performance{}}
	if _, err := s.authorize(ctx, permissionOverview, false); err != nil {
		return result, err
	}
	page, err := normalizePage(input.PageInput)
	if err != nil {
		return result, err
	}
	input.PageInput = page
	input.CurrencyCode = strings.ToUpper(strings.TrimSpace(input.CurrencyCode))
	if input.BeneficiaryType != "" && input.BeneficiaryType != beneficiarySales && input.BeneficiaryType != beneficiaryAffiliate {
		return result, bizerr.NewCode(codeInvalid)
	}
	if input.CurrencyCode != "" && (len(input.CurrencyCode) != 3 || strings.IndexFunc(input.CurrencyCode, func(r rune) bool { return r < 'A' || r > 'Z' }) >= 0) {
		return result, bizerr.NewCode(codeInvalid)
	}
	merchant, err := s.database.Model(dao.MerchantProfile.Table()).Ctx(ctx).Where(do.MerchantProfile{TenantId: 0, Enabled: true}).Fields("timezone").One()
	if err != nil {
		return result, managementError(err)
	}
	if merchant.IsEmpty() {
		return result, bizerr.NewCode(codeUnavailable)
	}
	location, err := time.LoadLocation(merchant["timezone"].String())
	if err != nil {
		return result, managementError(err)
	}
	start, err := time.ParseInLocation("2006-01", input.Month, location)
	if err != nil || start.Format("2006-01") != input.Month {
		return result, bizerr.NewCode(codeInvalid)
	}
	now := time.Now().UTC()
	result.Timezone, result.AsOf = location.String(), now.UnixMilli()
	where, args, err := s.performanceOwnerFilter(ctx, input)
	if err != nil {
		return result, err
	}
	// Both role rows intentionally receive the same payment; they are distinct
	// dimensions and are never combined into a single merchant revenue total.
	cte := `WITH payments AS (
 SELECT f.tenant_id,f.invoice_no,f.order_no,f.currency_code,f.operation_type_code,f.received_amount_minor FROM ` + dao.SalesPaymentFact.Table() + ` f WHERE f.tenant_id=0 AND f.received_at>=? AND f.received_at<?
 ), refunds AS (
 SELECT r.invoice_no,r.currency_code,SUM(CASE WHEN r.direction_code='apply' THEN r.amount_minor ELSE -r.amount_minor END) AS amount
 FROM ` + dao.BillingRefundAllocation.Table() + ` r INNER JOIN payments p ON p.invoice_no=r.invoice_no AND p.currency_code=r.currency_code
 WHERE r.tenant_id=0 AND r.created_at<=? GROUP BY r.invoice_no,r.currency_code
 ), performance AS (
 SELECT o.beneficiary_type_code,o.beneficiary_reference,p.currency_code,
 COUNT(DISTINCT CASE WHEN p.operation_type_code='new_purchase' THEN p.order_no END) AS paid_order_count,
 COUNT(DISTINCT CASE WHEN p.operation_type_code='renewal' THEN p.invoice_no END) AS paid_renewal_count,
 SUM(p.received_amount_minor) AS received_minor,SUM(COALESCE(r.amount,0)) AS refunded_minor,
 SUM(p.received_amount_minor-COALESCE(r.amount,0)) AS net_received_minor
 FROM payments p INNER JOIN ` + dao.SalesPaymentOwner.Table() + ` o ON o.tenant_id=p.tenant_id AND o.invoice_no=p.invoice_no
 LEFT JOIN refunds r ON r.invoice_no=p.invoice_no AND r.currency_code=p.currency_code
 WHERE ` + where + ` GROUP BY o.beneficiary_type_code,o.beneficiary_reference,p.currency_code
 ) `
	params := append([]any{start, start.AddDate(0, 1, 0), now}, args...)
	total, err := s.database.GetValue(ctx, cte+"SELECT COUNT(*) FROM performance", params...)
	if err != nil {
		return result, managementError(err)
	}
	rows, err := s.database.GetAll(ctx, cte+"SELECT beneficiary_type_code,beneficiary_reference,currency_code,paid_order_count,paid_renewal_count,received_minor,refunded_minor,net_received_minor FROM performance ORDER BY beneficiary_type_code,beneficiary_reference,currency_code LIMIT ? OFFSET ?", append(params, input.PageSize, (input.Page-1)*input.PageSize)...)
	if err != nil {
		return result, managementError(err)
	}
	owners := map[string]string{}
	for _, row := range rows {
		owners[row["beneficiary_type_code"].String()+":"+row["beneficiary_reference"].String()] = row["beneficiary_reference"].String()
	}
	names, err := s.commissionBeneficiaryNames(ctx, owners)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		kind, reference := row["beneficiary_type_code"].String(), row["beneficiary_reference"].String()
		result.Items = append(result.Items, Performance{BeneficiaryType: kind, BeneficiaryReference: reference, BeneficiaryName: names[kind+":"+reference], CurrencyCode: row["currency_code"].String(), PaidOrderCount: row["paid_order_count"].Int64(), PaidRenewalCount: row["paid_renewal_count"].Int64(), ReceivedMinor: row["received_minor"].String(), RefundedMinor: row["refunded_minor"].String(), NetReceivedMinor: row["net_received_minor"].String()})
	}
	result.Total = total.Int()
	return result, nil
}

func (s *serviceImpl) performanceOwnerFilter(ctx context.Context, input PerformanceInput) (string, []any, error) {
	where := "o.tenant_id=0"
	args := []any{}
	if input.BeneficiaryType != "" {
		where += " AND o.beneficiary_type_code=?"
		args = append(args, input.BeneficiaryType)
	}
	if input.CurrencyCode != "" {
		where += " AND p.currency_code=?"
		args = append(args, input.CurrencyCode)
	}
	if input.Search == "" {
		return where, args, nil
	}
	condition := `(o.beneficiary_type_code=? AND o.beneficiary_reference IN (SELECT customer_no FROM ` + dao.CustomerSubject.Table() + ` WHERE tenant_id=0 AND display_name LIKE ?))`
	args = append(args, beneficiaryAffiliate, "%"+input.Search+"%")
	if input.BeneficiaryType != beneficiaryAffiliate {
		users, err := s.users.List(ctx, usercap.ListInput{Keyword: input.Search, TenantID: capmodel.DomainID("0"), Page: capmodel.PageRequest{PageNum: 1, PageSize: 100}})
		if err != nil {
			return "", nil, managementError(err)
		}
		if users != nil {
			if users.Total > 100 {
				return "", nil, bizerr.NewCode(codeStaffSearchBroad)
			}
			ids := []string{}
			for _, user := range users.Items {
				if user != nil {
					ids = append(ids, string(user.ID))
				}
			}
			if len(ids) > 0 {
				condition += " OR (o.beneficiary_type_code=? AND o.beneficiary_reference IN (?))"
				args = append(args, beneficiarySales, ids)
			}
		}
	}
	return where + " AND (" + condition + ")", args, nil
}
