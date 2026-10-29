// This file plans explicit repository scopes and runs existing checks serially.
// Full command logs remain local; reports contain statuses and log paths only.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type checkStep struct {
	Name    string   `json:"name"`
	Dir     string   `json:"dir"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Env     []string `json:"-"`
}
type checkPlan struct {
	Files     []string    `json:"-"`
	FileCount int         `json:"fileCount"`
	Steps     []checkStep `json:"steps"`
	Manual    []string    `json:"manualRequired"`
}
type checkResult struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	ExitCode int    `json:"exitCode"`
	Log      string `json:"log"`
}
type checkReport struct {
	Complete bool          `json:"complete"`
	Results  []checkResult `json:"results"`
	Manual   []string      `json:"manualRequired"`
	Note     string        `json:"note"`
}

var changeNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
var localLinkPattern = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
var inlineCodePattern = regexp.MustCompile("`+[^`]*`+")

// sourcePath rejects traversal and private/generated roots before directory walks.
func sourcePath(root, name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" || name == "." || strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
		return "", errors.New("检查范围必须是具体仓库相对路径")
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." || excludedPart(part) {
			return "", errors.New("检查范围含越界、私有或生成目录")
		}
	}
	p := filepath.Join(root, filepath.FromSlash(name))
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", errors.New("指定检查路径不存在")
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("检查路径逸出仓库")
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if excludedPart(part) {
			return "", errors.New("检查路径指向私有或生成目录")
		}
	}
	return real, nil
}

func excludedPart(part string) bool {
	switch part {
	case ".git", ".site-runtime", "node_modules", "temp", "dist", ".output", ".static-output", ".nuxt", ".runtime", "runtime", "private":
		return true
	}
	return false
}

