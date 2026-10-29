// This file verifies guest facts never substitute the upstream account or expose credentials.
package zjmfcloud

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResourceGuestProjection(t *testing.T) {
	object, err := decodeObject([]byte(`{"hostname":"cloud-1","os_name":"Debian","osuser":"root","username":"upstream-secret-account","rootpassword":"guest-password","vncpass":"console-password","port":"1234","cpu":"2","memory":2048,"rescue":1,"network_type":"public","bootorder":"cd","ip":[{"ip":"203.0.113.10"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer clearRawObject(object)
	data := resourceObservationData(object, "on")
	if data["login_username"] != "root" || data["port"] != 1234 || data["cpu_cores"] != 2 || data["memory_mib"] != 2048 || data["rescue"] != true {
		t.Fatalf("guest facts: %+v", data)
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"upstream-secret-account", "guest-password", "console-password", "rootpassword", "vncpass"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("secret or provider account leaked")
		}
	}
	invalid, err := decodeObject([]byte(`{"hostname":"<script>","osuser":"root\nattack","port":"auto","cpu":0,"memory":-1,"rescue":2}`))
	if err != nil {
		t.Fatal(err)
	}
	defer clearRawObject(invalid)
	data = resourceObservationData(invalid, "on")
	for _, field := range []string{"hostname", "login_username", "port", "cpu_cores", "memory_mib", "rescue"} {
		if _, exists := data[field]; exists {
			t.Fatalf("invalid %s confirmed", field)
		}
	}
}
