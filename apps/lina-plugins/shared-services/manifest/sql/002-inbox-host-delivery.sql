-- Publication records durable intent; host delivery is separate append-only evidence.
ALTER TABLE plugin_shared_services_inbox_delivery_evidence
 ADD COLUMN IF NOT EXISTS host_message_id VARCHAR(40);
ALTER TABLE plugin_shared_services_inbox_delivery_evidence
 DROP CONSTRAINT IF EXISTS plugin_shared_services_inbox_delivery_evide_evidence_type_check,
 DROP CONSTRAINT IF EXISTS ck_shared_services_inbox_evidence_type,
 ADD CONSTRAINT ck_shared_services_inbox_evidence_type
 CHECK (evidence_type IN ('published','idempotent_replay','host_published')),
 DROP CONSTRAINT IF EXISTS ck_shared_services_inbox_host_message,
 ADD CONSTRAINT ck_shared_services_inbox_host_message CHECK (
  (evidence_type='host_published' AND host_message_id IS NOT NULL AND host_message_id ~ '^[1-9][0-9]{0,18}$')
  OR (evidence_type<>'host_published' AND host_message_id IS NULL)
 );
CREATE UNIQUE INDEX IF NOT EXISTS uq_shared_services_inbox_host_delivery
 ON plugin_shared_services_inbox_delivery_evidence(tenant_id,message_no) WHERE evidence_type='host_published';
CREATE INDEX IF NOT EXISTS idx_shared_services_inbox_self_sync
 ON plugin_shared_services_inbox_message(tenant_id,recipient_user_id,created_at,id);
