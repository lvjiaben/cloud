-- Upgrade customer assistance from a static read-only snapshot to a real,
-- single-use, fifteen-minute customer impersonation handoff.

ALTER TABLE plugin_idc_core_customer_assistance_session
    ADD COLUMN IF NOT EXISTS host_token_id VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS host_token_ciphertext TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS exchanged_at TIMESTAMPTZ;

ALTER TABLE plugin_idc_core_customer_assistance_event
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_core_assistance_event_type;

UPDATE plugin_idc_core_customer_assistance_event
SET event_type = 'exchanged'
WHERE event_type = 'viewed';

ALTER TABLE plugin_idc_core_customer_assistance_event
    ADD CONSTRAINT ck_plugin_idc_core_assistance_event_type
    CHECK (event_type IN ('started','exchanged','ended','expired','superseded'));

COMMENT ON COLUMN plugin_idc_core_customer_assistance_session.host_token_id
    IS 'Host-issued impersonation session ID used for audited lifecycle correlation';
COMMENT ON COLUMN plugin_idc_core_customer_assistance_session.host_token_ciphertext
    IS 'AES-GCM encrypted host access token; plaintext is never persisted';
COMMENT ON COLUMN plugin_idc_core_customer_assistance_session.exchanged_at
    IS 'Database time when the single-use browser handoff was consumed';
COMMENT ON TABLE plugin_idc_core_customer_assistance_session
    IS 'Fifteen-minute single-use customer impersonation sessions; host access tokens are encrypted at rest';
