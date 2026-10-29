-- Customer-entered server login choices. Password bytes live only in secretcap.
-- Immutable purchase evidence deliberately has no soft-delete lifecycle.
CREATE TABLE IF NOT EXISTS plugin_idc_core_purchase_server_login (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id=0),
 server_login_token VARCHAR(68) NOT NULL UNIQUE,
 customer_no VARCHAR(40) NOT NULL,
 user_id BIGINT NOT NULL,
 product_version_no VARCHAR(40) NOT NULL,
 configuration_revision VARCHAR(128) NOT NULL,
 cart_item_no VARCHAR(64) NOT NULL DEFAULT '',
 hostname VARCHAR(63) NOT NULL DEFAULT '',
 password_secret_ref TEXT NOT NULL DEFAULT '',
 channel_no VARCHAR(64) NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE plugin_idc_core_purchase_server_login IS 'Exact customer-login and product-version purchase choices; password is an owner-scoped opaque secret reference only';
COMMENT ON COLUMN plugin_idc_core_purchase_server_login.password_secret_ref IS 'Opaque resource-channel-scoped secretcap handle, never plaintext';
ALTER TABLE plugin_idc_core_commerce_configuration_snapshot ADD COLUMN IF NOT EXISTS server_login_token VARCHAR(68) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_core_commerce_structured_configuration_snapshot ADD COLUMN IF NOT EXISTS server_login_token VARCHAR(68) NOT NULL DEFAULT '';
