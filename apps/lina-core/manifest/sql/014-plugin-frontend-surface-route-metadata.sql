-- 014: Plugin frontend surfaces persist a governed, versioned route payload.
-- The payload contains the plugin, host, capability, asset URL and surface
-- version, so the legacy 255-character menu query limit is not sufficient.

ALTER TABLE sys_menu
    ALTER COLUMN "query_param" TYPE TEXT;

COMMENT ON COLUMN sys_menu."query_param"
    IS 'Route parameters in JSON format, including governed plugin frontend surface metadata';
