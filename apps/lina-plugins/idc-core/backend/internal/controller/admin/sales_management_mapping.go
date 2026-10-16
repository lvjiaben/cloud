// This file converts sales owner records into stable JSON response models.
package admin

import (
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
	"time"
)

func salesManagementLevel(item salesmanagement.Level) v1.SalesManagementLevel {
	return v1.SalesManagementLevel{PercentageBasisPoints: item.PercentageBasisPoints, PricingVersion: item.PricingVersion, LevelNo: item.LevelNo, Name: item.Name, EnglishName: item.EnglishName, Status: item.Status, Version: item.Version}
}
func salesManagementAgent(item salesmanagement.Agent) v1.SalesManagementAgent {
	return v1.SalesManagementAgent{CustomerNo: item.CustomerNo, CustomerCode: item.CustomerCode, CustomerName: item.CustomerName, LevelNo: item.LevelNo, LevelName: item.LevelName, Status: item.Status, ExpiresAt: salesManagementMillis(item.ExpiresAt), Version: item.Version}
}
func salesManagementPrice(item salesmanagement.PriceRule) v1.SalesManagementPrice {
	return v1.SalesManagementPrice{PriceNo: item.PriceNo, CustomerNo: item.CustomerNo, CustomerCode: item.CustomerCode, CustomerName: item.CustomerName, LevelNo: item.LevelNo, LevelName: item.LevelName, ProductNo: item.ProductNo, ProductName: item.ProductName, CurrencyCode: item.CurrencyCode, ChargeTypeCode: item.ChargeTypeCode, PeriodUnitCode: item.PeriodUnitCode, AdjustmentType: item.AdjustmentType, Status: item.Status, FixedPriceMinor: item.FixedPriceMinor, PeriodCount: item.PeriodCount, DiscountBasisPoints: item.DiscountBasisPoints, Version: item.Version}
}
func salesManagementOwnership(item salesmanagement.Ownership) v1.SalesManagementOwnership {
	return v1.SalesManagementOwnership{CustomerNo: item.CustomerNo, CustomerName: item.CustomerName, CustomerCode: item.CustomerCode, ReferrerCustomerNo: item.ReferrerCustomerNo, ReferrerName: item.ReferrerName, ReferrerCode: item.ReferrerCode, StaffName: item.StaffName, StaffUserID: item.StaffUserID, Version: item.Version}
}
func salesManagementRule(item salesmanagement.CommissionRule) v1.SalesManagementRule {
	return v1.SalesManagementRule{RuleNo: item.RuleNo, BeneficiaryType: item.BeneficiaryType, OperationType: item.OperationType, ProductNo: item.ProductNo, ProductName: item.ProductName, CurrencyCode: item.CurrencyCode, CalculationType: item.CalculationType, FixedAmountMinor: item.FixedAmountMinor, Status: item.Status, RateBasisPoints: item.RateBasisPoints, MaturityDays: item.MaturityDays, Version: item.Version}
}
func salesManagementTime(value *int64) *time.Time {
	if value == nil {
		return nil
	}
	t := time.UnixMilli(*value)
	return &t
}
func salesManagementMillis(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	t := value.UnixMilli()
	return &t
}

func salesManagementCommissionAccount(item salesmanagement.CommissionAccount) v1.SalesManagementCommissionAccount {
	return v1.SalesManagementCommissionAccount{AccountNo: item.AccountNo, BeneficiaryType: item.BeneficiaryType, BeneficiaryReference: item.BeneficiaryReference, BeneficiaryName: item.BeneficiaryName, CurrencyCode: item.CurrencyCode, CreditedMinor: item.CreditedMinor, PendingMinor: item.PendingMinor, AvailableMinor: item.AvailableMinor, PaidMinor: item.PaidMinor, ReversedMinor: item.ReversedMinor, RecoveryDueMinor: item.RecoveryDueMinor, Version: item.Version}
}

func salesManagementCommissionRecord(item salesmanagement.CommissionRecord) v1.SalesManagementCommissionRecord {
	return v1.SalesManagementCommissionRecord{CommissionNo: item.CommissionNo, InvoiceNo: item.InvoiceNo, OrderNo: item.OrderNo, CustomerName: item.CustomerName, BeneficiaryType: item.BeneficiaryType, BeneficiaryReference: item.BeneficiaryReference, BeneficiaryName: item.BeneficiaryName, CurrencyCode: item.CurrencyCode, Status: item.Status, BasisMinor: item.BasisMinor, OriginalMinor: item.OriginalMinor, PendingMinor: item.PendingMinor, MaturedMinor: item.MaturedMinor, ReversedMinor: item.ReversedMinor, MaturesAt: salesManagementMillis(item.MaturesAt)}
}

func salesManagementCommissionPayout(item salesmanagement.CommissionPayout) v1.SalesManagementCommissionPayout {
	return v1.SalesManagementCommissionPayout{PayoutNo: item.PayoutNo, AccountNo: item.AccountNo, BeneficiaryName: item.BeneficiaryName, CurrencyCode: item.CurrencyCode, AmountMinor: item.AmountMinor, EvidenceReference: item.EvidenceReference, PaidAt: salesManagementMillis(item.PaidAt), CreatedAt: salesManagementMillis(item.CreatedAt), RecordedBy: item.RecordedBy}
}
