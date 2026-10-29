// Build ordinary files without plugin metadata, WASM or host imports.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

func build(root, output string) error {
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("output already exists; choose a new -output directory: %s", output)
	} else if !os.IsNotExist(err) {
		return err
	}
	checksums := map[string]string{}
	write := func(name string, data []byte) error {
		path := filepath.Join(output, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return err
		}
		digest := sha256.Sum256(data)
		checksums[name] = hex.EncodeToString(digest[:])
		return nil
	}
	public := filepath.Join(root, "frontend", "public")
	if err := filepath.WalkDir(public, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular source resource: %s", path)
		}
		name, err := filepath.Rel(public, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return write("public/"+filepath.ToSlash(name), data)
	}); err != nil {
		return err
	}
	for _, name := range []string{"gateway/server.mjs", "gateway/site.mjs", "gateway/commerce.mjs", "gateway/brand-identity.mjs", "DEPLOYMENT.md", "DEPLOYMENT.zh-CN.md"} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		if err = write(name, data); err != nil {
			return err
		}
	}
	if err := write("package.json", []byte("{\"private\":true,\"type\":\"module\",\"scripts\":{\"start\":\"node gateway/server.mjs\"}}\n")); err != nil {
		return err
	}
	data, err := json.MarshalIndent(checksums, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "checksums.json"), append(data, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("Built ordinary website: %s (%d files plus checksums.json)\n", output, len(checksums))
	return nil
}

func main() {
	_, source, _, found := runtime.Caller(0)
	if !found {
		fmt.Fprintln(os.Stderr, "cannot locate website build source")
		os.Exit(1)
	}
	root := flag.String("root", filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..")), "website source directory")
	output := flag.String("output", "", "new output directory; defaults to dist/site")
	flag.Parse()
	if *output == "" {
		*output = filepath.Join(*root, "dist", "site")
	}
	if err := build(*root, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
