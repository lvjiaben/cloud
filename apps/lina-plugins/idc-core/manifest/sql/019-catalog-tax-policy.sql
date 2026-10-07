-- 019: Version-owned, per-currency tax and monetary rounding policy.

-- Purpose: Immutable per-currency tax and minor-unit rounding policy owned by one catalog version.
-- 用途：保存目录税务策略相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_tax_policy (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "tax_policy_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "price_includes_tax" BOOLEAN NOT NULL,
    "tax_treatment_code" VARCHAR(16) NOT NULL,
    "tax_rate_basis_points" INT,
    "tax_category_code" VARCHAR(64) NOT NULL,
    "rounding_scope_code" VARCHAR(16) NOT NULL,
    "rounding_mode_code" VARCHAR(16) NOT NULL,
    "rounding_quantum_minor" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_catalog_tax_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_catalog_tax_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_catalog_tax_treatment CHECK (
        "tax_treatment_code" IN ('taxable', 'zero_rated', 'exempt')
    ),
    CONSTRAINT ck_idc_catalog_tax_rate CHECK (
        ("tax_treatment_code" = 'taxable' AND "tax_rate_basis_points" BETWEEN 0 AND 10000)
        OR ("tax_treatment_code" = 'zero_rated' AND "tax_rate_basis_points" = 0)
        OR ("tax_treatment_code" = 'exempt' AND "tax_rate_basis_points" IS NULL)
    ),
    CONSTRAINT ck_idc_catalog_tax_category CHECK (
        "tax_category_code" ~ '^[a-z0-9][a-z0-9._-]{0,63}$'
    ),
    CONSTRAINT ck_idc_catalog_tax_rounding_scope CHECK (
        "rounding_scope_code" IN ('line', 'total')
    ),
    CONSTRAINT ck_idc_catalog_tax_rounding_mode CHECK (
        "rounding_mode_code" IN ('half_up', 'down', 'up')
    ),
    CONSTRAINT ck_idc_catalog_tax_rounding_quantum CHECK (
        "rounding_quantum_minor" BETWEEN 1 AND 1000000
    ),
    CONSTRAINT uk_idc_catalog_tax_policy_no UNIQUE ("tax_policy_no"),
    CONSTRAINT uk_idc_catalog_tax_currency UNIQUE ("version_no", "currency_code"),
    CONSTRAINT fk_idc_catalog_tax_version FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version ("version_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_catalog_tax_version
    ON plugin_idc_core_catalog_tax_policy (
        "tenant_id", "version_no", "currency_code", "id"
    );

COMMENT ON TABLE plugin_idc_core_catalog_tax_policy
    IS 'Immutable per-currency tax and minor-unit rounding policy owned by one catalog version';
COMMENT ON COLUMN plugin_idc_core_catalog_tax_policy."tax_rate_basis_points"
    IS 'Tax rate in basis points; null only for exempt treatment';
COMMENT ON COLUMN plugin_idc_core_catalog_tax_policy."rounding_quantum_minor"
    IS 'Positive rounding quantum expressed in the configured currency minor unit';
