// This file streams local session records and emits aggregate metadata only.
// Repeated-read detection is heuristic; cached tokens are not billing data.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var threadIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var commandPattern = regexp.MustCompile(`(?:\bcmd|"cmd")\s*:\s*("(?:[^"\\]|\\.)*")`)
var readPathPattern = regexp.MustCompile(`^(?:\.agents|openspec|apps)/[A-Za-z0-9_./-]+\.(?:md|go|ts|tsx|vue|js|mjs|json)$`)

type counted struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
type outputSize struct {
	Record    int   `json:"record"`
	TextBytes int64 `json:"textBytes"`
}
type tokenUsage struct {
	Input  *int64 `json:"input_tokens"`
	Cached *int64 `json:"cached_input_tokens"`
	Output *int64 `json:"output_tokens"`
}
type auditReport struct {
	Complete         bool         `json:"complete"`
	FileBytes        int64        `json:"fileBytes"`
	Records          int          `json:"records"`
	Malformed        int          `json:"malformedRecords"`
	ReadPathsLimited bool         `json:"readPathsLimited"`
	Calls            int          `json:"toolCalls"`
	Compactions      int          `json:"compactions"`
	TextBytes        int64        `json:"textOutputBytes"`
	Images           int          `json:"imageBlocks"`
	Unclassified     int64        `json:"unclassifiedOutputBytes"`
	TokenReports     int          `json:"tokenReports"`
	Latest           *tokenUsage  `json:"latestUsage"`
	Cumulative       *tokenUsage  `json:"cumulativeUsage"`
	Tools            []counted    `json:"topTools"`
	Reads            []counted    `json:"repeatedReadPaths"`
	Largest          []outputSize `json:"largestTextOutputs"`
	Notes            []string     `json:"notes"`
}

// sessionPath resolves one exact ID below the configured local sessions root.
func sessionPath(thread, file string) (string, error) {
	if file != "" {
		if filepath.Ext(file) != ".jsonl" {
			return "", errors.New("会话文件必须为JSONL")
		}
		return file, nil
	}
	if !threadIDPattern.MatchString(thread) {
		return "", errors.New("会话ID格式无效")
	}
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		user, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = filepath.Join(user, ".codex")
	}
	root, err := filepath.EvalSymlinks(filepath.Join(home, "sessions"))
	if err != nil {
		return "", errors.New("本地会话目录不可用")
	}
	var matches []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "-"+thread+".jsonl") {
			matches = append(matches, path)
			if len(matches) == 2 {
				return fs.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return "", errors.New("无法完整定位本地会话")
	}
	if len(matches) != 1 {
		return "", errors.New("会话未找到或匹配不唯一；可用--file显式选择")
	}
	return matches[0], nil
}

// auditSession bounds individual records to 8 MiB and the source to 1 GiB.
func auditSession(path string) (result *auditReport, err error) {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() || st.Size() > 1<<30 {
		return nil, errors.New("会话不是普通文件或超过1 GiB")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("无法读取指定会话")
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			if result != nil {
				result.Complete = false
			}
			err = errors.Join(err, closeErr)
		}
	}()
	st, err = f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() > 1<<30 {
		return nil, errors.New("会话不是普通文件或超过1 GiB")
	}
	r := &auditReport{Complete: true, FileBytes: st.Size(), Notes: []string{"日志计数含重复上下文；累计输入不能据此换算账户费用。", "读取热点只识别JSON引号命令中的cat和sed -n；压缩后重读可能必要。", "文本、图片及未知序列化输出分开计数；字节数不是token数。"}}
	tools, reads := map[string]int{}, map[string]int{}
	limited := &io.LimitedReader{R: f, N: (1 << 30) + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		r.Records++
		var record struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(scanner.Bytes(), &record) != nil {
			r.Malformed++
			continue
		}
		switch record.Type {
		case "compacted":
			r.Compactions++
		case "event_msg":
			readUsage(record.Payload, r)
		case "response_item":
			readResponse(record.Payload, r, tools, reads)
		}
	}
	r.Tools = topCounts(tools, false)
	r.Reads = topCounts(reads, true)
	if scanner.Err() != nil || r.Malformed > 0 || limited.N == 0 || r.ReadPathsLimited {
		r.Complete = false
		return r, errors.New("会话包含损坏或超限记录；摘要不完整")
	}
	return r, nil
}

// readUsage uses recorded cumulative totals rather than summing repeated reports.
func readUsage(data []byte, r *auditReport) {
	var value struct {
		Type string `json:"type"`
		Info *struct {
			Latest *tokenUsage `json:"last_token_usage"`
			Total  *tokenUsage `json:"total_token_usage"`
		} `json:"info"`
	}
	if json.Unmarshal(data, &value) != nil || value.Type != "token_count" || value.Info == nil {
		return
	}
	r.TokenReports++
	r.Latest = value.Info.Latest
	r.Cumulative = value.Info.Total
}

