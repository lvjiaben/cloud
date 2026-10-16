package commerce

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func TestConfigurationPriceSelectionReadQueryCountDoesNotGrowPostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	seedCommerceServiceCatalog(t, ctx, database)

	dimensionOne := "cpd_11111111111111111111111111111111"
	dimensionTwo := "cpd_22222222222222222222222222222222"
	snapshotOne := "CFG_11111111111111111111111111111111"
	snapshotTwo := "CFG_22222222222222222222222222222222"
	oneSelection := []catalog.ConfigurationPriceSelection{{DimensionNo: dimensionOne, Quantity: 100, PricingMode: catalog.ConfigurationPricePerUnit}}
	twoSelections := []catalog.ConfigurationPriceSelection{
		{DimensionNo: dimensionOne, Quantity: 100, PricingMode: catalog.ConfigurationPricePerUnit},
		{DimensionNo: dimensionTwo, Quantity: 1},
	}
	oneDigest, err := catalog.ConfigurationPriceSelectionDigest(oneSelection)
	if err != nil {
		t.Fatal(err)
	}
	twoDigest, err := catalog.ConfigurationPriceSelectionDigest(twoSelections)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_catalog_configuration_price_dimension
(tenant_id,dimension_no,version_no,configuration_version,field_key,candidate_value,field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,pricing_mode_code,status_code)
VALUES
(0,?,'version-commerce-service-1',1,'certificate_type','standard','证书类型','Certificate type','普通证书','Standard certificate','per_unit','active'),
(0,?,'version-commerce-service-1',1,'certificate_type','wildcard','证书类型','Certificate type','通配符证书','Wildcard certificate','fixed','active')`, dimensionOne, dimensionTwo); err != nil {
		t.Fatalf("seed configuration price dimensions: %v", err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_commerce_configuration_snapshot
(tenant_id,configuration_snapshot_no,product_no,version_no,source_revision_digest,selection_digest,selected_option_count,total_selected_quantity,content_digest,customer_value_digest)
VALUES
(0,?,'product-commerce-service','version-commerce-service-1',repeat('a',64),?,1,100,repeat('b',64),repeat('c',64)),
(0,?,'product-commerce-service','version-commerce-service-1',repeat('d',64),?,2,101,repeat('e',64),repeat('f',64))`, snapshotOne, oneDigest, snapshotTwo, twoDigest); err != nil {
		t.Fatalf("seed configuration snapshots: %v", err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_commerce_configuration_price_selection
(tenant_id,configuration_snapshot_no,version_no,dimension_no,quantity)
VALUES
(0,?,'version-commerce-service-1',?,100),
(0,?,'version-commerce-service-1',?,100),
(0,?,'version-commerce-service-1',?,1)`,
		snapshotOne, dimensionOne,
		snapshotTwo, dimensionOne,
		snapshotTwo, dimensionTwo,
	); err != nil {
		t.Fatalf("seed configuration price selection evidence: %v", err)
	}

	reader, err := catalog.NewPublicCatalogReader(database, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	selectionReader, err := catalog.ConfigurationPriceSelectionReaderFromPublicReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	logger := &checkoutBenchmarkQueryLogger{ILogger: database.GetLogger(), counts: make(map[int]int)}
	previousLogger, previousDebug := database.GetLogger(), database.GetDebug()
	database.SetLogger(logger)
	database.SetDebug(true)
	t.Cleanup(func() {
		database.SetDebug(previousDebug)
		database.SetLogger(previousLogger)
	})

	oneContext := context.WithValue(ctx, checkoutBenchmarkSampleKey{}, 9401)
	oneResult, err := selectionReader.LoadConfigurationPriceSelections(oneContext, []string{snapshotOne})
	if err != nil || len(oneResult) != 1 || len(oneResult[snapshotOne]) != 1 {
		t.Fatalf("one configuration price snapshot=%+v err=%v", oneResult, err)
	}
	manyContext := context.WithValue(ctx, checkoutBenchmarkSampleKey{}, 9402)
	manyResult, err := selectionReader.LoadConfigurationPriceSelections(manyContext, []string{snapshotOne, snapshotTwo})
	if err != nil || len(manyResult) != 2 || len(manyResult[snapshotOne]) != 1 || len(manyResult[snapshotTwo]) != 2 {
		t.Fatalf("many configuration price snapshots=%+v err=%v", manyResult, err)
	}
	if manyResult[snapshotOne][0].Quantity != 100 || manyResult[snapshotOne][0].PricingMode != catalog.ConfigurationPricePerUnit {
		t.Fatalf("frozen quantity algorithm lost: %+v", manyResult[snapshotOne])
	}
	if oneQueries, manyQueries := logger.count(9401), logger.count(9402); oneQueries != 1 || manyQueries != oneQueries {
		t.Fatalf("configuration price selection queries grew with rows: one=%d many=%d", oneQueries, manyQueries)
	}
}
