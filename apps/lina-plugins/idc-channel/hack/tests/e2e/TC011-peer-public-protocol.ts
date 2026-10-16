import { randomBytes } from 'node:crypto';
import { execFile } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';
import { expect, test } from '@host-tests/fixtures/auth';
import { config } from '@host-tests/fixtures/config';
import { execPgSQLStatements } from '@host-tests/support/postgres';

test.describe('TC-11 同系统公开协议', () => {
  test('TC-11c: 公开重复启用恢复授权并保持停止新业务和引用阻断', async () => {
    test.setTimeout(600_000);
    const script = fileURLToPath(new URL('../support/peer-protocol/lifecycle.py', import.meta.url));
    const { stdout } = await promisify(execFile)(process.env.E2E_PYTHON_BIN ?? 'python3', [script], {
      env: { ...process.env, PYTHONDONTWRITEBYTECODE: '1' },
      timeout: 550_000, maxBuffer: 1_000_000,
    }).catch((error: Error & { stdout?: string }) => { throw new Error(error.stdout || 'Isolated lifecycle subprocess failed'); });
    expect(stdout).toContain('TC011_PUBLIC_PEER_LIFECYCLE_OK');
  });
  test('TC-11b: 隔离双实例正式采购、一次性交付和长期真实续费', async () => {
    test.setTimeout(900_000);
    const script = fileURLToPath(new URL('../support/peer-protocol/scenario.py', import.meta.url));
    const { stdout } = await promisify(execFile)(process.env.E2E_PYTHON_BIN ?? 'python3', [script], {
      env: { ...process.env, PYTHONDONTWRITEBYTECODE: '1' },
      timeout: 850_000, maxBuffer: 1_000_000,
    }).catch((error: Error & { stdout?: string }) => { throw new Error(error.stdout || 'Isolated public protocol subprocess failed'); });
    expect(stdout).toContain('TC011_PUBLIC_DUAL_INSTANCE_OK');
  });
  test('TC-11a: 管理员正式创建计费、价格、税务并发布商品', async ({ request }) => {
    const stamp = randomBytes(16).toString('hex');
    const origin = config.backendBaseURL;
    const login = await (await request.post(`${origin}/api/v1/auth/login`, {
      data: { username: config.adminUser, password: config.adminPass, clientType: 'web' },
    })).json();
    expect(login.code).toBe(0);
    expect(login.data.accessToken).toBeTruthy();
    const headers = { Authorization: `Bearer ${login.data.accessToken}` };
    const prefix = `${origin}/x/idc-core/api/v1/admin/catalog`;
    const call = async (path: string, data?: unknown, method = 'POST') => {
      const response = await request.fetch(prefix + path, { method, headers, data });
      const body = await response.json();
      expect(body.code, `${path}: ${body.message}`).toBe(0);
      return body.data;
    };
    let categoryNo = '', productNo = '', versionNo = '';
    try {
      const category = await call('/categories', { nameZhCn: '双实例发布验收', nameEnUs: 'Peer publication acceptance', sortOrder: 999 });
      categoryNo = category.category.categoryNo;
      const product = await call('/products', {
        categoryNo, publicSlug: `peer-publication-${stamp}`, nameZhCn: '一次性交付协议验收', nameEnUs: 'One-time protocol acceptance',
        summaryZhCn: '管理员正式发布接口回归验收商品', summaryEnUs: 'Public staff publication regression fixture',
        contractPolicyCode: 'none', inventoryTrackingEnabled: true, sellableQuantity: 2,
      });
      productNo = product.productNo; versionNo = product.versionNo;
      const version = `/product-versions/${versionNo}`;
      await call(`${version}/fulfillment-policy`, { strategyCode: 'manual' }, 'PUT');
      const billing = await call(`${version}/billing-rules`, {
        ruleCode: 'once', chargeType: 'one_time', periodUnit: 'once', periodCount: 1,
        pricePhase: 'new_purchase', quantityMode: 'fixed', minimumPurchaseCycles: 1, status: 'active',
      });
      expect(billing.billingRuleNo).toMatch(/^bil_[a-f0-9]{32}$/);
      const price = await call(`${version}/billing-rules/${billing.billingRuleNo}/price-schedules`, {
        currencyCode: 'CNY', tiers: [{ minimumQuantity: 0, maximumQuantity: null, amountMinor: '100', setupFeeMinor: '0' }], status: 'active',
      });
      expect(price.tiers[0].amountMinor).toBe('100');
      const tax = await call(`${version}/tax-policies`, {
        currencyCode: 'CNY', priceIncludesTax: false, taxTreatment: 'exempt', taxCategoryCode: 'service',
        roundingScope: 'line', roundingMode: 'half_up', roundingQuantumMinor: '1',
      });
      expect(tax.taxTreatment).toBe('exempt');
      const preview = await call(`${version}/publication-preview`, undefined, 'GET');
      expect(preview.ready).toBe(true);
      const published = await call(`${version}/publish`, {});
      expect(published.status).toBe('published');
      const detail = await call(`/products/${productNo}`, undefined, 'GET');
      expect(JSON.stringify(detail)).toContain(versionNo);
      expect(JSON.stringify(detail)).toContain('published');
    } finally {
      if (categoryNo) execPgSQLStatements([
        'BEGIN;', 'SET LOCAL session_replication_role=replica;',
        `DO $cleanup$ DECLARE target RECORD; predicate TEXT; BEGIN
        FOR target IN SELECT table_name,array_agg(column_name::text) AS columns FROM information_schema.columns
          WHERE table_schema=current_schema() AND table_name LIKE 'plugin_idc_core_%'
          AND column_name IN('product_no','version_no','category_no') GROUP BY table_name LOOP
          SELECT string_agg(format('%I::text=ANY(%L::text[])',column_name,ARRAY['${productNo}','${versionNo}','${categoryNo}']),' OR ')
          INTO predicate FROM unnest(target.columns) AS column_name;
          EXECUTE format('DELETE FROM %I WHERE %s',target.table_name,predicate);
        END LOOP; END $cleanup$;`, 'COMMIT;',
      ]);
    }
  });
});
