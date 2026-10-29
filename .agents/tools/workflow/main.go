// Command workflow provides bounded session audits and scoped check execution.
// It never executes session contents or treats a partial check as full acceptance.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// values implements repeatable scope flags without shell parsing.
type values []string

func (v *values) String() string     { return fmt.Sprint([]string(*v)) }
func (v *values) Set(s string) error { *v = append(*v, s); return nil }

func main() {
	root, err := os.Getwd()
	args := os.Args[1:]
	if len(args) >= 2 && args[0] == "--root" {
		root = args[1]
		args = args[2:]
	}
	if err == nil {
		err = runWorkflow(root, args, os.Stdout, os.Stderr)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// boundedJSON refuses excess output rather than truncating a report.
func boundedJSON(out io.Writer, value any) error {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return err
	}
	if b.Len() > 8192 {
		return errors.New("摘要超过8 KiB；请缩小范围")
	}
	_, err := out.Write(b.Bytes())
	return err
}

// runWorkflow parses a narrow audit/check request; only checks execute commands.
func runWorkflow(root string, args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return errors.New("用法：audit --thread ID|--file JSONL；check --path 路径 [--change 变更] [--plan]")
	}
	switch args[0] {
	case "audit":
		f := flag.NewFlagSet("audit", flag.ContinueOnError)
		f.SetOutput(errOut)
		thread := f.String("thread", "", "精确会话ID")
		file := f.String("file", "", "显式本地JSONL会话文件")
		if err := f.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return errors.New("audit参数无效")
		}
		if f.NArg() != 0 || (*thread == "") == (*file == "") {
			return errors.New("audit必须且只能选择--thread或--file")
		}
		path, err := sessionPath(*thread, *file)
		if err != nil {
			return err
		}
		report, err := auditSession(path)
		if report != nil {
			if outputErr := boundedJSON(out, report); outputErr != nil {
				return outputErr
			}
		}
		return err
	case "check":
		f := flag.NewFlagSet("check", flag.ContinueOnError)
		f.SetOutput(errOut)
		var paths, changes values
		f.Var(&paths, "path", "本次仓库相对路径，可重复")
		f.Var(&changes, "change", "相关活跃变更，可重复")
		planOnly := f.Bool("plan", false, "仅输出计划，不运行命令或创建日志")
		timeout := f.Duration("timeout", 2*time.Minute, "单项命令超时，最大10m")
		if err := f.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return errors.New("check参数无效")
		}
		if f.NArg() != 0 || *timeout <= 0 || *timeout > 10*time.Minute {
			return errors.New("check参数或超时无效")
		}
		root, err := filepath.Abs(root)
		if err != nil {
			return err
		}
		root, err = filepath.EvalSymlinks(root)
		if err != nil {
			return err
		}
		plan, err := planChecks(root, paths, changes)
		if err != nil {
			return err
		}
		if err = boundedJSON(io.Discard, plan); err != nil {
			return err
		}
		if *planOnly {
			return boundedJSON(out, plan)
		}
		report, err := executeChecks(root, plan, *timeout)
		if report != nil {
			if outputErr := boundedJSON(out, report); outputErr != nil {
				return outputErr
			}
		}
		return err
	default:
		return errors.New("未知工作流命令")
	}
}
