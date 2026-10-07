// This file defines the installation-local, no-shell document conversion boundary.

package electroniccontracttemplate

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

const maximumPDFBytes = 50 * 1024 * 1024

// DocumentConverter converts one already-validated source without network or shell access.
type DocumentConverter interface {
	Convert(ctx context.Context, mediaType string, source []byte) ([]byte, error)
}

// UnavailableDocumentConverter keeps conversion explicitly fail-closed.
type UnavailableDocumentConverter struct{}

// Convert reports that the installation has no reviewed converter runtime.
func (UnavailableDocumentConverter) Convert(context.Context, string, []byte) ([]byte, error) {
	return nil, gerror.New("installation-local contract document converter is unavailable")
}

// LibreOfficeDocumentConverter invokes one absolute, deployment-owned soffice binary.
type LibreOfficeDocumentConverter struct {
	executable string
}

// NewLibreOfficeDocumentConverter validates the exact executable path without PATH lookup.
func NewLibreOfficeDocumentConverter(executable string) (DocumentConverter, error) {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return UnavailableDocumentConverter{}, nil
	}
	if !filepath.IsAbs(executable) {
		return nil, gerror.New("contract document converter path must be absolute")
	}
	info, err := os.Stat(executable)
	if err != nil || info.IsDir() {
		return nil, gerror.New("contract document converter executable is unavailable")
	}
	return &LibreOfficeDocumentConverter{executable: filepath.Clean(executable)}, nil
}

// Convert writes bounded temporary plaintext, runs a fixed argument vector, and removes all files.
func (c *LibreOfficeDocumentConverter) Convert(
	ctx context.Context,
	mediaType string,
	source []byte,
) ([]byte, error) {
	if c == nil || c.executable == "" || ctx == nil || ctx.Err() != nil || len(source) == 0 {
		return nil, gerror.New("contract document conversion input is invalid")
	}
	extension, ok := conversionExtension(mediaType)
	if !ok {
		return nil, gerror.New("contract document conversion media type is unsupported")
	}
	working, err := os.MkdirTemp("", "idc-contract-convert-")
	if err != nil {
		return nil, gerror.Wrap(err, "create contract conversion directory")
	}
	defer os.RemoveAll(working)
	inputPath := filepath.Join(working, "source"+extension)
	outputDir := filepath.Join(working, "output")
	profileDir := filepath.Join(working, "profile")
	if err = os.Mkdir(outputDir, 0o700); err != nil {
		return nil, gerror.Wrap(err, "create contract conversion output directory")
	}
	if err = os.Mkdir(profileDir, 0o700); err != nil {
		return nil, gerror.Wrap(err, "create contract conversion profile directory")
	}
	if err = os.WriteFile(inputPath, source, 0o600); err != nil {
		return nil, gerror.Wrap(err, "write contract conversion source")
	}
	profileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(profileDir)}).String()
	command := exec.CommandContext(
		ctx,
		c.executable,
		"--headless",
		"--nologo",
		"--nodefault",
		"--nolockcheck",
		"--norestore",
		"-env:UserInstallation="+profileURL,
		"--convert-to",
		"pdf",
		"--outdir",
		outputDir,
		inputPath,
	)
	command.Dir = working
	command.Env = converterEnvironment(working)
	output := &boundedOutput{maximum: 64 * 1024}
	command.Stdout = output
	command.Stderr = output
	if err = command.Run(); err != nil {
		return nil, gerror.New("installation-local contract document conversion failed")
	}
	result, err := os.ReadFile(filepath.Join(outputDir, "source.pdf"))
	if err != nil || len(result) == 0 || len(result) > maximumPDFBytes {
		clearContentBytes(result)
		return nil, gerror.New("contract document converter produced no bounded PDF")
	}
	return result, nil
}

func conversionExtension(mediaType string) (string, bool) {
	switch mediaType {
	case "application/msword":
		return ".doc", true
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx", true
	case "text/html":
		return ".html", true
	default:
		return "", false
	}
}

func converterEnvironment(working string) []string {
	allowed := []string{"LANG", "LC_ALL", "PATH", "SYSTEMROOT", "TEMP", "TMP", "WINDIR"}
	environment := make([]string, 0, len(allowed)+2)
	for _, key := range allowed {
		if value, ok := os.LookupEnv(key); ok && value != "" {
			environment = append(environment, key+"="+value)
		}
	}
	if runtime.GOOS != "windows" {
		environment = append(environment, "HOME="+working)
	}
	return environment
}

type boundedOutput struct {
	buffer  bytes.Buffer
	maximum int
}

func (w *boundedOutput) Write(value []byte) (int, error) {
	if w == nil || w.maximum <= 0 {
		return 0, io.ErrClosedPipe
	}
	remaining := w.maximum - w.buffer.Len()
	if remaining > 0 {
		_, _ = w.buffer.Write(value[:min(len(value), remaining)])
	}
	return len(value), nil
}
