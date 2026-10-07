-- 078: Generic product fulfillment execution and immutable purchase quantity.
-- This iteration deliberately does not read legacy server_quantity columns.

-- Purpose: Stores Commerce Cart Item Quantity records.
-- 用途：保存交易购物车明细数量相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_cart_item_quantity (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "cart_item_no" VARCHAR(40) PRIMARY KEY,
    "quantity" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_cart_item_quantity_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_cart_item_quantity_value CHECK ("quantity" BETWEEN 1 AND 1000),
    CONSTRAINT fk_idc_cart_item_quantity_item FOREIGN KEY ("cart_item_no")
        REFERENCES plugin_idc_core_commerce_cart_item("cart_item_no") ON DELETE CASCADE
);

-- Purpose: Stores Commerce Quote Item Quantity Snapshot records.
-- 用途：保存交易报价明细数量快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_quote_item_quantity_snapshot (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "quote_item_no" VARCHAR(40) PRIMARY KEY,
    "quantity" INT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_quote_item_quantity_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_quote_item_quantity_value CHECK ("quantity" BETWEEN 1 AND 1000),
    CONSTRAINT ck_idc_quote_item_quantity_digest CHECK ("content_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT fk_idc_quote_item_quantity_item FOREIGN KEY ("quote_item_no")
        REFERENCES plugin_idc_core_commerce_quote_item_snapshot("quote_item_no") ON DELETE RESTRICT
);

-- Purpose: Stores Commerce Assisted Quote Item Quantity records.
-- 用途：保存交易协助报价明细数量相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_assisted_quote_item_quantity (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "request_item_no" VARCHAR(40) PRIMARY KEY,
    "quantity" INT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_assisted_item_quantity_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_assisted_item_quantity_value CHECK ("quantity" BETWEEN 1 AND 1000),
    CONSTRAINT ck_idc_assisted_item_quantity_digest CHECK ("content_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT fk_idc_assisted_item_quantity_item FOREIGN KEY ("request_item_no")
        REFERENCES plugin_idc_core_commerce_assisted_quote_request_item("request_item_no") ON DELETE CASCADE
);

-- Purpose: Stores Commerce Order Item Quantity Snapshot records.
-- 用途：保存交易订单明细数量快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_order_item_quantity_snapshot (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "order_item_no" VARCHAR(40) PRIMARY KEY,
    "order_no" VARCHAR(40) NOT NULL,
    "quote_item_no" VARCHAR(40) NOT NULL,
    "quantity" INT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_order_item_quantity_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_order_item_quantity_value CHECK ("quantity" BETWEEN 1 AND 1000),
    CONSTRAINT ck_idc_order_item_quantity_digest CHECK ("content_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT uk_idc_order_item_quantity_quote UNIQUE ("quote_item_no"),
    CONSTRAINT fk_idc_order_item_quantity_item FOREIGN KEY ("order_item_no")
        REFERENCES plugin_idc_core_commerce_order_item("order_item_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_order_item_quantity_order FOREIGN KEY ("order_no")
        REFERENCES plugin_idc_core_commerce_order("order_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_order_item_quantity_quote FOREIGN KEY ("quote_item_no")
        REFERENCES plugin_idc_core_commerce_quote_item_snapshot("quote_item_no") ON DELETE RESTRICT
);

-- Preserve development fixtures while moving the runtime truth to the generic
-- sidecars. Production code must never fall back to these legacy columns.
INSERT INTO plugin_idc_core_commerce_cart_item_quantity("tenant_id", "cart_item_no", "quantity")
SELECT "tenant_id", "cart_item_no", "server_quantity"
FROM plugin_idc_core_commerce_cart_item
WHERE "server_quantity" BETWEEN 1 AND 1000
ON CONFLICT ("cart_item_no") DO NOTHING;

INSERT INTO plugin_idc_core_commerce_quote_item_quantity_snapshot(
    "tenant_id", "quote_item_no", "quantity", "content_digest"
)
SELECT "tenant_id", "quote_item_no", "server_quantity",
       "content_digest"
FROM plugin_idc_core_commerce_quote_item_snapshot
WHERE "server_quantity" BETWEEN 1 AND 1000
ON CONFLICT ("quote_item_no") DO NOTHING;

INSERT INTO plugin_idc_core_commerce_assisted_quote_item_quantity("tenant_id","request_item_no","quantity","content_digest")
SELECT "tenant_id","request_item_no","server_quantity",
       "content_digest"
FROM plugin_idc_core_commerce_assisted_quote_request_item
WHERE "server_quantity" BETWEEN 1 AND 1000
ON CONFLICT ("request_item_no") DO NOTHING;

INSERT INTO plugin_idc_core_commerce_order_item_quantity_snapshot(
    "tenant_id", "order_item_no", "order_no", "quote_item_no", "quantity", "content_digest"
)
SELECT "tenant_id", "order_item_no", "order_no", "quote_item_no", "server_quantity",
       "content_digest"
FROM plugin_idc_core_commerce_order_item
WHERE "server_quantity" BETWEEN 1 AND 1000
ON CONFLICT ("order_item_no") DO NOTHING;

DO $$
DECLARE invalid_count BIGINT;
BEGIN
    SELECT count(*) INTO invalid_count
    FROM plugin_idc_core_commerce_assisted_quote_request_item parent
    LEFT JOIN plugin_idc_core_commerce_assisted_quote_item_quantity child
      ON child."tenant_id"=parent."tenant_id" AND child."request_item_no"=parent."request_item_no"
    WHERE child."request_item_no" IS NULL OR child."quantity" <> parent."server_quantity";
    IF invalid_count <> 0 THEN RAISE EXCEPTION 'assisted quote generic quantity migration incomplete: %', invalid_count; END IF;
    SELECT count(*) INTO invalid_count
    FROM plugin_idc_core_commerce_cart_item parent
    LEFT JOIN plugin_idc_core_commerce_cart_item_quantity child
      ON child."tenant_id"=parent."tenant_id" AND child."cart_item_no"=parent."cart_item_no"
    WHERE child."cart_item_no" IS NULL OR child."quantity" <> parent."server_quantity";
    IF invalid_count <> 0 THEN RAISE EXCEPTION 'cart generic quantity migration incomplete: %', invalid_count; END IF;
    SELECT count(*) INTO invalid_count
    FROM plugin_idc_core_commerce_quote_item_snapshot parent
    LEFT JOIN plugin_idc_core_commerce_quote_item_quantity_snapshot child
      ON child."tenant_id"=parent."tenant_id" AND child."quote_item_no"=parent."quote_item_no"
    WHERE child."quote_item_no" IS NULL OR child."quantity" <> parent."server_quantity";
    IF invalid_count <> 0 THEN RAISE EXCEPTION 'quote generic quantity migration incomplete: %', invalid_count; END IF;
    SELECT count(*) INTO invalid_count
    FROM plugin_idc_core_commerce_order_item parent
    LEFT JOIN plugin_idc_core_commerce_order_item_quantity_snapshot child
      ON child."tenant_id"=parent."tenant_id" AND child."order_item_no"=parent."order_item_no"
    WHERE child."order_item_no" IS NULL OR child."quantity" <> parent."server_quantity"
       OR child."order_no" <> parent."order_no" OR child."quote_item_no" <> parent."quote_item_no";
    IF invalid_count <> 0 THEN RAISE EXCEPTION 'order generic quantity migration incomplete: %', invalid_count; END IF;
END $$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_generic_quantity_shadow()
RETURNS TRIGGER AS $$
DECLARE legacy_quantity INT;
BEGIN
    IF TG_TABLE_NAME = 'plugin_idc_core_commerce_cart_item_quantity' THEN
        SELECT "server_quantity" INTO legacy_quantity FROM plugin_idc_core_commerce_cart_item
        WHERE "tenant_id"=NEW."tenant_id" AND "cart_item_no"=NEW."cart_item_no";
    ELSIF TG_TABLE_NAME = 'plugin_idc_core_commerce_assisted_quote_item_quantity' THEN
        SELECT "server_quantity" INTO legacy_quantity FROM plugin_idc_core_commerce_assisted_quote_request_item
        WHERE "tenant_id"=NEW."tenant_id" AND "request_item_no"=NEW."request_item_no";
    ELSIF TG_TABLE_NAME = 'plugin_idc_core_commerce_quote_item_quantity_snapshot' THEN
        SELECT "server_quantity" INTO legacy_quantity FROM plugin_idc_core_commerce_quote_item_snapshot
        WHERE "tenant_id"=NEW."tenant_id" AND "quote_item_no"=NEW."quote_item_no";
    ELSE
        SELECT "server_quantity" INTO legacy_quantity FROM plugin_idc_core_commerce_order_item
        WHERE "tenant_id"=NEW."tenant_id" AND "order_item_no"=NEW."order_item_no"
          AND "order_no"=NEW."order_no" AND "quote_item_no"=NEW."quote_item_no";
    END IF;
    IF legacy_quantity IS NULL OR legacy_quantity <> NEW."quantity" THEN
        RAISE EXCEPTION 'generic quantity evidence does not match the temporary legacy shadow';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_validate_cart_item_quantity ON plugin_idc_core_commerce_cart_item_quantity;
CREATE TRIGGER trg_idc_validate_cart_item_quantity
BEFORE INSERT OR UPDATE ON plugin_idc_core_commerce_cart_item_quantity
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_generic_quantity_shadow();

DROP TRIGGER IF EXISTS trg_idc_validate_assisted_item_quantity ON plugin_idc_core_commerce_assisted_quote_item_quantity;
CREATE TRIGGER trg_idc_validate_assisted_item_quantity
BEFORE INSERT OR UPDATE ON plugin_idc_core_commerce_assisted_quote_item_quantity
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_generic_quantity_shadow();

DROP TRIGGER IF EXISTS trg_idc_validate_quote_item_quantity ON plugin_idc_core_commerce_quote_item_quantity_snapshot;
CREATE TRIGGER trg_idc_validate_quote_item_quantity
BEFORE INSERT OR UPDATE ON plugin_idc_core_commerce_quote_item_quantity_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_generic_quantity_shadow();

DROP TRIGGER IF EXISTS trg_idc_validate_order_item_quantity ON plugin_idc_core_commerce_order_item_quantity_snapshot;
CREATE TRIGGER trg_idc_validate_order_item_quantity
BEFORE INSERT OR UPDATE ON plugin_idc_core_commerce_order_item_quantity_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_generic_quantity_shadow();

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_legacy_quantity_shadow()
RETURNS TRIGGER AS $$
DECLARE generic_quantity INT;
BEGIN
    IF TG_TABLE_NAME = 'plugin_idc_core_commerce_cart_item' THEN
        SELECT "quantity" INTO generic_quantity FROM plugin_idc_core_commerce_cart_item_quantity
        WHERE "tenant_id"=NEW."tenant_id" AND "cart_item_no"=NEW."cart_item_no";
    ELSIF TG_TABLE_NAME = 'plugin_idc_core_commerce_assisted_quote_request_item' THEN
        SELECT "quantity" INTO generic_quantity FROM plugin_idc_core_commerce_assisted_quote_item_quantity
        WHERE "tenant_id"=NEW."tenant_id" AND "request_item_no"=NEW."request_item_no";
    ELSIF TG_TABLE_NAME = 'plugin_idc_core_commerce_quote_item_snapshot' THEN
        SELECT "quantity" INTO generic_quantity FROM plugin_idc_core_commerce_quote_item_quantity_snapshot
        WHERE "tenant_id"=NEW."tenant_id" AND "quote_item_no"=NEW."quote_item_no";
    ELSE
        SELECT "quantity" INTO generic_quantity FROM plugin_idc_core_commerce_order_item_quantity_snapshot
        WHERE "tenant_id"=NEW."tenant_id" AND "order_item_no"=NEW."order_item_no"
          AND "order_no"=NEW."order_no" AND "quote_item_no"=NEW."quote_item_no";
    END IF;
    IF generic_quantity IS NULL OR generic_quantity <> NEW."server_quantity" THEN
        RAISE EXCEPTION 'temporary legacy quantity shadow does not match generic quantity evidence';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DO $constraint_trigger$ BEGIN
IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'plugin_idc_core_commerce_cart_item'::regclass AND conname = 'trg_idc_check_cart_item_legacy_quantity' AND contype = 't') THEN
IF NOT EXISTS (SELECT 1 FROM pg_constraint AS constraint_row JOIN pg_trigger AS trigger_row ON trigger_row.tgconstraint = constraint_row.oid WHERE constraint_row.conrelid = 'plugin_idc_core_commerce_cart_item'::regclass AND constraint_row.conname = 'trg_idc_check_cart_item_legacy_quantity' AND constraint_row.contype = 't' AND trigger_row.tgrelid = constraint_row.conrelid AND NOT trigger_row.tgisinternal AND trigger_row.tgname = 'trg_idc_check_cart_item_legacy_quantity' AND trigger_row.tgfoid = 'plugin_idc_core_validate_legacy_quantity_shadow()'::regprocedure) THEN RAISE EXCEPTION 'constraint trigger trg_idc_check_cart_item_legacy_quantity catalog contract is invalid' USING ERRCODE='23514'; END IF;
ELSE CREATE CONSTRAINT TRIGGER trg_idc_check_cart_item_legacy_quantity AFTER INSERT OR UPDATE ON plugin_idc_core_commerce_cart_item DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_legacy_quantity_shadow(); END IF;
END $constraint_trigger$;
DO $constraint_trigger$ BEGIN
IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'plugin_idc_core_commerce_assisted_quote_request_item'::regclass AND conname = 'trg_idc_check_assisted_item_legacy_quantity' AND contype = 't') THEN
IF NOT EXISTS (SELECT 1 FROM pg_constraint AS constraint_row JOIN pg_trigger AS trigger_row ON trigger_row.tgconstraint = constraint_row.oid WHERE constraint_row.conrelid = 'plugin_idc_core_commerce_assisted_quote_request_item'::regclass AND constraint_row.conname = 'trg_idc_check_assisted_item_legacy_quantity' AND constraint_row.contype = 't' AND trigger_row.tgrelid = constraint_row.conrelid AND NOT trigger_row.tgisinternal AND trigger_row.tgname = 'trg_idc_check_assisted_item_legacy_quantity' AND trigger_row.tgfoid = 'plugin_idc_core_validate_legacy_quantity_shadow()'::regprocedure) THEN RAISE EXCEPTION 'constraint trigger trg_idc_check_assisted_item_legacy_quantity catalog contract is invalid' USING ERRCODE='23514'; END IF;
ELSE CREATE CONSTRAINT TRIGGER trg_idc_check_assisted_item_legacy_quantity AFTER INSERT OR UPDATE ON plugin_idc_core_commerce_assisted_quote_request_item DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_legacy_quantity_shadow(); END IF;
END $constraint_trigger$;
DO $constraint_trigger$ BEGIN
IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'plugin_idc_core_commerce_quote_item_snapshot'::regclass AND conname = 'trg_idc_check_quote_item_legacy_quantity' AND contype = 't') THEN
IF NOT EXISTS (SELECT 1 FROM pg_constraint AS constraint_row JOIN pg_trigger AS trigger_row ON trigger_row.tgconstraint = constraint_row.oid WHERE constraint_row.conrelid = 'plugin_idc_core_commerce_quote_item_snapshot'::regclass AND constraint_row.conname = 'trg_idc_check_quote_item_legacy_quantity' AND constraint_row.contype = 't' AND trigger_row.tgrelid = constraint_row.conrelid AND NOT trigger_row.tgisinternal AND trigger_row.tgname = 'trg_idc_check_quote_item_legacy_quantity' AND trigger_row.tgfoid = 'plugin_idc_core_validate_legacy_quantity_shadow()'::regprocedure) THEN RAISE EXCEPTION 'constraint trigger trg_idc_check_quote_item_legacy_quantity catalog contract is invalid' USING ERRCODE='23514'; END IF;
ELSE CREATE CONSTRAINT TRIGGER trg_idc_check_quote_item_legacy_quantity AFTER INSERT OR UPDATE ON plugin_idc_core_commerce_quote_item_snapshot DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_legacy_quantity_shadow(); END IF;
END $constraint_trigger$;
DO $constraint_trigger$ BEGIN
IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'plugin_idc_core_commerce_order_item'::regclass AND conname = 'trg_idc_check_order_item_legacy_quantity' AND contype = 't') THEN
IF NOT EXISTS (SELECT 1 FROM pg_constraint AS constraint_row JOIN pg_trigger AS trigger_row ON trigger_row.tgconstraint = constraint_row.oid WHERE constraint_row.conrelid = 'plugin_idc_core_commerce_order_item'::regclass AND constraint_row.conname = 'trg_idc_check_order_item_legacy_quantity' AND constraint_row.contype = 't' AND trigger_row.tgrelid = constraint_row.conrelid AND NOT trigger_row.tgisinternal AND trigger_row.tgname = 'trg_idc_check_order_item_legacy_quantity' AND trigger_row.tgfoid = 'plugin_idc_core_validate_legacy_quantity_shadow()'::regprocedure) THEN RAISE EXCEPTION 'constraint trigger trg_idc_check_order_item_legacy_quantity catalog contract is invalid' USING ERRCODE='23514'; END IF;
ELSE CREATE CONSTRAINT TRIGGER trg_idc_check_order_item_legacy_quantity AFTER INSERT OR UPDATE ON plugin_idc_core_commerce_order_item DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_legacy_quantity_shadow(); END IF;
END $constraint_trigger$;

-- Purpose: Stores Catalog Product Version Fulfillment Policy records.
-- 用途：保存目录商品版本履约策略相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_product_version_fulfillment_policy (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "version_no" VARCHAR(40) PRIMARY KEY,
    "strategy_code" VARCHAR(32) NOT NULL,
    "granularity_code" VARCHAR(32) NOT NULL,
    "provision_method" VARCHAR(128) NOT NULL DEFAULT '',
    "query_method" VARCHAR(128) NOT NULL DEFAULT '',
    "maximum_attempts" INT NOT NULL DEFAULT 10,
    "retry_base_seconds" INT NOT NULL DEFAULT 30,
    "policy_digest" CHAR(64) NOT NULL,
    "created_by" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_fulfillment_policy_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_fulfillment_policy_strategy CHECK (
        "strategy_code" IN ('manual','approval_then_plugin','payment_confirmed_plugin')
    ),
    CONSTRAINT ck_idc_fulfillment_policy_granularity CHECK (
        "granularity_code" IN ('per_order_item','per_quantity_unit')
    ),
    CONSTRAINT ck_idc_fulfillment_policy_methods CHECK (
        ("strategy_code" = 'manual' AND "provision_method" = '' AND "query_method" = '') OR
        ("strategy_code" <> 'manual'
         AND "provision_method" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$'
         AND "query_method" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$')
    ),
    CONSTRAINT ck_idc_fulfillment_policy_retry CHECK (
        "maximum_attempts" = 10 AND "retry_base_seconds" = 30
    ),
    CONSTRAINT ck_idc_fulfillment_policy_digest CHECK ("policy_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_idc_fulfillment_policy_actor CHECK ("created_by" > 0),
    CONSTRAINT fk_idc_fulfillment_policy_version FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version("version_no") ON DELETE RESTRICT
);

-- Purpose: Stores Product Fulfillment Intent records.
-- 用途：保存商品履约意图相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_fulfillment_intent (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "intent_no" VARCHAR(40) PRIMARY KEY,
    "settlement_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "order_item_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "product_name_zh_cn" VARCHAR(200) NOT NULL,
    "product_name_en_us" VARCHAR(200) NOT NULL,
    "quantity" INT NOT NULL,
    "unit_sequence" INT NOT NULL,
    "strategy_code" VARCHAR(32) NOT NULL,
    "granularity_code" VARCHAR(32) NOT NULL,
    "initial_status_code" VARCHAR(32) NOT NULL,
    "status_code" VARCHAR(32) NOT NULL,
    "configuration_snapshot_no" VARCHAR(40) NOT NULL DEFAULT '',
    "interface_instance_no" VARCHAR(40) NOT NULL DEFAULT '',
    "interface_configuration_revision" BIGINT NOT NULL DEFAULT 0,
    "plugin_id" VARCHAR(64) NOT NULL DEFAULT '',
    "plugin_release_version" VARCHAR(32) NOT NULL DEFAULT '',
    "contract_version" VARCHAR(32) NOT NULL DEFAULT '',
    "release_digest" CHAR(64) NOT NULL DEFAULT '',
    "declaration_digest" CHAR(64) NOT NULL DEFAULT '',
    "provision_method" VARCHAR(128) NOT NULL DEFAULT '',
    "query_method" VARCHAR(128) NOT NULL DEFAULT '',
    "maximum_attempts" INT NOT NULL DEFAULT 10,
    "retry_base_seconds" INT NOT NULL DEFAULT 30,
    "policy_digest" CHAR(64) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "financially_settled_at" TIMESTAMPTZ NOT NULL,
    "approved_by" BIGINT,
    "approved_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_product_fulfillment_unit UNIQUE ("order_item_no", "unit_sequence"),
    CONSTRAINT ck_idc_product_fulfillment_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_product_fulfillment_quantity CHECK (
        "quantity" BETWEEN 1 AND 1000 AND
        (("granularity_code" = 'per_order_item' AND "unit_sequence" = 1) OR
         ("granularity_code" = 'per_quantity_unit' AND "unit_sequence" BETWEEN 1 AND "quantity"))
    ),
    CONSTRAINT ck_idc_product_fulfillment_strategy CHECK (
        "strategy_code" IN ('manual','approval_then_plugin','payment_confirmed_plugin')
        AND "granularity_code" IN ('per_order_item','per_quantity_unit')
    ),
    CONSTRAINT ck_idc_product_fulfillment_status CHECK (
        "initial_status_code" IN ('manual_pending','review_pending','gate_pending','queued') AND
        "status_code" IN ('manual_pending','review_pending','gate_pending','queued','processing','completed','incomplete','canceled')
    ),
    CONSTRAINT ck_idc_product_fulfillment_approval CHECK (
        ("approved_by" IS NULL AND "approved_at" IS NULL) OR
        ("strategy_code" = 'approval_then_plugin' AND "approved_by" > 0 AND "approved_at" IS NOT NULL
         AND "approved_at" >= "financially_settled_at"
         AND "status_code" IN ('queued','processing','manual_pending','completed','incomplete'))
    ),
    CONSTRAINT ck_idc_product_fulfillment_plugin_evidence CHECK (
        ("strategy_code" = 'manual' AND "provision_method" = '' AND "query_method" = '' AND (
            ("interface_instance_no" = '' AND "plugin_id" = '' AND "interface_configuration_revision" = 0
             AND "configuration_snapshot_no" = '' AND "plugin_release_version" = '' AND "contract_version" = ''
             AND "release_digest" = '' AND "declaration_digest" = '') OR
            ("interface_instance_no" ~ '^pfi_[0-9a-f]{32}$' AND "interface_configuration_revision" > 0
             AND "configuration_snapshot_no" <> ''
             AND "plugin_id" ~ '^[a-z0-9]+(-[a-z0-9]+)*$'
             AND btrim("plugin_release_version") <> '' AND btrim("contract_version") <> ''
             AND "release_digest" ~ '^[0-9a-f]{64}$' AND "declaration_digest" ~ '^[0-9a-f]{64}$')
         )) OR
        ("strategy_code" <> 'manual' AND "interface_instance_no" ~ '^pfi_[0-9a-f]{32}$'
         AND "interface_configuration_revision" > 0 AND "configuration_snapshot_no" <> ''
         AND "plugin_id" ~ '^[a-z0-9]+(-[a-z0-9]+)*$'
         AND btrim("plugin_release_version") <> '' AND btrim("contract_version") <> ''
         AND "provision_method" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$'
         AND "query_method" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$'
         AND "release_digest" ~ '^[0-9a-f]{64}$' AND "declaration_digest" ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT ck_idc_product_fulfillment_digests CHECK (
        "policy_digest" ~ '^[0-9a-f]{64}$' AND "evidence_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_product_fulfillment_retry CHECK (
        "maximum_attempts" = 10 AND "retry_base_seconds" = 30
    ),
    CONSTRAINT fk_idc_product_fulfillment_order FOREIGN KEY ("order_no")
        REFERENCES plugin_idc_core_commerce_order("order_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_product_fulfillment_order_item FOREIGN KEY ("order_item_no")
        REFERENCES plugin_idc_core_commerce_order_item("order_item_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_product_fulfillment_settlement_owner FOREIGN KEY (
        "settlement_no", "order_no", "invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_financial_settlement(
        "settlement_no", "order_no", "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_product_fulfillment_intent_order
    ON plugin_idc_core_product_fulfillment_intent("tenant_id", "order_no", "order_item_no", "unit_sequence");
CREATE INDEX IF NOT EXISTS idx_idc_product_fulfillment_intent_work
    ON plugin_idc_core_product_fulfillment_intent("tenant_id", "status_code", "updated_at", "intent_no");
CREATE INDEX IF NOT EXISTS idx_idc_product_fulfillment_intent_customer_cursor
    ON plugin_idc_core_product_fulfillment_intent("tenant_id", "customer_no", "intent_no");

-- Purpose: Stores Product Fulfillment Task records.
-- 用途：保存商品履约任务相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_fulfillment_task (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "task_no" VARCHAR(40) PRIMARY KEY,
    "intent_no" VARCHAR(40) NOT NULL UNIQUE,
    "status_code" VARCHAR(32) NOT NULL DEFAULT 'pending_query',
    "phase_code" VARCHAR(32) NOT NULL DEFAULT 'pre_submit_query',
    "attempt_count" INT NOT NULL DEFAULT 0,
    "query_attempt_count" INT NOT NULL DEFAULT 0,
    "submit_attempt_count" INT NOT NULL DEFAULT 0,
    "maximum_attempts" INT NOT NULL,
    "retry_base_seconds" INT NOT NULL,
    "next_attempt_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "lease_token" VARCHAR(64) NOT NULL DEFAULT '',
    "lease_expires_at" TIMESTAMPTZ,
    "external_reference" VARCHAR(256) NOT NULL DEFAULT '',
    "last_result_code" VARCHAR(64) NOT NULL DEFAULT '',
    "takeover_requested" BOOLEAN NOT NULL DEFAULT FALSE,
    "cancel_requested" BOOLEAN NOT NULL DEFAULT FALSE,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_product_fulfillment_task_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_product_fulfillment_task_status CHECK (
        "status_code" IN ('pending_query','querying','pending_submit','submitting','accepted','running',
                        'retry_wait','succeeded','failed','result_unknown','manual_takeover','cancellation_attention','canceled')
    ),
    CONSTRAINT ck_idc_product_fulfillment_task_phase CHECK (
        "phase_code" IN ('pre_submit_query','submit_allowed','post_submit_query','frozen_unknown')
    ),
    CONSTRAINT ck_idc_product_fulfillment_task_retry CHECK (
        "maximum_attempts" = 10 AND "retry_base_seconds" = 30
        AND "query_attempt_count" BETWEEN 0 AND 10
        AND "submit_attempt_count" BETWEEN 0 AND 3
        AND "attempt_count" = "query_attempt_count" + "submit_attempt_count"
    ),
    CONSTRAINT ck_idc_product_fulfillment_task_lease CHECK (
        ("lease_token" = '' AND "lease_expires_at" IS NULL) OR
        ("lease_token" ~ '^pfl_[0-9a-f]{32}$' AND "lease_expires_at" IS NOT NULL)
    ),
	CONSTRAINT ck_idc_product_fulfillment_task_takeover CHECK (
		"status_code" <> 'manual_takeover' OR "takeover_requested"
	),
	CONSTRAINT ck_idc_product_fulfillment_task_cancel CHECK (
		"status_code" NOT IN ('cancellation_attention','canceled') OR "cancel_requested"
	),
	CONSTRAINT ck_idc_product_fulfillment_task_external_reference CHECK (
		"external_reference"='' OR "external_reference" ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,255}$'
	),
    CONSTRAINT fk_idc_product_fulfillment_task_intent FOREIGN KEY ("intent_no")
        REFERENCES plugin_idc_core_product_fulfillment_intent("intent_no") ON DELETE RESTRICT
);

-- SQL 078 may be replayed while this not-yet-released iteration is under
-- development. Bring an earlier 078 shape forward without relying on a new
-- migration number or leaving stale CHECK constraints behind.
ALTER TABLE plugin_idc_core_product_fulfillment_task
    ADD COLUMN IF NOT EXISTS "cancel_requested" BOOLEAN NOT NULL DEFAULT FALSE;
DO $migration$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='plugin_idc_core_product_fulfillment_task'::regclass AND conname='ck_idc_product_fulfillment_task_status' AND pg_get_constraintdef(oid) NOT LIKE '%cancellation_attention%') THEN
        ALTER TABLE plugin_idc_core_product_fulfillment_task DROP CONSTRAINT ck_idc_product_fulfillment_task_status;
        ALTER TABLE plugin_idc_core_product_fulfillment_task ADD CONSTRAINT ck_idc_product_fulfillment_task_status CHECK ("status_code" IN ('pending_query','querying','pending_submit','submitting','accepted','running','retry_wait','succeeded','failed','result_unknown','manual_takeover','cancellation_attention','canceled'));
    END IF;
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='plugin_idc_core_product_fulfillment_task'::regclass AND conname='ck_idc_product_fulfillment_task_takeover' AND pg_get_constraintdef(oid) NOT LIKE '%OR takeover_requested%') THEN
        ALTER TABLE plugin_idc_core_product_fulfillment_task DROP CONSTRAINT ck_idc_product_fulfillment_task_takeover;
        ALTER TABLE plugin_idc_core_product_fulfillment_task ADD CONSTRAINT ck_idc_product_fulfillment_task_takeover CHECK ("status_code" <> 'manual_takeover' OR "takeover_requested");
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='plugin_idc_core_product_fulfillment_task'::regclass AND conname='ck_idc_product_fulfillment_task_cancel') THEN
        ALTER TABLE plugin_idc_core_product_fulfillment_task ADD CONSTRAINT ck_idc_product_fulfillment_task_cancel CHECK ("status_code" NOT IN ('cancellation_attention','canceled') OR "cancel_requested");
    ELSIF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='plugin_idc_core_product_fulfillment_task'::regclass AND conname='ck_idc_product_fulfillment_task_cancel' AND pg_get_constraintdef(oid) NOT LIKE '%OR cancel_requested%') THEN
        ALTER TABLE plugin_idc_core_product_fulfillment_task DROP CONSTRAINT ck_idc_product_fulfillment_task_cancel;
        ALTER TABLE plugin_idc_core_product_fulfillment_task ADD CONSTRAINT ck_idc_product_fulfillment_task_cancel CHECK ("status_code" NOT IN ('cancellation_attention','canceled') OR "cancel_requested");
    END IF;
END $migration$;

CREATE INDEX IF NOT EXISTS idx_idc_product_fulfillment_task_claim
    ON plugin_idc_core_product_fulfillment_task("tenant_id", "status_code", "next_attempt_at", "task_no");

-- Purpose: Stores Product Fulfillment Attempt records.
-- 用途：保存商品履约尝试相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_fulfillment_attempt (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "task_no" VARCHAR(40) NOT NULL,
    "attempt_no" INT NOT NULL,
    "action_code" VARCHAR(16) NOT NULL,
    "started_at" TIMESTAMPTZ NOT NULL,
    "finished_at" TIMESTAMPTZ,
    "result_code" VARCHAR(64) NOT NULL DEFAULT '',
    "evidence_digest" CHAR(64) NOT NULL,
    "result_evidence_digest" VARCHAR(64) NOT NULL DEFAULT '',
    "result_status_code" VARCHAR(32) NOT NULL DEFAULT '',
    "external_reference" VARCHAR(256) NOT NULL DEFAULT '',
    "safe_result_evidence" JSONB,
    "safe_summary" VARCHAR(512) NOT NULL DEFAULT '',
    PRIMARY KEY ("task_no", "attempt_no"),
    CONSTRAINT ck_idc_product_fulfillment_attempt_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_product_fulfillment_attempt_action CHECK ("attempt_no" > 0 AND "action_code" IN ('query','submit')),
    CONSTRAINT ck_idc_product_fulfillment_attempt_digest CHECK (
        "evidence_digest" ~ '^[0-9a-f]{64}$' AND
        ("result_evidence_digest" = '' OR "result_evidence_digest" ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT ck_idc_product_fulfillment_attempt_completion CHECK (
        ("finished_at" IS NULL AND "result_code" = '' AND "result_evidence_digest" = '' AND "result_status_code" = ''
         AND "external_reference" = '' AND "safe_result_evidence" IS NULL AND "safe_summary" = '') OR
        ("finished_at" IS NOT NULL AND "result_code" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$'
         AND "result_evidence_digest" ~ '^[0-9a-f]{64}$'
         AND "result_status_code" IN ('not_found','not_accepted','accepted','running','succeeded','failed','result_unknown')
         AND ("external_reference" = '' OR "external_reference" ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,255}$')
         AND (("result_status_code" IN ('accepted','running') AND "external_reference" <> '')
              OR ("result_status_code" IN ('not_found','not_accepted') AND "external_reference" = '')
              OR "result_status_code" IN ('succeeded','failed','result_unknown'))
         AND "safe_result_evidence" IS NOT NULL AND jsonb_typeof("safe_result_evidence") = 'object'
         AND octet_length("safe_result_evidence"::TEXT) <= 65536)
    ),
    CONSTRAINT ck_idc_product_fulfillment_attempt_action_result CHECK (
        NOT ("action_code"='query' AND "result_status_code"='not_accepted')
        AND NOT ("action_code"='submit' AND "result_status_code"='not_found')
    ),
    CONSTRAINT ck_idc_product_fulfillment_attempt_time CHECK ("finished_at" IS NULL OR "finished_at" >= "started_at"),
    CONSTRAINT ck_idc_product_fulfillment_attempt_summary CHECK (octet_length("safe_summary") <= 2048),
    CONSTRAINT fk_idc_product_fulfillment_attempt_task FOREIGN KEY ("task_no")
        REFERENCES plugin_idc_core_product_fulfillment_task("task_no") ON DELETE RESTRICT
);

-- Provider evidence is owner-private encrypted material. Ordinary management
-- and customer projections never join this table. The digest is over the
-- plaintext evidence and the AEAD additional data binds task + attempt.
-- Purpose: Stores Product Fulfillment Attempt Provider Evidence records.
-- 用途：保存商品履约尝试供应商证据相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_fulfillment_attempt_provider_evidence (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "task_no" VARCHAR(40) NOT NULL,
    "attempt_no" INT NOT NULL,
    "key_id" VARCHAR(64) NOT NULL,
    "nonce" BYTEA NOT NULL,
    "ciphertext" BYTEA NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY("task_no","attempt_no"),
    CONSTRAINT ck_idc_product_fulfillment_provider_evidence_tenant CHECK("tenant_id"=0),
    CONSTRAINT ck_idc_product_fulfillment_provider_evidence_shape CHECK(
        "key_id" ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$'
        AND octet_length("nonce")=12
        AND octet_length("ciphertext") BETWEEN 17 AND 65552
        AND "evidence_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT fk_idc_product_fulfillment_provider_evidence_attempt FOREIGN KEY("task_no","attempt_no")
        REFERENCES plugin_idc_core_product_fulfillment_attempt("task_no","attempt_no") ON DELETE RESTRICT
);

-- Purpose: Stores Product Fulfillment Manual Conclusion records.
-- 用途：保存商品履约人工结论相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_fulfillment_manual_conclusion (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "conclusion_no" VARCHAR(40) PRIMARY KEY,
    "intent_no" VARCHAR(40) NOT NULL UNIQUE,
    "outcome_code" VARCHAR(16) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "note" VARCHAR(1000) NOT NULL DEFAULT '',
    "actor_user_id" BIGINT NOT NULL,
    "intent_version" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_product_fulfillment_conclusion_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_product_fulfillment_conclusion_outcome CHECK ("outcome_code" IN ('completed','unable')),
    CONSTRAINT ck_idc_product_fulfillment_conclusion_reason CHECK (
        "reason_code" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$' AND "actor_user_id" > 0 AND "intent_version" > 0
        AND length("note") BETWEEN 10 AND 512
    ),
    CONSTRAINT fk_idc_product_fulfillment_conclusion_intent FOREIGN KEY ("intent_no")
        REFERENCES plugin_idc_core_product_fulfillment_intent("intent_no") ON DELETE RESTRICT
);

-- Purpose: Stores Product Fulfillment Reason Option records.
-- 用途：保存商品履约原因选项相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_fulfillment_reason_option (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "action_code" VARCHAR(32) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "label_zh_cn" VARCHAR(128) NOT NULL,
    "label_en_us" VARCHAR(128) NOT NULL,
    "sort_order" INT NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    PRIMARY KEY ("action_code","reason_code"),
    CONSTRAINT ck_idc_product_fulfillment_reason_tenant CHECK ("tenant_id"=0),
    CONSTRAINT ck_idc_product_fulfillment_reason_action CHECK (
        "action_code" IN ('review_approve','review_reject','takeover','manual_conclude_completed','manual_conclude_unable')
    ),
    CONSTRAINT ck_idc_product_fulfillment_reason_code CHECK (
        "reason_code" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$' AND
        btrim("label_zh_cn")<>'' AND btrim("label_en_us")<>'' AND "sort_order" BETWEEN 1 AND 1000 AND
        "status_code" IN ('active','inactive')
    )
);

INSERT INTO plugin_idc_core_product_fulfillment_reason_option
    ("tenant_id","action_code","reason_code","label_zh_cn","label_en_us","sort_order","status_code")
VALUES
    (0,'review_approve','review_verified','审核确认可执行','Reviewed and ready',10,'active'),
    (0,'review_reject','manual_processing_required','需要人工处理','Manual processing required',10,'active'),
    (0,'review_reject','fulfillment_evidence_incomplete','履约资料不完整','Fulfillment evidence is incomplete',20,'active'),
    (0,'takeover','provider_result_unknown','供应商结果未知','Provider result is unknown',10,'active'),
    (0,'takeover','provider_processing_exception','供应商处理异常','Provider processing exception',20,'active'),
    (0,'manual_conclude_completed','manual_fulfillment_completed','人工履约已完成','Manual fulfillment completed',10,'active'),
    (0,'manual_conclude_unable','manual_fulfillment_unable','无法完成履约','Unable to complete fulfillment',10,'active')
ON CONFLICT ("action_code","reason_code") DO UPDATE SET
    "label_zh_cn"=EXCLUDED."label_zh_cn","label_en_us"=EXCLUDED."label_en_us",
    "sort_order"=EXCLUDED."sort_order","status_code"=EXCLUDED."status_code";

-- Purpose: Stores Product Fulfillment Command records.
-- 用途：保存商品履约指令相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_fulfillment_command (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "command_no" VARCHAR(40) PRIMARY KEY,
    "intent_no" VARCHAR(40) NOT NULL,
    "action_code" VARCHAR(32) NOT NULL,
    "idempotency_key" VARCHAR(128) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "reason_label_zh_cn" VARCHAR(128) NOT NULL,
    "reason_label_en_us" VARCHAR(128) NOT NULL,
    "safe_note" VARCHAR(512) NOT NULL DEFAULT '',
    "actor_user_id" BIGINT NOT NULL,
    "resulting_version" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_product_fulfillment_command UNIQUE ("action_code","idempotency_key"),
    CONSTRAINT ck_idc_product_fulfillment_command_tenant CHECK ("tenant_id"=0),
    CONSTRAINT ck_idc_product_fulfillment_command_action CHECK (
        "action_code" IN ('review_approve','review_reject','takeover','manual_conclude_completed','manual_conclude_unable')
    ),
    CONSTRAINT ck_idc_product_fulfillment_command_evidence CHECK (
        length("idempotency_key") BETWEEN 16 AND 128 AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND "reason_code" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$'
        AND btrim("reason_label_zh_cn")<>'' AND btrim("reason_label_en_us")<>''
        AND "actor_user_id">0 AND "resulting_version">0 AND octet_length("safe_note")<=2048
    ),
    CONSTRAINT fk_idc_product_fulfillment_command_intent FOREIGN KEY ("intent_no")
        REFERENCES plugin_idc_core_product_fulfillment_intent("intent_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_product_fulfillment_command_intent_history
    ON plugin_idc_core_product_fulfillment_command("tenant_id","intent_no","created_at","command_no");

-- Purpose: Stores Product Fulfillment System Transition records.
-- 用途：保存商品履约系统流转相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_fulfillment_system_transition (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "transition_no" VARCHAR(40) PRIMARY KEY,
    "intent_no" VARCHAR(40) NOT NULL,
    "event_code" VARCHAR(32) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "reason_label_zh_cn" VARCHAR(128) NOT NULL,
    "reason_label_en_us" VARCHAR(128) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_product_fulfillment_system_transition UNIQUE("intent_no","event_code"),
    CONSTRAINT ck_idc_product_fulfillment_system_transition CHECK (
        "tenant_id"=0 AND "event_code" IN ('moved_to_manual','cancelled')
        AND "reason_code" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$'
        AND btrim("reason_label_zh_cn")<>'' AND btrim("reason_label_en_us")<>''
    ),
    CONSTRAINT fk_idc_product_fulfillment_system_transition_intent FOREIGN KEY("intent_no")
        REFERENCES plugin_idc_core_product_fulfillment_intent("intent_no") ON DELETE RESTRICT
);

DO $migration$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='plugin_idc_core_product_fulfillment_system_transition'::regclass AND conname='uk_idc_product_fulfillment_system_transition' AND pg_get_constraintdef(oid) NOT LIKE '%event_code%') THEN
        ALTER TABLE plugin_idc_core_product_fulfillment_system_transition DROP CONSTRAINT uk_idc_product_fulfillment_system_transition;
        ALTER TABLE plugin_idc_core_product_fulfillment_system_transition ADD CONSTRAINT uk_idc_product_fulfillment_system_transition UNIQUE("intent_no","event_code");
    END IF;
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='plugin_idc_core_product_fulfillment_system_transition'::regclass AND conname='ck_idc_product_fulfillment_system_transition' AND pg_get_constraintdef(oid) NOT LIKE '%cancelled%') THEN
        ALTER TABLE plugin_idc_core_product_fulfillment_system_transition DROP CONSTRAINT ck_idc_product_fulfillment_system_transition;
        ALTER TABLE plugin_idc_core_product_fulfillment_system_transition ADD CONSTRAINT ck_idc_product_fulfillment_system_transition CHECK ("tenant_id"=0 AND "event_code" IN ('moved_to_manual','cancelled') AND "reason_code" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$' AND btrim("reason_label_zh_cn")<>'' AND btrim("reason_label_en_us")<>'');
    END IF;
END $migration$;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_fulfillment_immutable_evidence()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'UPDATE' OR TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'product fulfillment frozen evidence is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_guard_fulfillment_provider_evidence ON plugin_idc_core_product_fulfillment_attempt_provider_evidence;
CREATE TRIGGER trg_idc_guard_fulfillment_provider_evidence
BEFORE UPDATE OR DELETE ON plugin_idc_core_product_fulfillment_attempt_provider_evidence
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_fulfillment_immutable_evidence();

DROP TRIGGER IF EXISTS trg_idc_guard_quote_item_quantity ON plugin_idc_core_commerce_quote_item_quantity_snapshot;
CREATE TRIGGER trg_idc_guard_quote_item_quantity
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_quote_item_quantity_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_fulfillment_immutable_evidence();

DROP TRIGGER IF EXISTS trg_idc_guard_order_item_quantity ON plugin_idc_core_commerce_order_item_quantity_snapshot;
CREATE TRIGGER trg_idc_guard_order_item_quantity
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_order_item_quantity_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_fulfillment_immutable_evidence();

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_fulfillment_task()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN RAISE EXCEPTION 'product fulfillment task evidence is immutable'; END IF;
	IF TG_OP = 'INSERT' THEN
		IF NEW."status_code" <> 'pending_query' OR NEW."phase_code" <> 'pre_submit_query'
		   OR NEW."attempt_count" <> 0 OR NEW."query_attempt_count" <> 0 OR NEW."submit_attempt_count" <> 0
		   OR NEW."lease_token" <> '' OR NEW."lease_expires_at" IS NOT NULL OR NEW."external_reference" <> ''
		   OR NEW."last_result_code" <> '' OR NEW."takeover_requested" OR NEW."cancel_requested"
		   OR NEW."version" <> 1 THEN
			RAISE EXCEPTION 'initial product fulfillment task state is invalid';
		END IF;
		RETURN NEW;
	END IF;
    IF NEW."tenant_id" IS DISTINCT FROM OLD."tenant_id" OR NEW."task_no" IS DISTINCT FROM OLD."task_no"
       OR NEW."intent_no" IS DISTINCT FROM OLD."intent_no" OR NEW."maximum_attempts" IS DISTINCT FROM OLD."maximum_attempts"
       OR NEW."retry_base_seconds" IS DISTINCT FROM OLD."retry_base_seconds" OR NEW."created_at" IS DISTINCT FROM OLD."created_at" THEN
        RAISE EXCEPTION 'product fulfillment task frozen evidence is immutable';
    END IF;
    IF NEW."version" <> OLD."version" + 1 OR NEW."updated_at" < OLD."updated_at"
       OR NEW."attempt_count" < OLD."attempt_count" OR NEW."query_attempt_count" < OLD."query_attempt_count"
       OR NEW."submit_attempt_count" < OLD."submit_attempt_count"
       OR NEW."query_attempt_count" - OLD."query_attempt_count" NOT IN (0,1)
       OR NEW."submit_attempt_count" - OLD."submit_attempt_count" NOT IN (0,1)
       OR (NEW."query_attempt_count" - OLD."query_attempt_count") + (NEW."submit_attempt_count" - OLD."submit_attempt_count") NOT IN (0,1) THEN
        RAISE EXCEPTION 'product fulfillment task version or counter transition is invalid';
    END IF;
	IF (NEW."query_attempt_count" = OLD."query_attempt_count" + 1) IS DISTINCT FROM (NEW."status_code"='querying' AND OLD."status_code"<>'querying') OR
	   (NEW."submit_attempt_count" = OLD."submit_attempt_count" + 1) IS DISTINCT FROM (NEW."status_code"='submitting' AND OLD."status_code"<>'submitting') THEN
		RAISE EXCEPTION 'product fulfillment task counters do not match the claimed action';
	END IF;
	IF OLD."takeover_requested" AND NOT NEW."takeover_requested" THEN
		RAISE EXCEPTION 'product fulfillment takeover request is immutable';
	END IF;
	IF OLD."cancel_requested" AND NOT NEW."cancel_requested" THEN
		RAISE EXCEPTION 'product fulfillment cancellation request is immutable';
	END IF;
	IF NEW."cancel_requested" IS DISTINCT FROM OLD."cancel_requested" AND NOT (
		NOT OLD."cancel_requested" AND NEW."cancel_requested" AND NEW."status_code" IN ('querying','submitting','cancellation_attention','canceled')
	) THEN RAISE EXCEPTION 'product fulfillment cancellation transition is invalid'; END IF;
	IF NEW."takeover_requested" IS DISTINCT FROM OLD."takeover_requested" AND NOT (
		NOT OLD."takeover_requested" AND NEW."takeover_requested" AND
		((OLD."status_code" IN ('querying','submitting') AND NEW."status_code"=OLD."status_code") OR NEW."status_code"='manual_takeover')
	) THEN RAISE EXCEPTION 'product fulfillment takeover transition is invalid'; END IF;
	IF OLD."status_code" IN ('succeeded','failed','manual_takeover','cancellation_attention','canceled') OR
	   (OLD."status_code"='result_unknown' AND NEW."status_code"<>'manual_takeover') THEN
		RAISE EXCEPTION 'terminal product fulfillment task evidence is immutable';
	END IF;
	IF NEW."status_code" IS NOT DISTINCT FROM OLD."status_code" AND NOT (
		OLD."status_code" IN ('querying','submitting') AND
		((NOT OLD."takeover_requested" AND NEW."takeover_requested") OR (NOT OLD."cancel_requested" AND NEW."cancel_requested"))
	) THEN RAISE EXCEPTION 'product fulfillment task update must advance its state'; END IF;
    IF NEW."status_code" IS DISTINCT FROM OLD."status_code" AND NOT (
        (OLD."status_code" IN ('pending_query','retry_wait','accepted','running') AND NEW."status_code"='querying') OR
        (OLD."status_code"='pending_submit' AND NEW."status_code"='submitting') OR
        (OLD."status_code" IN ('querying','submitting') AND NEW."status_code" IN ('pending_submit','retry_wait','accepted','running','succeeded','failed','result_unknown','manual_takeover','cancellation_attention','canceled')) OR
        (OLD."status_code" IN ('pending_query','pending_submit','retry_wait','accepted','running','result_unknown') AND NEW."status_code" IN ('manual_takeover','cancellation_attention','canceled'))
    ) THEN RAISE EXCEPTION 'product fulfillment task status transition is invalid'; END IF;
    IF (NEW."status_code" IN ('querying','submitting')) <> (NEW."lease_token" <> '' AND NEW."lease_expires_at" IS NOT NULL) THEN
        RAISE EXCEPTION 'product fulfillment task lease does not match its state';
    END IF;
	IF NEW."status_code" IN ('querying','submitting') AND NEW."lease_expires_at" <= CURRENT_TIMESTAMP THEN
		RAISE EXCEPTION 'product fulfillment task lease must expire in the future';
	END IF;
    IF (NEW."status_code"='pending_query' AND NEW."phase_code" <> 'pre_submit_query') OR
	   (NEW."status_code" IN ('pending_submit','submitting') AND NEW."phase_code" <> 'submit_allowed') OR
       (NEW."status_code" IN ('accepted','running','succeeded') AND NEW."phase_code" <> 'post_submit_query') OR
	   (NEW."status_code"='retry_wait' AND NEW."phase_code" NOT IN ('pre_submit_query','post_submit_query')) OR
	   (NEW."status_code" IN ('result_unknown','manual_takeover','cancellation_attention','canceled') AND NEW."phase_code" <> 'frozen_unknown') OR
	   (NEW."status_code"='querying' AND NEW."phase_code" NOT IN ('pre_submit_query','post_submit_query')) OR
	   (NEW."status_code"='failed' AND NEW."phase_code" NOT IN ('pre_submit_query','post_submit_query')) THEN
        RAISE EXCEPTION 'product fulfillment task phase does not match its state';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_guard_fulfillment_task ON plugin_idc_core_product_fulfillment_task;
CREATE TRIGGER trg_idc_guard_fulfillment_task
BEFORE INSERT OR UPDATE OR DELETE ON plugin_idc_core_product_fulfillment_task
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_fulfillment_task();

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_fulfillment_attempt()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN RAISE EXCEPTION 'product fulfillment attempt evidence is immutable'; END IF;
    IF OLD."finished_at" IS NOT NULL OR NEW."finished_at" IS NULL
       OR NEW."tenant_id" IS DISTINCT FROM OLD."tenant_id" OR NEW."task_no" IS DISTINCT FROM OLD."task_no"
       OR NEW."attempt_no" IS DISTINCT FROM OLD."attempt_no" OR NEW."action_code" IS DISTINCT FROM OLD."action_code"
       OR NEW."started_at" IS DISTINCT FROM OLD."started_at" OR NEW."evidence_digest" IS DISTINCT FROM OLD."evidence_digest" THEN
        RAISE EXCEPTION 'product fulfillment attempt may only be closed once';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_guard_fulfillment_attempt ON plugin_idc_core_product_fulfillment_attempt;
CREATE TRIGGER trg_idc_guard_fulfillment_attempt
BEFORE UPDATE OR DELETE ON plugin_idc_core_product_fulfillment_attempt
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_fulfillment_attempt();

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_fulfillment_attempt_sequence(target_task_no VARCHAR)
RETURNS VOID AS $$
DECLARE task_attempts INT; task_queries INT; task_submits INT; stored_attempts INT; stored_queries INT; stored_submits INT; maximum_no INT;
BEGIN
    SELECT "attempt_count","query_attempt_count","submit_attempt_count"
      INTO task_attempts,task_queries,task_submits
      FROM plugin_idc_core_product_fulfillment_task WHERE "task_no"=target_task_no;
    IF NOT FOUND THEN RETURN; END IF;
    SELECT count(*),count(*) FILTER(WHERE "action_code"='query'),count(*) FILTER(WHERE "action_code"='submit'),COALESCE(max("attempt_no"),0)
      INTO stored_attempts,stored_queries,stored_submits,maximum_no
      FROM plugin_idc_core_product_fulfillment_attempt WHERE "task_no"=target_task_no;
    IF task_attempts<>stored_attempts OR task_queries<>stored_queries OR task_submits<>stored_submits OR maximum_no<>stored_attempts THEN
        RAISE EXCEPTION 'product fulfillment attempt sequence does not match its task';
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION plugin_idc_core_check_fulfillment_attempt_sequence_trigger()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM plugin_idc_core_validate_fulfillment_attempt_sequence(COALESCE(NEW."task_no",OLD."task_no"));
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DO $constraint_trigger$ BEGIN
IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'plugin_idc_core_product_fulfillment_task'::regclass AND conname = 'trg_idc_check_fulfillment_task_attempt_sequence' AND contype = 't') THEN
IF NOT EXISTS (SELECT 1 FROM pg_constraint AS constraint_row JOIN pg_trigger AS trigger_row ON trigger_row.tgconstraint = constraint_row.oid WHERE constraint_row.conrelid = 'plugin_idc_core_product_fulfillment_task'::regclass AND constraint_row.conname = 'trg_idc_check_fulfillment_task_attempt_sequence' AND constraint_row.contype = 't' AND trigger_row.tgrelid = constraint_row.conrelid AND NOT trigger_row.tgisinternal AND trigger_row.tgname = 'trg_idc_check_fulfillment_task_attempt_sequence' AND trigger_row.tgfoid = 'plugin_idc_core_check_fulfillment_attempt_sequence_trigger()'::regprocedure) THEN RAISE EXCEPTION 'constraint trigger trg_idc_check_fulfillment_task_attempt_sequence catalog contract is invalid' USING ERRCODE='23514'; END IF;
ELSE CREATE CONSTRAINT TRIGGER trg_idc_check_fulfillment_task_attempt_sequence AFTER INSERT OR UPDATE OR DELETE ON plugin_idc_core_product_fulfillment_task DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_check_fulfillment_attempt_sequence_trigger(); END IF;
END $constraint_trigger$;
DO $constraint_trigger$ BEGIN
IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'plugin_idc_core_product_fulfillment_attempt'::regclass AND conname = 'trg_idc_check_fulfillment_attempt_task_sequence' AND contype = 't') THEN
IF NOT EXISTS (SELECT 1 FROM pg_constraint AS constraint_row JOIN pg_trigger AS trigger_row ON trigger_row.tgconstraint = constraint_row.oid WHERE constraint_row.conrelid = 'plugin_idc_core_product_fulfillment_attempt'::regclass AND constraint_row.conname = 'trg_idc_check_fulfillment_attempt_task_sequence' AND constraint_row.contype = 't' AND trigger_row.tgrelid = constraint_row.conrelid AND NOT trigger_row.tgisinternal AND trigger_row.tgname = 'trg_idc_check_fulfillment_attempt_task_sequence' AND trigger_row.tgfoid = 'plugin_idc_core_check_fulfillment_attempt_sequence_trigger()'::regprocedure) THEN RAISE EXCEPTION 'constraint trigger trg_idc_check_fulfillment_attempt_task_sequence catalog contract is invalid' USING ERRCODE='23514'; END IF;
ELSE CREATE CONSTRAINT TRIGGER trg_idc_check_fulfillment_attempt_task_sequence AFTER INSERT OR UPDATE OR DELETE ON plugin_idc_core_product_fulfillment_attempt DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_check_fulfillment_attempt_sequence_trigger(); END IF;
END $constraint_trigger$;

DROP TRIGGER IF EXISTS trg_idc_guard_fulfillment_conclusion ON plugin_idc_core_product_fulfillment_manual_conclusion;
CREATE TRIGGER trg_idc_guard_fulfillment_conclusion
BEFORE UPDATE OR DELETE ON plugin_idc_core_product_fulfillment_manual_conclusion
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_fulfillment_immutable_evidence();

DROP TRIGGER IF EXISTS trg_idc_guard_fulfillment_command ON plugin_idc_core_product_fulfillment_command;
CREATE TRIGGER trg_idc_guard_fulfillment_command
BEFORE UPDATE OR DELETE ON plugin_idc_core_product_fulfillment_command
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_fulfillment_immutable_evidence();

DROP TRIGGER IF EXISTS trg_idc_guard_fulfillment_system_transition ON plugin_idc_core_product_fulfillment_system_transition;
CREATE TRIGGER trg_idc_guard_fulfillment_system_transition
BEFORE UPDATE OR DELETE ON plugin_idc_core_product_fulfillment_system_transition
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_fulfillment_immutable_evidence();

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_fulfillment_intent()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN RAISE EXCEPTION 'product fulfillment intent evidence is immutable'; END IF;
    IF TG_OP = 'INSERT' THEN
        IF NEW."initial_status_code" IS DISTINCT FROM NEW."status_code" OR
           NEW."approved_by" IS NOT NULL OR NEW."approved_at" IS NOT NULL OR NOT (
            (NEW."strategy_code"='manual' AND NEW."status_code"='manual_pending') OR
            (NEW."strategy_code"='approval_then_plugin' AND NEW."status_code" IN ('review_pending','manual_pending')) OR
            (NEW."strategy_code"='payment_confirmed_plugin' AND NEW."status_code" IN ('queued','gate_pending','manual_pending'))
        ) THEN RAISE EXCEPTION 'initial product fulfillment intent state is invalid'; END IF;
        IF NOT EXISTS(
            SELECT 1 FROM plugin_idc_core_commerce_order_item item
            JOIN plugin_idc_core_commerce_order_item_quantity_snapshot "quantity"
              ON "quantity"."tenant_id"=item."tenant_id" AND "quantity"."order_item_no"=item."order_item_no"
            WHERE item."tenant_id"=NEW."tenant_id" AND item."order_item_no"=NEW."order_item_no"
              AND item."order_no"=NEW."order_no" AND item."product_no"=NEW."product_no"
              AND item."version_no"=NEW."version_no" AND "quantity"."quantity"=NEW."quantity"
        ) THEN RAISE EXCEPTION 'product fulfillment order item evidence is invalid'; END IF;
        RETURN NEW;
    END IF;
    IF NEW."tenant_id" IS DISTINCT FROM OLD."tenant_id" OR NEW."intent_no" IS DISTINCT FROM OLD."intent_no"
       OR NEW."settlement_no" IS DISTINCT FROM OLD."settlement_no" OR NEW."order_no" IS DISTINCT FROM OLD."order_no"
       OR NEW."order_item_no" IS DISTINCT FROM OLD."order_item_no" OR NEW."customer_no" IS DISTINCT FROM OLD."customer_no"
       OR NEW."invoice_no" IS DISTINCT FROM OLD."invoice_no" OR NEW."currency_code" IS DISTINCT FROM OLD."currency_code"
       OR NEW."product_no" IS DISTINCT FROM OLD."product_no" OR NEW."version_no" IS DISTINCT FROM OLD."version_no"
       OR NEW."product_name_zh_cn" IS DISTINCT FROM OLD."product_name_zh_cn"
       OR NEW."product_name_en_us" IS DISTINCT FROM OLD."product_name_en_us"
       OR NEW."quantity" IS DISTINCT FROM OLD."quantity" OR NEW."unit_sequence" IS DISTINCT FROM OLD."unit_sequence"
       OR NEW."strategy_code" IS DISTINCT FROM OLD."strategy_code" OR NEW."granularity_code" IS DISTINCT FROM OLD."granularity_code"
       OR NEW."initial_status_code" IS DISTINCT FROM OLD."initial_status_code"
       OR NEW."configuration_snapshot_no" IS DISTINCT FROM OLD."configuration_snapshot_no"
       OR NEW."interface_instance_no" IS DISTINCT FROM OLD."interface_instance_no"
       OR NEW."interface_configuration_revision" IS DISTINCT FROM OLD."interface_configuration_revision"
       OR NEW."plugin_id" IS DISTINCT FROM OLD."plugin_id" OR NEW."plugin_release_version" IS DISTINCT FROM OLD."plugin_release_version"
       OR NEW."contract_version" IS DISTINCT FROM OLD."contract_version" OR NEW."release_digest" IS DISTINCT FROM OLD."release_digest"
       OR NEW."declaration_digest" IS DISTINCT FROM OLD."declaration_digest" OR NEW."provision_method" IS DISTINCT FROM OLD."provision_method"
       OR NEW."query_method" IS DISTINCT FROM OLD."query_method" OR NEW."policy_digest" IS DISTINCT FROM OLD."policy_digest"
       OR NEW."maximum_attempts" IS DISTINCT FROM OLD."maximum_attempts"
       OR NEW."retry_base_seconds" IS DISTINCT FROM OLD."retry_base_seconds"
       OR NEW."evidence_digest" IS DISTINCT FROM OLD."evidence_digest" OR NEW."financially_settled_at" IS DISTINCT FROM OLD."financially_settled_at"
       OR NEW."created_at" IS DISTINCT FROM OLD."created_at" THEN
        RAISE EXCEPTION 'product fulfillment intent frozen evidence is immutable';
    END IF;
    IF OLD."approved_by" IS NOT NULL AND
       (NEW."approved_by" IS DISTINCT FROM OLD."approved_by" OR NEW."approved_at" IS DISTINCT FROM OLD."approved_at") THEN
        RAISE EXCEPTION 'product fulfillment approval evidence is immutable';
    END IF;
    IF NEW."version" <> OLD."version" + 1 OR NEW."updated_at" < OLD."updated_at" THEN
        RAISE EXCEPTION 'product fulfillment intent version transition is invalid';
    END IF;
    IF NEW."approved_by" IS DISTINCT FROM OLD."approved_by" OR NEW."approved_at" IS DISTINCT FROM OLD."approved_at" THEN
        IF OLD."strategy_code" <> 'approval_then_plugin' OR OLD."status_code" <> 'review_pending'
           OR NEW."status_code" <> 'queued' OR OLD."approved_by" IS NOT NULL
           OR NEW."approved_by" IS NULL OR NEW."approved_by" <= 0 OR NEW."approved_at" IS NULL THEN
            RAISE EXCEPTION 'product fulfillment approval transition is invalid';
        END IF;
    END IF;
    IF NEW."status_code" IS DISTINCT FROM OLD."status_code" AND NOT (
        (OLD."status_code" = 'review_pending' AND NEW."status_code" = 'queued'
         AND OLD."strategy_code" = 'approval_then_plugin' AND NEW."approved_by" IS NOT NULL) OR
        (OLD."status_code" = 'review_pending' AND NEW."status_code" = 'manual_pending'
         AND NEW."approved_by" IS NULL) OR
        (OLD."status_code" = 'gate_pending' AND NEW."status_code" = 'queued'
         AND OLD."strategy_code" = 'payment_confirmed_plugin' AND NEW."approved_by" IS NULL) OR
        (OLD."status_code" = 'gate_pending' AND NEW."status_code" = 'manual_pending'
         AND OLD."strategy_code" = 'payment_confirmed_plugin' AND NEW."approved_by" IS NULL) OR
        (OLD."status_code" = 'queued' AND NEW."status_code" IN ('processing','manual_pending')) OR
        (OLD."status_code" = 'processing' AND NEW."status_code" IN ('completed','incomplete','manual_pending')) OR
        (OLD."status_code" = 'manual_pending' AND NEW."status_code" IN ('completed','incomplete')) OR
		(OLD."status_code" IN ('manual_pending','review_pending','gate_pending','queued','processing') AND NEW."status_code"='canceled'
		 AND EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_system_transition transition
		            WHERE transition."intent_no"=OLD."intent_no" AND transition."event_code"='cancelled'))
    ) THEN
        RAISE EXCEPTION 'product fulfillment intent status transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_guard_fulfillment_intent ON plugin_idc_core_product_fulfillment_intent;
CREATE TRIGGER trg_idc_guard_fulfillment_intent
BEFORE INSERT OR UPDATE OR DELETE ON plugin_idc_core_product_fulfillment_intent
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_fulfillment_intent();

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_fulfillment_terminal(target_intent_no VARCHAR)
RETURNS VOID AS $$
DECLARE
    intent_status VARCHAR(32);
    intent_strategy VARCHAR(32);
    conclusion_outcome VARCHAR(16);
    task_status VARCHAR(32);
    terminal_attempt_count INT;
BEGIN
    SELECT "status_code","strategy_code" INTO intent_status,intent_strategy
    FROM plugin_idc_core_product_fulfillment_intent
    WHERE "intent_no"=target_intent_no;
    IF NOT FOUND THEN RETURN; END IF;

    SELECT "outcome_code" INTO conclusion_outcome
    FROM plugin_idc_core_product_fulfillment_manual_conclusion
    WHERE "intent_no"=target_intent_no;

    IF intent_status NOT IN ('completed','incomplete') THEN
        IF conclusion_outcome IS NOT NULL THEN
            RAISE EXCEPTION 'manual conclusion requires a terminal fulfillment intent';
        END IF;
        RETURN;
    END IF;

    IF conclusion_outcome IS NOT NULL THEN
        IF (intent_status = 'completed' AND conclusion_outcome <> 'completed') OR
           (intent_status = 'incomplete' AND conclusion_outcome <> 'unable') THEN
            RAISE EXCEPTION 'manual conclusion does not match fulfillment intent status';
        END IF;
        RETURN;
    END IF;

    IF intent_strategy = 'manual' THEN
        RAISE EXCEPTION 'manual fulfillment terminal status requires a conclusion';
    END IF;
    SELECT "status_code" INTO task_status
    FROM plugin_idc_core_product_fulfillment_task
    WHERE "intent_no"=target_intent_no;
    IF (intent_status = 'completed' AND task_status <> 'succeeded') OR
       (intent_status = 'incomplete' AND task_status <> 'failed') OR task_status IS NULL THEN
        RAISE EXCEPTION 'automatic fulfillment terminal status requires a matching task result';
    END IF;
    SELECT count(*) INTO terminal_attempt_count
    FROM plugin_idc_core_product_fulfillment_attempt attempt
    JOIN plugin_idc_core_product_fulfillment_task task ON task."task_no"=attempt."task_no"
    WHERE task."intent_no"=target_intent_no AND attempt."finished_at" IS NOT NULL
      AND ((intent_status='completed' AND attempt."result_status_code"='succeeded') OR
           (intent_status='incomplete' AND attempt."result_status_code"='failed'));
    IF terminal_attempt_count < 1 THEN
        RAISE EXCEPTION 'automatic fulfillment terminal status requires immutable attempt evidence';
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION plugin_idc_core_check_fulfillment_terminal_trigger()
RETURNS TRIGGER AS $$
DECLARE target_intent_no VARCHAR(40);
BEGIN
    IF TG_TABLE_NAME = 'plugin_idc_core_product_fulfillment_intent' THEN
        target_intent_no := COALESCE(NEW."intent_no",OLD."intent_no");
    ELSIF TG_TABLE_NAME = 'plugin_idc_core_product_fulfillment_manual_conclusion' THEN
        target_intent_no := COALESCE(NEW."intent_no",OLD."intent_no");
    ELSIF TG_TABLE_NAME = 'plugin_idc_core_product_fulfillment_task' THEN
        target_intent_no := COALESCE(NEW."intent_no",OLD."intent_no");
    ELSE
        SELECT "intent_no" INTO target_intent_no
        FROM plugin_idc_core_product_fulfillment_task
        WHERE "task_no"=COALESCE(NEW."task_no",OLD."task_no");
    END IF;
    IF target_intent_no IS NOT NULL THEN
        PERFORM plugin_idc_core_validate_fulfillment_terminal(target_intent_no);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DO $constraint_trigger$ BEGIN
IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'plugin_idc_core_product_fulfillment_intent'::regclass AND conname = 'trg_idc_check_fulfillment_intent_terminal' AND contype = 't') THEN
IF NOT EXISTS (SELECT 1 FROM pg_constraint AS constraint_row JOIN pg_trigger AS trigger_row ON trigger_row.tgconstraint = constraint_row.oid WHERE constraint_row.conrelid = 'plugin_idc_core_product_fulfillment_intent'::regclass AND constraint_row.conname = 'trg_idc_check_fulfillment_intent_terminal' AND constraint_row.contype = 't' AND trigger_row.tgrelid = constraint_row.conrelid AND NOT trigger_row.tgisinternal AND trigger_row.tgname = 'trg_idc_check_fulfillment_intent_terminal' AND trigger_row.tgfoid = 'plugin_idc_core_check_fulfillment_terminal_trigger()'::regprocedure) THEN RAISE EXCEPTION 'constraint trigger trg_idc_check_fulfillment_intent_terminal catalog contract is invalid' USING ERRCODE='23514'; END IF;
ELSE CREATE CONSTRAINT TRIGGER trg_idc_check_fulfillment_intent_terminal AFTER INSERT OR UPDATE OR DELETE ON plugin_idc_core_product_fulfillment_intent DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_check_fulfillment_terminal_trigger(); END IF;
END $constraint_trigger$;
DO $constraint_trigger$ BEGIN
IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'plugin_idc_core_product_fulfillment_task'::regclass AND conname = 'trg_idc_check_fulfillment_task_terminal' AND contype = 't') THEN
IF NOT EXISTS (SELECT 1 FROM pg_constraint AS constraint_row JOIN pg_trigger AS trigger_row ON trigger_row.tgconstraint = constraint_row.oid WHERE constraint_row.conrelid = 'plugin_idc_core_product_fulfillment_task'::regclass AND constraint_row.conname = 'trg_idc_check_fulfillment_task_terminal' AND constraint_row.contype = 't' AND trigger_row.tgrelid = constraint_row.conrelid AND NOT trigger_row.tgisinternal AND trigger_row.tgname = 'trg_idc_check_fulfillment_task_terminal' AND trigger_row.tgfoid = 'plugin_idc_core_check_fulfillment_terminal_trigger()'::regprocedure) THEN RAISE EXCEPTION 'constraint trigger trg_idc_check_fulfillment_task_terminal catalog contract is invalid' USING ERRCODE='23514'; END IF;
ELSE CREATE CONSTRAINT TRIGGER trg_idc_check_fulfillment_task_terminal AFTER INSERT OR UPDATE OR DELETE ON plugin_idc_core_product_fulfillment_task DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_check_fulfillment_terminal_trigger(); END IF;
END $constraint_trigger$;
DO $constraint_trigger$ BEGIN
IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'plugin_idc_core_product_fulfillment_attempt'::regclass AND conname = 'trg_idc_check_fulfillment_attempt_terminal' AND contype = 't') THEN
IF NOT EXISTS (SELECT 1 FROM pg_constraint AS constraint_row JOIN pg_trigger AS trigger_row ON trigger_row.tgconstraint = constraint_row.oid WHERE constraint_row.conrelid = 'plugin_idc_core_product_fulfillment_attempt'::regclass AND constraint_row.conname = 'trg_idc_check_fulfillment_attempt_terminal' AND constraint_row.contype = 't' AND trigger_row.tgrelid = constraint_row.conrelid AND NOT trigger_row.tgisinternal AND trigger_row.tgname = 'trg_idc_check_fulfillment_attempt_terminal' AND trigger_row.tgfoid = 'plugin_idc_core_check_fulfillment_terminal_trigger()'::regprocedure) THEN RAISE EXCEPTION 'constraint trigger trg_idc_check_fulfillment_attempt_terminal catalog contract is invalid' USING ERRCODE='23514'; END IF;
ELSE CREATE CONSTRAINT TRIGGER trg_idc_check_fulfillment_attempt_terminal AFTER INSERT OR UPDATE OR DELETE ON plugin_idc_core_product_fulfillment_attempt DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_check_fulfillment_terminal_trigger(); END IF;
END $constraint_trigger$;
DO $constraint_trigger$ BEGIN
IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'plugin_idc_core_product_fulfillment_manual_conclusion'::regclass AND conname = 'trg_idc_check_fulfillment_conclusion_terminal' AND contype = 't') THEN
IF NOT EXISTS (SELECT 1 FROM pg_constraint AS constraint_row JOIN pg_trigger AS trigger_row ON trigger_row.tgconstraint = constraint_row.oid WHERE constraint_row.conrelid = 'plugin_idc_core_product_fulfillment_manual_conclusion'::regclass AND constraint_row.conname = 'trg_idc_check_fulfillment_conclusion_terminal' AND constraint_row.contype = 't' AND trigger_row.tgrelid = constraint_row.conrelid AND NOT trigger_row.tgisinternal AND trigger_row.tgname = 'trg_idc_check_fulfillment_conclusion_terminal' AND trigger_row.tgfoid = 'plugin_idc_core_check_fulfillment_terminal_trigger()'::regprocedure) THEN RAISE EXCEPTION 'constraint trigger trg_idc_check_fulfillment_conclusion_terminal catalog contract is invalid' USING ERRCODE='23514'; END IF;
ELSE CREATE CONSTRAINT TRIGGER trg_idc_check_fulfillment_conclusion_terminal AFTER INSERT OR UPDATE OR DELETE ON plugin_idc_core_product_fulfillment_manual_conclusion DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_check_fulfillment_terminal_trigger(); END IF;
END $constraint_trigger$;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_fulfillment_policy()
RETURNS TRIGGER AS $$
DECLARE version_status VARCHAR(16);
BEGIN
    IF TG_OP = 'DELETE' THEN
        SELECT "status_code" INTO version_status FROM plugin_idc_core_catalog_product_version
        WHERE "tenant_id"=OLD."tenant_id" AND "version_no"=OLD."version_no" FOR UPDATE;
        IF version_status <> 'draft' THEN RAISE EXCEPTION 'published fulfillment policy is immutable'; END IF;
        RETURN OLD;
    END IF;
    SELECT "status_code" INTO version_status FROM plugin_idc_core_catalog_product_version
    WHERE "tenant_id"=NEW."tenant_id" AND "version_no"=NEW."version_no" FOR UPDATE;
    IF version_status <> 'draft' THEN RAISE EXCEPTION 'only draft product versions may change fulfillment policy'; END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_guard_fulfillment_policy ON plugin_idc_core_catalog_product_version_fulfillment_policy;
CREATE TRIGGER trg_idc_guard_fulfillment_policy
BEFORE INSERT OR UPDATE OR DELETE ON plugin_idc_core_catalog_product_version_fulfillment_policy
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_fulfillment_policy();
