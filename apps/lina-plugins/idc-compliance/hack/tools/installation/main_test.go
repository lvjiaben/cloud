// These regressions cover first-run generation, restart stability and damaged
// storage refusal without contacting a vendor or changing the development database.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestRuntimeConfigurationPreservesOperatorChanges covers repeatable external configuration.
func TestRuntimeConfigurationPreservesOperatorChanges(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	template := filepath.Join(root, "apps", "lina-core", "manifest", "config", "config.template.yaml")
	if err := os.MkdirAll(filepath.Dir(template), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(template, []byte("database: original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := prepareRuntimeConfig(root, state, nil); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(state, "config.yaml")
	if err := os.WriteFile(config, []byte("database: customized\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := prepareRuntimeConfig(root, state, nil); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(config)
	if err != nil || string(content) != "database: customized\n" {
		t.Fatal("setup overwrote operator database settings")
	}
	other := t.TempDir()
	if err = prepareRuntimeConfig(root, other, []string{"GF_GCFG_FILE=external.yaml"}); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(other, "config.yaml")); !os.IsNotExist(err) {
		t.Fatal("explicit configuration created an unused default")
	}
}

// TestDeploymentPersists verifies first installation and subsequent restarts keep identity.
func TestDeploymentPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployment.json")
	first, err := loadDeployment(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadDeployment(path)
	if err != nil {
		t.Fatal(err)
	}
	if first.JWT != second.JWT || first.Security.Reservation != second.Security.Reservation || first.Security.Keys["v1"] != second.Security.Keys["v1"] {
		t.Fatal("restart replaced installation keys")
	}
	other, err := loadDeployment(filepath.Join(t.TempDir(), "deployment.json"))
	if err != nil {
		t.Fatal(err)
	}
	if first.JWT == other.JWT || first.Security.Reservation == other.Security.Reservation {
		t.Fatal("installations share secrets")
	}
}

// TestPreparationUsesExplicitDeployment verifies the child receives operator inputs without replacement.
func TestPreparationUsesExplicitDeployment(t *testing.T) {
	root := t.TempDir()
	keyring, identity := filepath.Join(root, "gateway.json"), filepath.Join(root, "identity")
	if err := os.WriteFile(keyring, []byte(`{"active":"test"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(identity, []byte("linst_"+strings.Repeat("a", 64)), 0o600); err != nil {
		t.Fatal(err)
	}
	env := []string{"LINA_JWT_SECRET=explicit-jwt", "LINAPRO_IDC_CORE_SECURITY_KEYRING=explicit-keyring", "LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_KEYRING_FILE=" + keyring, "LINAPRO_COMPLIANCE_GATEWAY_INSTALLATION_ID_FILE=" + identity}
	env = append(env, "GF_GCFG_FILE=config/custom.yaml")
	prepared, err := prepare(root, filepath.Join(root, "private"), env)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range env {
		key, value, _ := strings.Cut(input, "=")
		if key == "GF_GCFG_FILE" {
			value = filepath.Join(root, value)
		}
		if envValue(prepared, key) != value {
			t.Fatal("replaced an explicit deployment input")
		}
	}
}

// TestUnsafePrivateFiles verifies public permissions and linked material cannot be reused on Unix.
func TestUnsafePrivateFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission and symlink checks; Windows access control is deployment-owned")
	}
	root := t.TempDir()
	path := filepath.Join(root, "public")
	if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPrivate(path, 128); err == nil {
		t.Fatal("accepted public permissions")
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readPrivate(link, 128); err == nil {
		t.Fatal("accepted linked installation material")
	}
}

// TestDamagedDeploymentNeverReplaced verifies recovery never silently destroys encrypted data access.
func TestDamagedDeploymentNeverReplaced(t *testing.T) {
	for _, content := range []string{"broken", "{}", "{\"jwt\":\"public\"}"} {
		t.Run(content, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "deployment.json")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadDeployment(path); err == nil {
				t.Fatal("accepted corrupt installation")
			}
			saved, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(saved, []byte(content)) {
				t.Fatal("replaced corrupt installation")
			}
		})
	}
}

// TestExplicitConfigurationPreserved ensures deployment inputs are not overridden by defaults.
func TestExplicitConfigurationPreserved(t *testing.T) {
	env := []string{"A=private", "B="}
	env = defaultEnv(env, "A", "generated")
	env = defaultEnv(env, "B", "generated")
	if envValue(env, "A") != "private" || envValue(env, "B") != "generated" {
		t.Fatal("incorrect defaults")
	}
}

// TestPartialGatewayRefused ensures preparation cannot create a new signing identity beside an old one.
func TestPartialGatewayRefused(t *testing.T) {
	state := t.TempDir()
	path := filepath.Join(state, "installation-id")
	if err := os.WriteFile(path, []byte("old-installation"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := prepareGateway(state, filepath.Join(state, "gateway-auth.json"), path, nil); err == nil {
		t.Fatal("accepted partial gateway installation")
	}
}

// TestExclusivePrivateWrite ensures parallel preparation cannot overwrite existing material.
func TestExclusivePrivateWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private")
	if err := writePrivate(path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := writePrivate(path, []byte("second")); err == nil {
		t.Fatal("overwrote existing material")
	}
}

// TestInitRequiresConfirmation proves an accidental invocation never prepares or touches a database.
func TestInitRequiresConfirmation(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "apps", "lina-core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps", "lina-core", "go.mod"), []byte("module test"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-root", root, "db.init"}); err == nil {
		t.Fatal("database initialization accepted without confirmation")
	}
	if _, err := os.Stat(filepath.Join(root, ".site-runtime")); !os.IsNotExist(err) {
		t.Fatal("unconfirmed initialization changed installation state")
	}
}

// TestDeliveryOutputBoundary prevents generated assets from being written into host sources.
func TestDeliveryOutputBoundary(t *testing.T) {
	root := t.TempDir()
	for _, output := range []string{"apps/lina-core", "apps/lina-vben/apps/web-antd/dist", "hack/generated", "."} {
		if _, _, _, err := deliveryOptions(root, []string{"out=" + output}); err == nil {
			t.Fatalf("accepted source output %s", output)
		}
	}
	target, output, plan, err := deliveryOptions(root, []string{"plugins=1", "target=templates", "plan=1"})
	if err != nil || target != "templates" || !plan || output != filepath.Join(root, "temp", "output") {
		t.Fatalf("incorrect plan: %s %s %t %v", target, output, plan, err)
	}
	if _, _, _, err = deliveryOptions(root, []string{"plugins=0"}); err == nil {
		t.Fatal("accepted a delivery missing business plugins")
	}
}

// TestRendererDeliveryRetainsTracedRuntimeDependencies prevents a build that crashes after copying.
func TestRendererDeliveryRetainsTracedRuntimeDependencies(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	module := filepath.Join("server", "node_modules", "vue-router", "dist", "index.mjs")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(source, module)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, module), []byte("export const route = '/';\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyDeliveryTree(source, target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, module)); err != nil {
		t.Fatal("renderer package lost its traced runtime dependencies", err)
	}
}

// TestDeliveryWorkspaceIncludesSourceBackends verifies the public registration closure.
func TestDeliveryWorkspaceIncludesSourceBackends(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "output", ".build")
	core := filepath.Join(root, "apps", "lina-core")
	for _, item := range []struct{ id, kind string }{{"business", "source"}, {"template", "dynamic"}} {
		dir := filepath.Join(root, "apps", "lina-plugins", item.id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte("id: "+item.id+"\ntype: "+item.kind+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module plugin-"+item.id+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	work, err := deliveryWorkspace(root, stage, core)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(work)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), filepath.ToSlash(core)) {
		t.Fatal("workspace omitted unmodified host")
	}
	registrations, err := os.ReadFile(filepath.Join(stage, "official-plugins", "plugins.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(registrations), "plugin-business/backend") || strings.Contains(string(registrations), "plugin-template/backend") {
		t.Fatal("incorrect source/dynamic registration")
	}
	if _, err = os.Stat(core); !os.IsNotExist(err) {
		t.Fatal("workspace generator wrote host sources")
	}
}

// TestWindowsPackageManagerShim verifies pnpm uses the platform command interpreter.
func TestWindowsPackageManagerShim(t *testing.T) {
	program, args := executionCommand("windows", "pnpm.cmd", []string{"run", "build"})
	if program != "cmd.exe" || strings.Join(args, " ") != "/d /c pnpm.cmd run build" {
		t.Fatal("invalid Windows shim invocation")
	}
	program, args = executionCommand("linux", "pnpm", []string{"run", "build"})
	if program != "pnpm" || strings.Join(args, " ") != "run build" {
		t.Fatal("unexpected Unix shell dependency")
	}
}

// TestCleanCheckoutRefusesMissingEmbeddedAssets prevents a successful unusable delivery.
func TestCleanCheckoutRefusesMissingEmbeddedAssets(t *testing.T) {
	root := t.TempDir()
	err := checkPackedDeliveryAssets(root)
	if err == nil || !strings.Contains(err.Error(), "pack.assets") || !strings.Contains(err.Error(), "target=templates") {
		t.Fatal("missing actionable clean-checkout refusal")
	}
	if _, err = os.Stat(filepath.Join(root, "apps")); !os.IsNotExist(err) {
		t.Fatal("preflight wrote host sources")
	}
}

// TestDevelopmentPlan verifies isolated ports propagate to every independent entry.
func TestDevelopmentPlan(t *testing.T) {
	root := t.TempDir()
	output, plan, services, err := developmentPlan(root, []string{"plan=1", "rebuild=0", "backend_port=16030", "frontend_port=16031", "customer_port=16032", "website_port=16680"}, nil)
	if err != nil || !plan || len(services) != 6 || output != filepath.Join(root, "temp", "output") {
		t.Fatalf("invalid plan: %s %v %d %v", output, plan, len(services), err)
	}
	if envValue(services[0].env, "LINAPRO_SERVER_ADDRESS") != ":16030" || envValue(services[2].env, "LINAPRO_BACKEND_PROXY_TARGET") != "http://127.0.0.1:16030" {
		t.Fatal("backend/customer port override was lost")
	}
	if envValue(services[2].env, "WEBSITE_PUBLIC_ORIGIN") != "http://localhost:16680" {
		t.Fatal("customer purchase redirect lost configured website origin")
	}
	if envValue(services[3].env, "NUXT_PUBLIC_CUSTOMER_URL") != "http://localhost:16032" || envValue(services[3].env, "NUXT_API_ORIGIN") != "http://127.0.0.1:16030" {
		t.Fatal("renderer used default upstreams")
	}
	if !strings.Contains(envValue(services[5].env, "WEBSITE_SSR_ORIGINS"), "16682") {
		t.Fatal("second renderer is missing")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("planning changed the filesystem")
	}
	for _, args := range [][]string{{"rebuild=invalid"}, {"backend_port=0"}, {"website_port=65535"}, {"customer_port=6030"}, {"frontend_port=6681"}, {"dir=apps/lina-core"}, {"out=" + filepath.Join(root, "apps")}} {
		if _, _, _, err := developmentPlan(root, args, nil); err == nil {
			t.Fatalf("accepted invalid options: %v", args)
		}
	}
}

// TestDevelopmentOccupiedPort verifies an unrelated listener remains untouched.
func TestDevelopmentOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	if err := checkDevelopmentPorts([]developmentService{{name: "existing", port: port}}); err == nil {
		t.Fatal("accepted occupied port")
	}
	connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal("preflight interrupted existing listener")
	}
	_ = connection.Close()
}

// TestDevelopmentProcessCleanup exercises partial-start failure and cancellation.
func TestDevelopmentProcessCleanup(t *testing.T) {
	child := developmentService{name: "owned child", program: os.Args[0], args: []string{"-test.run=^TestDevelopmentChildProcess$"}, env: append(os.Environ(), "LINA_INSTALLATION_TEST_CHILD=1")}
	start := time.Now()
	if err := superviseDevelopment(context.Background(), []developmentService{child, {name: "missing", program: filepath.Join(t.TempDir(), "missing")}}); err == nil {
		t.Fatal("partial-start failure was swallowed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := superviseDevelopment(ctx, []developmentService{child}); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("owned children were not promptly reaped")
	}
}

// TestDevelopmentChildProcess is a subprocess fixture, never an external service.
func TestDevelopmentChildProcess(t *testing.T) {
	if os.Getenv("LINA_INSTALLATION_TEST_CHILD") == "1" {
		if address := os.Getenv("LINA_INSTALLATION_TEST_ADDRESS"); address != "" {
			server := &http.Server{Addr: address, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/exit" {
					code, _ := strconv.Atoi(r.URL.Query().Get("code"))
					os.Exit(code)
				}
				fmt.Fprint(w, os.Getpid())
			}), ReadHeaderTimeout: time.Second}
			_ = server.ListenAndServe()
			os.Exit(2)
		}
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

// TestDevelopmentBackendReadiness rejects a listening port without a successful API.
func TestDevelopmentBackendReadiness(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api.json" {
			t.Errorf("unexpected readiness endpoint: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	port := server.Listener.Addr().(*net.TCPAddr).Port
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if consumed, err := waitDevelopmentBackend(ctx, port, make(chan error)); consumed || err == nil {
		t.Fatal("unready API was accepted")
	}
	results := make(chan error, 1)
	results <- errors.New("backend exited")
	if consumed, err := waitDevelopmentBackend(context.Background(), port, results); !consumed || err == nil {
		t.Fatal("backend exit was not propagated")
	}
	ready := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer ready.Close()
	if consumed, err := waitDevelopmentBackend(context.Background(), ready.Listener.Addr().(*net.TCPAddr).Port, make(chan error)); consumed || err != nil {
		t.Fatalf("ready API refused: %v", err)
	}
}

// TestDevelopmentIndependentRecovery exercises real HTTP children, both clean and failed exits,
// repeated recovery and shutdown during retry, without restarting an unaffected peer.
func TestDevelopmentIndependentRecovery(t *testing.T) {
	address := func() string {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		value := listener.Addr().String()
		_ = listener.Close()
		return value
	}
	childAddress, peerAddress := address(), address()
	service := func(name, address string) developmentService {
		return developmentService{name: name, program: os.Args[0], args: []string{"-test.run=^TestDevelopmentChildProcess$"}, env: append(os.Environ(), "LINA_INSTALLATION_TEST_CHILD=1", "LINA_INSTALLATION_TEST_ADDRESS="+address)}
	}
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() {
		finished <- superviseDevelopment(ctx, []developmentService{service("recovering", childAddress), service("peer", peerAddress)})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-finished:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("supervisor leaked owned children")
		}
	})
	client := &http.Client{Timeout: 200 * time.Millisecond}
	pid := func(address string) string {
		response, err := client.Get("http://" + address)
		if err != nil {
			return ""
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		return string(body)
	}
	awaitPID := func(address, previous string) string {
		deadline := time.Now().Add(6 * time.Second)
		for time.Now().Before(deadline) {
			current := pid(address)
			if current != "" && current != previous {
				return current
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("service %s did not recover", address)
		return ""
	}
	previous, peerPID := awaitPID(childAddress, ""), awaitPID(peerAddress, "")
	for _, code := range []string{"1", "0"} {
		response, _ := client.Get("http://" + childAddress + "/exit?code=" + code)
		if response != nil {
			_ = response.Body.Close()
		}
		previous = awaitPID(childAddress, previous)
		if current := pid(peerAddress); current != peerPID {
			t.Fatalf("healthy peer was restarted: %s -> %s", peerPID, current)
		}
	}
	response, _ := client.Get("http://" + childAddress + "/exit?code=1")
	if response != nil {
		_ = response.Body.Close()
	}
	// The cleanup cancels while recovery is pending; neither service may survive it.
	cancel()
	time.Sleep(100 * time.Millisecond)
	if pid(childAddress) != "" || pid(peerAddress) != "" {
		t.Fatal("cancellation left a service listening")
	}
}
