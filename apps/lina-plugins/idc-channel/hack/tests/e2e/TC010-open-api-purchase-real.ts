// Real public API acceptance against local PostgreSQL and HTTP. All fixtures
// belong to this test; dedicated users use public APIs and are deleted afterward.
// No existing host user or third-party service is modified.
import { createHash, randomBytes } from 'node:crypto';
import { expect, test } from '@host-tests/fixtures/auth';
import { config as testConfig, workspacePath } from '@host-tests/fixtures/config';
import { execPgSQLStatements, queryPgScalar } from '@host-tests/support/postgres';

test.describe('TC010 开放 API 真实购买', () => {
test('TC-10a: 真实授权、冻结报价、余额不足回滚及并发幂等扣款', async ({ adminPage }) => {
 test.setTimeout(120_000);
 const stamp=randomBytes(16).toString('hex');
 const customer=`CUS_${stamp}`,category=`ctg_${stamp}`,product=`prd_${stamp}`,version=`ver_${stamp}`,billing=`bil_${stamp}`,price=`prc_${stamp}`,tax=`tax_${stamp}`;
 const slug=`api-acceptance-${stamp}`,customerName=`API验收${stamp.slice(0,8)}`,productName=`API商品${stamp.slice(0,8)}`;
 const balanceAccount=`bac_${createHash('sha256').update(['customer',customer,'customer_balance','CNY'].join('\0')).digest('hex').slice(0,32)}`;
 const api='/x/idc-channel/api/v1';
 let appId='', otherAppId='', adminAuthorization='';
 let otherUserId=0;
 const otherCustomer=`CUX_${stamp}`;
 try {
  const authRequest=adminPage.waitForRequest(request=>request.url().includes('/admin/channel/accounts'));
  await adminPage.goto(workspacePath('/idc/channel'),{waitUntil:'domcontentloaded'});
  const authorization=(await (await authRequest).allHeaders()).authorization;
  expect(authorization).toBeTruthy(); adminAuthorization=authorization!;
  const users=await(await adminPage.request.get(`${testConfig.apiBaseURL}user?pageSize=100`,{headers:{Authorization:authorization!}})).json();
  expect(users.code).toBe(0);
  const user=users.data.list.find((row:{id:number})=>queryPgScalar(`SELECT count(*) FROM plugin_idc_core_customer_identity_binding WHERE sys_user_id=${row.id}`)==='0');
  expect(user,'A public-directory user without an existing customer binding is required').toBeTruthy();
      execPgSQLStatements([
        'BEGIN;',
        `INSERT INTO plugin_idc_core_customer_subject(customer_no,customer_type_code,status_code,display_name,search_name) VALUES('${customer}','individual','active','${customerName}','${customerName}');`,
        `INSERT INTO plugin_idc_core_customer_individual_profile(customer_no,legal_name,legal_name_masked,identity_identifier_masked,identity_lookup_digest,identity_case_no,identity_status_code,identity_verified_at) VALUES('${customer}','API fixture','A**','fixture-${stamp}','${stamp}${stamp}','CAS_${stamp}','verified',CURRENT_TIMESTAMP);`,
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
        `INSERT INTO plugin_idc_core_customer_identity_binding(binding_no,sys_user_id,customer_no,source_code) VALUES('BND_${stamp}',${user.id},'${customer}','migration');`,
        'COMMIT;',
      ]);

  // API access may be issued before verification; purchasing still uses the
  // production transaction owners and their independent identity gates.
  execPgSQLStatements([`UPDATE plugin_idc_core_customer_individual_profile SET identity_status_code='unverified' WHERE customer_no='${customer}';`]);
  const granted=await adminPage.request.post(`${api}/admin/channel/tokens`,{headers:{Authorization:authorization!},data:{customerNo:customer,displayName:customerName,scopes:['catalog.read','order.create','order.read','service.read','service.renew','settlement.read','capability.read']}});
  const grant=await granted.json();expect(grant.code,JSON.stringify({code:grant.code,message:grant.message})).toBe(0);
  appId=grant.data.appId;
  expect(queryPgScalar(`SELECT identity_status_code FROM plugin_idc_core_customer_individual_profile WHERE customer_no='${customer}'`)).toBe('unverified');
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_channel_account WHERE customer_no='${customer}' AND status_code='approved'`)).toBe('1');
  execPgSQLStatements([`UPDATE plugin_idc_core_customer_individual_profile SET identity_status_code='verified' WHERE customer_no='${customer}';`]);
  // The real list response must render in the standard grid after searching,
  // not merely exist in PostgreSQL or return successfully from HTTP.
  const listResponse = adminPage.waitForResponse(response => response.url().includes('/admin/channel/accounts?') && new URL(response.url()).searchParams.get('keyword') === customerName);
  await adminPage.getByTestId('openapi-search').fill(customerName);
  await adminPage.getByTestId('openapi-search').press('Enter');
  const listed = await (await listResponse).json();
  expect(listed.code).toBe(0); expect(listed.data.total).toBe(1);
  expect(listed.data.list[0]).toMatchObject({ channelId: appId, customerNo: customer, displayName: customerName });
  await expect(adminPage.getByRole('row').filter({ hasText: customerName })).toContainText(appId);
  const headers={Authorization:`Basic ${Buffer.from(`${appId}:${grant.data.token}`).toString('base64')}`};
  const get=async(path:string)=>{const response=await adminPage.request.get(path,{headers});return response.json();};
  const post=async(path:string,data:unknown)=>{const response=await adminPage.request.post(path,{headers,data});return response.json();};
  const catalog=await get(`${api}/channel/catalog?namePrefix=${encodeURIComponent(productName)}`);expect(catalog.code).toBe(0);expect(catalog.data.products.some((row:{publicSlug:string})=>row.publicSlug===slug)).toBe(true);
  const config=await get(`${api}/channel/catalog/products/${slug}/configuration`);expect(config.code,JSON.stringify(config)).toBe(0);
  const preview=await get(`${api}/channel/catalog/products/${slug}/billing-terms?${new URLSearchParams({configurationRevision:config.data.configurationRevision,currencyCode:'CNY',values:'[]'})}`);expect(preview.code,JSON.stringify(preview)).toBe(0);
  expect(preview.data.terms[0]).toMatchObject({chargeType:'recurring',periodUnit:'month',periodCount:1});
  const numbered=await get(`${api}/channel/catalog/products?namePrefix=${encodeURIComponent(productName)}&page=1&pageSize=1`);expect(numbered.code).toBe(0);expect(numbered.data.total).toBe(1);
  const snapshot=await get(`${api}/channel/catalog/products/${slug}/snapshot`);expect(snapshot.code).toBe(0);expect(snapshot.data.configurationMode).toBe('remote_evaluation');expect(snapshot.data.pricingReady).toBe(false);expect(snapshot.data.currencies).toContainEqual({currencyCode:'CNY',precision:2});
  const quote=await post(`${api}/channel/quotes`,{requestId:`quote-${stamp}`,purchase:{publicSlug:slug,configurationRevision:config.data.configurationRevision,billingTermToken:preview.data.terms[0].billingTermToken,currencyCode:'CNY',purchaseCycles:1,quantity:1,values:[]}});expect(quote.code,JSON.stringify(quote)).toBe(0);expect(quote.data.totalAmountMinor).toBe(10000);
  const input={requestId:`order-${stamp}`,quoteNo:quote.data.quoteNo};
  const insufficient=await post(`${api}/channel/orders`,input);expect(insufficient.code).not.toBe(0);
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_commerce_order WHERE customer_no='${customer}'`)).toBe('0');
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_billing_invoice WHERE customer_no='${customer}'`)).toBe('0');
  expect(queryPgScalar(`SELECT sellable_quantity FROM plugin_idc_core_catalog_product_version_inventory WHERE version_no='${version}'`)).toBe('10');
  // Seed one balanced local recharge with complete immutable ledger snapshots.
  execPgSQLStatements(['BEGIN;',
   `INSERT INTO plugin_idc_core_billing_ledger_account(account_no,owner_type_code,owner_reference,customer_no,account_type_code,normal_balance_code,currency_code) VALUES('${balanceAccount}','customer','${customer}','${customer}','customer_balance','credit','CNY'),('cas_${stamp}','merchant','test-${stamp}',NULL,'merchant_cash','debit','CNY');`,
   `INSERT INTO plugin_idc_core_billing_ledger_transaction(transaction_no,transaction_type_code,currency_code,source_type_code,source_reference,source_snapshot_digest,entry_count,debit_total_minor,credit_total_minor,content_digest) VALUES('btx_${stamp}','recharge','CNY','test.recharge','${stamp}',repeat('a',64),2,20000,20000,repeat('b',64));`,
   `INSERT INTO plugin_idc_core_billing_ledger_entry(entry_no,transaction_no,currency_code,line_number,account_no,direction_code,entry_role_code,amount_minor,reason_code,content_digest) VALUES('cre_${stamp}','btx_${stamp}','CNY',1,'${balanceAccount}','credit','balance',20000,'test.recharge',repeat('a',64)),('deb_${stamp}','btx_${stamp}','CNY',2,'cas_${stamp}','debit','cash',20000,'test.recharge',repeat('b',64));`,
   `INSERT INTO plugin_idc_core_billing_balance_snapshot(snapshot_no,account_no,currency_code,normal_balance_code,transaction_no,sequence_no,opening_balance_minor,debit_amount_minor,credit_amount_minor,closing_balance_minor,content_digest) VALUES('bal_${stamp}','${balanceAccount}','CNY','credit','btx_${stamp}',1,0,0,20000,20000,repeat('a',64)),('bca_${stamp}','cas_${stamp}','CNY','debit','btx_${stamp}',1,0,20000,0,20000,repeat('b',64));`,
   'COMMIT;']);
  const results=await Promise.all([post(`${api}/channel/orders`,input),post(`${api}/channel/orders`,input)]);
  for(const result of results) expect(result.code,JSON.stringify(result)).toBe(0);
  expect(results[0].data.id).toBe(results[1].data.id);
  const order=await get(`${api}/channel/orders/${results[0].data.id}`);expect(order.code,JSON.stringify(order)).toBe(0);expect(order.data.financiallySettled).toBe(true);
  const recovered=await get(`${api}/channel/orders/by-request/${input.requestId}`);expect(recovered.code).toBe(0);expect(recovered.data.found).toBe(true);expect(recovered.data.order.id).toBe(order.data.id);
  const absent=await get(`${api}/channel/orders/by-request/missing-${stamp}`);expect(absent.code).toBe(0);expect(absent.data).toMatchObject({found:false,order:null});
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_commerce_order WHERE customer_no='${customer}'`)).toBe('1');
  expect(queryPgScalar(`SELECT closing_balance_minor FROM plugin_idc_core_billing_balance_snapshot WHERE account_no='${balanceAccount}' ORDER BY sequence_no DESC LIMIT 1`)).toBe('10000');
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_billing_ledger_entry WHERE account_no='${balanceAccount}' AND direction_code='debit'`)).toBe('1');
  const services=await get(`${api}/channel/services?orderNo=${order.data.coreOrderNo}`);expect(services.code,JSON.stringify(services)).toBe(0);
  // A historical local entitlement is the only synthetic precondition. Every
  // renewal quote, operation, invoice, payment and expiry update below uses HTTP
  // and the live production renewal/billing services with normal DB triggers.
  const entitlement=`ent_${stamp}`;
  execPgSQLStatements(['BEGIN;','SET LOCAL session_replication_role=replica;',
   `INSERT INTO plugin_idc_core_commerce_structured_configuration_snapshot(snapshot_no,customer_no,uploader_user_id,quote_no,quote_item_no,product_no,version_no,configuration_revision,interface_instance_no,interface_configuration_revision,customer_declaration_digest,candidate_snapshot_digest,customer_value_digest,candidate_snapshot,typed_value_snapshot,content_digest,order_no,order_item_no,bound_at)
    SELECT 'scf_${stamp}','${customer}',${user.id},quote_no,quote_item_no,product_no,version_no,'fixture-v1','pfi_${stamp}',1,repeat('d',64),repeat('d',64),repeat('d',64),'{}','{}',repeat('d',64),order_no,order_item_no,CURRENT_TIMESTAMP FROM plugin_idc_core_commerce_order_item WHERE order_no='${order.data.coreOrderNo}';`,
   `INSERT INTO plugin_idc_core_product_delivery_record(delivery_no,order_no,order_item_no,customer_no,product_no,version_no,quantity,strategy_code,policy_digest,configuration_snapshot_no,interface_instance_no,plugin_id,plugin_release_version,contract_version,declaration_digest,ongoing_management,renewal_mode_code,entitlement_owner_id,status_code,started_at,completed_at,content_digest)
    SELECT 'dlv_${stamp}',order_no,order_item_no,'${customer}',product_no,version_no,1,'payment_confirmed_plugin',repeat('d',64),'scf_${stamp}','pfi_${stamp}','test-local-entitlement','1.0.0','v1',repeat('d',64),TRUE,'local_entitlement','test-local-entitlement','completed',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,repeat('d',64) FROM plugin_idc_core_commerce_order_item WHERE order_no='${order.data.coreOrderNo}';`,
   `INSERT INTO plugin_idc_core_product_fulfillment_intent(intent_no,settlement_no,order_no,order_item_no,customer_no,payer_customer_no,invoice_no,currency_code,product_no,version_no,product_name_zh_cn,product_name_en_us,quantity,unit_sequence,strategy_code,granularity_code,initial_status_code,status_code,configuration_snapshot_no,interface_instance_no,interface_configuration_revision,plugin_id,plugin_release_version,contract_version,release_digest,declaration_digest,provision_method,query_method,policy_digest,evidence_digest,financially_settled_at,ongoing_management,renewal_mode_code,entitlement_owner_id)
    SELECT 'pft_${stamp}',settlement.settlement_no,item.order_no,item.order_item_no,'${customer}','${customer}',settlement.invoice_no,'CNY',item.product_no,item.version_no,item.product_name_zh_cn,item.product_name_en_us,1,1,'payment_confirmed_plugin','per_order_item','queued','completed','scf_${stamp}','pfi_${stamp}',1,'test-local-entitlement','1.0.0','v1',repeat('d',64),repeat('d',64),'provision','query',repeat('d',64),repeat('d',64),settlement.settled_at,TRUE,'local_entitlement','test-local-entitlement' FROM plugin_idc_core_commerce_order_item item JOIN plugin_idc_core_financial_settlement settlement ON settlement.order_no=item.order_no WHERE item.order_no='${order.data.coreOrderNo}';`,
   `INSERT INTO plugin_idc_core_service_entitlement(entitlement_no,delivery_no,source_intent_no,customer_no,order_no,order_item_no,product_no,version_no,unit_sequence,configuration_snapshot_no,interface_instance_no,plugin_id,plugin_release_version,contract_version,declaration_digest,status_code,started_at,expires_at,renewal_mode_code,entitlement_owner_id,currency_code,billing_rule_no,charge_type_code,period_unit_code,period_count)
    SELECT '${entitlement}',delivery.delivery_no,intent.intent_no,'${customer}',item.order_no,item.order_item_no,item.product_no,item.version_no,1,configuration.snapshot_no,'pfi_${stamp}','test-local-entitlement','1.0.0','v1',repeat('d',64),'active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 month','local_entitlement','test-local-entitlement','CNY',item.billing_rule_no,item.charge_type_code,item.period_unit_code,item.period_count
    FROM plugin_idc_core_commerce_order_item item
    JOIN plugin_idc_core_product_delivery_record delivery USING(order_item_no)
    JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.order_item_no=item.order_item_no
    JOIN plugin_idc_core_commerce_structured_configuration_snapshot configuration ON configuration.order_item_no=item.order_item_no
    WHERE item.order_no='${order.data.coreOrderNo}';`, 'COMMIT;']);
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_service_entitlement WHERE entitlement_no='${entitlement}'`)).toBe('1');
  const originalExpiry=queryPgScalar(`SELECT expires_at::text FROM plugin_idc_core_service_entitlement WHERE entitlement_no='${entitlement}'`);
  const renewalPath=`${api}/channel/services/${entitlement}/operations/renew`;
  const quoteCountBefore=queryPgScalar(`SELECT count(*) FROM plugin_idc_core_commerce_quote_snapshot WHERE customer_no='${customer}'`);
  const rejectedRenewal=await post(renewalPath,{requestId:`renew-large-${stamp}`,parameters:{renewalCycles:2},confirmed:true});
  expect(rejectedRenewal.code,JSON.stringify(rejectedRenewal)).not.toBe(0);
  expect(rejectedRenewal.message).toMatch(/balance|余额/i);
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_business_operation WHERE subject_no='${entitlement}'`)).toBe('0');
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_service_renewal_quote_request WHERE entitlement_no='${entitlement}'`)).toBe('0');
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_commerce_quote_snapshot WHERE customer_no='${customer}'`)).toBe(quoteCountBefore);
  expect(queryPgScalar(`SELECT closing_balance_minor FROM plugin_idc_core_billing_balance_snapshot WHERE account_no='${balanceAccount}' ORDER BY sequence_no DESC LIMIT 1`)).toBe('10000');
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_billing_invoice WHERE customer_no='${customer}'`)).toBe('1');
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_billing_ledger_entry WHERE account_no='${balanceAccount}' AND direction_code='debit'`)).toBe('1');
  expect(queryPgScalar(`SELECT expires_at::text FROM plugin_idc_core_service_entitlement WHERE entitlement_no='${entitlement}'`)).toBe(originalExpiry);
  const renewalInput={requestId:`renew-once-${stamp}`,parameters:{renewalCycles:1},confirmed:true};
  const renewed=await post(renewalPath,renewalInput);expect(renewed.code,JSON.stringify(renewed)).toBe(0);expect(renewed.data.status).toBe('succeeded');
  const paidInvoice=queryPgScalar(`SELECT invoice_no FROM plugin_idc_core_business_operation WHERE operation_no='${renewed.data.operationNo}'`);
  const renewedAgain=await post(renewalPath,renewalInput);expect(renewedAgain.code,JSON.stringify(renewedAgain)).toBe(0);expect(renewedAgain.data.operationNo).toBe(renewed.data.operationNo);
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_business_operation WHERE subject_no='${entitlement}' AND status_code='succeeded'`)).toBe('1');
  expect(queryPgScalar(`SELECT invoice_no FROM plugin_idc_core_business_operation WHERE operation_no='${renewedAgain.data.operationNo}'`)).toBe(paidInvoice);
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_service_renewal_quote_request WHERE entitlement_no='${entitlement}'`)).toBe('1');
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_business_operation WHERE operation_no='${renewed.data.operationNo}' AND customer_no='${customer}' AND payer_customer_no='${customer}' AND financial_settlement_required=TRUE AND execution_mode_code='local_entitlement' AND resource_owner_code='test-local-entitlement' AND expected_subject_version>0 AND financially_settled_at IS NOT NULL AND completed_at IS NOT NULL`)).toBe('1');
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_billing_invoice WHERE customer_no='${customer}' AND status_code='paid'`)).toBe('2');
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_billing_ledger_entry WHERE account_no='${balanceAccount}' AND direction_code='debit'`)).toBe('2');
  expect(queryPgScalar(`SELECT closing_balance_minor FROM plugin_idc_core_billing_balance_snapshot WHERE account_no='${balanceAccount}' ORDER BY sequence_no DESC LIMIT 1`)).toBe('0');
  expect(queryPgScalar(`SELECT (expires_at='${originalExpiry}'::timestamptz+INTERVAL '1 month')::text FROM plugin_idc_core_service_entitlement WHERE entitlement_no='${entitlement}'`)).toBe('true');
  const progress=await get(`${api}/channel/services/${entitlement}/operations/${renewed.data.operationNo}`);expect(progress.code,JSON.stringify(progress)).toBe(0);
  const otherUser=await(await adminPage.request.post(`${testConfig.apiBaseURL}user`,{headers:{Authorization:authorization!},data:{username:`api_renew_${stamp}`,nickname:'API renewal isolation fixture',password:randomBytes(16).toString('hex'),status:1}})).json();
  expect(otherUser.code,JSON.stringify({code:otherUser.code,message:otherUser.message})).toBe(0);otherUserId=otherUser.data.id;
  execPgSQLStatements(['BEGIN;',
   `INSERT INTO plugin_idc_core_customer_subject(customer_no,customer_type_code,status_code,display_name,search_name) VALUES('${otherCustomer}','individual','active','Other API fixture','Other API fixture');`,
   `INSERT INTO plugin_idc_core_customer_individual_profile(customer_no,legal_name,legal_name_masked,identity_identifier_masked,identity_lookup_digest,identity_case_no,identity_status_code,identity_verified_at) VALUES('${otherCustomer}','Other fixture','O**','other-${stamp}',repeat('e',64),'CAX_${stamp}','verified',CURRENT_TIMESTAMP);`,
   `INSERT INTO plugin_idc_core_customer_identity_binding(binding_no,sys_user_id,customer_no,source_code) VALUES('BNX_${stamp}',${otherUserId},'${otherCustomer}','migration');`,'COMMIT;']);
  const otherGrant=await(await adminPage.request.post(`${api}/admin/channel/tokens`,{headers:{Authorization:authorization!},data:{customerNo:otherCustomer,displayName:'Other renewal fixture',scopes:['service.read','service.renew']}})).json();
  expect(otherGrant.code,JSON.stringify({code:otherGrant.code,message:otherGrant.message})).toBe(0);otherAppId=otherGrant.data.appId;
  const otherHeaders={Authorization:`Basic ${Buffer.from(`${otherAppId}:${otherGrant.data.token}`).toString('base64')}`};
  const ownerRead=await get(`${api}/channel/services/${entitlement}/operations/${renewed.data.operationNo}`);expect(ownerRead.code,JSON.stringify(ownerRead)).toBe(0);
  const forbidden=await(await adminPage.request.post(renewalPath,{headers:otherHeaders,data:{...renewalInput,requestId:`foreign-${stamp}`}})).json();expect(forbidden.code).not.toBe(0);
  const forbiddenRead=await(await adminPage.request.get(`${api}/channel/services/${entitlement}/operations/${renewed.data.operationNo}`,{headers:otherHeaders})).json();expect(forbiddenRead.code).not.toBe(0);
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_business_operation WHERE subject_no='${entitlement}'`)).toBe('1');
  expect(queryPgScalar(`SELECT count(*) FROM plugin_idc_core_billing_ledger_entry WHERE account_no='${balanceAccount}' AND direction_code='debit'`)).toBe('2');

  const wrongHeaders={Authorization:`Basic ${Buffer.from(`wrong:${grant.data.token}`).toString('base64')}`};
  const wrong=await(await adminPage.request.get(`${api}/channel/orders/${results[0].data.id}`,{headers:wrongHeaders})).json();expect(wrong.code).not.toBe(0);
 } finally {
  try {
  execPgSQLStatements(['BEGIN;','SET LOCAL session_replication_role=replica;',
   `DO $cleanup$ DECLARE target RECORD; predicate TEXT; keys TEXT[];
    BEGIN
     SELECT array_agg(value) INTO keys FROM (
      SELECT entitlement_no::text AS value FROM plugin_idc_core_service_entitlement WHERE customer_no='${customer}' UNION SELECT operation_no::text FROM plugin_idc_core_business_operation WHERE customer_no='${customer}' UNION SELECT price_snapshot_no::text FROM plugin_idc_core_service_price_snapshot WHERE customer_no='${customer}' UNION SELECT order_no::text AS value FROM plugin_idc_core_commerce_order WHERE customer_no='${customer}' UNION SELECT quote_no::text FROM plugin_idc_core_commerce_quote_snapshot WHERE customer_no='${customer}' UNION SELECT invoice_no::text FROM plugin_idc_core_billing_invoice WHERE customer_no='${customer}' UNION SELECT order_item_no::text FROM plugin_idc_core_commerce_order_item WHERE order_no IN(SELECT order_no FROM plugin_idc_core_commerce_order WHERE customer_no='${customer}') UNION SELECT quote_item_no::text FROM plugin_idc_core_commerce_quote_item_snapshot WHERE quote_no IN(SELECT quote_no FROM plugin_idc_core_commerce_quote_snapshot WHERE customer_no='${customer}') UNION SELECT transaction_no::text FROM plugin_idc_core_billing_ledger_entry WHERE account_no='${balanceAccount}' UNION SELECT unnest(ARRAY['${customer}','${product}','${version}','${category}','${billing}','${price}','${tax}','${balanceAccount}','cas_${stamp}','btx_${stamp}','${appId}','${otherAppId}','${otherCustomer}'])) owned WHERE value<>'';
     FOR target IN SELECT table_name,array_agg(column_name::text) AS columns FROM information_schema.columns WHERE table_schema=current_schema() AND table_name LIKE 'plugin_idc_%' AND column_name IN('customer_no','order_no','quote_no','invoice_no','order_item_no','quote_item_no','product_no','version_no','category_no','billing_rule_no','price_no','tax_policy_no','account_no','transaction_no','channel_id','entitlement_no','subject_no','operation_no','price_snapshot_no') GROUP BY table_name LOOP
      SELECT string_agg(format('%I::text=ANY(%L::text[])',column_name,keys),' OR ') INTO predicate FROM unnest(target.columns) AS column_name;
      EXECUTE format('DELETE FROM %I WHERE %s',target.table_name,predicate);
     END LOOP;
    END $cleanup$;`, 'COMMIT;']);
  } finally {
   if(otherUserId>0){const removed=await(await adminPage.request.delete(`${testConfig.apiBaseURL}user/${otherUserId}`,{headers:{Authorization:adminAuthorization}})).json();expect(removed.code,JSON.stringify(removed)).toBe(0);}
  }
 }
});
});
