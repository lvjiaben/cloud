"""Fingerprint originals and remove only explicitly named, unpublished E2E copies."""
import os
from pathlib import Path
import re
import subprocess
import sys
import yaml

root = Path(__file__).resolve().parents[6]
link = yaml.safe_load((root / '.site-runtime/installation/config.yaml').read_text())['database']['default']['link']
match = re.fullmatch(r'pgsql:([^:]+):(.+)@tcp\(([^:]+):(\d+)\)/(.+)', link)
if not match:
    raise SystemExit('Unsupported local test database configuration')
user, password, host, port, database = match.groups()
if host not in {"localhost", "127.0.0.1"}:
    raise SystemExit("Fixture operations require an explicitly local database")
env = dict(os.environ, PGPASSWORD=password, PGHOST=host, PGPORT=port, PGUSER=user, PGDATABASE=database.split('?')[0])
mode, identity = sys.argv[1:3]
if mode == 'probe':
    sql = 'BEGIN; SET LOCAL session_replication_role=replica; ROLLBACK;'
elif mode == 'fingerprint':
    if not re.fullmatch(r'prd_[a-f0-9]{32}', identity):
        raise SystemExit('Invalid source product identity')
    sql = f"""BEGIN READ ONLY; SET LOCAL search_path=public;
SELECT md5(COALESCE((SELECT jsonb_agg(to_jsonb(p) ORDER BY p.product_no)::text FROM plugin_idc_core_catalog_product p WHERE p.product_no='{identity}'),'') ||
COALESCE((SELECT jsonb_agg(to_jsonb(v) ORDER BY v.version_no)::text FROM plugin_idc_core_catalog_product_version v WHERE v.product_no='{identity}'),'') ||
COALESCE((SELECT jsonb_agg(to_jsonb(p) ORDER BY p.price_no)::text FROM plugin_idc_core_catalog_price p JOIN plugin_idc_core_catalog_product_version v USING(version_no) WHERE v.product_no='{identity}'),'') ||
COALESCE((SELECT jsonb_agg(to_jsonb(c) ORDER BY c.version_no)::text FROM plugin_idc_core_catalog_product_version_configuration c JOIN plugin_idc_core_catalog_product_version v USING(version_no) WHERE v.product_no='{identity}'),'')); COMMIT;"""
elif mode == 'cleanup':
    if not re.fullmatch(r'e2e-local-config-[a-f0-9]{32}', identity):
        raise SystemExit('Cleanup accepts only generated E2E copy slugs')
    # Session-local trigger bypass applies only to this cleanup transaction;
    # immutable rows of other products are never selected or changed.
    sql = f"""BEGIN; SET LOCAL search_path=public; SET LOCAL session_replication_role=replica;
DO $$ DECLARE product text; versions text[]; entry record;
BEGIN
 SELECT product_no INTO product FROM plugin_idc_core_catalog_product WHERE public_slug='{identity}' AND name_zh_cn LIKE 'E2E配置副本%' AND status_code='draft';
 IF product IS NULL THEN RETURN; END IF;
 SELECT array_agg(version_no) INTO versions FROM plugin_idc_core_catalog_product_version WHERE product_no=product AND status_code='draft' AND published_at IS NULL;
 IF versions IS NULL OR EXISTS(SELECT 1 FROM plugin_idc_core_catalog_product_version WHERE product_no=product AND (status_code<>'draft' OR published_at IS NOT NULL)) THEN RAISE EXCEPTION 'Refuse cleanup of a published or invalid fixture'; END IF;
 FOR entry IN SELECT table_name,column_name FROM information_schema.columns WHERE table_schema='public' AND table_name LIKE 'plugin_idc_core_catalog_%' AND column_name IN ('version_no','product_no') LOOP
  IF entry.column_name='version_no' THEN EXECUTE format('DELETE FROM %I WHERE version_no=ANY($1)',entry.table_name) USING versions;
  ELSE EXECUTE format('DELETE FROM %I WHERE product_no=$1',entry.table_name) USING product; END IF;
 END LOOP;
END $$; COMMIT;"""
else:
    raise SystemExit('Unknown fixture operation')
result = subprocess.run(['psql', '-X', '-qAt', '-v', 'ON_ERROR_STOP=1'], input=sql, env=env, capture_output=True, text=True)
if result.returncode:
    raise SystemExit(result.stderr)
print(result.stdout.strip())
