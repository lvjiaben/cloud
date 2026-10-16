-- Allow explicit release-evidence refresh only before any resource interface use.
-- Historical bindings and configuration revisions remain immutable.

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_automation_interface_update()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD."ever_referenced" THEN
            RAISE EXCEPTION 'referenced product automation interface instances cannot be deleted';
        END IF;
        RETURN OLD;
    END IF;
    NEW."changed_at" := CURRENT_TIMESTAMP;
    -- Only never-referenced resource projections may refresh release evidence.
    -- Stable identity, business state, display name and reference history stay fixed.
    IF ROW(NEW."plugin_release_version",NEW."contract_version",NEW."authorized_methods",NEW."release_digest",NEW."declaration_digest")
       IS DISTINCT FROM ROW(OLD."plugin_release_version",OLD."contract_version",OLD."authorized_methods",OLD."release_digest",OLD."declaration_digest") THEN
        IF OLD."interface_origin_code" = 'resource' AND NOT OLD."ever_referenced" AND NOT NEW."ever_referenced"
           AND ROW(NEW."id",NEW."tenant_id",NEW."interface_instance_no",NEW."plugin_id",NEW."extension_point_code",NEW."created_by",NEW."created_at",NEW."interface_origin_code",NEW."resource_owner_id",NEW."resource_channel_ref",NEW."display_name",NEW."new_business_state")
               IS NOT DISTINCT FROM ROW(OLD."id",OLD."tenant_id",OLD."interface_instance_no",OLD."plugin_id",OLD."extension_point_code",OLD."created_by",OLD."created_at",OLD."interface_origin_code",OLD."resource_owner_id",OLD."resource_channel_ref",OLD."display_name",OLD."new_business_state")
           AND NEW."version" = OLD."version" + 1 AND NEW."changed_by" > 0 AND NEW."changed_at" >= OLD."changed_at" THEN
            RETURN NEW;
        END IF;
        RAISE EXCEPTION 'referenced or managed automation interface evidence cannot be refreshed';
    END IF;
    IF ROW(
        NEW."id", NEW."tenant_id", NEW."interface_instance_no",
        NEW."plugin_id", NEW."extension_point_code", NEW."plugin_release_version",
        NEW."contract_version", NEW."authorized_methods", NEW."release_digest",
        NEW."declaration_digest", NEW."created_by", NEW."created_at",
        NEW."interface_origin_code", NEW."resource_owner_id", NEW."resource_channel_ref"
    ) IS DISTINCT FROM ROW(
        OLD."id", OLD."tenant_id", OLD."interface_instance_no",
        OLD."plugin_id", OLD."extension_point_code", OLD."plugin_release_version",
        OLD."contract_version", OLD."authorized_methods", OLD."release_digest",
        OLD."declaration_digest", OLD."created_by", OLD."created_at",
        OLD."interface_origin_code", OLD."resource_owner_id", OLD."resource_channel_ref"
    ) THEN
        RAISE EXCEPTION 'product automation interface identity and evidence are immutable';
    END IF;
    IF NEW."version" <> OLD."version" + 1 OR NEW."changed_by" <= 0 OR NEW."changed_at" < OLD."changed_at" THEN
        RAISE EXCEPTION 'product automation interface revision is invalid';
    END IF;
    IF (CASE WHEN NEW."display_name" IS DISTINCT FROM OLD."display_name" THEN 1 ELSE 0 END)
       + (CASE WHEN NEW."new_business_state" IS DISTINCT FROM OLD."new_business_state" THEN 1 ELSE 0 END)
       + (CASE WHEN NEW."ever_referenced" IS DISTINCT FROM OLD."ever_referenced" THEN 1 ELSE 0 END) <> 1 THEN
        RAISE EXCEPTION 'product automation interface update must change exactly one managed fact';
    END IF;
    IF NEW."new_business_state" IS DISTINCT FROM OLD."new_business_state"
       AND (OLD."new_business_state", NEW."new_business_state") NOT IN (('accepting', 'stopped'), ('stopped', 'accepting')) THEN
        RAISE EXCEPTION 'product automation interface state transition is invalid';
    END IF;
    IF NEW."ever_referenced" IS DISTINCT FROM OLD."ever_referenced"
       AND (OLD."ever_referenced" OR NOT NEW."ever_referenced") THEN
        RAISE EXCEPTION 'product automation interface reference history is irreversible';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
