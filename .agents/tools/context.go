// context locates governing Markdown without loading unrelated project history.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxRead = 12 * 1024

var contextScopes = []string{".agents/rules/", ".agents/memory/preferences/", "openspec/specs/", "openspec/changes/"}
var bulletRE = regexp.MustCompile(`^(?:[-*+]|[0-9]+[.)])\s+`)
var taskStartRE = regexp.MustCompile(`^\s*[-*+]\s+\[[ xX]\]\s+`)
var headingRE = regexp.MustCompile(`^ {0,3}(#{1,6})\s+(.+?)\s*#*\s*$`)
var taskRE = regexp.MustCompile(`^\s*[-*+]\s+\[[ xX]\]\s+(?:\*\*|\x60)?([A-Za-z][A-Za-z0-9_-]*-[0-9]+|[0-9]+(?:\.[A-Za-z0-9]+)*)(?:\*\*|\x60)?(?:[:：\s]|\.(?:\s|$)|$)`)

type heading struct {
	line, level int
	title       string
}
type hit struct {
	path             string
	line             int
	heading, snippet string
	title            bool
}

func main() {
	cwd, err := os.Getwd()
	if err == nil {
		var root string
		root, err = findRoot(cwd)
		if err == nil {
			err = run(root, os.Args[1:], os.Stdout)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "context:", err)
		os.Exit(1)
	}
}

func findRoot(start string) (string, error) {
	for dir := start; ; dir = filepath.Dir(dir) {
		a, ae := os.Stat(filepath.Join(dir, "AGENTS.md"))
		o, oe := os.Stat(filepath.Join(dir, "openspec"))
		if ae == nil && !a.IsDir() && oe == nil && o.IsDir() {
			return filepath.EvalSymlinks(dir)
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	return "", errors.New("未找到项目根目录（需要 AGENTS.md 和 openspec/）")
}

func run(root string, args []string, out io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(out, "用法: context status [仓库相对路径] | audit --thread ID|--file JSONL | check --path 路径 [--change 变更] [--plan] | handoff <活跃变更> <任务ID> | order --order <订单号> [--session-file 私有会话JSON] | locate [--limit 1..50] <关键词...> | read <仓库相对路径> [标题/任务ID/@行号]")
		return nil
	}
	switch args[0] {
	case "audit", "check":
		tool := filepath.Join(root, ".agents", "tools", "workflow")
		command := exec.Command("go", append([]string{"run", filepath.Join(tool, "main.go"), filepath.Join(tool, "audit.go"), filepath.Join(tool, "check.go"), "--root", root}, args...)...)
		command.Stdout, command.Stderr = out, out
		if err := command.Run(); err != nil {
			return errors.New("工作流命令未完成，参见以上摘要")
		}
		return nil
	case "handoff":
		if len(args) != 3 || !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`).MatchString(args[1]) || !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`).MatchString(args[2]) {
			return errors.New("handoff 需要一个活跃变更名称和一个任务ID")
		}
		name := "openspec/changes/" + args[1] + "/tasks.md"
		p, err := safePath(root, name)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		block, err := selectBlock(string(data), args[2])
		if err != nil {
			return err
		}
		if len(block) > 4096 {
			return errors.New("交接任务超过4 KiB；缩小到具体子任务，不截断任务正文")
		}
		message := fmt.Sprintf("请在新会话处理以下任务或以其作为后续目标的背景；已完成项不自动重做。\n来源：%s，任务：%s。\n先读取当前AGENTS.md与相关权威条款，再确定本任务的验收结果和必要验证；交接不替代规范。\n%s\n只携带该任务的关键路径、仍有效的验证证据和未决条件；证据不足时如实补查，不复制整个旧会话。\n本命令只生成交接文字，没有创建会话或发送消息。\n", name, args[2], block)
		if len(message) > 4096 {
			return errors.New("交接总输出超过4 KiB；缩小到具体子任务，不截断任务正文")
		}
		fmt.Fprint(out, message)
		return nil
	case "order":
		tool := filepath.Join(root, "apps", "lina-plugins", "idc-core", "hack", "tools", "order-diagnose")
		command := exec.Command("go", append([]string{"run", filepath.Join(tool, "main.go"), filepath.Join(tool, "diagnose.go")}, args[1:]...)...)
		command.Stdout, command.Stderr = out, out
		if err := command.Run(); err != nil {
			return errors.New("订单诊断未完成，参见以上摘要；不会自动重试或执行开通")
		}
		return nil
	case "status":
		if len(args) > 2 {
			return errors.New("status 最多接受一个仓库相对路径；缩小路径查看具体文件")
		}
		scope := ""
		if len(args) == 2 {
			scope = strings.ReplaceAll(args[1], "\\", "/")
			if strings.HasPrefix(scope, "/") || strings.Contains(scope, ":") || strings.HasPrefix(scope, "-") {
				return errors.New("status 只接受仓库内相对路径")
			}
			for _, part := range strings.Split(scope, "/") {
				if part == ".." {
					return errors.New("禁止 .. 路径")
				}
			}
			scope = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(filepath.FromSlash(scope))), "./")
			if scope == "." {
				scope = ""
			}
		}
		return workspaceStatus(root, scope, out)
	case "locate":
		limit := 12
		var words []string
		for i := 1; i < len(args); i++ {
			if args[i] == "--limit" {
				i++
				if i >= len(args) {
					return errors.New("--limit 缺少数字")
				}
				n, e := strconv.Atoi(args[i])
				if e != nil || n < 1 || n > 50 {
					return errors.New("--limit 必须为 1..50")
				}
				limit = n
			} else if strings.HasPrefix(args[i], "--") {
				return fmt.Errorf("未知选项 %s", args[i])
			} else if strings.TrimSpace(args[i]) != "" {
				words = append(words, strings.ToLower(args[i]))
			}
		}
		if len(words) == 0 {
			return errors.New("请提供关键词")
		}
		return locate(root, words, limit, out)
	case "read":
		if len(args) < 2 {
			return errors.New("read 需要仓库相对路径")
		}
		p, e := safePath(root, args[1])
		if e != nil {
			return e
		}
		data, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		selector := strings.Join(args[2:], " ")
		block, e := selectBlock(string(data), selector)
		if e != nil {
			return e
		}
		fmt.Fprint(out, block)
		if !strings.HasSuffix(block, "\n") {
			fmt.Fprintln(out)
		}
		return nil
	default:
		return fmt.Errorf("未知命令 %s；使用 --help", args[0])
	}
}

