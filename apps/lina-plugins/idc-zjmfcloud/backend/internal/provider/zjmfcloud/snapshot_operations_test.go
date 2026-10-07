// This file verifies snapshot and backup mutation contracts against a controlled upstream.

package zjmfcloud

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestSnapshotAndBackupWriteMappings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		capability    resourcecap.CapabilityCode
		parameters    string
		method        string
		path          string
		query         url.Values
		form          url.Values
		ownershipPath string
		ownershipBody string
	}{
		{
			name: "snapshot-create", capability: resourcecap.CapabilityManageSnapshot,
			parameters: `{"action":"create","disk_ref":"12","name":"每日快照"}`,
			method:     http.MethodPost, path: "/administrator/v1/disks/12/snapshots",
			form:          url.Values{"type": {"snap"}, "name": {"每日快照"}},
			ownershipPath: "/administrator/v1/clouds/41",
			ownershipBody: `{"data":{"disk":[{"id":12},{"id":13}],"password":"must-not-leak"}}`,
		},
		{
			name: "backup-create", capability: resourcecap.CapabilityBackup,
			parameters: `{"action":"create","disk_ref":13,"name":"daily backup"}`,
			method:     http.MethodPost, path: "/administrator/v1/disks/13/snapshots",
			form:          url.Values{"type": {"backup"}, "name": {"daily backup"}},
			ownershipPath: "/administrator/v1/clouds/41",
			ownershipBody: `{"data":{"disk":[{"id":12},{"id":13}],"password":"must-not-leak"}}`,
		},
		{
			name: "snapshot-delete", capability: resourcecap.CapabilityManageSnapshot,
			parameters: `{"action":"delete","snapshot_ref":77}`,
			method:     http.MethodDelete, path: "/administrator/v1/snapshots/77",
			ownershipPath: "/administrator/v1/clouds/41/snapshots",
			ownershipBody: `{"data":{"data":[{"id":77,"type":"snap","password":"must-not-leak"}]}}`,
		},
		{
			name: "backup-restore", capability: resourcecap.CapabilityBackup,
			parameters: `{"action":"restore","snapshot_ref":"78"}`,
			method:     http.MethodPost, path: "/administrator/v1/snapshots/78/restore",
			query:         url.Values{"hostid": {"41"}},
			ownershipPath: "/administrator/v1/clouds/41/snapshots",
			ownershipBody: `{"data":{"data":[{"id":78,"type":"backup","password":"must-not-leak"}]}}`,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var loginCount int
			var mu sync.Mutex
			login := loginHandler(t, &loginCount, &mu, "token-one")
			var actionCount atomic.Int64
			fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/administrator/v1/login" {
					login(writer, request)
					return
				}
				if request.Method == http.MethodGet && request.URL.Path == test.ownershipPath {
					if request.URL.Query().Encode() != "page=1&per_page=9999" {
						if test.ownershipPath != "/administrator/v1/clouds/41" {
							t.Errorf("ownership query=%v", request.URL.Query())
						}
					}
					writeJSON(t, writer, http.StatusOK, test.ownershipBody)
					return
				}
				actionCount.Add(1)
				if request.Method != test.method || request.URL.Path != test.path {
					t.Errorf("upstream request=%s %s, want %s %s", request.Method, request.URL.Path, test.method, test.path)
				}
				if request.URL.Query().Encode() != test.query.Encode() {
					t.Errorf("upstream query=%v, want %v", request.URL.Query(), test.query)
				}
				if err := request.ParseForm(); err != nil {
					t.Errorf("parse upstream form: %v", err)
				}
				if request.PostForm.Encode() != test.form.Encode() {
					t.Errorf("upstream form=%v, want %v", request.PostForm, test.form)
				}
				writeJSON(t, writer, http.StatusAccepted,
					`{"data":{"taskid":8100,"password":"must-not-leak"}}`)
			}))
			request := resourcecap.OperationRequest{
				RequestID: "request-" + test.name, Capability: test.capability,
				ResourceRef: "41", Parameters: []byte(test.parameters),
			}
			result, err := fixture.provider.Execute(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != resourcecap.TaskStatusAccepted || result.ExternalTaskRef != "8100" ||
				result.ExternalResourceRef != "41" {
				t.Fatalf("unexpected normalized result: %+v", result)
			}
			if err := result.ValidateFor(request); err != nil {
				t.Fatalf("validate normalized result: %v", err)
			}
			if actionCount.Load() != 1 {
				t.Fatalf("upstream action count=%d, want 1", actionCount.Load())
			}
		})
	}
}

