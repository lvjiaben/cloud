package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func fixture(t *testing.T, root, name, data string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(data), 0600); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestSectionsAndTasks(t *testing.T) {
	data := "# Root\nintro\n## Alpha\na\n### Child\nb\n## Alpha extra\nc\n## Tasks\n- [ ] FB-1 first\n  continuation\n- [x] FB-10 other\n"
	for _, tc := range []struct{ selector, want string }{
		{"Alpha", "## Alpha\na\n### Child\nb\n"},
		{"Child", "### Child\nb\n"},
		{"FB-1", "- [ ] FB-1 first\n  continuation\n"},
	} {
		t.Run(tc.selector, func(t *testing.T) {
			got, e := selectBlock(data, tc.selector)
			if e != nil || got != tc.want {
				t.Fatalf("got %q, err %v", got, e)
			}
		})
	}
	if _, e := selectBlock(data, "Alph"); e == nil || !strings.Contains(e.Error(), "歧义") {
		t.Fatalf("missing ambiguity: %v", e)
	}
	if _, e := selectBlock(data, "FB-100"); e == nil {
		t.Fatal("matched task prefix")
	}
	if _, e := selectBlock("## Repeat\na\n## Repeat\nb\n", "Repeat"); e == nil {
		t.Fatal("duplicate exact heading accepted")
	}
	if _, e := selectBlock("````\n## fake\n````\n## real\nx\n", "fake"); e == nil {
		t.Fatal("matched code heading")
	}
}
func TestReadSize(t *testing.T) {
	huge := "## Huge\n" + strings.Repeat("中", maxRead)
	for _, selector := range []string{"", "Huge", "@2"} {
		if _, e := selectBlock(huge, selector); e == nil {
			t.Fatalf("accepted oversized %q", selector)
		}
	}
	got, e := selectBlock(huge+"\n## Small\nok\n", "Small")
	if e != nil || got != "## Small\nok\n" {
		t.Fatalf("small section: %q %v", got, e)
	}
}
func TestLineEntries(t *testing.T) {
	data := "# Preferences\n- first\n  continued\n\n  another paragraph\n- second\nlast\n"
	got, e := selectBlock(data, "@5")
	if e != nil || got != "- first\n  continued\n\n  another paragraph\n" {
		t.Fatalf("bullet incomplete: %q %v", got, e)
	}
	req := "# Spec\n## Requirement: Full\nrule\n### Scenario: one\nresult\n### Scenario: two\nresult2\n## Requirement: Next\nnext\n"
	got, e = selectBlock(req, "@5")
	if e != nil || got != "## Requirement: Full\nrule\n### Scenario: one\nresult\n### Scenario: two\nresult2\n" {
		t.Fatalf("requirement incomplete: %q %v", got, e)
	}
	got, e = selectBlock("# Text\nparagraph a\ncontinued\n\nparagraph b\n", "@3")
	if e != nil || got != "paragraph a\ncontinued\n" {
		t.Fatalf("paragraph: %q %v", got, e)
	}
	for _, s := range []string{"@0", "@abc", "@100"} {
		if _, e := selectBlock(req, s); e == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}
func TestSafePath(t *testing.T) {
	root := t.TempDir()
	fixture(t, root, ".agents/rules/a.md", "hello")
	for _, name := range []string{"../outside.md", ".agents/rules/../../AGENTS.md", "/tmp/a.md", "C:\\a.md", "apps/a.md", ".agents/rules/runtime/secret.md"} {
		if _, e := safePath(root, name); e == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	if _, e := safePath(root, ".agents/rules/a.md"); e != nil {
		t.Fatal(e)
	}
	outside := fixture(t, t.TempDir(), "outside.md", "secret")
	if e := os.Symlink(outside, filepath.Join(root, ".agents/rules/link.md")); e != nil {
		t.Skipf("symlink unavailable: %v", e)
	}
	if _, e := safePath(root, ".agents/rules/link.md"); e == nil {
		t.Fatal("symlink escaped")
	}
}
func TestLocateBoundedScopes(t *testing.T) {
	root := t.TempDir()
	fixture(t, root, ".agents/rules/rule.md", "# Governing\nneedle body\n"+strings.Repeat("a", 1024*1024)+"needle\n")
	fixture(t, root, "openspec/specs/a/spec.md", "# needle title\n"+strings.Repeat("needle body\n", 20))
	for _, name := range []string{"openspec/changes/archive/old.md", "openspec/changes/x/evidence/result.md", "openspec/changes/x/verification-result.md", ".agents/rules/runtime/credentials.md", "apps/code.md"} {
		fixture(t, root, name, "# needle hidden\n")
	}
	var out bytes.Buffer
	if e := locate(root, []string{"needle"}, 2, &out); e != nil {
		t.Fatal(e)
	}
	text := out.String()
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 4 {
		t.Fatalf("output not bounded: %d", len(lines))
	}
	if !strings.Contains(lines[1], "# needle title") {
		t.Fatalf("heading not prioritized: %s", text)
	}
	for _, line := range lines[1:3] {
		if utf8.RuneCountInString(line) > 300 {
			t.Fatalf("long result: %d", utf8.RuneCountInString(line))
		}
	}
	if !strings.Contains(text, "命中 23 行") {
		t.Fatalf("missing huge line or skipped filters: %s", text)
	}
	out.Reset()
	if e := locate(root, []string{"absent"}, 12, &out); e == nil {
		t.Fatal("no-match succeeded")
	}
}
func TestRootAndArguments(t *testing.T) {
	root := t.TempDir()
	fixture(t, root, "AGENTS.md", "entry")
	fixture(t, root, "openspec/specs/a/spec.md", "# Key\nrule\n")
	got, e := findRoot(filepath.Join(root, "openspec/specs/a"))
	if e != nil || got != root {
		t.Fatalf("root %s %v", got, e)
	}
	for _, args := range [][]string{{"locate"}, {"locate", "--limit", "0", "a"}, {"locate", "--limit", "51", "a"}, {"read"}, {"nope"}} {
		if e := run(root, args, &bytes.Buffer{}); e == nil {
			t.Fatalf("accepted args %v", args)
		}
	}
	var out bytes.Buffer
	if e := run(root, []string{"read", "openspec/specs/a/spec.md", "Key"}, &out); e != nil || out.String() != "# Key\nrule\n" {
		t.Fatalf("read %q %v", out.String(), e)
	}
}

func TestHistoricalWorkflowExcludedButReadable(t *testing.T) {
	root := t.TempDir()
	old := "# 历史开发流程\n全部代码五步流程\n"
	for _, name := range []string{"00-title.md", "02-scope.md", "05-development-workflow.md", "06-memory-maintenance.md", "index.md"} {
		fixture(t, root, ".agents/memory/preferences/"+name, old)
	}
	var out bytes.Buffer
	if err := locate(root, []string{"全部代码五步流程"}, 12, &out); err == nil {
		t.Fatal("historical workflow leaked into current context")
	}
	out.Reset()
	if err := run(root, []string{"read", ".agents/memory/preferences/05-development-workflow.md"}, &out); err != nil || out.String() != old {
		t.Fatalf("explicit historical read failed: %q %v", out.String(), err)
	}
}

func TestSourcePriorityAndByteBudget(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{".agents/rules/current.md", ".agents/memory/preferences/01-business.md", "openspec/changes/live/proposal.md", "openspec/specs/base/spec.md"} {
		fixture(t, root, path, "# 退款\n"+strings.Repeat("退款"+strings.Repeat("字", 400)+"\n", 12))
	}
	var out bytes.Buffer
	if err := locate(root, []string{"退款"}, 12, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() > 6*1024 {
		t.Fatalf("default output too large: %d bytes", out.Len())
	}
	lines := strings.Split(out.String(), "\n")
	for i, want := range []string{".agents/rules/", ".agents/memory/preferences/", "openspec/changes/", "openspec/specs/"} {
		if !strings.HasPrefix(lines[i+1], want) {
			t.Fatalf("wrong source order: %s", out.String())
		}
	}
	if !utf8.Valid(out.Bytes()) {
		t.Fatal("output truncated UTF-8")
	}
}

func TestPhaseValidationReportsExcluded(t *testing.T) {
	root := t.TempDir()
	reports := []string{
		"phase-10-tax-invoice-red-letter-validation.md",
		"phase-6-credit-refund-regression-validation.md",
		"phase-6-credit-refund-verification.md",
		"phase-6-validation-results.md",
		"phase-6-verification-results.md",
	}
	for _, name := range reports {
		fixture(t, root, "openspec/changes/billing/"+name, "# 退款报告\n退款历史证据\n")
	}
	fixture(t, root, "openspec/specs/refund-validation/spec.md", "# 退款校验能力\n业务规范\n")
	var out bytes.Buffer
	if err := locate(root, []string{"退款"}, 12, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "refund-validation/spec.md") || strings.Contains(out.String(), "phase-") {
		t.Fatalf("incorrect report filtering: %s", out.String())
	}
	out.Reset()
	if err := run(root, []string{"read", "openspec/changes/billing/" + reports[0]}, &out); err != nil || !strings.Contains(out.String(), "退款历史证据") {
		t.Fatalf("explicit report read failed: %q %v", out.String(), err)
	}
}

func TestMixedTaskIDsAndNestedAcceptance(t *testing.T) {
	data := "# Tasks\n- [ ] 2.7.a parent\n  prose\n  - [ ] 2.7.a.1 first acceptance\n    continuation\n    - [ ] detail nested\n  - [ ] 2.7.a.2 second acceptance\n- [ ] 2.7.b.1 other\n- [ ] 2.7 parent numeric\n- [ ] 2.8. next\n"
	for _, tc := range []struct{ selector, want string }{
		{"2.7.a", "- [ ] 2.7.a parent\n  prose\n  - [ ] 2.7.a.1 first acceptance\n    continuation\n    - [ ] detail nested\n  - [ ] 2.7.a.2 second acceptance\n"},
		{"2.7.a.1", "  - [ ] 2.7.a.1 first acceptance\n    continuation\n    - [ ] detail nested\n"},
		{"2.7.b.1", "- [ ] 2.7.b.1 other\n"},
		{"2.7", "- [ ] 2.7 parent numeric\n"},
		{"2.8", "- [ ] 2.8. next\n"},
		{"@2", "- [ ] 2.7.a parent\n  prose\n  - [ ] 2.7.a.1 first acceptance\n    continuation\n    - [ ] detail nested\n  - [ ] 2.7.a.2 second acceptance\n"},
		{"@5", "  - [ ] 2.7.a.1 first acceptance\n    continuation\n    - [ ] detail nested\n"},
	} {
		got, err := selectBlock(data, tc.selector)
		if err != nil || got != tc.want {
			t.Fatalf("%s: got %q err %v", tc.selector, got, err)
		}
	}
	if _, err := selectBlock("- [ ] 2.7.a only\n- [ ] 2.7.b.1 next\n", "2.7"); err == nil {
		t.Fatal("numeric prefix matched a mixed task ID")
	}
}

func TestAmbiguityOutputBounded(t *testing.T) {
	data := strings.Repeat("## Duplicate "+strings.Repeat("长", 300)+"\nbody\n", 100)
	_, err := selectBlock(data, "Duplicate")
	if err == nil || len(err.Error()) > 6*1024 || !strings.Contains(err.Error(), "共 100 个候选，显示 12 个") || !strings.Contains(err.Error(), "@行号") {
		t.Fatalf("unbounded or misleading ambiguity: %v", err)
	}
	if !utf8.ValidString(err.Error()) {
		t.Fatal("ambiguity split UTF-8")
	}
}

func TestLocateDiverseFiles(t *testing.T) {
	root := t.TempDir()
	fixture(t, root, "openspec/specs/a/spec.md", strings.Repeat("## 退款集中章节\n退款条目\n", 100))
	for _, name := range []string{"b", "c", "d"} {
		fixture(t, root, "openspec/specs/"+name+"/spec.md", "## 退款独立章节\n退款条目\n")
	}
	var out bytes.Buffer
	if err := locate(root, []string{"退款"}, 12, &out); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b", "c", "d"} {
		n := strings.Count(out.String(), "openspec/specs/"+name+"/spec.md:")
		if n != 2 {
			t.Fatalf("file %s has %d results, expected 2: %s", name, n, out.String())
		}
	}
}

func TestFencedFakeTasksDoNotSelectOrEndTask(t *testing.T) {
	data := "# Tasks\n- [ ] 1.6 parent\n  - [ ] nested acceptance\n````text\n- [ ] 1.7 fake\n## fake boundary\n```\n- [ ] 1.8 also fake\n````\n  - [ ] another acceptance\n- [ ] 1.9 next real\n"
	want := "- [ ] 1.6 parent\n  - [ ] nested acceptance\n````text\n- [ ] 1.7 fake\n## fake boundary\n```\n- [ ] 1.8 also fake\n````\n  - [ ] another acceptance\n"
	for _, selector := range []string{"1.6", "@2"} {
		got, err := selectBlock(data, selector)
		if err != nil || got != want {
			t.Fatalf("%s incomplete parent: %q %v", selector, got, err)
		}
	}
	for _, selector := range []string{"1.7", "1.8", "fake boundary"} {
		if _, err := selectBlock(data, selector); err == nil {
			t.Fatalf("selected fenced fake %s", selector)
		}
	}
	got, err := selectBlock(data, "1.9")
	if err != nil || got != "- [ ] 1.9 next real\n" {
		t.Fatalf("real task not selected: %q %v", got, err)
	}
}
