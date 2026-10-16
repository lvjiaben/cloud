import { createHash } from 'node:crypto';
import { execPgSQLStatements, queryPgScalar } from '@host-tests/support/postgres';

// Every test owns these records; append-only history is removed only during
// isolated fixture cleanup, never through a business API.
export class CatalogPricingFixture {
  constructor(readonly suffix: string) {}
  get customer() { return `CUS_${this.suffix}`; }
  get level() { return `ALV_${this.suffix}`; }
  get customerName() { return `专价客户${this.suffix.slice(0, 8)}`; }
  get levelName() { return `价格等级${this.suffix.slice(0, 8)}`; }
  id(kind: string, index = 0) { return createHash('md5').update(`${this.suffix}:${kind}:${index}`).digest('hex'); }
  product(index = 0) { return `prd_${this.id('product', index)}`; }
  productName(index = 0) { return `价格商品${this.suffix.slice(0, 8)}_${index}`; }
  get customerLabel() { return `${this.customerName} · #${queryPgScalar(`SELECT customer_code FROM plugin_idc_core_customer_subject WHERE customer_no='${this.customer}'`)}`; }
  seed() {
    const s = this.suffix;
    const statements = [
      `INSERT INTO plugin_idc_core_customer_subject(customer_no,customer_type_code,status_code,display_name,search_name) VALUES('${this.customer}','individual','active','${this.customerName}','${this.customerName}');`,
      `INSERT INTO plugin_idc_core_customer_individual_profile(customer_no,identity_status_code) VALUES('${this.customer}','unverified');`,
      `INSERT INTO plugin_idc_core_sales_agent_level(level_no,level_code,name_zh_cn,name_en_us,status_code,version) VALUES('${this.level}','price-${s}','${this.levelName}','Pricing test','active',1);`,
      `INSERT INTO plugin_idc_core_catalog_category(category_no,name_zh_cn,name_en_us,status_code,created_by,updated_by) VALUES('ctg_${this.id('category')}','价格验收','Pricing test','active',1,1);`,
    ];
    for (const index of [0, 1]) {
      statements.push(
        `INSERT INTO plugin_idc_core_catalog_product(product_no,category_no,status_code,public_slug,name_zh_cn,name_en_us,created_by,updated_by) VALUES('${this.product(index)}','ctg_${this.id('category')}','active','prices-${s}-${index}','${this.productName(index)}','Pricing test','1','1');`,
        `INSERT INTO plugin_idc_core_catalog_product_version(version_no,product_no,version_number,status_code,source_code,source_version,source_fingerprint,created_by) VALUES('ver_${this.id('version', index)}','${this.product(index)}',1,'draft','manual','1',repeat('a',64),1);`,
        `INSERT INTO plugin_idc_core_catalog_product_version_inventory(version_no,tracking_enabled,sellable_quantity,version) VALUES('ver_${this.id('version', index)}',false,0,1);`,
        `UPDATE plugin_idc_core_catalog_product SET current_version_no='ver_${this.id('version', index)}' WHERE product_no='${this.product(index)}';`,
        `INSERT INTO plugin_idc_core_catalog_billing_rule(billing_rule_no,version_no,rule_code,charge_type_code,period_unit_code,period_count,price_phase_code,quantity_mode_code,status_code) VALUES('BIL_${s}_${index}','ver_${this.id('version', index)}','monthly','recurring','month',1,'new_purchase','fixed','active');`,
        `INSERT INTO plugin_idc_core_catalog_price(price_no,version_no,billing_rule_no,currency_code,amount_minor,status_code) VALUES('CPR_${s}_${index}','ver_${this.id('version', index)}','BIL_${s}_${index}','CNY',10000,'active');`,
      );
    }
    execPgSQLStatements(statements);
  }
  latest(priceNo: string) { return queryPgScalar(`SELECT status_code || ':' || fixed_price_minor || ':' || version FROM plugin_idc_core_sales_purchase_price WHERE price_no='${priceNo}' ORDER BY version DESC LIMIT 1`); }
  history(priceNo: string) { return queryPgScalar(`SELECT count(*) FROM plugin_idc_core_sales_purchase_price WHERE price_no='${priceNo}'`); }
  cleanup() {
    execPgSQLStatements(['BEGIN;', 'SET LOCAL session_replication_role=replica;',
      `DELETE FROM plugin_idc_core_sales_purchase_price WHERE product_no IN('${this.product()}','${this.product(1)}');`,
      `DELETE FROM plugin_idc_core_sales_level_price WHERE level_no='${this.level}';`,
      `DELETE FROM plugin_idc_core_sales_agent_level WHERE level_no='${this.level}';`,
      `DELETE FROM plugin_idc_core_catalog_price WHERE version_no IN('ver_${this.id('version', 0)}','ver_${this.id('version', 1)}');`,
      `DELETE FROM plugin_idc_core_catalog_billing_rule WHERE version_no IN('ver_${this.id('version', 0)}','ver_${this.id('version', 1)}');`,
      `DELETE FROM plugin_idc_core_catalog_product_version_inventory WHERE version_no IN('ver_${this.id('version', 0)}','ver_${this.id('version', 1)}');`,
      `DELETE FROM plugin_idc_core_catalog_product_version WHERE product_no IN('${this.product()}','${this.product(1)}');`,
      `DELETE FROM plugin_idc_core_catalog_product WHERE product_no IN('${this.product()}','${this.product(1)}');`,
      `DELETE FROM plugin_idc_core_catalog_category WHERE category_no='ctg_${this.id('category')}';`,
      `DELETE FROM plugin_idc_core_customer_individual_profile WHERE customer_no='${this.customer}';`,
      `DELETE FROM plugin_idc_core_customer_subject WHERE customer_no='${this.customer}';`, 'COMMIT;']);
  }
}