func TestSnapshotAndBackupRejectInvalidShapesBeforeNetwork(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeJSON(t, writer, http.StatusInternalServerError, `{}`)
	}))
	tooLongName := strings.Repeat("a", maximumSnapshotNameBytes+1)
	tests := []resourcecap.OperationRequest{
		{RequestID: "snapshot-no-action", Capability: resourcecap.CapabilityManageSnapshot, ResourceRef: "41", Parameters: []byte(`{}`)},
		{RequestID: "snapshot-create-no-disk", Capability: resourcecap.CapabilityManageSnapshot, ResourceRef: "41", Parameters: []byte(`{"action":"create","name":"valid"}`)},
		{RequestID: "snapshot-create-extra-ref", Capability: resourcecap.CapabilityManageSnapshot, ResourceRef: "41", Parameters: []byte(`{"action":"create","disk_ref":12,"snapshot_ref":77,"name":"valid"}`)},
		{RequestID: "snapshot-create-trim", Capability: resourcecap.CapabilityManageSnapshot, ResourceRef: "41", Parameters: []byte(`{"action":"create","disk_ref":12,"name":" padded "}`)},
		{RequestID: "snapshot-create-control", Capability: resourcecap.CapabilityManageSnapshot, ResourceRef: "41", Parameters: []byte(`{"action":"create","disk_ref":12,"name":"line\nbreak"}`)},
		{RequestID: "snapshot-create-unicode-control", Capability: resourcecap.CapabilityManageSnapshot, ResourceRef: "41", Parameters: []byte(`{"action":"create","disk_ref":12,"name":"line\u0086break"}`)},
		{RequestID: "snapshot-create-long", Capability: resourcecap.CapabilityManageSnapshot, ResourceRef: "41", Parameters: []byte(`{"action":"create","disk_ref":12,"name":"` + tooLongName + `"}`)},
		{RequestID: "snapshot-delete-extra", Capability: resourcecap.CapabilityManageSnapshot, ResourceRef: "41", Parameters: []byte(`{"action":"delete","snapshot_ref":77,"name":"invalid"}`)},
		{RequestID: "backup-restore-extra", Capability: resourcecap.CapabilityBackup, ResourceRef: "41", Parameters: []byte(`{"action":"restore","snapshot_ref":77,"disk_ref":12}`)},
		{RequestID: "snapshot-unknown-action", Capability: resourcecap.CapabilityManageSnapshot, ResourceRef: "41", Parameters: []byte(`{"action":"clone","snapshot_ref":77}`)},
		{RequestID: "snapshot-unknown-field", Capability: resourcecap.CapabilityManageSnapshot, ResourceRef: "41", Parameters: []byte(`{"action":"delete","snapshot_ref":77,"force":true}`)},
		{RequestID: "snapshot-invalid-resource", Capability: resourcecap.CapabilityManageSnapshot, ResourceRef: "cloud-41", Parameters: []byte(`{"action":"delete","snapshot_ref":77}`)},
	}
	for _, request := range tests {
		if _, err := fixture.provider.Execute(context.Background(), request); err == nil {
			t.Errorf("invalid request %s was accepted", request.RequestID)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid inputs made %d network requests", requests.Load())
	}
}

