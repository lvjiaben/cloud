// This file manages fixed-size traffic packages using immutable accessory price
// versions. Inventory is a sale ceiling, not a server-resource reservation;
// existing addon intents are its authority and historical versions never change.

package serveroperation

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/util/guid"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// TrafficPackageSearch is a bounded numeric page; keyword searches names only.
type TrafficPackageSearch struct {
	Page, PageSize int
	Keyword        string
}

// TrafficPackageInput freezes capacity, final tax-inclusive price and product
// scope. A nil StockQuantity explicitly means unlimited; zero means sold out.
type TrafficPackageInput struct {
	PackageNo                        string
	ExpectedVersion                  int64
	NameZhCN, NameEnUS, CurrencyCode string
	TrafficGB                        int
	AmountMinor                      int64
	Enabled                          bool
	StockQuantity                    *int64
	ProductNos                       []string
}

// TrafficPackage contains sale facts, never upstream credentials or customer IDs.
type TrafficPackage struct {
	PackageNo         string    `json:"packageNo" orm:"policy_no"`
	Version           int64     `json:"version" orm:"policy_version"`
	NameZhCN          string    `json:"nameZhCN" orm:"name_zh_cn"`
	NameEnUS          string    `json:"nameEnUS" orm:"name_en_us"`
	CurrencyCode      string    `json:"currencyCode" orm:"currency_code"`
	TrafficGB         int       `json:"trafficGb" orm:"traffic_gb"`
	AmountMinor       int64     `json:"amountMinor" orm:"unit_amount_minor"`
	Enabled           bool      `json:"enabled" orm:"enabled"`
	StockQuantity     *int64    `json:"stockQuantity" orm:"stock_quantity"`
	OccupiedQuantity  int64     `json:"occupiedQuantity" orm:"occupied_quantity"`
	RemainingQuantity *int64    `json:"remainingQuantity"`
	ProductNos        []string  `json:"productNos"`
	PublishedAt       time.Time `json:"-" orm:"published_at"`
}

// TrafficPackagePage keeps the total independent of current-page row count.
type TrafficPackagePage struct {
	Items []TrafficPackage `json:"items"`
	Total int              `json:"total"`
}

const trafficPackageProjection = `policy.policy_no,policy.currency_code,policy.enabled,policy.stock_quantity,
version.policy_version,version.name_zh_cn,version.name_en_us,version.traffic_gb,version.unit_amount_minor,version.published_at`

// trafficPackageModel only uses server-operation-owned policy and scope tables.
func (s *accessoryPricingImpl) trafficPackageModel(ctx context.Context) *gdb.Model {
	return s.database.Model(dao.ServerAccessoryPricePolicy.Table()+" policy").Ctx(ctx).
		InnerJoin(dao.ServerAccessoryPricePolicyVersion.Table()+" version", "version.policy_no=policy.policy_no AND version.policy_version=policy.current_version").
		Where("policy.tenant_id", 0).Where("policy.accessory_kind", serveroperationresourcecap.AccessoryKindTraffic).
		WhereGT("version.traffic_gb", 0)
}

