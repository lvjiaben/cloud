// These tests construct isolated session files and repositories, and use a
// disposable child test process to prove failure and timeout reporting.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFixture(t *testing.T, root, name, text string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAuditCountsWithoutContent(t *testing.T) {
	var lines bytes.Buffer
	add := func(kind string, payload any) {
		t.Helper()
		if err := json.NewEncoder(&lines).Encode(map[string]any{"type": kind, "payload": payload}); err != nil {
			t.Fatal(err)
		}
	}
	add("compacted", map[string]any{"message": "PRIVATE_INTERNAL_CONTENT"})
	add("response_item", map[string]any{"type": "function_call", "name": "PRIVATE_TOOL_TOKEN", "arguments": "{}"})
	for i := 0; i < 2; i++ {
		add("response_item", map[string]any{"type": "custom_tool_call", "name": "exec", "input": `text(await tools.exec_command({cmd:"cat .agents/rules/context-loading.md\necho PRIVATE_TOKEN"}))`})
	}
	add("response_item", map[string]any{"type": "custom_tool_call_output", "output": []any{map[string]any{"type": "text", "text": "PRIVATE_OUTPUT"}, map[string]any{"type": "image", "data": "PRIVATE_IMAGE_BASE64"}}})
	add("event_msg", map[string]any{"type": "token_count", "info": map[string]any{"last_token_usage": map[string]any{"input_tokens": 100, "cached_input_tokens": 80, "output_tokens": 2}, "total_token_usage": map[string]any{"input_tokens": 500, "cached_input_tokens": 400, "output_tokens": 10}}})
	p := writeFixture(t, t.TempDir(), "session.jsonl", lines.String())
	r, err := auditSession(p)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Complete || r.Calls != 3 || r.Compactions != 1 || r.Images != 1 || r.TextBytes != int64(len("PRIVATE_OUTPUT")) || r.Latest == nil || r.Latest.Input == nil || *r.Latest.Input != 100 || *r.Cumulative.Input != 500 || len(r.Reads) != 1 || r.Reads[0].Count != 2 {
		t.Fatalf("wrong aggregates: %+v", r)
	}
	var out bytes.Buffer
	if err = boundedJSON(&out, r); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "PRIVATE_") || out.Len() > 8192 {
		t.Fatal("raw content leaked or report unbounded")
	}
	if _, err = measureAndFailTest(); err != nil {
		t.Fatal(err)
	}
}

// measureAndFailTest exercises unknown serialized output without treating it as
// rendered text or executing any of its content.
func measureAndFailTest() (int64, error) {
	n, images, unknown := measureOutput(json.RawMessage(`{"other":"PRIVATE_UNKNOWN"}`), 0)
	if n != 0 || images != 0 || unknown == 0 {
		return 0, fmt.Errorf("unknown output misclassified")
	}
	return unknown, nil
}

func TestAuditMalformedAndSourceSelection(t *testing.T) {
	root := t.TempDir()
	p := writeFixture(t, root, "broken.jsonl", "{bad}\n")
	r, err := auditSession(p)
	if err == nil || r.Complete || r.Malformed != 1 {
		t.Fatal("corrupt source accepted")
	}
	p = writeFixture(t, root, "oversized.jsonl", strings.Repeat("x", 8*1024*1024+1))
	r, err = auditSession(p)
	if err == nil || r.Complete {
		t.Fatal("oversized record accepted")
	}
	t.Setenv("CODEX_HOME", root)
	id := "01a1533d-0a8d-7dd3-a0bb-c6b704a6716a"
	one := writeFixture(t, root, "sessions/day/rollout-"+id+".jsonl", "{}\n")
	one, err = filepath.EvalSymlinks(one)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := sessionPath(id, ""); err != nil || got != one {
		t.Fatalf("exact source %q %v", got, err)
	}
	writeFixture(t, root, "sessions/other/rollout-"+id+".jsonl", "{}\n")
	if _, err := sessionPath(id, ""); err == nil {
		t.Fatal("ambiguous session accepted")
	}
	if _, err := sessionPath("../escape", ""); err == nil {
		t.Fatal("traversal ID accepted")
	}
}

func TestMissingUsageAndOutputBounds(t *testing.T) {
	p := writeFixture(t, t.TempDir(), "session.jsonl", "{\"type\":\"session_meta\",\"payload\":{}}\n")
	r, err := auditSession(p)
	if err != nil || r.Latest != nil || r.Cumulative != nil {
		t.Fatal("invented missing counters")
	}
	var out bytes.Buffer
	if err = boundedJSON(&out, strings.Repeat("x", 9000)); err == nil || out.Len() != 0 {
		t.Fatal("oversized summary was partially emitted")
	}
}

