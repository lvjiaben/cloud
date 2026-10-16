// This file projects bounded commission pages and resolves beneficiary labels
// with two batch calls at most, rather than querying per returned record.
package salesmanagement

import (
	"context"
	"strconv"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const (
	permissionCommissionView   = "idc:admin:sales-channel:commission:view"
	permissionCommissionPayout = "idc:admin:sales-channel:commission:payout"
	beneficiarySales           = "sales"
	beneficiaryAffiliate       = "affiliate"
)

func (s *serviceImpl) commissionPage(ctx context.Context, input PageInput, table, searchColumn string) (*gdb.Model, PageInput, int, error) {
	if _, err := s.authorize(ctx, permissionCommissionView, false); err != nil {
		return nil, input, 0, err
	}
	input, err := normalizePage(input)
	if err != nil {
		return nil, input, 0, err
	}
	query := s.database.Model(table).Ctx(ctx).Where("tenant_id", 0)
	if input.Search != "" {
		switch table {
		case dao.SalesCommissionAccount.Table():
			staff, lookupErr := s.users.List(ctx, usercap.ListInput{Keyword: input.Search, TenantID: capmodel.DomainID("0"), Page: capmodel.PageRequest{PageNum: 1, PageSize: 100}})
			if lookupErr != nil {
				return nil, input, 0, managementError(lookupErr)
			}
			staffIDs := []string{}
			if staff != nil {
				if staff.Total > 100 {
					return nil, input, 0, bizerr.NewCode(codeStaffSearchBroad)
				}
				for _, user := range staff.Items {
					if user != nil {
						staffIDs = append(staffIDs, string(user.ID))
					}
				}
			}
			condition := query.Builder().WhereLike("currency_code", "%"+input.Search+"%").WhereOr("beneficiary_type_code=? AND beneficiary_reference IN (SELECT customer_no FROM plugin_idc_core_customer_subject WHERE tenant_id=0 AND display_name LIKE ?)", beneficiaryAffiliate, "%"+input.Search+"%")
			if len(staffIDs) > 0 {
				condition = condition.WhereOr(query.Builder().Where("beneficiary_type_code", beneficiarySales).WhereIn("beneficiary_reference", staffIDs))
			}
			query = query.Where(condition)
		case dao.CommissionAccrual.Table():
			query = query.Where("order_no LIKE ? OR invoice_no LIKE ?", "%"+input.Search+"%", "%"+input.Search+"%")
		default:
			query = query.WhereLike(searchColumn, "%"+input.Search+"%")
		}
	}
	total, err := query.Clone().Count()
	return query.OrderDesc("id").Page(input.Page, input.PageSize), input, total, managementError(err)
}
func (s *serviceImpl) ListCommissionAccounts(ctx context.Context, input PageInput) (CommissionAccountPage, error) {
	result := CommissionAccountPage{Items: []CommissionAccount{}}
	query, _, total, err := s.commissionPage(ctx, input, dao.SalesCommissionAccount.Table(), "beneficiary_reference")
	if err != nil {
		return result, err
	}
	var rows []entity.SalesCommissionAccount
	if err = query.Scan(&rows); err != nil {
		return result, managementError(err)
	}
	owners := map[string]string{}
	accountNos := make([]string, 0, len(rows))
	for _, row := range rows {
		owners[row.BeneficiaryTypeCode+":"+row.BeneficiaryReference] = row.BeneficiaryReference
		accountNos = append(accountNos, row.AccountNo)
	}
	recoveries, err := s.commissionRecoveryDue(ctx, accountNos)
	if err != nil {
		return result, managementError(err)
	}
	names, err := s.commissionBeneficiaryNames(ctx, owners)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		result.Items = append(result.Items, CommissionAccount{AccountNo: row.AccountNo, BeneficiaryType: row.BeneficiaryTypeCode, BeneficiaryReference: row.BeneficiaryReference, BeneficiaryName: names[row.BeneficiaryTypeCode+":"+row.BeneficiaryReference], CurrencyCode: row.CurrencyCode, CreditedMinor: minorString(row.CreditedAmountMinor), PendingMinor: minorString(row.PendingAmountMinor), AvailableMinor: minorString(row.AvailableAmountMinor), PaidMinor: minorString(row.PaidAmountMinor), ReversedMinor: minorString(row.ReversedAmountMinor), RecoveryDueMinor: minorString(recoveries[row.AccountNo]), Version: row.Version})
	}
	result.Total = total
	return result, nil
}
func (s *serviceImpl) ListCommissionRecords(ctx context.Context, input PageInput) (CommissionRecordPage, error) {
	result := CommissionRecordPage{Items: []CommissionRecord{}}
	query, _, total, err := s.commissionPage(ctx, input, dao.CommissionAccrual.Table(), "order_no")
	if err != nil {
		return result, err
	}
	var rows []entity.CommissionAccrual
	if err = query.Scan(&rows); err != nil {
		return result, managementError(err)
	}
	owners := map[string]string{}
	for _, row := range rows {
		owners[row.BeneficiaryTypeCode+":"+row.BeneficiaryReference] = row.BeneficiaryReference
	}
	names, err := s.commissionBeneficiaryNames(ctx, owners)
	if err != nil {
		return result, err
	}
	invoiceNos := []string{}
	for _, row := range rows {
		invoiceNos = append(invoiceNos, row.InvoiceNo)
	}
	customerNames := map[string]string{}
	if len(invoiceNos) > 0 {
		facts, lookupErr := s.database.Model(dao.BillingInvoice.Table()+" i").Ctx(ctx).InnerJoin(dao.CustomerSubject.Table()+" c", "c.tenant_id=i.tenant_id AND c.customer_no=i.customer_no").Where("i.tenant_id", 0).WhereIn("i.invoice_no", invoiceNos).Fields("i.invoice_no,c.display_name").All()
		if lookupErr != nil {
			return result, managementError(lookupErr)
		}
		for _, fact := range facts {
			customerNames[fact["invoice_no"].String()] = fact["display_name"].String()
		}
	}
	for _, row := range rows {
		result.Items = append(result.Items, CommissionRecord{CustomerName: customerNames[row.InvoiceNo], CommissionNo: row.CommissionNo, OrderNo: row.OrderNo, InvoiceNo: row.InvoiceNo, BeneficiaryType: row.BeneficiaryTypeCode, BeneficiaryReference: row.BeneficiaryReference, BeneficiaryName: names[row.BeneficiaryTypeCode+":"+row.BeneficiaryReference], CurrencyCode: row.CurrencyCode, Status: row.StatusCode, BasisMinor: minorString(row.BasisAmountMinor), OriginalMinor: minorString(row.OriginalAmountMinor), PendingMinor: minorString(row.PendingAmountMinor), MaturedMinor: minorString(row.MaturedAmountMinor), ReversedMinor: minorString(row.ReversedAmountMinor), MaturesAt: row.MaturesAt})
	}
	result.Total = total
	return result, nil
}
func (s *serviceImpl) ListCommissionPayouts(ctx context.Context, input PageInput) (CommissionPayoutPage, error) {
	result := CommissionPayoutPage{Items: []CommissionPayout{}}
	query, _, total, err := s.commissionPage(ctx, input, dao.SalesCommissionOfflinePayout.Table(), "evidence_reference")
	if err != nil {
		return result, err
	}
	var rows []entity.SalesCommissionOfflinePayout
	if err = query.Scan(&rows); err != nil {
		return result, managementError(err)
	}
	accountNos := []string{}
	for _, row := range rows {
		accountNos = append(accountNos, row.AccountNo)
	}
	accounts := map[string]entity.SalesCommissionAccount{}
	owners := map[string]string{}
	if len(accountNos) > 0 {
		var found []entity.SalesCommissionAccount
		if err = s.database.Model(dao.SalesCommissionAccount.Table()).Ctx(ctx).Where(do.SalesCommissionAccount{TenantId: 0}).WhereIn("account_no", accountNos).Fields("account_no,beneficiary_type_code,beneficiary_reference").Scan(&found); err != nil {
			return result, managementError(err)
		}
		for _, a := range found {
			accounts[a.AccountNo] = a
			owners[a.BeneficiaryTypeCode+":"+a.BeneficiaryReference] = a.BeneficiaryReference
		}
	}
	names, err := s.commissionBeneficiaryNames(ctx, owners)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		a := accounts[row.AccountNo]
		item := payoutProjection(row)
		item.BeneficiaryName = names[a.BeneficiaryTypeCode+":"+a.BeneficiaryReference]
		result.Items = append(result.Items, item)
	}
	result.Total = total
	return result, nil
}
func (s *serviceImpl) commissionBeneficiaryNames(ctx context.Context, owners map[string]string) (map[string]string, error) {
	names := map[string]string{}
	staff := []usercap.UserID{}
	customers := []string{}
	for key, value := range owners {
		names[key] = ""
		if len(key) > 6 && key[:6] == beneficiarySales+":" {
			staff = append(staff, usercap.UserID(value))
		} else {
			customers = append(customers, value)
		}
	}
	if len(staff) > 0 {
		users, err := s.users.BatchGet(ctx, staff)
		if err != nil {
			return nil, managementError(err)
		}
		if users != nil {
			for _, user := range users.Items {
				if user == nil {
					continue
				}
				name := user.Nickname
				if name == "" {
					name = user.Username
				}
				names[beneficiarySales+":"+string(user.ID)] = name
			}
		}
	}
	if len(customers) > 0 {
		var rows []entity.CustomerSubject
		if err := s.database.Model(dao.CustomerSubject.Table()).Ctx(ctx).Where(do.CustomerSubject{TenantId: 0}).WhereIn("customer_no", customers).Fields("customer_no,display_name").Scan(&rows); err != nil {
			return nil, managementError(err)
		}
		for _, row := range rows {
			names[beneficiaryAffiliate+":"+row.CustomerNo] = row.DisplayName
			names["agent:"+row.CustomerNo] = row.DisplayName
		}
	}
	return names, nil
}
func minorString(amount int64) string { return strconv.FormatInt(amount, 10) }

