// These fixtures verify wallet batching, financial facts and customer product isolation.

package customermanagement

import (
	"context"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/internal/dao"
)

const managementWorkspaceFixtureSQL = `
CREATE TABLE plugin_idc_core_billing_ledger_account (id BIGINT GENERATED ALWAYS AS IDENTITY,tenant_id INT,customer_no TEXT,account_no TEXT,account_type_code TEXT,currency_code TEXT,status_code TEXT,updated_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_core_billing_balance_snapshot (tenant_id INT,account_no TEXT,sequence_no BIGINT,closing_balance_minor BIGINT,created_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_core_billing_fund_hold (tenant_id INT,account_no TEXT,amount_minor BIGINT,consumed_amount_minor BIGINT,released_amount_minor BIGINT,status_code TEXT,expires_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_core_billing_payment_allocation(tenant_id INT,customer_no TEXT,currency_code TEXT,direction_code TEXT,amount_minor BIGINT,payment_source_type_code TEXT);
CREATE TABLE plugin_idc_core_billing_refund_allocation(tenant_id INT,customer_no TEXT,currency_code TEXT,direction_code TEXT,amount_minor BIGINT);
CREATE TABLE plugin_idc_core_billing_invoice(tenant_id INT,customer_no TEXT,currency_code TEXT,status_code TEXT,total_amount_minor BIGINT,paid_amount_minor BIGINT,waived_amount_minor BIGINT);
CREATE TABLE plugin_idc_core_support_ticket(tenant_id INT,customer_no TEXT,base_stage TEXT);
CREATE TABLE plugin_idc_core_service_instance(tenant_id INT,customer_no TEXT,service_no TEXT,order_item_no TEXT,product_type_code TEXT,customer_alias TEXT,customer_remark TEXT,period_unit_code TEXT,period_count INT,entitlement_expires_at TIMESTAMPTZ,entitlement_status_code TEXT,current_price_snapshot_no TEXT,configuration_snapshot_no TEXT);
CREATE TABLE plugin_idc_core_service_entitlement(tenant_id INT,customer_no TEXT,entitlement_no TEXT,order_item_no TEXT,entitlement_owner_id TEXT,plugin_id TEXT,period_unit_code TEXT,period_count INT,expires_at TIMESTAMPTZ,status_code TEXT,current_price_snapshot_no TEXT,configuration_snapshot_no TEXT);
CREATE TABLE plugin_idc_core_service_price_snapshot(tenant_id INT,price_snapshot_no TEXT,total_amount_minor BIGINT);
CREATE TABLE plugin_idc_core_commerce_order(tenant_id INT,customer_no TEXT,order_no TEXT,quote_no TEXT,created_at TIMESTAMPTZ,status_code TEXT);
CREATE TABLE plugin_idc_core_commerce_order_item(id BIGINT GENERATED ALWAYS AS IDENTITY,tenant_id INT,order_no TEXT,order_item_no TEXT,product_no TEXT,version_no TEXT,configuration_snapshot_no TEXT,product_name_zh_cn TEXT,product_name_en_us TEXT,total_amount_minor BIGINT,currency_code TEXT,fulfillment_mode_code TEXT,period_unit_code TEXT,period_count INT);
CREATE TABLE plugin_idc_core_commerce_quote_snapshot(tenant_id INT,customer_no TEXT,quote_no TEXT,purpose_code TEXT);
CREATE TABLE plugin_idc_core_catalog_product_version_fulfillment_binding(tenant_id INT,version_no TEXT,plugin_id TEXT);
CREATE TABLE plugin_idc_core_commerce_configuration_snapshot_item(id BIGINT GENERATED ALWAYS AS IDENTITY,tenant_id INT,configuration_snapshot_no TEXT,option_name_zh_cn TEXT,option_name_en_us TEXT,quantity INT,sort_order INT);
`

