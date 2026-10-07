package bridge

import (
	"context"
	"encoding/json"
	"testing"

	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
)

func TestClientUsesOpaqueContextAsGovernedResource(t *testing.T) {
	t.Parallel()
	text := "https://supplier-a.example"
	client, err := NewClient(func(owner, service, version, method, resourceRef, table string, payload []byte) ([]byte, error) {
		if owner != supplierinterfacecap.OwnerPluginID || service != supplierinterfacecap.ServiceName ||
			version != supplierinterfacecap.ContractVersion || method != supplierinterfacecap.MethodConfigurationGet ||
			resourceRef != "src_0123456789abcdef0123456789abcdef" || table != "" || string(payload) != "{}" {
			t.Fatalf("unexpected governed envelope: %q %q %q %q %q %q %s", owner, service, version, method, resourceRef, table, payload)
		}
		return json.Marshal(&supplierinterfacecap.ConfigurationView{Values: []supplierinterfacecap.ConfigurationValue{{FieldKey: "endpoint", TextValue: &text}}})
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err := client.GetConfiguration(context.Background(), "src_0123456789abcdef0123456789abcdef")
	if err != nil || len(view.Values) != 1 || view.Values[0].TextValue == nil || *view.Values[0].TextValue != text {
		t.Fatalf("configuration view=%#v err=%v", view, err)
	}
}

func TestDynamicClientSecretAccessFailsClosedWithoutInvokingHost(t *testing.T) {
	calls := 0
	client, err := NewClient(func(_, _, _, _, _, _ string, _ []byte) ([]byte, error) {
		calls++
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.AccessSecret(context.Background(), "src_0123456789abcdef0123456789abcdef", "api_secret", func([]byte) error { return nil }); err == nil {
		t.Fatal("dynamic secret access must fail closed")
	}
	if calls != 0 {
		t.Fatalf("dynamic secret access reached an ordinary host payload: %d", calls)
	}
}

func TestDynamicClientExecutesGovernedOutboundSession(t *testing.T) {
	t.Parallel()
	contextID := "src_0123456789abcdef0123456789abcdef"
	request := supplierinterfacecap.OutboundSessionRequest{
		EndpointField: "endpoint",
		Authentication: supplierinterfacecap.OutboundSessionAuthentication{
			Method:           "POST",
			Path:             "/login",
			Form:             map[string]string{"username": "operator"},
			SecretFormFields: map[string]string{"password": "api_secret"},
			TokenSelector:    supplierinterfacecap.OutboundTokenRootString,
			TokenHeader:      "Authorization",
		},
		Steps: []supplierinterfacecap.OutboundSessionStep{{
			Method:         "GET",
			Path:           "/servers",
			ResponseFields: []string{"data"},
		}},
	}
	client, err := NewClient(func(owner, service, version, method, resourceRef, table string, payload []byte) ([]byte, error) {
		if owner != supplierinterfacecap.OwnerPluginID || service != supplierinterfacecap.ServiceName ||
			version != supplierinterfacecap.ContractVersion || method != supplierinterfacecap.MethodOutboundSession ||
			resourceRef != contextID || table != "" {
			t.Fatalf("unexpected governed envelope: %q %q %q %q %q %q", owner, service, version, method, resourceRef, table)
		}
		var received supplierinterfacecap.OutboundSessionRequest
		if err := json.Unmarshal(payload, &received); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if received.EndpointField != request.EndpointField || received.Authentication.SecretFormFields["password"] != "api_secret" ||
			len(received.Steps) != 1 || received.Steps[0].Path != "/servers" {
			t.Fatalf("unexpected session payload: %#v", received)
		}
		return json.Marshal(&supplierinterfacecap.OutboundSessionResponse{Steps: []supplierinterfacecap.OutboundSessionStepResponse{{
			StatusCode: 200,
			Data:       json.RawMessage(`{"data":[]}`),
		}}})
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.ExecuteOutboundSession(context.Background(), contextID, request)
	if err != nil || len(response.Steps) != 1 || response.Steps[0].StatusCode != 200 || string(response.Steps[0].Data) != `{"data":[]}` {
		t.Fatalf("session response=%#v err=%v", response, err)
	}
}

func TestDynamicClientReadsOnlyBoundedFileMetadataAndChunks(t *testing.T) {
	client, err := NewClient(func(owner, service, version, method, resourceRef, table string, payload []byte) ([]byte, error) {
		if owner != supplierinterfacecap.OwnerPluginID || service != supplierinterfacecap.ServiceName || version != supplierinterfacecap.ContractVersion || resourceRef != "src_0123456789abcdef0123456789abcdef" || table != "" {
			t.Fatalf("unexpected envelope")
		}
		switch method {
		case supplierinterfacecap.MethodFilesList:
			var request supplierinterfacecap.ConfigurationFileListRequest
			if json.Unmarshal(payload, &request) != nil || request.FieldKey != "certificate" {
				t.Fatalf("list payload: %s", payload)
			}
			return json.Marshal(&supplierinterfacecap.ConfigurationFileList{Files: []supplierinterfacecap.ConfigurationFile{{Sequence: 1, FileName: "request.csr", MediaType: "text/plain", SizeBytes: 3, SHA256Hex: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}})
		case supplierinterfacecap.MethodFileRead:
			var request supplierinterfacecap.ConfigurationFileReadRequest
			if json.Unmarshal(payload, &request) != nil || request.FieldKey != "certificate" || request.Sequence != 1 || request.MaximumBytes != 3 {
				t.Fatalf("read payload: %s", payload)
			}
			return json.Marshal(&supplierinterfacecap.ConfigurationFileChunk{Sequence: 1, Data: []byte("csr"), EOF: true})
		default:
			t.Fatalf("unexpected method %q", method)
			return nil, nil
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := client.ListConfigurationFiles(context.Background(), "src_0123456789abcdef0123456789abcdef", "certificate")
	if err != nil || len(list.Files) != 1 || list.Files[0].FileName != "request.csr" {
		t.Fatalf("list=%#v err=%v", list, err)
	}
	chunk, err := client.ReadConfigurationFile(context.Background(), "src_0123456789abcdef0123456789abcdef", supplierinterfacecap.ConfigurationFileReadRequest{FieldKey: "certificate", Sequence: 1, MaximumBytes: 3})
	if err != nil || string(chunk.Data) != "csr" || !chunk.EOF {
		t.Fatalf("chunk=%#v err=%v", chunk, err)
	}
}
