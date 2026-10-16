// Real browser and HTTP acceptance. Fixtures and cleanup touch only this test's
// plugin-owned rows; no host tables, external payments or fulfillment are used.
import { randomBytes } from 'node:crypto';
import { expect, test } from '@host-tests/fixtures/auth';
import { execPgSQLStatements, queryPgScalar } from '@host-tests/support/postgres';
import { AssistedQuotesPage } from '../pages/AssistedQuotesPage';

test.describe('TC-29 协助下单真实创建入口', () => {
  test('TC-29a: 客户行直接生成真实待付款订单并核验未扣余额', async ({ adminPage }) => {
    const stamp = randomBytes(16).toString('hex');
    const customer = `CUS_${stamp}`, category = `ctg_${stamp}`, product = `prd_${stamp}`;
    const version = `ver_${stamp}`, billing = `bil_${stamp}`, price = `prc_${stamp}`, tax = `tax_${stamp}`;
    const slug = `assisted-acceptance-${stamp}`;
    const customerName = `报价验收${stamp.slice(0, 8)}`;
    const productName = `报价验收商品${stamp.slice(0, 8)}`;
    const requestPath = '/x/idc-core/api/v1/admin/commerce/assisted-quotes';
    try {
      execPgSQLStatements([
        'BEGIN;',
        `INSERT INTO plugin_idc_core_customer_subject(customer_no,customer_type_code,status_code,display_name,search_name) VALUES('${customer}','individual','active','${customerName}','${customerName}');`,
        `INSERT INTO plugin_idc_core_customer_individual_profile(customer_no,identity_status_code) VALUES('${customer}','unverified');`,
        `INSERT INTO plugin_idc_core_catalog_category(category_no,status_code,name_zh_cn,name_en_us) VALUES('${category}','active','${productName}','Assisted acceptance');`,
        `INSERT INTO plugin_idc_core_catalog_product(product_no,category_no,status_code,public_slug,name_zh_cn,name_en_us) VALUES('${product}','${category}','active','${slug}','${productName}','Assisted acceptance');`,
        `INSERT INTO plugin_idc_core_catalog_product_version(version_no,product_no,version_number,status_code,source_code) VALUES('${version}','${product}',1,'draft','manual');`,
        `INSERT INTO plugin_idc_core_catalog_product_version_inventory(version_no,tracking_enabled,sellable_quantity,version) VALUES('${version}',TRUE,10,1);`,
        `INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy(version_no,strategy_code,granularity_code,ongoing_management,maximum_attempts,retry_base_seconds,policy_digest,created_by) VALUES('${version}','manual','per_order_item',FALSE,10,30,repeat('b',64),1);`,
        `INSERT INTO plugin_idc_core_catalog_billing_rule(billing_rule_no,version_no,rule_code,charge_type_code,period_unit_code,period_count,price_phase_code,status_code) VALUES('${billing}','${version}','monthly','recurring','month',1,'both','active');`,
        `INSERT INTO plugin_idc_core_catalog_price(price_no,version_no,billing_rule_no,currency_code,amount_minor,setup_fee_minor,renewal_amount_minor,status_code) VALUES('${price}','${version}','${billing}','CNY',10000,0,10000,'active');`,
        `INSERT INTO plugin_idc_core_catalog_tax_policy(tax_policy_no,version_no,currency_code,price_includes_tax,tax_treatment_code,tax_rate_basis_points,tax_category_code,rounding_scope_code,rounding_mode_code,rounding_quantum_minor) VALUES('${tax}','${version}','CNY',FALSE,'exempt',NULL,'service','line','half_up',1);`,
        `UPDATE plugin_idc_core_catalog_product_version SET status_code='published',published_at=CURRENT_TIMESTAMP WHERE version_no='${version}';`,
        `UPDATE plugin_idc_core_catalog_product SET current_version_no='${version}' WHERE product_no='${product}';`,
        'COMMIT;',
      ]);
      const view = new AssistedQuotesPage(adminPage);
      await view.goto();
      await view.fill('customer-search-name', customerName);
      await adminPage.getByRole('button', {name:/搜\s*索/u}).click();
      await expect(adminPage.getByTestId(`customer-assist-${customer}`)).toHaveText('协助下单');
      await view.screenshot('assisted-real-list'); await view.openCreate(customer);
      await expect(adminPage.getByTestId('assisted-create-selected-customer')).toContainText(customerName);
      await view.select('assisted-create-currency', 'CNY');
      await view.fill('assisted-item-product-0', productName);
      await view.select('assisted-item-product-0', productName);
      await expect(adminPage.getByTestId('assisted-item-price-0')).toBeVisible();
      await view.fill('assisted-create-manualDiscount', '1.00');
      await view.fill('assisted-create-reason', '真实创建验收，客户自行付款');
      await view.screenshot('assisted-real-create-ready');
      const submitted = adminPage.waitForResponse(response => new URL(response.url()).pathname === requestPath && response.request().method() === 'POST');
      await view.submitCreate();
      const response = await submitted;
      const payload = await response.json();
      expect(payload.code, JSON.stringify(payload)).toBe(0);
      const requestNo: string = payload.data.request.requestNo;
      expect(payload.data.request.statusCode).toBe('ordered');
      expect(payload.data.request.canApprove).toBe(false);
      expect(payload.data.request.manualDiscountMinor).toBe('100');
      expect(payload.data.request.orderNo).toBeTruthy();
      expect(payload.data.request.orderStatusCode).toBe('pending_payment');
      expect(queryPgScalar(`SELECT COUNT(*) FROM plugin_idc_core_commerce_assisted_quote_request WHERE customer_no='${customer}' AND status_code='ordered'`)).toBe('1');
      expect(queryPgScalar(`SELECT COUNT(*) FROM plugin_idc_core_commerce_assisted_configuration WHERE request_no='${requestNo}'`)).toBe('1');
      await expect(view.createDrawer).toBeHidden();
      expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_commerce_order WHERE customer_no='${customer}' AND paid_at IS NULL AND total_amount_minor=9900`)).toBe('1');
      expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_billing_payment_allocation WHERE invoice_no IN (SELECT invoice_no FROM plugin_idc_core_billing_invoice WHERE customer_no='${customer}')`)).toBe('0');
      expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_financial_settlement WHERE customer_no='${customer}'`)).toBe('0');
      expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_commerce_assisted_quote_request WHERE customer_no='${customer}' AND decision_by IS NOT NULL`)).toBe('0');
      await view.screenshot('assisted-real-created-order');
    } finally {
      // Local immutable test snapshots need session-local trigger suppression.
      // This transaction only deletes exact randomly named plugin fixtures.
      execPgSQLStatements([
        'BEGIN;',
        'SET LOCAL session_replication_role = replica;',
        `DO $cleanup$ DECLARE target RECORD; predicate TEXT; orders TEXT[]; quotes TEXT[]; invoices TEXT[]; order_items TEXT[]; quote_items TEXT[];
         BEGIN
          SELECT array_agg(order_no::text) INTO orders FROM plugin_idc_core_commerce_order WHERE customer_no='${customer}';
          SELECT array_agg(quote_no::text) INTO quotes FROM plugin_idc_core_commerce_quote_snapshot WHERE customer_no='${customer}';
          SELECT array_agg(invoice_no::text) INTO invoices FROM plugin_idc_core_billing_invoice WHERE customer_no='${customer}';
          SELECT array_agg(order_item_no::text) INTO order_items FROM plugin_idc_core_commerce_order_item WHERE order_no=ANY(orders);
          SELECT array_agg(quote_item_no::text) INTO quote_items FROM plugin_idc_core_commerce_quote_item_snapshot WHERE quote_no=ANY(quotes);
          FOR target IN SELECT c.table_name,array_agg(c.column_name::text) AS columns FROM information_schema.columns c
           WHERE c.table_schema=current_schema() AND c.table_name LIKE 'plugin_idc_core_%' AND c.table_name <> 'plugin_idc_core_commerce_assisted_quote_request' AND c.column_name IN ('order_no','quote_no','invoice_no','order_item_no','quote_item_no') GROUP BY c.table_name
          LOOP
           predicate:='false';
           IF 'order_no'=ANY(target.columns) THEN predicate:=predicate||format(' OR order_no=ANY(%L::text[])',orders); END IF;
           IF 'quote_no'=ANY(target.columns) THEN predicate:=predicate||format(' OR quote_no=ANY(%L::text[])',quotes); END IF;
           IF 'invoice_no'=ANY(target.columns) THEN predicate:=predicate||format(' OR invoice_no=ANY(%L::text[])',invoices); END IF;
           IF 'order_item_no'=ANY(target.columns) THEN predicate:=predicate||format(' OR order_item_no=ANY(%L::text[])',order_items); END IF;
           IF 'quote_item_no'=ANY(target.columns) THEN predicate:=predicate||format(' OR quote_item_no=ANY(%L::text[])',quote_items); END IF;
           EXECUTE format('DELETE FROM %I WHERE %s',target.table_name,predicate);
          END LOOP;
         END $cleanup$;`,
        `DELETE FROM plugin_idc_core_commerce_assisted_configuration WHERE request_no IN (SELECT request_no FROM plugin_idc_core_commerce_assisted_quote_request WHERE customer_no='${customer}');`,
        `DELETE FROM plugin_idc_core_commerce_assisted_quote_item_quantity WHERE request_item_no IN (SELECT request_item_no FROM plugin_idc_core_commerce_assisted_quote_request_item WHERE version_no='${version}');`,
        `DELETE FROM plugin_idc_core_commerce_assisted_quote_request_item WHERE version_no='${version}';`,
        `DELETE FROM plugin_idc_core_commerce_assisted_quote_request WHERE customer_no='${customer}';`,
        `DELETE FROM plugin_idc_core_commerce_configuration_price_selection WHERE version_no='${version}';`,
        `DELETE FROM plugin_idc_core_commerce_configuration_snapshot_item WHERE configuration_snapshot_no IN (SELECT configuration_snapshot_no FROM plugin_idc_core_commerce_configuration_snapshot WHERE version_no='${version}');`,
        `DELETE FROM plugin_idc_core_commerce_configuration_snapshot WHERE version_no='${version}';`,
        ...['price', 'tax_policy', 'billing_rule', 'product_version_inventory', 'product_version_fulfillment_policy'].map(table => `DELETE FROM plugin_idc_core_catalog_${table} WHERE version_no='${version}';`),
        `DELETE FROM plugin_idc_core_catalog_product_version WHERE version_no='${version}';`,
        `DELETE FROM plugin_idc_core_catalog_product WHERE product_no='${product}';`,
        `DELETE FROM plugin_idc_core_catalog_category WHERE category_no='${category}';`,
        `DELETE FROM plugin_idc_core_customer_individual_profile WHERE customer_no='${customer}';`,
        `DELETE FROM plugin_idc_core_customer_subject WHERE customer_no='${customer}';`,
        'COMMIT;',
      ]);
      expect(queryPgScalar(`SELECT COUNT(*) FROM plugin_idc_core_catalog_product WHERE product_no='${product}'`)).toBe('0');
    }
  });
});
