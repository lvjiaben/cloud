INSERT INTO plugin_idc_core_customer_subject (
    "customer_no", "customer_type_code", "status_code", "display_name"
)
SELECT
    'benchmark-customer-' || LPAD("sequence_no"::TEXT, 3, '0'),
    'individual',
    'active',
    'Benchmark Customer ' || "sequence_no"::TEXT
FROM generate_series(1, 73) AS "sequence_no";

INSERT INTO plugin_idc_core_catalog_inventory_snapshot (
    "snapshot_no", "binding_no", "source_snapshot_no", "source_fingerprint",
    "availability_code", "available_quantity", "safety_margin", "sellable_quantity",
    "observed_at", "source_completed_at", "expires_at"
) VALUES (
    'inventory-commerce-benchmark-v1', 'binding-commerce-service',
    'source-commerce-benchmark-v1', REPEAT('c', 64),
    'available', 10000, 0, 10000,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '1 hour'
);

INSERT INTO plugin_idc_core_customer_individual_profile (
    "customer_no", "legal_name", "legal_name_masked", "identity_identifier_masked",
    "identity_lookup_digest", "identity_case_no", "identity_status_code",
    "identity_verified_at", "identity_expires_at"
) VALUES (
    'customer-commerce-service-1', 'Query Shape Customer', 'Q**** S**** C*******',
    '110***********001X',
    MD5('benchmark-query-shape-a') || MD5('benchmark-query-shape-b'),
    'benchmark-query-shape-case-001', 'verified',
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '1 year'
);

INSERT INTO plugin_idc_core_customer_individual_profile (
    "customer_no", "legal_name", "legal_name_masked", "identity_identifier_masked",
    "identity_lookup_digest", "identity_case_no", "identity_status_code",
    "identity_verified_at", "identity_expires_at"
)
SELECT
    'benchmark-customer-' || LPAD("sequence_no"::TEXT, 3, '0'),
    'Benchmark Customer ' || "sequence_no"::TEXT,
    'B******** C*******',
    '110***********001X',
    MD5('benchmark-identity-a-' || "sequence_no"::TEXT) ||
        MD5('benchmark-identity-b-' || "sequence_no"::TEXT),
    'benchmark-identity-case-' || LPAD("sequence_no"::TEXT, 3, '0'),
    'verified',
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP + INTERVAL '1 year'
FROM generate_series(1, 73) AS "sequence_no";