type statusEntry struct {
	code, path, original string
}

// Git -z preserves spaces, newlines, and Unicode in filenames and emits rename
// destinations before sources. Keep both so moves out of a scope remain visible.
func parseStatus(data []byte) ([]statusEntry, error) {
	var entries []statusEntry
	parts := strings.Split(string(data), "\x00")
	if parts[len(parts)-1] != "" {
		return nil, errors.New("Git 状态缺少 NUL 终止符")
	}
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if len(part) < 4 || part[2] != ' ' {
			return nil, errors.New("无法解析 Git 状态记录")
		}
		e := statusEntry{code: part[:2], path: part[3:]}
		if strings.ContainsAny(e.code, "RC") {
			i++
			if i >= len(parts)-1 || parts[i] == "" {
				return nil, errors.New("Git 重命名状态缺少原路径")
			}
			e.original = parts[i]
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func inScope(path, scope string) bool {
	return scope == "" || path == scope || strings.HasPrefix(path, scope+"/")
}

func workspaceStatus(root, scope string, out io.Writer) error {
	cmd := exec.Command("git", "-C", root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	data, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("读取 Git 状态失败：%w", err)
	}
	entries, err := parseStatus(data)
	if err != nil {
		return err
	}
	groups := map[string]int{}
	var selected []statusEntry
	for _, e := range entries {
		parts := strings.Split(e.path, "/")
		n := 2
		if len(parts) >= 3 && parts[0] == "apps" && parts[1] == "lina-plugins" {
			n = 3
		}
		if n > len(parts) {
			n = len(parts)
		}
		groups[strings.Join(parts[:n], "/")]++
		if inScope(e.path, scope) || (e.original != "" && inScope(e.original, scope)) {
			selected = append(selected, e)
		}
	}
	fmt.Fprintf(out, "工作区：%d 条状态记录（含未跟踪文件和生成产物；重命名计一条）。\n", len(entries))
	if scope != "" {
		fmt.Fprintf(out, "范围 %s：%d 条；最多显示12条，过长路径仅作定位。\n", strconv.Quote(short(scope, 160)), len(selected))
		for i, e := range selected {
			if i == 12 {
				fmt.Fprintf(out, "另有 %d 条未展开；缩小路径或定向执行 Git 命令。\n", len(selected)-i)
				break
			}
			fmt.Fprintf(out, "  %s %s", e.code, strconv.Quote(short(e.path, 240)))
			if e.original != "" {
				fmt.Fprintf(out, " <- %s", strconv.Quote(short(e.original, 240)))
			}
			fmt.Fprintln(out)
		}
		return nil
	}
	changes, err := os.ReadDir(filepath.Join(root, "openspec", "changes"))
	if err != nil {
		return fmt.Errorf("读取活跃变更失败：%w", err)
	}
	var active []string
	for _, entry := range changes {
		if entry.IsDir() && entry.Name() != "archive" {
			active = append(active, entry.Name())
		}
	}
	var names []string
	for name := range groups {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if groups[names[i]] != groups[names[j]] {
			return groups[names[i]] > groups[names[j]]
		}
		return names[i] < names[j]
	})
	for i, name := range names {
		if i == 12 {
			fmt.Fprintf(out, "另有 %d 个目录分组未展开。\n", len(names)-i)
			break
		}
		fmt.Fprintf(out, "  %s：%d\n", strconv.Quote(short(name, 160)), groups[name])
	}
	fmt.Fprintf(out, "活跃变更：%d 个。\n", len(active))
	for i, name := range active {
		if i == 12 {
			fmt.Fprintf(out, "另有 %d 个活跃变更未展开；按相关变更路径查询。\n", len(active)-i)
			break
		}
		fmt.Fprintf(out, "  %s\n", strconv.Quote(short(name, 160)))
	}
	fmt.Fprintln(out, "具体文件未展开；使用 status <仓库相对路径>，必要时定向执行 Git 命令。")
	return nil
}

func safePath(root, name string) (string, error) {
	normalized := strings.ReplaceAll(name, "\\", "/")
	if normalized == "" || filepath.IsAbs(name) || strings.HasPrefix(normalized, "/") || strings.Contains(normalized, ":") {
		return "", errors.New("只接受仓库内相对路径")
	}
	for _, part := range strings.Split(normalized, "/") {
		if part == ".." {
			return "", errors.New("禁止 .. 路径")
		}
	}
	allowed := false
	for _, scope := range contextScopes {
		if strings.HasPrefix(normalized, scope) {
			allowed = true
		}
	}
	if !allowed || privatePath(normalized) {
		return "", errors.New("只允许规则、偏好、OpenSpec specs/changes 的上下文文件")
	}
	p, e := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(normalized)))
	if e != nil {
		return "", e
	}
	rel, e := filepath.Rel(root, p)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("路径或符号链接逸出仓库")
	}
	resolved := filepath.ToSlash(rel)
	allowed = false
	for _, scope := range contextScopes {
		if strings.HasPrefix(resolved, scope) {
			allowed = true
		}
	}
	if !allowed || privatePath(resolved) {
		return "", errors.New("符号链接指向上下文范围之外或私有运行文件")
	}
	st, e := os.Stat(p)
	if e != nil {
		return "", e
	}
	if !st.Mode().IsRegular() {
		return "", errors.New("需要普通文件")
	}
	return p, nil
}

