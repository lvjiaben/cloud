-- Channel transport trust is independent from frozen connector declarations.
-- Empty fingerprints retain system certificate validation.
ALTER TABLE plugin_idc_resource_channel
    ADD COLUMN IF NOT EXISTS tls_certificate_sha256 varchar(64) NOT NULL DEFAULT '';
COMMENT ON COLUMN plugin_idc_resource_channel.tls_certificate_sha256 IS
    'Optional administrator-trusted exact leaf certificate SHA-256; empty uses system trust';