// commissionRecoveryDue aggregates only the requested account page. Payouts
// call it while holding the same account lock used by refund and offset writers.
func (s *serviceImpl) commissionRecoveryDue(ctx context.Context, accountNos []string) (map[string]int64, error) {
	result := make(map[string]int64, len(accountNos))
	if len(accountNos) == 0 {
		return result, nil
	}
	var rows []struct {
		AccountNo string `orm:"account_no"`
		Amount    int64  `orm:"amount"`
	}
	if err := s.database.Model(dao.SalesCommissionRecovery.Table()).Ctx(ctx).
		Where(do.SalesCommissionRecovery{TenantId: 0}).WhereIn("account_no", accountNos).
		Fields("account_no,SUM(amount_minor) AS amount").Group("account_no").Scan(&rows); err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.AccountNo] = row.Amount
	}
	rows = nil
	if err := s.database.Model(dao.SalesCommissionRecoveryOffset.Table()).Ctx(ctx).
		Where(do.SalesCommissionRecoveryOffset{TenantId: 0}).WhereIn("account_no", accountNos).
		Fields("account_no,SUM(amount_minor) AS amount").Group("account_no").Scan(&rows); err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.AccountNo] -= row.Amount
	}
	return result, nil
}

func payoutProjection(row entity.SalesCommissionOfflinePayout) CommissionPayout {
	return CommissionPayout{PayoutNo: row.PayoutNo, AccountNo: row.AccountNo, CurrencyCode: row.CurrencyCode, AmountMinor: minorString(row.AmountMinor), EvidenceReference: row.EvidenceReference, PaidAt: row.PaidAt, CreatedAt: row.CreatedAt, RecordedBy: row.RecordedBy}
}