// normalizeTrafficSearch bounds offset arithmetic and avoids fake page totals.
func normalizeTrafficSearch(input TrafficPackageSearch) (TrafficPackageSearch, error) {
	input.Keyword = strings.TrimSpace(input.Keyword)
	if input.Page == 0 {
		input.Page = 1
	}
	if input.PageSize == 0 {
		input.PageSize = 20
	}
	if input.Page < 1 || input.PageSize < 1 || input.PageSize > 100 || input.Page-1 > math.MaxInt/input.PageSize || utf8.RuneCountInString(input.Keyword) > 120 {
		return input, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	return input, nil
}

// ListTrafficPackages authenticates staff before any policy or occupancy query.
func (s *accessoryPricingImpl) ListTrafficPackages(ctx context.Context, input TrafficPackageSearch) (*TrafficPackagePage, error) {
	if _, err := s.authorize(ctx, PermissionAccessoryPriceRead); err != nil {
		return nil, err
	}
	return s.listTrafficPackages(ctx, input, "", "")
}

// listTrafficPackages uses a repeatable snapshot, two page queries and two
// bounded batch projections, independent of the number of returned packages.
func (s *accessoryPricingImpl) listTrafficPackages(ctx context.Context, input TrafficPackageSearch, serviceNo, customerNo string) (*TrafficPackagePage, error) {
	input, err := normalizeTrafficSearch(input)
	if err != nil {
		return nil, err
	}
	result := &TrafficPackagePage{Items: []TrafficPackage{}}
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, err := s.database.Exec(txCtx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); err != nil {
			return err
		}
		query := s.trafficPackageModel(txCtx)
		if input.Keyword != "" {
			query = query.Where("(version.name_zh_cn ILIKE ? OR version.name_en_us ILIKE ?)", "%"+input.Keyword+"%", "%"+input.Keyword+"%")
		}
		if serviceNo != "" {
			query = query.Where("policy.enabled", true).Where(`EXISTS (SELECT 1 FROM plugin_idc_core_traffic_package_product scope
JOIN plugin_idc_core_service_instance service ON service.product_no=scope.product_no AND service.tenant_id=scope.tenant_id
WHERE scope.policy_no=policy.policy_no AND scope.policy_version=policy.current_version AND scope.tenant_id=0
 AND service.service_no=? AND service.customer_no=? AND service.currency_code=policy.currency_code)`, serviceNo, customerNo)
		}
		var err error
		result.Total, err = query.Clone().Count()
		if err != nil {
			return err
		}
		if err = query.Clone().Fields(trafficPackageProjection).OrderDesc("policy.updated_at").OrderDesc("policy.policy_no").Page(input.Page, input.PageSize).Scan(&result.Items); err != nil {
			return err
		}
		return s.fillTrafficPackageFacts(txCtx, result.Items)
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAccessoryPriceUnavailable)
	}
	return result, nil
}

// fillTrafficPackageFacts aggregates only this page, with no per-row queries.
func (s *accessoryPricingImpl) fillTrafficPackageFacts(ctx context.Context, items []TrafficPackage) error {
	if len(items) == 0 {
		return nil
	}
	numbers := make([]string, len(items))
	for i := range items {
		numbers[i] = items[i].PackageNo
		items[i].ProductNos = []string{}
	}
	var scopes []entity.TrafficPackageProduct
	if err := s.database.Model(dao.TrafficPackageProduct.Table()).Ctx(ctx).Where(do.TrafficPackageProduct{TenantId: 0}).WhereIn("policy_no", numbers).
		Where("EXISTS (SELECT 1 FROM plugin_idc_core_server_accessory_price_policy policy WHERE policy.policy_no=plugin_idc_core_traffic_package_product.policy_no AND policy.current_version=plugin_idc_core_traffic_package_product.policy_version)").Scan(&scopes); err != nil {
		return err
	}
	var occupancy []struct {
		PolicyNo string
		Quantity int64
	}
	if err := s.trafficPackageOccupancy(ctx, numbers).Scan(&occupancy); err != nil {
		return err
	}
	byNumber := make(map[string]*TrafficPackage, len(items))
	for i := range items {
		byNumber[items[i].PackageNo] = &items[i]
	}
	for _, scope := range scopes {
		byNumber[scope.PolicyNo].ProductNos = append(byNumber[scope.PolicyNo].ProductNos, scope.ProductNo)
	}
	for _, row := range occupancy {
		byNumber[row.PolicyNo].OccupiedQuantity = row.Quantity
	}
	for i := range items {
		sort.Strings(items[i].ProductNos)
		if items[i].StockQuantity != nil {
			remaining := max(int64(0), *items[i].StockQuantity-items[i].OccupiedQuantity)
			items[i].RemainingQuantity = &remaining
		}
	}
	return nil
}

// trafficPackageOccupancy counts every created intent except an authority-
// confirmed unpaid cancellation. Paid, failed and unknown executions retain use.
func (s *accessoryPricingImpl) trafficPackageOccupancy(ctx context.Context, numbers []string) *gdb.Model {
	return s.database.Model(dao.ServerAccessoryQuote.Table()+" quote").Ctx(ctx).
		InnerJoin(dao.BusinessOperation.Table()+" operation", "operation.tenant_id=quote.tenant_id AND operation.quote_no=quote.quote_no").
		Where("quote.tenant_id", 0).WhereIn("quote.policy_no", numbers).
		Where("operation.operation_type_code", "addon").WhereNot("operation.status_code", "cancelled").
		Fields("quote.policy_no,COUNT(*) AS quantity").Group("quote.policy_no")
}