func TestCustomerWorkspaceWalletAndStatisticsPostgres(t *testing.T) {
	ctx := context.Background()
	configureManagementIntegrationDB(t, ctx)
	db := dao.CustomerSubject.DB()
	queries := []string{
		`INSERT INTO plugin_idc_core_billing_ledger_account(tenant_id,customer_no,account_no,account_type_code,currency_code,status_code,updated_at) VALUES(0,'visible','wallet-cny','balance','CNY','active',CURRENT_TIMESTAMP),(0,'visible','wallet-usd','balance','USD','active',CURRENT_TIMESTAMP),(0,'other','other-wallet','balance','CNY','active',CURRENT_TIMESTAMP)`,
		`INSERT INTO plugin_idc_core_billing_balance_snapshot VALUES(0,'wallet-cny',1,10000,CURRENT_TIMESTAMP),(0,'wallet-cny',2,20000,CURRENT_TIMESTAMP),(0,'wallet-usd',1,3000,CURRENT_TIMESTAMP),(0,'other-wallet',1,999999,CURRENT_TIMESTAMP)`,
		`INSERT INTO plugin_idc_core_billing_fund_hold VALUES(0,'wallet-cny',5000,1000,1000,'active',CURRENT_TIMESTAMP+INTERVAL '1 day'),(0,'wallet-cny',1000,0,0,'active',CURRENT_TIMESTAMP-INTERVAL '1 day')`,
		`INSERT INTO plugin_idc_core_billing_payment_allocation VALUES(0,'visible','CNY','apply',12000,'balance'),(0,'visible','USD','apply',5000,'channel_payment'),(0,'other','CNY','apply',999999,'balance')`,
		`INSERT INTO plugin_idc_core_billing_refund_allocation VALUES(0,'visible','CNY','apply',2000),(0,'visible','CNY','reversal',500)`,
		`INSERT INTO plugin_idc_core_billing_invoice VALUES(0,'visible','CNY','pending_payment',10000,2000,1000),(0,'visible','CNY','cancelled',90000,0,0)`,
		`INSERT INTO plugin_idc_core_support_ticket VALUES(0,'visible','new'),(0,'visible','in_progress'),(0,'visible','waiting_customer'),(0,'visible','closed'),(0,'other','closed')`,
		`INSERT INTO plugin_idc_core_service_instance(tenant_id,customer_no,entitlement_expires_at,entitlement_status_code) VALUES(0,'visible',CURRENT_TIMESTAMP+INTERVAL '10 days','active'),(0,'visible',CURRENT_TIMESTAMP-INTERVAL '1 day','active'),(0,'visible',CURRENT_TIMESTAMP-INTERVAL '2 days','terminated')`,
		`INSERT INTO plugin_idc_core_service_entitlement(tenant_id,customer_no,expires_at,status_code) VALUES(0,'visible',CURRENT_TIMESTAMP+INTERVAL '31 days','active')`,
	}
	for _, query := range queries {
		if _, err := db.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	wallets := map[string][]BalanceSummary{}
	if err := loadWalletProjection(ctx, []string{"visible"}, wallets); err != nil {
		t.Fatal(err)
	}
	if len(wallets) != 1 || len(wallets["visible"]) != 2 || wallets["visible"][0].AvailableBalanceMinor != 17000 || wallets["visible"][1].CurrencyCode != "USD" {
		t.Fatalf("wallets=%+v", wallets)
	}
	stats, err := loadOperationsStatistics(ctx, "visible")
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.Currencies) != 2 || stats.Currencies[0].PaidMinor != 12000 || stats.Currencies[0].RefundedMinor != 1500 || stats.Currencies[0].UnpaidMinor != 7000 || stats.Tickets.Total != 4 || stats.Tickets.Processing != 2 || stats.ProductCategories == nil || len(stats.ProductCategories) != 0 || stats.Products != 0 || stats.Expiring != 0 || stats.Expired != 0 {
		t.Fatalf("statistics=%+v", stats)
	}
}