// planChecks selects exact Go packages, one lint per owning module, and related
// active OpenSpec changes. Unknown verification categories remain explicit gaps.
func planChecks(root string, paths, changes []string) (*checkPlan, error) {
	root, rootErr := filepath.EvalSymlinks(root)
	if rootErr != nil {
		return nil, rootErr
	}
	if len(paths)+len(changes) == 0 || len(paths) > 20 || len(changes) > 10 {
		return nil, errors.New("需要显式范围；最多20个路径和10个变更")
	}
	plan := &checkPlan{Steps: []checkStep{}, Manual: []string{}}
	files, changeSet, manual := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, change := range changes {
		if !changeNamePattern.MatchString(change) || change == "archive" {
			return nil, errors.New("活跃变更名称无效")
		}
		if _, err := sourcePath(root, "openspec/changes/"+change); err != nil {
			return nil, err
		}
		changeSet[change] = true
		paths = append(paths, "openspec/changes/"+change)
	}
	for _, name := range paths {
		path, err := sourcePath(root, name)
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(path, func(p string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() && excludedPart(entry.Name()) {
				return filepath.SkipDir
			}
			if entry.Type()&os.ModeSymlink != 0 {
				manual["symlink_requires_explicit_review"] = true
				return nil
			}
			if !entry.IsDir() {
				rel, err := filepath.Rel(root, p)
				if err != nil {
					return err
				}
				files[filepath.ToSlash(rel)] = true
				if len(files) > 2000 {
					return errors.New("范围超过2000个文件；请缩小路径")
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	modules, standalone := map[string]map[string]bool{}, map[string]bool{}
	for name := range files {
		plan.Files = append(plan.Files, name)
		parts := strings.Split(name, "/")
		if len(parts) > 3 && parts[0] == "openspec" && parts[1] == "changes" && parts[2] != "archive" {
			changeSet[parts[2]] = true
		}
		switch filepath.Ext(name) {
		case ".md", ".json":
		case ".mod", ".sum":
			if filepath.Base(name) == "go.mod" || filepath.Base(name) == "go.sum" {
				module := nearestModule(root, filepath.Dir(filepath.Join(root, filepath.FromSlash(name))))
				if module != "" {
					if modules[module] == nil {
						modules[module] = map[string]bool{}
					}
					modules[module]["..."] = true
				} else {
					manual["go_module_not_found"] = true
				}
			} else {
				manual["workspace_dependency_change_requires_explicit_modules"] = true
			}
		case ".yaml", ".yml":
			if filepath.Base(name) != ".openspec.yaml" || len(parts) < 4 || !changeSet[parts[2]] {
				manual["unconfigured_source_type:"+filepath.Ext(name)] = true
			}
		case ".go":
			dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(name)))
			module := nearestModule(root, dir)
			if module == "" {
				standalone[dir] = true
			} else {
				if modules[module] == nil {
					modules[module] = map[string]bool{}
				}
				modules[module][dir] = true
			}
		default:
			manual["unconfigured_source_type:"+filepath.Ext(name)] = true
		}
	}
	sort.Strings(plan.Files)
	plan.FileCount = len(plan.Files)
	if plan.FileCount == 0 {
		return nil, errors.New("指定范围没有可验证的源文件")
	}
	for _, name := range plan.Files {
		if filepath.Ext(name) == ".md" || filepath.Ext(name) == ".json" {
			plan.Steps = append(plan.Steps, checkStep{Name: "documents-json-whitespace", Dir: ".", Command: "internal"})
			break
		}
	}
	for _, dir := range sortedKeys(standalone) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		args := []string{"test", "-count=1"}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
				rel, err := filepath.Rel(root, filepath.Join(dir, entry.Name()))
				if err != nil {
					return nil, err
				}
				args = append(args, filepath.ToSlash(rel))
			}
		}
		if len(args) > 34 {
			return nil, errors.New("独立Go工具文件过多；需要明确组件入口")
		}
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return nil, err
		}
		plan.Steps = append(plan.Steps, checkStep{Name: "go-test:" + filepath.ToSlash(rel), Dir: ".", Command: "go", Args: args})
		plan.Steps = append(plan.Steps, checkStep{Name: "go-vet:" + filepath.ToSlash(rel), Dir: ".", Command: "go", Args: append([]string{"vet"}, args[2:]...)})
	}
	for _, module := range sortedKeys(modules) {
		if len(modules[module]) > 32 && !modules[module]["..."] {
			return nil, errors.New("Go包超过32个；请缩小验收范围")
		}
		moduleRel, err := filepath.Rel(root, module)
		if err != nil {
			return nil, err
		}
		args := []string{"test", "-p=1", "-count=1"}
		for _, dir := range sortedKeys(modules[module]) {
			if modules[module]["..."] {
				args = append(args, "./...")
				break
			}
			rel, err := filepath.Rel(module, dir)
			if err != nil {
				return nil, err
			}
			if rel == "." {
				args = append(args, ".")
			} else {
				args = append(args, "./"+filepath.ToSlash(rel))
			}
		}
		plan.Steps = append(plan.Steps, checkStep{Name: "go-test:" + filepath.ToSlash(moduleRel), Dir: filepath.ToSlash(moduleRel), Command: "go", Args: args, Env: []string{"GOWORK=off"}})
		plan.Steps = append(plan.Steps, checkStep{Name: "go-lint:" + filepath.ToSlash(moduleRel), Dir: ".", Command: "go", Args: []string{"run", "./hack/tools/linactl", "lint.go", "dir=" + filepath.ToSlash(moduleRel), "plugins=1"}})
	}
	for _, change := range sortedKeys(changeSet) {
		if !changeNamePattern.MatchString(change) {
			return nil, errors.New("变更名称无效")
		}
		plan.Steps = append(plan.Steps, checkStep{Name: "openspec:" + change, Dir: ".", Command: "openspec", Args: []string{"validate", change, "--strict"}})
	}
	for name := range manual {
		plan.Manual = append(plan.Manual, name)
	}
	sort.Strings(plan.Manual)
	if len(plan.Steps) > 24 {
		return nil, errors.New("检查计划超过24项；请缩小范围")
	}
	return plan, nil
}

