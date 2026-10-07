// Command idc-compliance-local-gateway-evidence-keyring generates the
// current installation's private callback-evidence keyring.
package main

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
)

func main() {
	output := flag.String("out", "", "new callback evidence keyring output file")
	flag.Parse()
	if err := run(*output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(output string) error {
	output = strings.TrimSpace(output)
	if output == "" || !filepath.IsAbs(output) || strings.ContainsAny(output, "\r\n\x00") {
		return fmt.Errorf("-out must be an absolute file path")
	}
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("refusing to overwrite existing output file")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect output file: %w", err)
	}
	file, err := callbacksecurity.Generate(rand.Reader)
	if err != nil {
		return fmt.Errorf("prepare callback evidence keyring: %w", err)
	}
	content, err := callbacksecurity.Marshal(file)
	if err != nil {
		return err
	}
	defer clear(content)
	parent := filepath.Dir(output)
	if info, statErr := os.Stat(parent); statErr != nil || !info.IsDir() {
		return fmt.Errorf("output parent directory must exist")
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	created, err := os.OpenFile(output, flags, 0o600)
	if err != nil {
		return fmt.Errorf("create callback evidence keyring: %w", err)
	}
	_, writeErr := created.Write(content)
	if writeErr == nil {
		writeErr = created.Sync()
	}
	closeErr := created.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(output)
		return errors.Join(writeErr, closeErr)
	}
	if runtime.GOOS != "windows" {
		if err = os.Chmod(output, 0o600); err != nil {
			_ = os.Remove(output)
			return fmt.Errorf("protect callback evidence keyring: %w", err)
		}
	}
	fmt.Printf("wrote installation-local callback evidence keyring %s\n", output)
	return nil
}