// TestCustomerWorkspaceProductPagesPostgres covers generic products, pending purchases,
// immutable prices, renewal exclusion, exact totals and staff/customer isolation.
func TestCustomerWorkspaceProductPagesPostgres(t *testing.T) {
	ctx := context.Background()
	configureManagementIntegrationDB(t, ctx)
	customerNo := "CUS_11111111111111111111111111111111"
	otherNo := "CUS_22222222222222222222222222222222"
	insertManagementCustomer(t, ctx, customerNo, customer.TypeIndividual, customer.VerificationUnverified, "Products", time.Now())
	insertManagementCustomer(t, ctx, otherNo, customer.TypeIndividual, customer.VerificationUnverified, "Other", time.Now())
	db := dao.CustomerSubject.DB()
	queries := []string{
		`INSERT INTO plugin_idc_core_commerce_quote_snapshot VALUES(0,'` + customerNo + `','purchase','new_purchase'),(0,'` + customerNo + `','renew','renewal'),(0,'` + otherNo + `','other','new_purchase')`,
		`INSERT INTO plugin_idc_core_commerce_order VALUES(0,'` + customerNo + `','order','purchase',CURRENT_TIMESTAMP,'paid'),(0,'` + customerNo + `','renewal','renew',CURRENT_TIMESTAMP,'paid'),(0,'` + otherNo + `','other-order','other',CURRENT_TIMESTAMP,'paid')`,
		`INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding VALUES(0,'server-version','idc-resource'),(0,'ssl-version','yunsu-ssl')`,
		`INSERT INTO plugin_idc_core_commerce_order_item(tenant_id,order_no,order_item_no,product_no,version_no,configuration_snapshot_no,product_name_zh_cn,product_name_en_us,total_amount_minor,currency_code,fulfillment_mode_code,period_unit_code,period_count) VALUES(0,'order','server-item','server','server-version','server-config','云服务器','Cloud server',10000,'CNY','automatic','month',1),(0,'order','ssl-item','ssl','ssl-version','ssl-config','SSL证书','SSL certificate',5000,'CNY','automatic','year',1),(0,'order','pending-item','ssl','ssl-version','ssl-config','待开通证书','Pending certificate',6000,'CNY','automatic','year',1),(0,'renewal','renew-item','server','server-version','server-config','续费','Renewal',10000,'CNY','automatic','month',1),(0,'other-order','private-item','ssl','ssl-version','ssl-config','他人产品','Other product',99999,'CNY','automatic','year',1)`,
		`INSERT INTO plugin_idc_core_service_instance(tenant_id,customer_no,service_no,order_item_no,product_type_code,customer_alias,customer_remark,period_unit_code,period_count,entitlement_expires_at,entitlement_status_code,current_price_snapshot_no,configuration_snapshot_no) VALUES(0,'` + customerNo + `','SVC_SERVER','server-item','cloud_server','报表服务器','备注','month',1,CURRENT_TIMESTAMP+INTERVAL '10 days','active','server-price','server-config')`,
		`INSERT INTO plugin_idc_core_service_entitlement(tenant_id,customer_no,entitlement_no,order_item_no,entitlement_owner_id,plugin_id,period_unit_code,period_count,expires_at,status_code,current_price_snapshot_no,configuration_snapshot_no) VALUES(0,'` + customerNo + `','ENT_SSL','ssl-item','yunsu-ssl','yunsu-ssl','year',1,CURRENT_TIMESTAMP-INTERVAL '1 day','active','ssl-price','ssl-config')`,
		`INSERT INTO plugin_idc_core_commerce_quote_snapshot VALUES(1,'` + customerNo + `','tenant-quote','new_purchase')`,
		`INSERT INTO plugin_idc_core_commerce_order VALUES(1,'` + customerNo + `','tenant-order','tenant-quote',CURRENT_TIMESTAMP,'paid')`,
		`INSERT INTO plugin_idc_core_commerce_order_item(tenant_id,order_no,order_item_no,version_no) VALUES(1,'tenant-order','tenant-item','tenant-version')`,
		`INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding VALUES(1,'tenant-version','private-owner')`,
		`INSERT INTO plugin_idc_core_service_price_snapshot VALUES(0,'server-price',12000),(0,'ssl-price',5000)`,
		`INSERT INTO plugin_idc_core_commerce_configuration_snapshot_item(tenant_id,configuration_snapshot_no,option_name_zh_cn,option_name_en_us,quantity,sort_order) VALUES(0,'server-config','4核','4 cores',1,1),(0,'server-config','8GB内存','8 GB RAM',1,2),(0,'ssl-config','单域名','Single domain',1,1)`,
	}
	for _, query := range queries {
		if _, err := db.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	svc := authorizedManagementTestService(t, newDAOPersistence()).(*serviceImpl)
	grants := svc.authorization.(*managementTestAuthz)
	grants.grants[authz.PermissionKey(permissionCustomerQuery)] = true
	seen := map[string]ProductItem{}
	for page := 1; page <= 3; page++ {
		result, err := svc.SearchProducts(ctx, customerNo, page, 1)
		if err != nil || result.Total != 3 || len(result.Items) != 1 {
			t.Fatalf("page %d: %+v %v", page, result, err)
		}
		seen[result.Items[0].Number] = result.Items[0]
	}
	if len(seen) != 3 || seen["SVC_SERVER"].AmountMinor != 12000 || seen["ENT_SSL"].PluginID != "yunsu-ssl" || seen["ENT_SSL"].DeliveryKind != "entitlement" || seen["SVC_SERVER"].DeliveryKind != "server" || seen["pending-item"].DeliveryKind != "pending" || seen["pending-item"].PluginID != "yunsu-ssl" || !strings.Contains(seen["SVC_SERVER"].Information, "4核") || !strings.Contains(seen["SVC_SERVER"].InformationEnUs, "8 GB RAM") {
		t.Fatalf("products: %+v", seen)
	}
	stats, err := loadOperationsStatistics(ctx, customerNo)
	if err != nil || stats.Products != 3 || stats.Expiring != 1 || stats.Expired != 1 {
		t.Fatalf("statistics: %+v %v", stats, err)
	}
	if len(stats.ProductCategories) != 2 || stats.ProductCategories[0] != (ProductCategoryStatistics{ProductType: "cloud_server", PluginID: "idc-resource", Total: 1}) || stats.ProductCategories[1] != (ProductCategoryStatistics{ProductType: "yunsu-ssl", PluginID: "yunsu-ssl", Total: 2}) {
		t.Fatalf("purchased category counts must include pending units and exclude renewal/other customer: %+v", stats.ProductCategories)
	}
	empty, err := svc.SearchProducts(ctx, customerNo, 10, 10)
	if err != nil || empty.Total != 3 || len(empty.Items) != 0 {
		t.Fatalf("empty: %+v %v", empty, err)
	}
	if _, err = svc.SearchProducts(ctx, "CUS_ffffffffffffffffffffffffffffffff", 1, 10); err == nil {
		t.Fatal("absent customer allowed")
	}
	grants.grants[authz.PermissionKey(permissionCustomerQuery)] = false
	if _, err = svc.SearchProducts(ctx, customerNo, 1, 10); err == nil {
		t.Fatal("missing exact query permission allowed")
	}
}

// TestCustomerWorkspaceProductCategoryLimitPostgres verifies the overview never silently truncates categories.
func TestCustomerWorkspaceProductCategoryLimitPostgres(t *testing.T) {
	ctx := context.Background()
	configureManagementIntegrationDB(t, ctx)
	db := dao.CustomerSubject.DB()
	queries := []string{
		`INSERT INTO plugin_idc_core_commerce_quote_snapshot VALUES(0,'visible','purchase','new_purchase')`,
		`INSERT INTO plugin_idc_core_commerce_order VALUES(0,'visible','order','purchase',CURRENT_TIMESTAMP,'paid')`,
		`INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding SELECT 0,'version-'||n,'owner-'||n FROM generate_series(1,64) n`,
		`INSERT INTO plugin_idc_core_commerce_order_item(tenant_id,order_no,order_item_no,version_no) SELECT 0,'order','item-'||n,'version-'||n FROM generate_series(1,64) n`,
	}
	for _, query := range queries {
		if _, err := db.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	stats, err := loadOperationsStatistics(ctx, "visible")
	if err != nil || len(stats.ProductCategories) != maximumProductCategories || stats.Products != maximumProductCategories {
		t.Fatalf("bounded categories=%+v err=%v", stats, err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding VALUES(0,'version-65','owner-65')`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_order_item(tenant_id,order_no,order_item_no,version_no) VALUES(0,'order','item-65','version-65')`); err != nil {
		t.Fatal(err)
	}
	if _, err = loadOperationsStatistics(ctx, "visible"); err == nil {
		t.Fatal("category overflow silently truncated")
	}
}