func sortedKeys[V any](input map[string]V) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// nearestModule never walks above the selected repository boundary.
func nearestModule(root, dir string) string {
	for {
		if st, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !st.IsDir() {
			return dir
		}
		if dir == root {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// executeChecks saves private logs and preserves failures while finishing other
// independent checks. No raw command output is copied into the short report.
func executeChecks(root string, plan *checkPlan, timeout time.Duration) (*checkReport, error) {
	day := time.Now().In(time.FixedZone("UTC+8", 8*3600)).Format("20060102")
	parent := filepath.Join(root, "temp", day)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return nil, err
	}
	prefix := time.Now().In(time.FixedZone("UTC+8", 8*3600)).Format("150405")
	dir, err := os.MkdirTemp(parent, prefix+"-checks-")
	if err != nil {
		return nil, err
	}
	r := &checkReport{Complete: len(plan.Manual) == 0, Manual: plan.Manual, Note: "仅表示所选自动检查的覆盖情况；业务E2E、外部状态和领域门禁仍按任务核验。"}
	for i, step := range plan.Steps {
		path := filepath.Join(dir, fmt.Sprintf("%s-%02d.log", prefix, i+1))
		log, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		code := 0
		if step.Command == "internal" {
			err = checkDocuments(root, plan.Files, log)
		} else {
			err = runCheckCommand(root, step, timeout, log)
		}
		if err != nil {
			code = 1
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				code = exit.ExitCode()
			}
			if _, writeErr := fmt.Fprintln(log, "check failed"); writeErr != nil {
				return nil, errors.Join(writeErr, log.Close())
			}
		}
		if closeErr := log.Close(); closeErr != nil {
			return nil, closeErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil, relErr
		}
		status := "passed"
		if err != nil {
			status = "failed"
			if errors.Is(err, exec.ErrNotFound) {
				status = "not_started"
				code = 127
			}
			if errors.Is(err, os.ErrPermission) {
				status = "not_started"
				code = 126
			}
			if errors.Is(err, context.DeadlineExceeded) {
				status = "timeout"
				code = 124
			}
			r.Complete = false
		}
		r.Results = append(r.Results, checkResult{Name: step.Name, Status: status, ExitCode: code, Log: filepath.ToSlash(rel)})
	}
	if !r.Complete {
		return r, errors.New("检查存在失败或未覆盖项，参见日志与manualRequired")
	}
	return r, nil
}

func runCheckCommand(root string, step checkStep, timeout time.Duration, log *os.File) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, step.Command, step.Args...)
	cmd.Dir = filepath.Join(root, filepath.FromSlash(step.Dir))
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.Env = os.Environ()
	for _, override := range step.Env {
		key := strings.SplitN(override, "=", 2)[0] + "="
		filtered := cmd.Env[:0]
		for _, item := range cmd.Env {
			if !strings.HasPrefix(item, key) {
				filtered = append(filtered, item)
			}
		}
		cmd.Env = append(filtered, override)
	}
	err := cmd.Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// checkDocuments validates syntax, whitespace, and local link existence. It does
// not claim to validate rendered layout, anchors, SQL semantics, or browser UX.
func checkDocuments(root string, files []string, log *os.File) error {
	failed := false
	totalBytes := 0
	for _, name := range files {
		ext := filepath.Ext(name)
		if ext != ".md" && ext != ".json" {
			continue
		}
		f, err := os.Open(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(f, 1024*1024+1))
		closeErr := f.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if len(data) > 1024*1024 {
			return errors.New("单份文档超过1 MiB；请定向验证")
		}
		totalBytes += len(data)
		if totalBytes > 16*1024*1024 {
			return errors.New("文档总量超过16 MiB；请缩小范围")
		}
		if ext == ".json" {
			if !json.Valid(data) {
				failed = true
				if _, err = fmt.Fprintln(log, "invalid JSON:", name); err != nil {
					return err
				}
			}
			continue
		}
		fence := ""
		for _, line := range strings.Split(string(data), "\n") {
			trim := strings.TrimSpace(line)
			if next, boundary := markdownFence(trim, fence); boundary {
				fence = next
				continue
			}
			if strings.TrimRight(line, " \t\r") != line {
				failed = true
				if _, err = fmt.Fprintln(log, "trailing whitespace:", name); err != nil {
					return err
				}
			}
			if fence != "" {
				continue
			}
			for _, match := range localLinkPattern.FindAllStringSubmatch(inlineCodePattern.ReplaceAllString(line, ""), -1) {
				target := strings.Trim(match[1], "<>")
				link, err := url.Parse(target)
				if err != nil {
					failed = true
					continue
				}
				if link.Scheme != "" || link.Path == "" {
					continue
				}
				path, err := url.PathUnescape(link.Path)
				if err != nil {
					failed = true
					continue
				}
				if !filepath.IsAbs(path) {
					path = filepath.Join(root, filepath.Dir(filepath.FromSlash(name)), filepath.FromSlash(path))
				}
				if _, err = os.Stat(path); err != nil {
					failed = true
					if _, err = fmt.Fprintln(log, "missing local link:", name); err != nil {
						return err
					}
				}
			}
		}
	}
	if failed {
		return errors.New("文档或JSON检查失败")
	}
	return nil
}

// markdownFence retains delimiter length so an inner shorter fence cannot
// expose example links as real document links.
func markdownFence(line, active string) (string, bool) {
	if len(line) < 3 || (line[0] != '`' && line[0] != '~') {
		return active, false
	}
	n := 0
	for n < len(line) && line[n] == line[0] {
		n++
	}
	if n < 3 {
		return active, false
	}
	if active == "" {
		return line[:n], true
	}
	if line[0] == active[0] && n >= len(active) && strings.TrimSpace(line[n:]) == "" {
		return "", true
	}
	return active, false
}