// TrafficPackageProducts delegates catalog visibility and names to their owner.
func (s *accessoryPricingImpl) TrafficPackageProducts(ctx context.Context, keyword string, numbers []string) ([]catalog.TrafficPackageProduct, error) {
	if _, err := s.authorize(ctx, PermissionAccessoryPriceRead); err != nil {
		return nil, err
	}
	if s.catalog == nil {
		return nil, bizerr.NewCode(CodeAccessoryPriceUnavailable)
	}
	return s.catalog.TrafficPackageProducts(ctx, keyword, numbers)
}

// SaveTrafficPackage preserves all earlier versions and serializes edits with
// checkouts. Reducing the ceiling never cancels existing purchases.
func (s *accessoryPricingImpl) SaveTrafficPackage(ctx context.Context, input TrafficPackageInput) (*TrafficPackage, error) {
	actor, err := s.authorize(ctx, PermissionAccessoryPriceWrite)
	if err != nil {
		return nil, err
	}
	if err = validateTrafficPackageInput(input); err != nil {
		return nil, err
	}
	if s.catalog == nil {
		return nil, bizerr.NewCode(CodeAccessoryPriceUnavailable)
	}
	products, err := s.catalog.TrafficPackageProducts(ctx, "", input.ProductNos)
	if err != nil {
		return nil, err
	}
	if len(products) != len(input.ProductNos) {
		return nil, bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	if input.PackageNo == "" {
		input.PackageNo = "tpk_" + guid.S()
	}
	sort.Strings(input.ProductNos)
	var result *TrafficPackage
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var current *entity.ServerAccessoryPricePolicy
		if err := s.database.Model(dao.ServerAccessoryPricePolicy.Table()).Ctx(txCtx).
			Where(do.ServerAccessoryPricePolicy{TenantId: 0, PolicyNo: input.PackageNo}).LockUpdate().Scan(&current); err != nil {
			return err
		}
		if (current == nil && input.ExpectedVersion != 0) || (current != nil && (current.CurrentVersion != input.ExpectedVersion || current.AccessoryKind != string(serveroperationresourcecap.AccessoryKindTraffic) || current.CurrencyCode != input.CurrencyCode)) {
			return bizerr.NewCode(CodeAccessoryPriceConflict)
		}
		now, err := databaseNow(txCtx, s.database)
		if err != nil {
			return err
		}
		version := input.ExpectedVersion + 1
		if current == nil {
			if _, err = s.database.Model(dao.ServerAccessoryPricePolicy.Table()).Ctx(txCtx).Data(do.ServerAccessoryPricePolicy{TenantId: 0, PolicyNo: input.PackageNo, AccessoryKind: serveroperationresourcecap.AccessoryKindTraffic, CurrencyCode: input.CurrencyCode, CurrentVersion: version, Enabled: input.Enabled, StockQuantity: input.StockQuantity}).Insert(); err != nil {
				return err
			}
		}
		digest, err := digestValue(input)
		if err != nil {
			return err
		}
		if _, err = s.database.Model(dao.ServerAccessoryPricePolicyVersion.Table()).Ctx(txCtx).Data(do.ServerAccessoryPricePolicyVersion{TenantId: 0, PolicyNo: input.PackageNo, PolicyVersion: version, AccessoryKind: serveroperationresourcecap.AccessoryKindTraffic, CurrencyCode: input.CurrencyCode, FreeCountPerServer: 0, UnitAmountMinor: input.AmountMinor, PeriodUnitCode: "once", PeriodCount: 1, ContentDigest: digest, PublishedBy: actor, PublishedAt: &now, TrafficGb: input.TrafficGB, NameZhCn: input.NameZhCN, NameEnUs: input.NameEnUS}).Insert(); err != nil {
			return err
		}
		scopes := make([]do.TrafficPackageProduct, len(input.ProductNos))
		for i, number := range input.ProductNos {
			scopes[i] = do.TrafficPackageProduct{TenantId: 0, PolicyNo: input.PackageNo, PolicyVersion: version, ProductNo: number}
		}
		if _, err = s.database.Model(dao.TrafficPackageProduct.Table()).Ctx(txCtx).Data(scopes).Insert(); err != nil {
			return err
		}
		if current != nil {
			updated, updateErr := s.database.Model(dao.ServerAccessoryPricePolicy.Table()).Ctx(txCtx).
				Where(do.ServerAccessoryPricePolicy{TenantId: 0, PolicyNo: input.PackageNo, CurrentVersion: input.ExpectedVersion}).
				Data(do.ServerAccessoryPricePolicy{CurrentVersion: version, Enabled: input.Enabled, StockQuantity: input.StockQuantity}).Update()
			if updateErr != nil {
				return updateErr
			}
			count, countErr := updated.RowsAffected()
			if countErr != nil {
				return countErr
			}
			if count != 1 {
				return bizerr.NewCode(CodeAccessoryPriceConflict)
			}
			// The generated pointer field is omitted when nil; the explicit ORM
			// null setter clears only this column within the same locked transaction.
			if input.StockQuantity == nil {
				if _, err = s.database.Model(dao.ServerAccessoryPricePolicy.Table()).Ctx(txCtx).
					Where(do.ServerAccessoryPricePolicy{TenantId: 0, PolicyNo: input.PackageNo, CurrentVersion: version}).
					Data("stock_quantity", gdb.Raw("NULL")).Update(); err != nil {
					return err
				}
			}
		}
		if err = s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{IdempotencyScope: "server.accessory.price.publish", IdempotencyKey: input.PackageNo + ":" + strconv.FormatInt(version, 10), OperationCode: "server.accessory.price.publish", TargetType: "server_accessory_price", TargetNo: input.PackageNo, ResultCode: "published", ReasonCode: "staff_price_publication", RiskCode: "high", RequiredPermission: PermissionAccessoryPriceWrite, BeforeSnapshot: map[string]any{"version": input.ExpectedVersion}, AfterSnapshot: input}); err != nil {
			return err
		}
		result = &TrafficPackage{PackageNo: input.PackageNo, Version: version, NameZhCN: input.NameZhCN, NameEnUS: input.NameEnUS, CurrencyCode: input.CurrencyCode, TrafficGB: input.TrafficGB, AmountMinor: input.AmountMinor, Enabled: input.Enabled, StockQuantity: input.StockQuantity, ProductNos: input.ProductNos, PublishedAt: now}
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeAccessoryPriceUnavailable)
	}
	return result, nil
}

