package localops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

type verifierStub struct{ callback VerifiedCallback }

func (v *verifierStub) Verify(context.Context, []byte, []byte) (VerifiedCallback, error) {
	return v.callback, nil
}

type querierStub struct{ observation Observation }

func (q *querierStub) Query(context.Context, Operation) (Observation, error) {
	return q.observation, nil
}

type circuitStub struct{ observed error }

func (c *circuitStub) Allow(context.Context, string) error            { return nil }
func (c *circuitStub) Observe(_ context.Context, _ string, err error) { c.observed = err }

type sealerStub struct {
	eventKey []byte
	payload  []byte
}

func (s *sealerStub) Seal(_ string, eventKey, payload []byte) (SealedEvidence, error) {
	s.eventKey = append([]byte(nil), eventKey...)
	s.payload = append([]byte(nil), payload...)
	eventDigest := sha256.Sum256(eventKey)
	payloadDigest := sha256.Sum256(payload)
	return SealedEvidence{
		EventHMAC: eventDigest, PayloadSHA256: payloadDigest,
		PayloadCiphertext: bytes.Repeat([]byte{9}, 32), KeyVersion: "local-v1",
	}, nil
}

type authorityStub struct{ authorization EmployeeAuthorization }

func (a *authorityStub) AuthorizeTakeover(context.Context, string) (EmployeeAuthorization, error) {
	return a.authorization, nil
}

type storeStub struct {
	callback       VerifiedCallback
	callbackSealed SealedEvidence
	query          Observation
	querySealed    SealedEvidence
	takeover       TakeoverInput
	authorization  EmployeeAuthorization
}

func (s *storeStub) AppendCallback(
	_ context.Context,
	_ string,
	callback VerifiedCallback,
	sealed SealedEvidence,
) error {
	s.callback, s.callbackSealed = callback, sealed
	return nil
}

func (s *storeStub) AppendQuery(
	_ context.Context,
	_ Operation,
	observation Observation,
	sealed SealedEvidence,
) error {
	s.query, s.querySealed = observation, sealed
	return nil
}

func (s *storeStub) AppendTakeover(
	_ context.Context,
	input TakeoverInput,
	authorization EmployeeAuthorization,
) error {
	s.takeover, s.authorization = input, authorization
	return nil
}

func TestCallbackAndQueryPersistOnlyLocalSealedEvidence(t *testing.T) {
	now := time.Date(2026, time.August, 13, 9, 0, 0, 0, time.UTC)
	verifier := &verifierStub{callback: VerifiedCallback{
		OperationNo: "lop_callback01", EventKey: []byte("provider-event-0001"),
		Observation: Observation{
			Status: StatusSucceeded, StableCode: "completed",
			ExternalRef: []byte("external-reference"), EvidenceKind: "contract.completed",
			Evidence: []byte("raw callback evidence"), ObservedAt: now,
		},
	}}
	querier := &querierStub{observation: Observation{
		Status: StatusResultUnknown, StableCode: "provider_pending",
		ExternalRef: []byte("external-reference"), EvidenceKind: "contract.query",
		Evidence: []byte("raw query evidence"), ObservedAt: now.Add(time.Minute),
	}}
	store := &storeStub{}
	sealer := &sealerStub{}
	service, err := New(
		map[string]CallbackVerifier{ProviderTencentESSBasic: verifier},
		map[string]ActiveQuerier{ProviderTencentESSBasic: querier},
		&circuitStub{}, sealer,
		&authorityStub{authorization: EmployeeAuthorization{
			PrincipalNo: "employee_1", CurrentMerchantEmployee: true, FullAuthAt: now,
		}}, store,
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	service.now = func() time.Time { return now }
	if err = service.AcceptCallback(context.Background(), CallbackInput{
		Provider: ProviderTencentESSBasic, Body: []byte("callback"), Signature: []byte("signature"),
	}); err != nil {
		t.Fatalf("accept callback: %v", err)
	}
	if store.callback.EventKey != nil || store.callback.Observation.Evidence != nil ||
		store.callback.Observation.ExternalRef != nil || len(store.callbackSealed.PayloadCiphertext) == 0 {
		t.Fatalf("callback leaked raw material: callback=%+v sealed=%+v", store.callback, store.callbackSealed)
	}
	if err = service.Reconcile(context.Background(), Operation{
		OperationNo: "lop_query0001", Provider: ProviderTencentESSBasic, Status: StatusResultUnknown,
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if store.query.Evidence != nil || store.query.ExternalRef != nil ||
		store.query.Status != StatusResultUnknown || len(store.querySealed.PayloadCiphertext) == 0 {
		t.Fatalf("query leaked raw material: query=%+v sealed=%+v", store.query, store.querySealed)
	}
	if !bytes.Equal(sealer.payload, []byte("raw query evidence")) {
		t.Fatalf("sealer did not receive exact bounded evidence: %q", sealer.payload)
	}
}

func TestMissingClientDoesNotFallbackAndTakeoverRequiresCurrentMerchantEmployee(t *testing.T) {
	now := time.Date(2026, time.August, 13, 9, 0, 0, 0, time.UTC)
	authority := &authorityStub{authorization: EmployeeAuthorization{
		PrincipalNo: "vendor_platform_operator", CurrentMerchantEmployee: false, FullAuthAt: now,
	}}
	store := &storeStub{}
	service, err := New(
		map[string]CallbackVerifier{}, map[string]ActiveQuerier{},
		&circuitStub{}, &sealerStub{}, authority, store,
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	service.now = func() time.Time { return now }
	if err = service.AcceptCallback(context.Background(), CallbackInput{
		Provider: ProviderNuonuoInvoice, Body: []byte("callback"), Signature: []byte("signature"),
	}); !errors.Is(err, ErrAdapterDisabled) {
		t.Fatalf("missing client fallback err=%v", err)
	}
	digest := sha256.Sum256([]byte("local-evidence-set"))
	input := TakeoverInput{
		OperationNo: "lop_takeover01", ReasonCode: "result_unknown_review",
		EvidenceSetSHA256: digest,
	}
	if err = service.Takeover(context.Background(), input); !errors.Is(err, ErrRecentAuth) {
		t.Fatalf("platform operator takeover err=%v", err)
	}
	authority.authorization = EmployeeAuthorization{
		PrincipalNo: "employee_1", CurrentMerchantEmployee: true, FullAuthAt: now.Add(-time.Minute),
	}
	if err = service.Takeover(context.Background(), input); err != nil {
		t.Fatalf("current merchant employee takeover: %v", err)
	}
	if store.authorization.PrincipalNo != "employee_1" ||
		store.takeover.OperationNo != input.OperationNo {
		t.Fatalf("takeover=%+v authorization=%+v", store.takeover, store.authorization)
	}
}

func TestCallbackRejectsPayloadAbove256KiB(t *testing.T) {
	service, err := New(
		map[string]CallbackVerifier{ProviderTencentESSBasic: &verifierStub{}},
		map[string]ActiveQuerier{},
		&circuitStub{}, &sealerStub{}, &authorityStub{}, &storeStub{},
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	input := CallbackInput{
		Provider:  ProviderTencentESSBasic,
		Body:      bytes.Repeat([]byte{'x'}, maximumPayloadBytes+1),
		Signature: []byte("signature"),
	}
	if err = service.AcceptCallback(context.Background(), input); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversized callback err=%v", err)
	}
}
