-- Derived customer-owned login projection; host passwords and tables are never
-- copied. Contact digests support exact lookup; identities remain encrypted.
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_login_contact (
 tenant_id INTEGER NOT NULL DEFAULT 0 CHECK(tenant_id=0),
 user_id INTEGER NOT NULL CHECK(user_id>0),
 email_digest VARCHAR(64) NOT NULL DEFAULT '',
 phone_digest VARCHAR(64) NOT NULL DEFAULT '',
 identity_ciphertext TEXT NOT NULL,
 revision BIGINT NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY(tenant_id,user_id),
 CHECK(email_digest='' OR email_digest ~ '^[0-9a-f]{64}$'),
 CHECK(phone_digest='' OR phone_digest ~ '^[0-9a-f]{64}$')
);
CREATE INDEX IF NOT EXISTS idx_idc_core_customer_login_email
 ON plugin_idc_core_customer_login_contact(tenant_id,email_digest) WHERE email_digest<>'';
CREATE INDEX IF NOT EXISTS idx_idc_core_customer_login_phone
 ON plugin_idc_core_customer_login_contact(tenant_id,phone_digest) WHERE phone_digest<>'';
