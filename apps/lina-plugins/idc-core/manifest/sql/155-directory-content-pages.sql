-- Explicit plugin-owned content blocks also render for empty product menus.
-- This is replaceable navigation configuration, not service ownership.
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_directory_page (
    tenant_id BIGINT NOT NULL DEFAULT 0,
    directory_no VARCHAR(40) NOT NULL,
    plugin_id VARCHAR(128) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, directory_no, plugin_id),
    CHECK (sort_order >= 0 AND sort_order < 16)
);
COMMENT ON TABLE plugin_idc_core_product_directory_page IS 'Configured plugin-owned content blocks for member product menus';
