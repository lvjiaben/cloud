package renewalsource

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/internal/service/serveroperationsource"
)

func TestIPChangeReservationPostgreSQLSerializesConcurrentLastAddress(t *testing.T) {
	ctx, database := newRenewalSourceDatabase(t)
	seedRenewalSourceRoute(t, ctx, database)
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_provider_capacity_observation
    (observation_no,channel_no,capacity_kind,status_code,observation_fingerprint,item_count,observed_at,completed_at)
VALUES ('ip-stock-concurrency','channel-renewal-1','stock','complete',repeat('a',64),1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_resource_provider_capacity_observation_item
    (observation_no,capacity_kind,external_ref,projection,observed_at)
VALUES ('ip-stock-concurrency','stock','stock-1','{"ip_free":"1"}'::JSONB,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	service, err := serveroperationsource.New(database)
	if err != nil {
		t.Fatal(err)
	}
	var capacityFacts struct {
		Pools     int `orm:"pools"`
		Snapshots int `orm:"snapshots"`
		Items     int `orm:"items"`
		Free      int `orm:"free"`
	}
	if err = database.GetScan(ctx, &capacityFacts, `SELECT
 (SELECT count(*) FROM plugin_idc_resource_pool WHERE pool_no='pool-renewal-1') pools,
	(SELECT count(*) FROM plugin_idc_resource_provider_capacity_observation WHERE observation_no='ip-stock-concurrency') snapshots,
	(SELECT count(*) FROM plugin_idc_resource_provider_capacity_observation_item WHERE observation_no='ip-stock-concurrency') items,
	(SELECT COALESCE(sum((projection->>'ip_free')::INT),0) FROM plugin_idc_resource_provider_capacity_observation_item WHERE observation_no='ip-stock-concurrency') free`); err != nil {
		t.Fatal(err)
	}
	if capacityFacts.Pools != 1 || capacityFacts.Snapshots != 1 || capacityFacts.Items != 1 || capacityFacts.Free != 1 {
		t.Fatalf("invalid concurrent IP fixture: %+v", capacityFacts)
	}
	nowValue, err := database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
	if err != nil || nowValue.Time().IsZero() {
		t.Fatalf("invalid database clock value=%v err=%v", nowValue, err)
	}
	poolValue, err := database.GetValue(ctx, `SELECT id FROM plugin_idc_resource_pool
WHERE pool_no='pool-renewal-1' AND channel_no='channel-renewal-1' AND status_code='enabled'`)
	if err != nil || poolValue.Int64() <= 0 {
		t.Fatalf("invalid pool lock target value=%v err=%v", poolValue, err)
	}
	var snapshotFacts struct {
		ObservationNo string    `orm:"observation_no"`
		ObservedAt    time.Time `orm:"observed_at"`
	}
	if err = database.GetScan(ctx, &snapshotFacts, `SELECT observation_no,observed_at
FROM plugin_idc_resource_provider_capacity_observation
WHERE channel_no='channel-renewal-1' AND capacity_kind='stock' AND status_code='complete'
 AND observed_at>=? ORDER BY observed_at DESC,id DESC LIMIT 1`, nowValue.Time().Add(-5*time.Minute)); err != nil || snapshotFacts.ObservationNo == "" {
		t.Fatalf("invalid stock snapshot=%+v err=%v", snapshotFacts, err)
	}
	var stockFacts struct {
		ItemCount  int   `orm:"item_count"`
		ValidCount int   `orm:"valid_count"`
		FreeCount  int64 `orm:"free_count"`
	}
	if err = database.GetScan(ctx, &stockFacts, `SELECT count(*) item_count,
count(*) FILTER (WHERE jsonb_exists(projection,'ip_free') AND projection->>'ip_free' ~ '^[0-9]+$') valid_count,
COALESCE(sum(CASE WHEN jsonb_exists(projection,'ip_free') AND projection->>'ip_free' ~ '^[0-9]+$'
 THEN (projection->>'ip_free')::BIGINT ELSE 0 END),0) free_count
FROM plugin_idc_resource_provider_capacity_observation_item
WHERE observation_no=? AND capacity_kind='stock'`, snapshotFacts.ObservationNo); err != nil ||
		stockFacts.ItemCount != 1 || stockFacts.ValidCount != 1 || stockFacts.FreeCount != 1 {
		t.Fatalf("invalid stock projection=%+v err=%v", stockFacts, err)
	}
	reservedBefore, err := database.GetValue(ctx, `SELECT COALESCE(sum(quantity),0)
FROM plugin_idc_resource_ip_reservation
WHERE channel_no='channel-renewal-1' AND pool_no='pool-renewal-1' AND status_code='reserved' AND expires_at>?`, nowValue.Time())
	if err != nil || reservedBefore.Int64() != 0 {
		t.Fatalf("invalid reservation baseline value=%v err=%v", reservedBefore, err)
	}
	expires := time.Now().UTC().Add(30 * time.Minute)
	requests := []serveroperationresourcecap.IPReservationRequest{
		{Route: serveroperationresourcecap.CapabilityRoute{ServiceNo: "service-renewal-1", ChannelNo: "channel-renewal-1", PoolNo: "pool-renewal-1"},
			Quantity: 1, MaximumCapacityAgeSeconds: 300, ExpiresAt: expires, IdempotencyKey: "ip-concurrency-request-1"},
		{Route: serveroperationresourcecap.CapabilityRoute{ServiceNo: "service-renewal-2", ChannelNo: "channel-renewal-1", PoolNo: "pool-renewal-1"},
			Quantity: 1, MaximumCapacityAgeSeconds: 300, ExpiresAt: expires, IdempotencyKey: "ip-concurrency-request-2"},
	}
	for index := range requests {
		requests[index].RequestDigest, err = serveroperationresourcecap.IPReservationRequestDigest(requests[index])
		if err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	results := make(chan error, len(requests))
	var group sync.WaitGroup
	for index := range requests {
		request := requests[index]
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, reserveErr := service.ReserveIP(context.Background(), request)
			results <- reserveErr
		}()
	}
	close(start)
	group.Wait()
	close(results)
	succeeded, unavailable := 0, 0
	for reserveErr := range results {
		if reserveErr == nil {
			succeeded++
		} else if errors.Is(reserveErr, serveroperationresourcecap.ErrIPCapacityUnavailable) {
			unavailable++
		} else {
			t.Fatalf("unexpected concurrent reservation error: %v", reserveErr)
		}
	}
	reservedValue, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_resource_ip_reservation WHERE status_code='reserved'`)
	if err != nil {
		t.Fatal(err)
	}
	reserved := reservedValue.Int()
	if succeeded != 1 || unavailable != 1 || reserved != 1 {
		t.Fatalf("concurrent last-address result succeeded=%d unavailable=%d reserved=%d", succeeded, unavailable, reserved)
	}
	reservationValue, err := database.GetValue(ctx, `SELECT reservation_no
FROM plugin_idc_resource_ip_reservation WHERE status_code='reserved'`)
	if err != nil || reservationValue.String() == "" {
		t.Fatalf("read concurrent reservation value=%v err=%v", reservationValue, err)
	}
	consumed, err := service.ConsumeIP(ctx, serveroperationresourcecap.IPReservationConsumeRequest{
		ReservationNo: reservationValue.String(), ProviderTaskNo: "TASK_ip_concurrency_1",
	})
	if err != nil || consumed.Status != serveroperationresourcecap.IPReservationConsumed {
		t.Fatalf("consume concurrent reservation result=%+v err=%v", consumed, err)
	}

	releaseRequest := serveroperationresourcecap.IPReservationRequest{
		Route: serveroperationresourcecap.CapabilityRoute{ServiceNo: "service-renewal-2",
			ChannelNo: "channel-renewal-1", PoolNo: "pool-renewal-1"},
		Quantity: 1, MaximumCapacityAgeSeconds: 300,
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute), IdempotencyKey: "ip-release-request-1",
	}
	releaseRequest.RequestDigest, err = serveroperationresourcecap.IPReservationRequestDigest(releaseRequest)
	if err != nil {
		t.Fatal(err)
	}
	releasable, err := service.ReserveIP(ctx, releaseRequest)
	if err != nil {
		t.Fatalf("reserve releasable IP: %v", err)
	}
	released, err := service.ReleaseIP(ctx, serveroperationresourcecap.IPReservationReleaseRequest{
		ReservationNo: releasable.ReservationNo, ReasonCode: "test_release",
	})
	if err != nil || released.Status != serveroperationresourcecap.IPReservationReleased {
		t.Fatalf("release IP reservation result=%+v err=%v", released, err)
	}
}