// validateTrafficPackageInput checks immutable price and a unique bounded scope.
func validateTrafficPackageInput(input TrafficPackageInput) error {
	if (input.PackageNo != "" && !validReference(input.PackageNo, 40)) || input.ExpectedVersion < 0 || input.ExpectedVersion == math.MaxInt64 || (input.PackageNo == "" && input.ExpectedVersion != 0) || strings.TrimSpace(input.NameZhCN) == "" || strings.TrimSpace(input.NameEnUS) == "" || utf8.RuneCountInString(input.NameZhCN) > 120 || utf8.RuneCountInString(input.NameEnUS) > 120 || input.TrafficGB < 1 || input.TrafficGB > 1000000 || input.AmountMinor < 0 || input.AmountMinor > 1000000000 || (input.StockQuantity != nil && (*input.StockQuantity < 0 || *input.StockQuantity > 1000000000)) || len(input.ProductNos) < 1 || len(input.ProductNos) > 100 {
		return bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	if !validAccessoryPriceInput(AccessoryPricePolicyInput{Kind: serveroperationresourcecap.AccessoryKindTraffic, CurrencyCode: input.CurrencyCode, PeriodUnitCode: "once", PeriodCount: 1}) {
		return bizerr.NewCode(CodeAccessoryPriceInvalid)
	}
	seen := make(map[string]bool, len(input.ProductNos))
	for _, number := range input.ProductNos {
		if !validReference(number, 40) || seen[number] {
			return bizerr.NewCode(CodeAccessoryPriceInvalid)
		}
		seen[number] = true
	}
	return nil
}
