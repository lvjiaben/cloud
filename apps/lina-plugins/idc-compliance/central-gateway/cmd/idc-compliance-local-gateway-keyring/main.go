// Command idc-compliance-local-gateway-keyring manages one
// installation's local process-authentication keyring file.
package main

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

// maximumInputBytes bounds existing keyring files read during rotation.
const maximumInputBytes = processauth.MaximumKeyringSize

// main parses the operator-only key lifecycle command and reports failures.
func main() {
	output := flag.String("out", "", "new keyring output file")
	installationIDOutput := flag.String(
		"installation-id-out", "", "new installation identifier output file",
	)
	rotate := flag.String("rotate", "", "existing keyring file to rotate")
	revoke := flag.String("revoke", "", "verify-only key id to revoke")
	flag.Parse()
	if err := run(*output, *installationIDOutput, *rotate, *revoke); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run validates one fresh-generation or rotation operation and writes new files.
func run(output, installationIDOutput, rotate, revoke string) error {
	output = strings.TrimSpace(output)
	installationIDOutput = strings.TrimSpace(installationIDOutput)
	rotate = strings.TrimSpace(rotate)
	revoke = strings.TrimSpace(revoke)
	if err := validateNewOutput(output, "-out"); err != nil {
		return err
	}
	var file processauth.File
	var (
		installationID string
		err            error
	)
	switch {
	case rotate == "" && revoke == "":
		if err = validateNewOutput(installationIDOutput, "-installation-id-out"); err != nil {
			return err
		}
		if filepath.Clean(installationIDOutput) == filepath.Clean(output) {
			return fmt.Errorf("installation identifier and keyring outputs must be distinct")
		}
		file, err = processauth.Generate(rand.Reader)
		if err == nil {
			installationID, err = processauth.GenerateInstallationID(rand.Reader)
		}
	case rotate != "":
		if installationIDOutput != "" {
			return fmt.Errorf("rotation must preserve the existing installation identifier")
		}
		if !filepath.IsAbs(rotate) || rotate == output || strings.ContainsAny(rotate, "\r\n\x00") {
			return fmt.Errorf("-rotate must name a different absolute file")
		}
		file, err = readKeyring(rotate)
		if err == nil {
			file, err = processauth.Rotate(file, rand.Reader)
		}
		if err == nil && revoke != "" {
			file, err = processauth.Revoke(file, revoke)
		}
	case revoke != "":
		return fmt.Errorf("-revoke requires -rotate so an active replacement exists")
	}
	if err != nil {
		return fmt.Errorf("prepare keyring: %w", err)
	}
	content, err := processauth.Marshal(file)
	if err != nil {
		return fmt.Errorf("encode keyring: %w", err)
	}
	defer clearBytes(content)
	if installationID != "" {
		installationContent := []byte(installationID + "\n")
		defer clearBytes(installationContent)
		if err = writePrivateFile(installationIDOutput, installationContent); err != nil {
			return fmt.Errorf("write installation identifier: %w", err)
		}
		if err = writePrivateFile(output, content); err != nil {
			cleanupError := os.Remove(installationIDOutput)
			if cleanupError != nil {
				return errors.Join(
					fmt.Errorf("write keyring: %w", err),
					fmt.Errorf("remove partial installation identifier: %w", cleanupError),
				)
			}
			return fmt.Errorf("write keyring: %w", err)
		}
		fmt.Printf(
			"wrote installation-local identifier %s\nwrote installation-local keyring %s\nactive key %s\n",
			installationIDOutput, output, file.ActiveKeyID,
		)
		return nil
	}
	if err = writePrivateFile(output, content); err != nil {
		return fmt.Errorf("write keyring: %w", err)
	}
	fmt.Printf("wrote installation-local keyring %s\nactive key %s\n", output, file.ActiveKeyID)
	return nil
}

// validateNewOutput requires a new absolute path that does not already exist.
func validateNewOutput(path string, flagName string) error {
	if path == "" || !filepath.IsAbs(path) || strings.ContainsAny(path, "\r\n\x00") {
		return fmt.Errorf("%s must be an absolute file path", flagName)
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("refusing to overwrite existing %s file", flagName)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect %s output: %w", flagName, err)
	}
	return nil
}

// writePrivateFile creates one complete file without overwriting existing data.
func writePrivateFile(path string, content []byte) (returnError error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create private configuration directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		if complete {
			return
		}
		if cleanupError := os.Remove(path); cleanupError != nil && !errors.Is(cleanupError, os.ErrNotExist) {
			returnError = errors.Join(
				returnError, fmt.Errorf("remove partial private file: %w", cleanupError),
			)
		}
	}()
	if _, err = file.Write(content); err != nil {
		closeError := file.Close()
		if closeError != nil {
			return errors.Join(err, closeError)
		}
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err = os.Chmod(path, 0o600); err != nil {
			return err
		}
	}
	complete = true
	return nil
}

// readKeyring loads one private regular file for an operator-triggered rotation.
func readKeyring(path string) (result processauth.File, returnError error) {
	file, err := os.Open(path)
	if err != nil {
		return processauth.File{}, err
	}
	defer func() {
		if closeError := file.Close(); closeError != nil && returnError == nil {
			returnError = fmt.Errorf("close keyring: %w", closeError)
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return processauth.File{}, err
	}
	if !info.Mode().IsRegular() || runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return processauth.File{}, fmt.Errorf("keyring must be a private regular file")
	}
	content, err := io.ReadAll(io.LimitReader(file, maximumInputBytes+1))
	if err != nil {
		clearBytes(content)
		return processauth.File{}, err
	}
	defer clearBytes(content)
	if len(content) > maximumInputBytes {
		return processauth.File{}, fmt.Errorf("keyring file is too large")
	}
	result, returnError = processauth.Parse(content)
	return result, returnError
}

// clearBytes removes encoded secret content from the supplied slice.
func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
