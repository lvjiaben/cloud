// A copied ordinary artifact must run outside the repository without source imports.
package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestOrdinaryArtifactRunsOutsideProject(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("ordinary website requires Node.js: ", err)
	}
	_, source, _, _ := runtime.Caller(0)
	pluginRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	fixture := t.TempDir()
	for _, name := range []string{"gateway/server.mjs", "gateway/site.mjs", "gateway/commerce.mjs", "gateway/brand-identity.mjs", "DEPLOYMENT.md", "DEPLOYMENT.zh-CN.md"} {
		data, err := os.ReadFile(filepath.Join(pluginRoot, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(fixture, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	public := filepath.Join(fixture, "frontend", "public")
	if err := os.MkdirAll(public, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(public, "index.html"), []byte("<html><head></head><body>standalone delivery</body></html>"), 0644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "ordinary-artifact")
	if err := build(fixture, output); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"code":0,"data":{"brand":null}}`)
	}))
	defer upstream.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	command := exec.Command(node, filepath.Join(output, "gateway", "server.mjs"))
	command.Dir = t.TempDir()
	command.Env = append(os.Environ(), "HOST=127.0.0.1", fmt.Sprintf("PORT=%d", port), "WEBSITE_API_ORIGIN="+upstream.URL, "WEBSITE_CUSTOMER_URL=https://customer.example", "WEBSITE_BRAND_CACHE_DIR="+filepath.Join(output, "private-brand-cache"))
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	client := &http.Client{Timeout: time.Second}
	address := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(10 * time.Second)
	for {
		response, err := client.Get(address + "/index.html")
		if err == nil {
			data, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode != 200 || !strings.Contains(string(data), "standalone delivery") || !strings.Contains(string(data), "customer.example") {
				t.Fatalf("ordinary artifact response %d: %s", response.StatusCode, data)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("ordinary artifact did not start outside project: ", err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	response, err := client.Get(address + "/__website/session")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || !strings.Contains(string(data), `"checked":false`) {
		t.Fatalf("commerce module missing from ordinary artifact: %d %s", response.StatusCode, data)
	}
}