// readResponse counts canonical response items, excluding duplicated event items.
func readResponse(data []byte, r *auditReport, tools, reads map[string]int) {
	var value struct {
		Type      string          `json:"type"`
		Name      string          `json:"name"`
		Input     string          `json:"input"`
		Arguments string          `json:"arguments"`
		Output    json.RawMessage `json:"output"`
	}
	if json.Unmarshal(data, &value) != nil {
		r.Malformed++
		return
	}
	switch value.Type {
	case "custom_tool_call", "function_call":
		r.Calls++
		name := toolClass(value.Name)
		tools[name]++
		for path := range readPaths(value.Input + "\n" + value.Arguments) {
			if reads[path] > 0 || len(reads) < 4096 {
				reads[path]++
			} else {
				r.ReadPathsLimited = true
			}
		}
	case "custom_tool_call_output", "function_call_output":
		text, images, unknown := measureOutput(value.Output, 0)
		r.TextBytes += text
		r.Images += images
		r.Unclassified += unknown
		if text > 0 {
			r.Largest = append(r.Largest, outputSize{Record: r.Records, TextBytes: text})
			sort.Slice(r.Largest, func(i, j int) bool { return r.Largest[i].TextBytes > r.Largest[j].TextBytes })
			if len(r.Largest) > 8 {
				r.Largest = r.Largest[:8]
			}
		}
	}
}

// toolClass emits a finite control-plane vocabulary; arbitrary names in an
// untrusted session file never become report text.
func toolClass(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.LastIndex(name, "__"); i >= 0 {
		name = name[i+2:]
	}
	switch name {
	case "exec", "exec_command", "write_stdin", "js", "sleep", "wait", "request_user_input_async", "apply_patch", "view_image", "read_thread", "wait_threads", "open_in_codex":
		return name
	}
	return "other"
}

// measureOutput reads known content blocks only; image encodings stay excluded.
func measureOutput(raw json.RawMessage, depth int) (int64, int, int64) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return 0, 0, 0
	}
	if depth > 3 {
		return 0, 0, int64(len(raw))
	}
	if raw[0] == '"' {
		var text string
		if json.Unmarshal(raw, &text) != nil {
			return 0, 0, int64(len(raw))
		}
		trim := strings.TrimSpace(text)
		if strings.HasPrefix(trim, "[") || strings.HasPrefix(trim, "{") {
			return measureOutput([]byte(trim), depth+1)
		}
		return int64(len(text)), 0, 0
	}
	if raw[0] == '[' {
		var blocks []json.RawMessage
		if json.Unmarshal(raw, &blocks) != nil {
			return 0, 0, int64(len(raw))
		}
		var total, unknown int64
		images := 0
		for _, block := range blocks {
			n, i, u := measureOutput(block, depth+1)
			total += n
			images += i
			unknown += u
		}
		return total, images, unknown
	}
	var block struct {
		Type    string          `json:"type"`
		Text    string          `json:"text"`
		Content json.RawMessage `json:"content"`
		Output  json.RawMessage `json:"output"`
	}
	if json.Unmarshal(raw, &block) != nil {
		return 0, 0, int64(len(raw))
	}
	switch block.Type {
	case "text", "input_text", "output_text":
		return int64(len(block.Text)), 0, 0
	case "image", "image_url", "input_image", "output_image":
		return 0, 1, 0
	}
	if len(block.Content) > 0 {
		return measureOutput(block.Content, depth+1)
	}
	if len(block.Output) > 0 {
		return measureOutput(block.Output, depth+1)
	}
	return 0, 0, int64(len(raw))
}

// readPaths returns source paths only, never a command or its argument values.
func readPaths(input string) map[string]bool {
	found := map[string]bool{}
	for _, match := range commandPattern.FindAllStringSubmatch(input, -1) {
		var command string
		if json.Unmarshal([]byte(match[1]), &command) != nil {
			continue
		}
		for _, line := range strings.Split(command, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "cat ") && !strings.HasPrefix(line, "sed -n ") {
				continue
			}
			for _, part := range strings.Fields(line) {
				path := strings.Trim(part, "\"'`;|")
				if len(path) <= 200 && readPathPattern.MatchString(path) && !strings.Contains(path, "..") && !strings.Contains(path, "private") && !strings.Contains(path, "runtime/") {
					found[path] = true
				}
			}
		}
	}
	return found
}

func topCounts(input map[string]int, repeated bool) []counted {
	result := []counted{}
	for name, count := range input {
		if !repeated || count > 1 {
			result = append(result, counted{Name: name, Count: count})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Name < result[j].Name
	})
	if len(result) > 8 {
		result = result[:8]
	}
	return result
}
