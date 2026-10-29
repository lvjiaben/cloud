// This standalone command audits every plugin's documentation and public owner contract.
// It creates an isolated Go workspace and never rewrites the shared development workspace.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	manifests, err := filepath.Glob(filepath.Join(root, "apps", "lina-plugins", "*", "plugin.yaml"))
	if err != nil {
		return err
	}
	if len(manifests) == 0 {
		return fmt.Errorf("run plugincheck from the repository root")
	}
	dateDir := filepath.Join(root, "temp", time.Now().Format("20060102"))
	if err = os.MkdirAll(dateDir, 0755); err != nil {
		return err
	}
	workDir, err := os.MkdirTemp(dateDir, time.Now().Format("150405")+"-plugin-contracts-")
	if err != nil {
		return err
	}
	paths := []string{filepath.Join(root, "apps", "lina-core")}
	for _, file := range manifests {
		module := filepath.Join(filepath.Dir(file), "go.mod")
		if _, err = os.Stat(module); err == nil {
			paths = append(paths, filepath.Dir(file))
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	tests := filepath.Join(root, "apps", "lina-plugins", "plugin-manager", "hack", "tests", "contracts")
	paths = append(paths, tests)
	var workspace strings.Builder
	workspace.WriteString("go 1.25.0\n\nuse (\n")
	for _, path := range paths {
		workspace.WriteString("\t" + strconv.Quote(filepath.ToSlash(path)) + "\n")
	}
	workspace.WriteString(")\n")
	if err = os.WriteFile(filepath.Join(workDir, "go.work"), []byte(workspace.String()), 0600); err != nil {
		return err
	}
	logPath := filepath.Join(workDir, time.Now().Format("150405")+"-audit.log")
	log, err := os.Create(logPath)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "go", "test", "./...", "-count=1", "-v")
	command.Dir = tests
	command.Env = append(withoutWork(os.Environ()), "GOWORK="+filepath.Join(workDir, "go.work"))
	command.Stdout = log
	command.Stderr = log
	runErr := command.Run()
	closeErr := log.Close()
	fmt.Printf("Plugin manifest count: %d\nAudit log: %s\n", len(manifests), logPath)
	if runErr != nil {
		return fmt.Errorf("plugin contract audit failed: %w; see %s", runErr, logPath)
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Println("Plugin documentation and owner-contract audit passed.")
	return nil
}

func withoutWork(env []string) []string {
	result := make([]string, 0, len(env))
	for _, value := range env {
		if !strings.HasPrefix(strings.ToUpper(value), "GOWORK=") {
			result = append(result, value)
		}
	}
	return result
}