func TestCheckPlanIsScopedAndDeduplicated(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, ".agents/tools/a.go", "package main\n")
	writeFixture(t, root, ".agents/tools/a_test.go", "package main\n")
	writeFixture(t, root, "openspec/changes/selected/tasks.md", "- [ ] 1.1 task\n")
	writeFixture(t, root, "openspec/changes/unrelated/tasks.md", "other\n")
	p, err := planChecks(root, []string{".agents/tools", ".agents/tools/a.go"}, []string{"selected"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Steps) != 4 || p.FileCount != 3 || len(p.Manual) != 0 {
		t.Fatalf("unexpected plan: %+v", p)
	}
	var out bytes.Buffer
	if err = runWorkflow(root, []string{"check", "--path", ".agents/tools", "--change", "selected", "--plan"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "unrelated") {
		t.Fatal("expanded unrelated change")
	}
	if _, err = os.Stat(filepath.Join(root, "temp")); !os.IsNotExist(err) {
		t.Fatal("plan mode created logs")
	}
	for _, scope := range []string{".", "../outside", "temp", "/tmp"} {
		if _, err := sourcePath(root, scope); err == nil {
			t.Fatalf("accepted scope %q", scope)
		}
	}
	writeFixture(t, root, "docs/unknown.vue", "<template/>\n")
	plan, err := planChecks(root, []string{"docs/unknown.vue"}, nil)
	if err != nil || len(plan.Manual) != 1 {
		t.Fatal("unknown coverage hidden")
	}
	checked, checkErr := executeChecks(root, plan, time.Second)
	if checkErr == nil || checked.Complete || len(checked.Results) != 0 {
		t.Fatal("unconfigured verification was reported as passed")
	}
}

func TestModulePlanningUsesExistingLint(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "apps/lina-plugins/example/go.mod", "module example.test/plugin\ngo 1.25\n")
	writeFixture(t, root, "apps/lina-plugins/example/backend/a.go", "package backend\n")
	p, err := planChecks(root, []string{"apps/lina-plugins/example/backend"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Steps) != 2 || p.Steps[0].Env[0] != "GOWORK=off" || !strings.Contains(strings.Join(p.Steps[1].Args, " "), "./hack/tools/linactl lint.go") {
		t.Fatalf("did not reuse module checks: %+v", p)
	}
	p, err = planChecks(root, []string{"apps/lina-plugins/example/go.mod"}, nil)
	if err != nil || len(p.Steps) != 2 || !strings.Contains(strings.Join(p.Steps[0].Args, " "), "./...") {
		t.Fatal("dependency change did not select its full module")
	}
}

func TestDocumentChecksAndCodeFences(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "docs/target.md", "# Target\n")
	writeFixture(t, root, "docs/readme.md", "[real](target.md)\n`[example](absent.md)`\n````text\n```\n[example](absent.md)\n````\n")
	writeFixture(t, root, "docs/data.json", "{}")
	log, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := log.Close(); err != nil {
			t.Error(err)
		}
	})
	if err = checkDocuments(root, []string{"docs/readme.md", "docs/data.json"}, log); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "docs/readme.md", "[broken](absent.md)\n")
	if err = checkDocuments(root, []string{"docs/readme.md"}, log); err == nil {
		t.Fatal("missing link accepted")
	}
	writeFixture(t, root, "docs/data.json", "{broken}")
	if err = checkDocuments(root, []string{"docs/data.json"}, log); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}

func TestCheckChildProcess(t *testing.T) {
	mode := os.Getenv("WORKFLOW_CHECK_HELPER")
	if mode == "fail" {
		fmt.Print("PRIVATE_SUBPROCESS_OUTPUT")
		os.Exit(3)
	}
	if mode == "hang" {
		time.Sleep(time.Second)
		os.Exit(0)
	}
}

func TestFailureTimeoutAndLogs(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"fail", "hang"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			p := &checkPlan{Steps: []checkStep{{Name: "fixture", Dir: ".", Command: exe, Args: []string{"-test.run=^TestCheckChildProcess$"}, Env: []string{"WORKFLOW_CHECK_HELPER=" + mode}}}}
			timeout := time.Second
			if mode == "hang" {
				timeout = 20 * time.Millisecond
			}
			r, err := executeChecks(root, p, timeout)
			if err == nil || r == nil || r.Complete || len(r.Results) != 1 {
				t.Fatalf("failure hidden: %+v %v", r, err)
			}
			if mode == "fail" && r.Results[0].ExitCode != 3 {
				t.Fatal("lost exit code")
			}
			if mode == "hang" && (r.Results[0].Status != "timeout" || r.Results[0].ExitCode != 124) {
				t.Fatal("timeout hidden")
			}
			if _, err = os.Stat(filepath.Join(root, filepath.FromSlash(r.Results[0].Log))); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err = boundedJSON(&out, r); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out.String(), "PRIVATE_") {
				t.Fatal("raw log leaked")
			}
		})
	}
}

func TestUnavailableCommandIsNotPassed(t *testing.T) {
	root := t.TempDir()
	p := &checkPlan{Steps: []checkStep{{Name: "missing", Dir: ".", Command: "workflow-command-that-does-not-exist-20261020"}}}
	r, err := executeChecks(root, p, time.Second)
	if err == nil || r.Complete || r.Results[0].Status != "not_started" || r.Results[0].ExitCode != 127 {
		t.Fatalf("unstarted command hidden: %+v %v", r, err)
	}
}