func headings(lines []string) []heading {
	var result []heading
	code := codeLines(lines)
	for i, line := range lines {
		if code[i] {
			continue
		}
		m := headingRE.FindStringSubmatch(line)
		if m != nil {
			result = append(result, heading{i, len(m[1]), m[2]})
		}
	}
	return result
}

type fenceState struct {
	marker byte
	length int
}

func fenced(line string, state *fenceState) bool {
	trim := strings.TrimSpace(line)
	marker, count := byte(0), 0
	if len(trim) > 0 && (trim[0] == '`' || trim[0] == '~') {
		marker = trim[0]
		for count < len(trim) && trim[count] == marker {
			count++
		}
	}
	if state.length > 0 {
		if marker == state.marker && count >= state.length && strings.TrimSpace(trim[count:]) == "" {
			state.marker = 0
			state.length = 0
		}
		return true
	}
	if count >= 3 {
		state.marker = marker
		state.length = count
		return true
	}
	return false
}
func codeLines(lines []string) []bool {
	result := make([]bool, len(lines))
	state := fenceState{}
	for i, line := range lines {
		result[i] = fenced(line, &state)
	}
	return result
}

func selectBlock(data, selector string) (string, error) {
	if selector == "" {
		if len(data) > maxRead {
			return "", errors.New("正文超过 12 KiB；请先 locate，再指定标题、Requirement 或完整任务 ID")
		}
		return data, nil
	}
	lines := strings.SplitAfter(data, "\n")
	hs := headings(lines)
	code := codeLines(lines)
	if strings.HasPrefix(selector, "@") {
		n, err := strconv.Atoi(strings.TrimPrefix(selector, "@"))
		if err != nil || n < 1 || n > len(lines) {
			return "", errors.New("@line 必须为文件内有效行号")
		}
		return selectLine(lines, hs, n-1)
	}
	var exact, partial []heading
	for _, h := range hs {
		if h.title == selector || strings.TrimPrefix(h.title, "Requirement: ") == selector {
			exact = append(exact, h)
		} else if strings.Contains(strings.ToLower(h.title), strings.ToLower(selector)) {
			partial = append(partial, h)
		}
	}
	candidates := exact
	if len(exact) == 0 {
		var taskLines []int
		for i, l := range lines {
			if code[i] {
				continue
			}
			m := taskRE.FindStringSubmatch(l)
			if m != nil && m[1] == selector {
				taskLines = append(taskLines, i)
			}
		}
		if len(taskLines) > 1 {
			return "", fmt.Errorf("任务 ID %q 有 %d 个匹配，请指定唯一标题", selector, len(taskLines))
		}
		if len(taskLines) == 1 {
			start := taskLines[0]
			return bounded(strings.Join(lines[start:taskEnd(lines, start)], ""))
		}
		candidates = partial
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("没有标题或完整任务 ID 匹配 %q", selector)
	}
	if len(candidates) > 1 {
		var names []string
		for i, h := range candidates {
			if i == 12 {
				break
			}
			names = append(names, fmt.Sprintf("%d: %s", h.line+1, shortBytes(short(h.title, 80), 240)))
		}
		return "", fmt.Errorf("标题匹配有歧义，共 %d 个候选，显示 %d 个；请缩小标题或用 @行号：\n%s", len(candidates), len(names), strings.Join(names, "\n"))
	}
	h := candidates[0]
	end := len(lines)
	for _, n := range hs {
		if n.line > h.line && n.level <= h.level {
			end = n.line
			break
		}
	}
	return bounded(strings.Join(lines[h.line:end], ""))
}
func bounded(block string) (string, error) {
	if len(block) > maxRead {
		return "", errors.New("选中章节超过 12 KiB；请指定更小的子标题或完整任务 ID")
	}
	return block, nil
}