func TestSnapshotAndBackupRequireTaskProof(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"empty":     `{}`,
		"id-only":   `{"data":{"id":77}}`,
		"status-ok": `{"status":"success"}`,
	} {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var loginCount int
			var mu sync.Mutex
			login := loginHandler(t, &loginCount, &mu, "token-one")
			var actionCount atomic.Int64
			fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/administrator/v1/login" {
					login(writer, request)
					return
				}
				if request.URL.Path == "/administrator/v1/clouds/41/snapshots" {
					writeJSON(t, writer, http.StatusOK,
						`{"data":{"data":[{"id":77,"type":"snap"}]}}`)
					return
				}
				actionCount.Add(1)
				writeJSON(t, writer, http.StatusOK, body)
			}))
			result, err := fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{
				RequestID: "snapshot-task-proof-" + name, Capability: resourcecap.CapabilityManageSnapshot,
				ResourceRef: "41", Parameters: []byte(`{"action":"delete","snapshot_ref":77}`),
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil ||
				!result.Failure.ResultUnknown || result.Failure.Retryable {
				t.Fatalf("unproven write result=%+v", result)
			}
			if actionCount.Load() != 1 {
				t.Fatalf("unproven write attempts=%d, want 1", actionCount.Load())
			}
		})
	}
}

func TestSnapshotAndBackupVerifyOwnershipBeforeWrite(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		capability resourcecap.CapabilityCode
		parameters string
		readPath   string
		readBody   string
	}{
		{
			name: "disk-absent", capability: resourcecap.CapabilityManageSnapshot,
			parameters: `{"action":"create","disk_ref":12,"name":"safe"}`,
			readPath:   "/administrator/v1/clouds/41", readBody: `{"data":{"disk":[{"id":13}]}}`,
		},
		{
			name: "snapshot-absent", capability: resourcecap.CapabilityManageSnapshot,
			parameters: `{"action":"delete","snapshot_ref":77}`,
			readPath:   "/administrator/v1/clouds/41/snapshots", readBody: `{"data":{"data":[{"id":78,"type":"snap"}]}}`,
		},
		{
			name: "backup-type-mismatch", capability: resourcecap.CapabilityBackup,
			parameters: `{"action":"restore","snapshot_ref":77}`,
			readPath:   "/administrator/v1/clouds/41/snapshots", readBody: `{"data":{"data":[{"id":77,"type":"snap"}]}}`,
		},
		{
			name: "duplicate-reference", capability: resourcecap.CapabilityManageSnapshot,
			parameters: `{"action":"delete","snapshot_ref":77}`,
			readPath:   "/administrator/v1/clouds/41/snapshots", readBody: `{"data":{"data":[{"id":77,"type":"snap"},{"id":77,"type":"snap"}]}}`,
		},
		{
			name: "malformed-list", capability: resourcecap.CapabilityManageSnapshot,
			parameters: `{"action":"delete","snapshot_ref":77}`,
			readPath:   "/administrator/v1/clouds/41/snapshots", readBody: `{"data":{"data":{"id":77}}}`,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var loginCount int
			var mu sync.Mutex
			login := loginHandler(t, &loginCount, &mu, "token-one")
			var writes atomic.Int64
			fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/administrator/v1/login" {
					login(writer, request)
					return
				}
				if request.Method == http.MethodGet && request.URL.Path == test.readPath {
					writeJSON(t, writer, http.StatusOK, test.readBody)
					return
				}
				writes.Add(1)
				writeJSON(t, writer, http.StatusAccepted, `{"taskid":8100}`)
			}))
			result, err := fixture.provider.Execute(context.Background(), resourcecap.OperationRequest{
				RequestID: "ownership-" + test.name, Capability: test.capability,
				ResourceRef: "41", Parameters: []byte(test.parameters),
			})
			if result != nil || err == nil {
				t.Fatalf("ownership failure result=%+v err=%v", result, err)
			}
			if writes.Load() != 0 {
				t.Fatalf("ownership failure made %d writes", writes.Load())
			}
		})
	}
}
