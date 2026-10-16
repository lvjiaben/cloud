-- Remove unused interfaces together with their private configuration revisions.
-- The parent guard still rejects deleting every previously referenced interface.
-- Active sessions may predate action proofs; proof expiry and nonce fencing remain.
DO $$
BEGIN
 IF to_regclass('plugin_idc_core_recent_auth_consumption') IS NOT NULL THEN
  ALTER TABLE plugin_idc_core_recent_auth_consumption DROP CONSTRAINT IF EXISTS ck_plugin_idc_core_reauth_time;
  ALTER TABLE plugin_idc_core_recent_auth_consumption ADD CONSTRAINT ck_plugin_idc_core_reauth_time CHECK (
   login_at <= issued_at + INTERVAL '1 minute' AND
   expires_at > issued_at AND expires_at <= issued_at + INTERVAL '5 minutes' AND
   issued_at <= consumed_at + INTERVAL '1 minute' AND consumed_at < expires_at + INTERVAL '1 minute'
  );
 END IF;
END;
$$;
-- Interface configuration cleanup.
ALTER TABLE plugin_idc_core_product_automation_interface_configuration_revision
 DROP CONSTRAINT IF EXISTS fk_idc_interface_configuration_interface;
ALTER TABLE plugin_idc_core_product_automation_interface_configuration_revision
 ADD CONSTRAINT fk_idc_interface_configuration_interface FOREIGN KEY (interface_instance_no)
 REFERENCES plugin_idc_core_product_automation_interface_instance(interface_instance_no) ON DELETE CASCADE;
ALTER TABLE plugin_idc_core_product_automation_interface_secret_reference
 DROP CONSTRAINT IF EXISTS fk_idc_interface_secret_reference_revision;
ALTER TABLE plugin_idc_core_product_automation_interface_secret_reference
 ADD CONSTRAINT fk_idc_interface_secret_reference_revision FOREIGN KEY (configuration_revision_id)
 REFERENCES plugin_idc_core_product_automation_interface_configuration_revision(id) ON DELETE CASCADE;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_interface_configuration_revision()
RETURNS TRIGGER AS $$
DECLARE expected_revision BIGINT;
BEGIN
 IF TG_OP = 'DELETE' THEN
  IF EXISTS (SELECT 1 FROM plugin_idc_core_product_automation_interface_instance
    WHERE tenant_id=OLD.tenant_id AND interface_instance_no=OLD.interface_instance_no) THEN
   RAISE EXCEPTION 'interface configuration revisions are append-only';
  END IF;
  RETURN OLD;
 END IF;
 IF TG_OP <> 'INSERT' THEN RAISE EXCEPTION 'interface configuration revisions are append-only'; END IF;
 SELECT COALESCE(MAX(revision),0)+1 INTO expected_revision
 FROM plugin_idc_core_product_automation_interface_configuration_revision
 WHERE tenant_id=NEW.tenant_id AND interface_instance_no=NEW.interface_instance_no;
 IF NEW.revision <> expected_revision THEN RAISE EXCEPTION 'interface configuration revision is not contiguous'; END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_interface_secret_reference()
RETURNS TRIGGER AS $$
DECLARE expected_count INT; actual_count INT;
BEGIN
 IF TG_OP = 'DELETE' THEN
  IF EXISTS (SELECT 1 FROM plugin_idc_core_product_automation_interface_configuration_revision
    WHERE id=OLD.configuration_revision_id AND tenant_id=OLD.tenant_id) THEN
   RAISE EXCEPTION 'interface secret references are append-only';
  END IF;
  RETURN OLD;
 END IF;
 IF TG_OP <> 'INSERT' THEN RAISE EXCEPTION 'interface secret references are append-only'; END IF;
 SELECT secret_field_count INTO expected_count
 FROM plugin_idc_core_product_automation_interface_configuration_revision
 WHERE id=NEW.configuration_revision_id AND tenant_id=NEW.tenant_id FOR UPDATE;
 IF expected_count IS NULL THEN RAISE EXCEPTION 'interface configuration revision is unavailable'; END IF;
 SELECT count(*) INTO actual_count FROM plugin_idc_core_product_automation_interface_secret_reference
 WHERE tenant_id=NEW.tenant_id AND configuration_revision_id=NEW.configuration_revision_id;
 IF actual_count >= expected_count OR actual_count >= 128 THEN RAISE EXCEPTION 'interface secret reference capacity is exhausted'; END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