func matches(s string, words []string) bool {
	s = strings.ToLower(s)
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}
func short(s string, n int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return string(r)
}
func less(a, b hit) bool {
	if a.title != b.title {
		return a.title
	}
	if sourceRank(a.path) != sourceRank(b.path) {
		return sourceRank(a.path) < sourceRank(b.path)
	}
	if a.path != b.path {
		return a.path < b.path
	}
	return a.line < b.line
}

func locate(root string, words []string, limit int, out io.Writer) error {
	scopes := []string{".agents/rules", ".agents/memory/preferences", "openspec/specs", "openspec/changes"}
	var results []hit
	total := 0
	for _, scope := range scopes {
		dir := filepath.Join(root, filepath.FromSlash(scope))
		if _, e := os.Stat(dir); os.IsNotExist(e) {
			continue
		}
		resolvedDir, resolveErr := filepath.EvalSymlinks(dir)
		if resolveErr != nil {
			return resolveErr
		}
		if resolvedDir != filepath.Clean(dir) {
			continue
		} // Never follow a symlinked scope or ancestor.
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			rel, e := filepath.Rel(root, p)
			if e != nil {
				return e
			}
			rel = filepath.ToSlash(rel)
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			name := strings.ToLower(d.Name())
			excluded := name == "archive" || name == "evidence" || name == "verification" || name == "validation" || name == "验证记录" || name == "runtime" || name == "secrets" || strings.HasPrefix(name, "verification-") || strings.HasPrefix(name, "validation-") || strings.HasPrefix(name, "验证记录")
			if excluded || privatePath(rel) || historicalPreference(rel) || validationReport(rel) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".md") {
				return nil
			}
			f, e := os.Open(p)
			if e != nil {
				return e
			}
			defer f.Close()
			reader := bufio.NewReader(f)
			lineNo := 0
			current := "(无标题)"
			fence := fenceState{}
			for {
				line, readErr := reader.ReadString('\n')
				if len(line) > 0 {
					lineNo++
					isTitle := false
					if !fenced(line, &fence) {
						if m := headingRE.FindStringSubmatch(line); m != nil {
							current = m[2]
							isTitle = true
						}
					}
					if matches(line, words) {
						total++
						h := hit{rel, lineNo, current, line, isTitle}
						results = append(results, h)
						sort.Slice(results, func(i, j int) bool { return less(results[i], results[j]) })
						counts := make(map[string]int)
						kept := results[:0]
						for _, candidate := range results {
							if counts[candidate.path] < 2 && len(kept) < limit {
								kept = append(kept, candidate)
								counts[candidate.path]++
							}
						}
						results = kept
					}
				}
				if readErr == io.EOF {
					break
				}
				if readErr != nil {
					return readErr
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	fmt.Fprintln(out, "范围: .agents/rules, .agents/memory/preferences, openspec/specs, openspec/changes（排除 archive/evidence/验证记录/历史流程偏好/私有运行路径）")
	for _, h := range results {
		prefix := fmt.Sprintf("%s:%d | %s | ", h.path, h.line, short(h.heading, 80))
		n := 300 - utf8.RuneCountInString(prefix)
		if n < 1 {
			fmt.Fprintln(out, shortBytes(short(prefix, 300), 440))
		} else {
			fmt.Fprintln(out, shortBytes(prefix+short(h.snippet, snippetLimit(n)), 440))
		}
	}
	fmt.Fprintf(out, "命中 %d 行，显示 %d 条。定位结果不是规范阅读；用 read 路径 标题读取完整章节，或 @行号读取完整条目（仍需补齐相关依赖）。扩展检索：更换关键词或 --limit 50；历史证据需显式指定路径。\n", total, len(results))
	if total == 0 {
		return errors.New("未命中；请使用模块名、需求名或同义词继续定位")
	}
	return nil
}

func privatePath(rel string) bool {
	for _, part := range strings.Split(strings.ToLower(rel), "/") {
		if part == "runtime" || part == "secrets" || part == "temp" || part == ".env" || strings.HasPrefix(part, ".env.") || part == "credentials.md" || part == "secrets.md" {
			return true
		}
	}
	return false
}

// @line reads a complete semantic entry, not an entire governing rule file.
func selectLine(lines []string, hs []heading, line int) (string, error) {
	start, end := 0, len(lines)
	code := codeLines(lines)
	var current *heading
	for i := range hs {
		h := &hs[i]
		if h.line <= line {
			current = h
		}
		if h.line > line {
			end = h.line
			break
		}
	}
	// A Requirement contains its scenarios and must never be read partially.
	for i := len(hs) - 1; i >= 0; i-- {
		h := hs[i]
		if h.line <= line && strings.HasPrefix(h.title, "Requirement:") {
			bound := len(lines)
			for _, next := range hs {
				if next.line > h.line && next.level <= h.level {
					bound = next.line
					break
				}
			}
			if line < bound {
				return bounded(strings.Join(lines[h.line:bound], ""))
			}
		}
	}
	if current != nil {
		start = current.line + 1
		if current.line == line {
			return selectBlock(strings.Join(lines, ""), current.title)
		}
	}
	// The nearest enclosing checkbox task owns its indented acceptance items.
	for i := line; i >= start; i-- {
		if !code[i] && taskStartRE.MatchString(lines[i]) {
			stop := taskEnd(lines, i)
			if line < stop {
				return bounded(strings.Join(lines[i:stop], ""))
			}
		}
	}
	// Only top-level bullets are entries; indented continuation paragraphs stay attached.
	bullet := -1
	for i := start; i <= line; i++ {
		if bulletRE.MatchString(lines[i]) {
			bullet = i
		}
	}
	if bullet >= 0 {
		stop := end
		for i := bullet + 1; i < end; i++ {
			if bulletRE.MatchString(lines[i]) {
				stop = i
				break
			}
		}
		if line < stop {
			return bounded(strings.Join(lines[bullet:stop], ""))
		}
	}
	start = line
	for start > 0 && strings.TrimSpace(lines[start-1]) != "" && !headingRE.MatchString(lines[start-1]) {
		start--
	}
	end = line + 1
	for end < len(lines) && strings.TrimSpace(lines[end]) != "" && !headingRE.MatchString(lines[end]) {
		end++
	}
	return bounded(strings.Join(lines[start:end], ""))
}

func sourceRank(path string) int {
	switch {
	case strings.HasPrefix(path, ".agents/rules/"):
		return 0
	case strings.HasPrefix(path, ".agents/memory/preferences/"):
		return 1
	case strings.HasPrefix(path, "openspec/changes/"):
		return 2
	default:
		return 3
	}
}
func historicalPreference(path string) bool {
	switch path {
	case ".agents/memory/preferences/00-title.md", ".agents/memory/preferences/02-scope.md", ".agents/memory/preferences/05-development-workflow.md", ".agents/memory/preferences/06-memory-maintenance.md", ".agents/memory/preferences/index.md":
		return true
	default:
		return false
	}
}
func shortBytes(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit - len("…")
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

func snippetLimit(n int) int {
	if n > 120 {
		return 120
	}
	return n
}

// Runtime evidence stays available through explicit read, not current-context lookup.
func validationReport(path string) bool {
	if !strings.HasPrefix(path, "openspec/changes/") {
		return false
	}
	name := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(name, "-validation.md") || strings.HasSuffix(name, "-verification.md") || strings.Contains(name, "-validation-") || strings.Contains(name, "-verification-")
}

func taskEnd(lines []string, start int) int {
	indent := indentation(lines[start])
	code := codeLines(lines)
	for i := start + 1; i < len(lines); i++ {
		if !code[i] && (headingRE.MatchString(lines[i]) || (taskStartRE.MatchString(lines[i]) && indentation(lines[i]) <= indent)) {
			return i
		}
	}
	return len(lines)
}
func indentation(line string) int {
	n := 0
	for _, r := range line {
		if r == ' ' {
			n++
		} else if r == '\t' {
			n += 4
		} else {
			break
		}
	}
	return n
}
