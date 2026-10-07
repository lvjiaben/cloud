package idcnotify

import (
	"bytes"
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestFrontendDeliveryOwnsBuildAndHostedMount(t *testing.T) {
	manifest, err := fs.ReadFile(EmbeddedFiles, "plugin.yaml")
	if err != nil {
		t.Fatalf("read embedded plugin manifest: %v", err)
	}
	if !bytes.Contains(manifest, []byte("source: frontend/dist")) {
		t.Fatal("plugin manifest must publish the plugin-owned frontend/dist directory")
	}
	manifestText := string(manifest)
	if strings.Count(manifestText, "pluginAccessMode: embedded-mount") != 4 ||
		strings.Count(manifestText, "embeddedSrc: /x-assets/idc-notify/v0.1.4/mount.js") != 4 {
		t.Fatal("every notification page menu must mount the plugin-owned frontend bundle")
	}
	for _, routePath := range []string{
		"/idc/notifications/wechat-work-binding",
		"/idc/notifications/channels",
		"/idc/notifications/templates",
		"/idc/notifications/deliveries",
	} {
		if !strings.Contains(manifestText, "routePath: "+routePath) {
			t.Fatalf("plugin manifest misses embedded routePath %q", routePath)
		}
	}

	mountBundle, err := fs.ReadFile(EmbeddedFiles, "frontend/dist/mount.js")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			t.Skip("run the plugin-owned frontend build before checking the embedded bundle")
		}
		t.Fatalf("read embedded frontend mount bundle: %v", err)
	}
	if !bytes.Contains(mountBundle, []byte("mount")) {
		t.Fatal("frontend bundle must export the LinaPro mount contract")
	}
	for _, forbidden := range [][]byte{[]byte("web-antd"), []byte("@vben/")} {
		if bytes.Contains(mountBundle, forbidden) {
			t.Fatalf("frontend bundle crosses the host dependency boundary: %s", forbidden)
		}
	}
}

func TestFrontendVisibleLabelsUsePluginTranslator(t *testing.T) {
	app, err := fs.ReadFile(EmbeddedFiles, "frontend/src/App.vue")
	if err != nil {
		t.Fatalf("read notification frontend app: %v", err)
	}
	for _, forbidden := range []string{
		">Provider<", ">Pending<", ">Event<", ">Channel<", ">Locale<",
		">简体中文<", ">English<", ">Subject<", ">Body<", ">Status<",
		">Retry<", ">Queued<",
	} {
		if strings.Contains(string(app), forbidden) {
			t.Fatalf("notification frontend contains untranslated visible label %q", forbidden)
		}
	}
	for _, required := range []string{
		"provider", "pending", "event", "channel",
		"locale", "localeZhCN", "localeEnUS", "subject",
		"body", "status", "retryCount", "queuedAt",
	} {
		if !containsTranslatorCall(string(app), required) {
			t.Fatalf("notification frontend does not translate visible label with %q", required)
		}
	}

	catalog, err := fs.ReadFile(EmbeddedFiles, "frontend/src/notification/messages.ts")
	if err != nil {
		t.Fatalf("read notification frontend message catalog: %v", err)
	}
	for _, key := range []string{
		"provider:", "pending:", "event:", "channel:", "locale:", "localeZhCN:",
		"localeEnUS:", "subject:", "body:", "status:", "retryCount:", "queuedAt:",
	} {
		if !strings.Contains(string(catalog), key) {
			t.Fatalf("notification frontend message catalog misses %q", key)
		}
	}
}

func containsTranslatorCall(source string, key string) bool {
	return strings.Contains(source, `tr("`+key+`")`) ||
		strings.Contains(source, `tr('`+key+`')`)
}

func TestFrontendRequestsUseNotificationPluginRoute(t *testing.T) {
	api, err := fs.ReadFile(EmbeddedFiles, "frontend/src/notification/api.ts")
	if err != nil {
		t.Fatalf("read notification frontend API client: %v", err)
	}
	if !strings.Contains(string(api), "'/x/idc-notify/api/v1/admin/notifications'") {
		t.Fatal("notification frontend API client does not use the notification plugin route")
	}
	if strings.Contains(string(api), "'/api/v1/") || strings.Contains(string(api), "`/api/v1/") ||
		strings.Contains(string(api), "/x/idc-core/") {
		t.Fatal("notification frontend API client can still route requests to the host or IDC core plugin")
	}
}

func TestFrontendUsesBusinessLabelsAndCursorPagination(t *testing.T) {
	app, err := fs.ReadFile(EmbeddedFiles, "frontend/src/App.vue")
	if err != nil {
		t.Fatalf("read notification frontend app: %v", err)
	}
	source := string(app)
	for _, forbidden := range []string{
		"{{ item.eventCode }}", "{{ event.code }}", "{{ selectedDelivery.eventCode }}",
		"{{ selectedDelivery.taskNo }}", "{{ item.providerCode || '—' }}",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("notification frontend exposes technical identifier through %q", forbidden)
		}
	}
	for _, required := range []string{
		"eventLabel(item.eventCode)", "eventLabel(event.code)", "providerLabel(item)",
		"credentialMaskedTail", "channelDisplayName", "loadMoreDeliveries", "api.deliveries(nextCursor.value)",
		"deliverySelectionGeneration", "eligibilityRequestGeneration", "selection !== deliverySelectionGeneration",
		"eligibilityCheckFailed", "resetDeliverySelection()",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("notification frontend misses safe label or pagination behavior %q", required)
		}
	}

	catalog, err := fs.ReadFile(EmbeddedFiles, "frontend/src/notification/messages.ts")
	if err != nil {
		t.Fatalf("read notification frontend message catalog: %v", err)
	}
	for _, eventKey := range []string{
		"event_account_password_reset", "event_support_ticket_updated", "event_service_expiry_reminder",
		"event_billing_payment_result", "event_marketing_campaign", "eventUnknown", "channelUnknown", "loadMore",
	} {
		if !strings.Contains(string(catalog), eventKey) {
			t.Fatalf("notification frontend message catalog misses %q", eventKey)
		}
	}
}
