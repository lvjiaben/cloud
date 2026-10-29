// This command prepares persistent installation secrets and launches existing
// public development commands. Host implementation stays unchanged; official
// embed-resource generation is an explicit installation exception. No suppliers
// are contacted by this tool.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// prepareRuntimeConfig gives operators an editable config outside protected sources.
func prepareRuntimeConfig(root, state string, env []string) error {
	if envValue(env, "GF_GCFG_FILE") != "" {
		return nil
	}
	path := filepath.Join(state, "config.yaml")
	if _, err := os.Lstat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	content, err := os.ReadFile(filepath.Join(root, "apps", "lina-core", "manifest", "config", "config.template.yaml"))
	if err != nil {
		return err
	}
	if err = writePrivate(path, content); err != nil {
		return err
	}
	fmt.Println("Database configuration:", path)
	return nil
}

// deployment stores installation-owned material outside the source tree.
type deployment struct {
	JWT      string          `json:"jwt"`
	Security securityKeyring `json:"security"`
}

// securityKeyring matches the IDC plugin's public deployment input.
type securityKeyring struct {
	Active      string            `json:"active_key_id"`
	Reservation string            `json:"reservation_key"`
	Keys        map[string]string `json:"keys"`
}

// main reports errors without ever printing secret contents or environment values.
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Installation preparation failed:", err)
		os.Exit(1)
	}
}

// run prepares one installation and forwards explicit commands with its environment.
func run(args []string) error {
	flags := flag.NewFlagSet("installation", flag.ContinueOnError)
	rootFlag := flags.String("root", ".", "repository root")
	stateFlag := flags.String("state", ".site-runtime/installation", "persistent private directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		return err
	}
	if _, err = os.Stat(filepath.Join(root, "apps", "lina-core", "go.mod")); err != nil {
		return fmt.Errorf("run from the repository root or specify -root")
	}
	state := *stateFlag
	if !filepath.IsAbs(state) {
		state = filepath.Join(root, state)
	}
	state, err = filepath.Abs(state)
	if err != nil {
		return err
	}
	command := flags.Args()
	if len(command) == 0 {
		command = []string{"prepare"}
	}
	switch command[0] {
	case "prepare", "env.setup", "dev", "build", "wasm", "db.init", "run", "plugins.install", "customer.run":
	default:
		return fmt.Errorf("supported commands: prepare, env.setup, dev, build, wasm, db.init, run, plugins.install, customer.run")
	}
	if command[0] == "db.init" && !contains(command[1:], "confirm=init") {
		return fmt.Errorf("database initialization requires confirm=init; existing databases are not rebuilt automatically")
	}
	if command[0] == "build" {
		return buildDelivery(root, state, command[1:])
	}
	if command[0] == "dev" {
		return runDevelopment(root, state, command[1:])
	}
	if command[0] == "env.setup" {
		if len(command) != 1 {
			return fmt.Errorf("env.setup does not accept additional arguments")
		}
		// Browser/OS packages belong to the optional E2E toolchain, not runtime
		// installation. In particular Playwright may not support the host OS.
		if err = execute(filepath.Join(root, "apps", "lina-vben"), os.Environ(), pnpmCommand(), "install", "--frozen-lockfile"); err != nil {
			return err
		}
	}
	env, err := prepare(root, state, os.Environ())
	if err != nil {
		return err
	}
	if command[0] == "prepare" || command[0] == "env.setup" {
		if command[0] == "env.setup" {
			if err = prepareRuntimeConfig(root, state, env); err != nil {
				return err
			}
		}
		fmt.Println("Installation is ready. Saved instance keys will be reused on restart.")
		return nil
	}
	if command[0] == "db.init" {
		if err = execute(filepath.Join(root, "hack", "tools", "linactl"), env, "go", "run", ".", "pack.assets"); err != nil {
			return err
		}
	}
	if command[0] == "customer.run" {
		return runCustomerGateway(root, env)
	}
	if command[0] == "run" {
		if len(command) < 2 {
			return fmt.Errorf("provide the built executable path after run")
		}
		binary, err := filepath.Abs(command[1])
		if err != nil {
			return err
		}
		return execute(root, env, binary, command[2:]...)
	}
	forward := append([]string{"run", ".", command[0]}, command[1:]...)
	if command[0] == "plugins.install" {
		forward = append(forward, "config="+filepath.Join(root, "apps", "lina-plugins", "idc-compliance", "hack", "official-plugins.yaml"))
	}
	return execute(filepath.Join(root, "hack", "tools", "linactl"), env, "go", forward...)
}

// prepare preserves explicit deployment inputs and generates only missing defaults.
func prepare(root, state string, env []string) ([]string, error) {
	if selected := envValue(env, "GF_GCFG_FILE"); selected != "" && !filepath.IsAbs(selected) {
		env = replaceEnv(env, "GF_GCFG_FILE", filepath.Join(root, selected))
	}
	if err := os.MkdirAll(state, 0o700); err != nil {
		return nil, err
	}
	material, err := loadDeployment(filepath.Join(state, "deployment.json"))
	if err != nil {
		return nil, err
	}
	security, err := json.Marshal(material.Security)
	if err != nil {
		return nil, err
	}
	env = defaultEnv(env, "LINAPRO_IDC_CORE_SECURITY_KEYRING", string(security))
	env = defaultEnv(env, "LINA_JWT_SECRET", material.JWT)
	keyringName := "LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_KEYRING_FILE"
	installationName := "LINAPRO_COMPLIANCE_GATEWAY_INSTALLATION_ID_FILE"
	keyring, installation := envValue(env, keyringName), envValue(env, installationName)
	if (keyring == "") != (installation == "") {
		return nil, fmt.Errorf("configure both the gateway keyring and installation identity files")
	}
	if keyring == "" {
		keyring, installation = filepath.Join(state, "gateway-auth.json"), filepath.Join(state, "installation-id")
		if err = prepareGateway(root, keyring, installation, env); err != nil {
			return nil, err
		}
	}
	if err = checkGatewayFiles(keyring, installation); err != nil {
		return nil, err
	}
	env = defaultEnv(env, keyringName, keyring)
	env = defaultEnv(env, installationName, installation)
	env = defaultEnv(env, "LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_WINDOW", "60s")
	env = defaultEnv(env, "LINAPRO_IDC_CUSTOMER_PUBLIC_ORIGIN", "http://localhost:6032")
	env = defaultEnv(env, "LINA_SITE_STORAGE", filepath.Dir(state))
	env = defaultEnv(env, "GF_PLUGIN_DYNAMIC_STORAGEPATH", filepath.Join(root, "temp", "output"))
	if _, configErr := os.Stat(filepath.Join(state, "config.yaml")); configErr == nil {
		env = defaultEnv(env, "GF_GCFG_FILE", filepath.Join(state, "config.yaml"))
	}
	return env, nil
}

// loadDeployment validates saved material and refuses to replace corrupt secrets.
func loadDeployment(path string) (deployment, error) {
	content, err := readPrivate(path, 16384)
	if errors.Is(err, os.ErrNotExist) {
		var material deployment
		jwt, jwtErr := randomSecret()
		reservation, reservationErr := randomSecret()
		root, rootErr := randomSecret()
		if err = errors.Join(jwtErr, reservationErr, rootErr); err != nil {
			return material, err
		}
		material = deployment{JWT: jwt, Security: securityKeyring{Active: "v1", Reservation: reservation, Keys: map[string]string{"v1": root}}}
		content, err = json.Marshal(material)
		if err != nil {
			return material, err
		}
		if err = writePrivate(path, content); err != nil {
			return material, err
		}
		return material, nil
	}
	if err != nil {
		return deployment{}, fmt.Errorf("cannot read installation key file: %w", err)
	}
	defer clear(content)
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	decoder.DisallowUnknownFields()
	var material deployment
	if err = decoder.Decode(&material); err != nil || decoder.Decode(&struct{}{}) != io.EOF || !validMaterial(material) {
		return deployment{}, fmt.Errorf("installation key file is damaged; restore a backup, automatic replacement is disabled")
	}
	return material, nil
}

// validMaterial checks canonical keys before reusing deployment storage.
func validMaterial(material deployment) bool {
	if !validSecret(material.JWT) || !validSecret(material.Security.Reservation) || len(material.Security.Keys) == 0 || len(material.Security.Keys) > 8 {
		return false
	}
	if _, ok := material.Security.Keys[material.Security.Active]; !ok {
		return false
	}
	for id, key := range material.Security.Keys {
		if id == "" || len(id) > 40 || !validSecret(key) || key == material.Security.Reservation {
			return false
		}
	}
	return true
}

// validSecret accepts a canonical, independently generated 256-bit root.
func validSecret(value string) bool {
	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	defer clear(decoded)
	return err == nil && len(decoded) == 32 && base64.StdEncoding.EncodeToString(decoded) == value
}

// randomSecret generates a fresh private root without any shared defaults.
func randomSecret() (string, error) {
	value := make([]byte, 32)
	defer clear(value)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(value), nil
}

// prepareGateway reuses the gateway's public generator and detects partial installs.
func prepareGateway(root, keyring, installation string, env []string) error {
	_, keyErr := os.Lstat(keyring)
	_, idErr := os.Lstat(installation)
	if keyErr == nil && idErr == nil {
		return nil
	}
	if !errors.Is(keyErr, os.ErrNotExist) || !errors.Is(idErr, os.ErrNotExist) {
		return fmt.Errorf("gateway installation files are incomplete; restore a backup, automatic replacement is disabled")
	}
	return execute(filepath.Join(root, "apps", "lina-plugins", "idc-compliance", "central-gateway"), replaceEnv(env, "GOWORK", "off"), "go", "run", "./cmd/idc-compliance-local-gateway-keyring", "-out", keyring, "-installation-id-out", installation)
}

// checkGatewayFiles checks private storage; the gateway's own parser validates its contract at startup.
func checkGatewayFiles(keyring, installation string) error {
	if !filepath.IsAbs(keyring) || !filepath.IsAbs(installation) || filepath.Clean(keyring) == filepath.Clean(installation) {
		return fmt.Errorf("gateway installation files require two different absolute paths")
	}
	content, err := readPrivate(keyring, 8192)
	if err != nil {
		return fmt.Errorf("cannot read gateway keyring: %w", err)
	}
	defer clear(content)
	if !json.Valid(content) {
		return fmt.Errorf("gateway keyring is damaged; restore a backup")
	}
	id, err := readPrivate(installation, 128)
	if err != nil {
		return fmt.Errorf("cannot read gateway installation identity: %w", err)
	}
	defer clear(id)
	value := strings.TrimSpace(string(id))
	if !strings.HasPrefix(value, "linst_") || len(value) != 70 {
		return fmt.Errorf("gateway installation identity is damaged; restore a backup")
	}
	return nil
}

// readPrivate rejects symlinks, oversized files and public Unix permissions.
func readPrivate(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit || (runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0) {
		return nil, fmt.Errorf("installation files must be nonempty private regular files")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	content, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	if err = errors.Join(readErr, closeErr); err != nil {
		clear(content)
		return nil, err
	}
	if int64(len(content)) > limit {
		clear(content)
		return nil, fmt.Errorf("installation file is too large")
	}
	return content, nil
}

// writePrivate creates complete private material exclusively, never overwriting a prior installation.
func writePrivate(path string, content []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(content)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return errors.Join(err, os.Remove(path))
	}
	return nil
}

// execute forwards output and stdin without using a platform-specific shell.
func execute(dir string, env []string, name string, args ...string) error {
	program, commandArgs := executionCommand(runtime.GOOS, name, args)
	command := exec.CommandContext(context.Background(), program, commandArgs...)
	command.Dir, command.Env = dir, env
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

// envValue resolves the effective value of a child-process input.
func envValue(env []string, key string) string {
	for _, value := range env {
		if strings.HasPrefix(value, key+"=") {
			return strings.TrimPrefix(value, key+"=")
		}
	}
	return ""
}

// defaultEnv preserves nonempty explicit operator configuration.
func defaultEnv(env []string, key, value string) []string {
	if envValue(env, key) != "" {
		return env
	}
	return replaceEnv(env, key, value)
}

// replaceEnv sets a unique child environment key without affecting the parent.
func replaceEnv(env []string, key, value string) []string {
	result := make([]string, 0, len(env)+1)
	for _, item := range env {
		if !strings.HasPrefix(item, key+"=") {
			result = append(result, item)
		}
	}
	return append(result, key+"="+value)
}

// contains checks one explicit CLI confirmation argument.
func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

// runCustomerGateway serves the selected installed template using public HTTP APIs.
// Port ownership remains explicit: an existing listener causes startup to fail.
func runCustomerGateway(root string, env []string) error {
	entry := filepath.Join(root, "apps", "lina-plugins", "customer-default", "frontend", "server.mjs")
	if _, err := os.Stat(entry); err != nil {
		return fmt.Errorf("customer gateway is missing; restore the customer-default plugin source: %w", err)
	}
	return execute(root, env, "node", entry)
}

// buildDelivery assembles installation outputs in an isolated staging directory.
// Host sources are read-only; obsolete website/customer workspaces are never used.
func buildDelivery(root, state string, args []string) error {
	target, output, plan, err := deliveryOptions(root, args)
	if err != nil {
		return err
	}
	if plan {
		fmt.Printf("Delivery target: %s\nOutput: %s\n", target, output)
		if target != "templates" {
			fmt.Println("1. Generate the official source-plugin workspace outside host sources.\n2. Build separate admin static assets into delivery output.\n3. Compile the unmodified host backend with all official source plugins.")
		}
		if target != "backend" {
			fmt.Println("4. Build independent customer and website template packages.\n5. Build remaining dynamic plugins with the public wasm command.\n6. Copy template gateways and SSR renderers into delivery output.")
		}
		return nil
	}
	env, err := prepare(root, state, os.Environ())
	if err != nil {
		return err
	}
	if err = os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	if target != "templates" {
		if err = buildDeliveryBackend(root, output, env); err != nil {
			return err
		}
	}
	if target != "backend" {
		if err = buildDeliveryTemplates(root, output, env); err != nil {
			return err
		}
	}
	fmt.Println("Installation delivery complete:", output)
	return nil
}

// deliveryOptions rejects unsupported flags and outputs inside protected sources.
func deliveryOptions(root string, args []string) (string, string, bool, error) {
	target, output, plan := "all", filepath.Join(root, "temp", "output"), false
	for _, arg := range args {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			return "", "", false, fmt.Errorf("expected key=value: %s", arg)
		}
		switch key {
		case "plugins":
			if value != "1" {
				return "", "", false, fmt.Errorf("installation delivery requires plugins=1")
			}
		case "target":
			target = value
		case "out", "--output-dir":
			output = value
		case "verbose":
			if value != "0" && value != "1" {
				return "", "", false, fmt.Errorf("verbose requires 0 or 1")
			}
		case "plan":
			if value != "1" {
				return "", "", false, fmt.Errorf("plan requires 1")
			}
			plan = true
		default:
			return "", "", false, fmt.Errorf("unsupported delivery option: %s", key)
		}
	}
	if target != "all" && target != "backend" && target != "templates" {
		return "", "", false, fmt.Errorf("target must be all, backend or templates")
	}
	if !filepath.IsAbs(output) {
		output = filepath.Join(root, output)
	}
	output = filepath.Clean(output)
	for _, protected := range []string{filepath.Join(root, "apps"), filepath.Join(root, "hack")} {
		rel, err := filepath.Rel(protected, output)
		if err != nil {
			return "", "", false, err
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return "", "", false, fmt.Errorf("delivery output must be outside source directories")
		}
	}
	if output == root {
		return "", "", false, fmt.Errorf("delivery output must not be the repository root")
	}
	return target, output, plan, nil
}

// buildDeliveryBackend compiles unmodified host sources and serves admin assets separately.
func buildDeliveryBackend(root, output string, env []string) error {
	// Exact installation exception: the official generator prepares go:embed
	// inputs; it does not change host source logic or the generator itself.
	if err := execute(filepath.Join(root, "hack", "tools", "linactl"), env, "go", "run", ".", "pack.assets"); err != nil {
		return err
	}
	if err := checkPackedDeliveryAssets(root); err != nil {
		return err
	}
	stage := filepath.Join(output, ".build")
	core := filepath.Join(root, "apps", "lina-core")
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return err
	}
	adminOutput := filepath.Join(output, "admin")
	admin := filepath.Join(root, "apps", "lina-vben", "apps", "web-antd")
	adminEnv := replaceEnv(env, "LINAPRO_SOURCE_PLUGINS", "1")
	adapter := filepath.Join(root, "apps", "lina-plugins", "vben-adapter", "frontend")
	if err := execute(adapter, env, pnpmCommand(), "install", "--frozen-lockfile"); err != nil {
		return err
	}
	if err := execute(adapter, env, pnpmCommand(), "run", "build:isolated"); err != nil {
		return err
	}
	if err := buildSourceFrontends(root, env); err != nil {
		return err
	}

	configPath := filepath.Join(stage, "admin-vite.config.mts")
	configSource := fmt.Sprintf(`import hostConfig from %s;
export default async (environment) => {
 const config=await (typeof hostConfig==='function'?hostConfig(environment):hostConfig);
 async function clean(items){const result=[];for(const value of items||[]){const item=await value;if(Array.isArray(item))result.push(await clean(item));else if(item&&item.name!=='vite:archiver')result.push(item);}return result;}
 const originalAlias=config.resolve?.alias||[];
 const aliases=Array.isArray(originalAlias)?originalAlias:Object.entries(originalAlias).map(([find,replacement])=>({find,replacement}));
 return {...config,resolve:{...config.resolve,conditions:['development',...(config.resolve?.conditions||[])],alias:[
  {find:'@vben-core/design/bem',replacement:%s},
  {find:'@linapro/vben-plugin-adapter/isolated',replacement:%s},
  {find:'@linapro/vben-plugin-adapter/isolated.css',replacement:%s},
  ...aliases
 ]},plugins:await clean(config.plugins)};
};
`, strconv.Quote(filepath.ToSlash(filepath.Join(admin, "vite.config.mts"))),
		strconv.Quote(filepath.ToSlash(filepath.Join(root, "apps", "lina-vben", "packages", "@core", "base", "design", "src", "scss-bem", "bem.scss"))),
		strconv.Quote(filepath.ToSlash(filepath.Join(adapter, "dist", "isolated", "index.js"))),
		strconv.Quote(filepath.ToSlash(filepath.Join(adapter, "dist", "isolated", "vben-plugin-adapter.css"))))
	if err := os.WriteFile(configPath, []byte(configSource), 0o644); err != nil {
		return err
	}
	if err := execute(admin, adminEnv, pnpmCommand(), "exec", "vite", "build", "--config", configPath, "--mode", "production", "--outDir", filepath.Join(adminOutput, "public"), "--emptyOutDir"); err != nil {
		return err
	}
	if err := copyDeliveryFile(filepath.Join(root, "apps", "lina-plugins", "idc-compliance", "hack", "tools", "installation", "admin-gateway.mjs"), filepath.Join(adminOutput, "server.mjs")); err != nil {
		return err
	}
	work, err := deliveryWorkspace(root, stage, core)
	if err != nil {
		return err
	}
	buildEnv := replaceEnv(env, "GOWORK", work)
	buildEnv = replaceEnv(buildEnv, "CGO_ENABLED", "0")
	name := "lina"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err = execute(core, buildEnv, "go", "build", "-tags=official_plugins", "-o", filepath.Join(output, name), "."); err != nil {
		return err
	}
	return copyDeliveryTree(filepath.Join(root, "apps", "lina-core", "manifest", "config"), filepath.Join(output, "config"))
}

// buildSourceFrontends prepares plugin-owned public assets before Go embeds them.
func buildSourceFrontends(root string, env []string) error {
	entries, err := os.ReadDir(filepath.Join(root, "apps", "lina-plugins"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, "apps", "lina-plugins", entry.Name())
		manifest, readErr := os.ReadFile(filepath.Join(dir, "plugin.yaml"))
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return readErr
		}
		if topLevelManifestValue(string(manifest), "type") != "source" {
			continue
		}
		frontend := filepath.Join(dir, "frontend")
		if _, readErr = os.Stat(filepath.Join(frontend, "package.json")); errors.Is(readErr, os.ErrNotExist) {
			continue
		} else if readErr != nil {
			return readErr
		}
		if err = execute(frontend, env, pnpmCommand(), "install", "--frozen-lockfile"); err != nil {
			return err
		}
		if err = execute(frontend, env, pnpmCommand(), "run", "build"); err != nil {
			return err
		}
	}
	return nil
}

// deliveryWorkspace uses the host's public official_plugins registration contract.
// All generated files stay in staging; installed source plugins retain their own modules.
func deliveryWorkspace(root, stage, core string) (string, error) {
	aggregate := filepath.Join(stage, "official-plugins")
	if err := os.MkdirAll(aggregate, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(aggregate, "go.mod"), []byte("module lina-plugins\n\ngo 1.25.0\n"), 0o644); err != nil {
		return "", err
	}
	modules := []string{core, aggregate}
	imports := []string{}
	pluginRoot := filepath.Join(root, "apps", "lina-plugins")
	err := filepath.WalkDir(pluginRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case "node_modules", "frontend", "vendor", "dist", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != "go.mod" {
			return nil
		}
		dir := filepath.Dir(path)
		modules = append(modules, dir)
		manifest, readErr := os.ReadFile(filepath.Join(dir, "plugin.yaml"))
		if errors.Is(readErr, os.ErrNotExist) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
		if topLevelManifestValue(string(manifest), "type") != "source" {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		module := topLevelModule(string(content))
		if module == "" {
			return fmt.Errorf("missing module in %s", path)
		}
		imports = append(imports, module+"/backend")
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(modules)
	sort.Strings(imports)
	var source strings.Builder
	source.WriteString("// Generated installation source-plugin registration.\npackage linaplugins\nimport (\n")
	for _, name := range imports {
		fmt.Fprintf(&source, "_ %q\n", name)
	}
	source.WriteString(")\n")
	if err = os.WriteFile(filepath.Join(aggregate, "plugins.go"), []byte(source.String()), 0o644); err != nil {
		return "", err
	}
	var workspace strings.Builder
	workspace.WriteString("go 1.25.0\nuse (\n")
	for _, dir := range modules {
		fmt.Fprintf(&workspace, "%s\n", strconv.Quote(filepath.ToSlash(dir)))
	}
	workspace.WriteString(")\n")
	path := filepath.Join(stage, "go.work")
	return path, os.WriteFile(path, []byte(workspace.String()), 0o644)
}

// topLevelManifestValue reads a scalar root field, never nested fields or comments.
func topLevelManifestValue(content, key string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, key+":") {
			return strings.Trim(strings.TrimSpace(strings.SplitN(strings.TrimPrefix(line, key+":"), " #", 2)[0]), "\"'")
		}
	}
	return ""
}

// topLevelModule reads a standard Go module directive without evaluating a shell.
func topLevelModule(content string) string {
	for _, line := range strings.Split(content, "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[0] == "module" {
			return strings.Trim(parts[1], "\"")
		}
	}
	return ""
}

// buildDeliveryTemplates builds self-contained templates and public dynamic artifacts.
func buildDeliveryTemplates(root, output string, env []string) error {
	entries, err := os.ReadDir(filepath.Join(root, "apps", "lina-plugins"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		dir := filepath.Join(root, "apps", "lina-plugins", id)
		manifest, readErr := os.ReadFile(filepath.Join(dir, "plugin.yaml"))
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return readErr
		}
		if topLevelManifestValue(string(manifest), "type") != "dynamic" {
			continue
		}
		switch id {
		case "customer-default", "website-default", "website-minimal", "website-html":
			if err = execute(dir, env, pnpmCommand(), "install", "--frozen-lockfile"); err != nil {
				return err
			}
			switch id {
			case "customer-default":
				err = execute(dir, env, pnpmCommand(), "run", "build")
			case "website-default", "website-minimal":
				site := filepath.Join(dir, "frontend", "site")
				if err = execute(site, env, pnpmCommand(), "install", "--frozen-lockfile"); err != nil {
					return err
				}
				if err = execute(site, env, pnpmCommand(), "run", "build:ssr"); err != nil {
					return err
				}
				staticEnv := replaceEnv(env, "WEBSITE_STATIC_BUILD", "1")
				if err = execute(site, staticEnv, pnpmCommand(), "exec", "nuxt", "generate"); err != nil {
					return err
				}
				err = execute(site, staticEnv, "node", filepath.Join(site, "scripts", "copy-static.mjs"))
			}
			if err != nil {
				return err
			}
			if err = execute(dir, env, pnpmCommand(), "run", "package"); err != nil {
				return err
			}
			if err = copyDeliveryFile(filepath.Join(dir, "dist", id+".wasm"), filepath.Join(output, id+".wasm")); err != nil {
				return err
			}
			if id == "customer-default" {
				if err = copyDeliveryFile(filepath.Join(dir, "frontend", "server.mjs"), filepath.Join(output, "customer", "server.mjs")); err != nil {
					return err
				}
			}
			if id == "website-default" || id == "website-minimal" {
				if err = copyDeliveryTree(filepath.Join(dir, "frontend", "site", ".output"), filepath.Join(output, "websites", id, "renderer")); err != nil {
					return err
				}
				if err = copyDeliveryTree(filepath.Join(dir, "gateway"), filepath.Join(output, "websites", id, "gateway")); err != nil {
					return err
				}
			}
		default:
			if err = execute(filepath.Join(root, "hack", "tools", "linactl"), env, "go", "run", ".", "build", "dir="+dir, "--output-dir="+output); err != nil {
				return err
			}
		}
	}
	return nil
}

// pnpmCommand keeps executable resolution portable on Windows, Linux and macOS.
func pnpmCommand() string {
	if runtime.GOOS == "windows" {
		return "pnpm.cmd"
	}
	return "pnpm"
}

// copyDeliveryTree copies ordinary build inputs without following source symlinks.
func copyDeliveryTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("delivery input is a symlink: %s", path)
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".cache":
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(target, rel), 0o755)
		}
		return copyDeliveryFile(path, filepath.Join(target, rel))
	})
}

// copyDeliveryFile writes generated artifacts without overwriting source inputs.
func copyDeliveryFile(source, target string) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, content, 0o644)
}

// executionCommand invokes Windows command shims through the standard interpreter.
func executionCommand(platform, name string, args []string) (string, []string) {
	if platform == "windows" && (strings.HasSuffix(strings.ToLower(name), ".cmd") || strings.HasSuffix(strings.ToLower(name), ".bat")) {
		return "cmd.exe", append([]string{"/d", "/c", name}, args...)
	}
	return name, args
}

// checkPackedDeliveryAssets verifies official resource generation produced usable inputs.
func checkPackedDeliveryAssets(root string) error {
	required := []string{"config/metadata.yaml", "config/config.template.yaml", "sql/001-user-auth-bootstrap.sql", "i18n/en-US", "i18n/zh-CN"}
	base := filepath.Join(root, "apps", "lina-core", "internal", "packed", "manifest")
	missing := []string{}
	for _, name := range required {
		info, err := os.Stat(filepath.Join(base, filepath.FromSlash(name)))
		if errors.Is(err, os.ErrNotExist) {
			missing = append(missing, name)
			continue
		}
		if err != nil {
			return err
		}
		if info.IsDir() {
			entries, readErr := os.ReadDir(filepath.Join(base, filepath.FromSlash(name)))
			if readErr != nil {
				return readErr
			}
			if len(entries) == 0 {
				missing = append(missing, name)
			}
		}
	}
	if len(missing) > 0 {
		messagePath := filepath.Join(root, "apps", "lina-plugins", "idc-compliance", "manifest", "i18n", "zh-CN", "installation.json")
		content, readErr := os.ReadFile(messagePath)
		if readErr == nil {
			var messages struct {
				Installation struct {
					MissingPacked string `json:"missingPacked"`
				} `json:"installation"`
			}
			if json.Unmarshal(content, &messages) == nil && messages.Installation.MissingPacked != "" {
				fmt.Fprintln(os.Stderr, messages.Installation.MissingPacked)
			}
		}
		return fmt.Errorf("required host embedded resources are missing after official pack.assets: %s; restore manifest sources and retry; target=templates remains available", strings.Join(missing, ", "))
	}
	return nil
}

// developmentService describes one direct child; no shell or external process owner is used.
type developmentService struct {
	name, program, dir string
	args, env          []string
	port               int
}

// developmentPlan keeps parsing side-effect free, including plan=1 and invalid options.
func developmentPlan(root string, args []string, env []string) (string, bool, []developmentService, error) {
	ports := map[string]int{"backend_port": 6030, "frontend_port": 6031, "customer_port": 6032, "website_port": 6680}
	outputArgs := []string{}
	for _, arg := range args {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			return "", false, nil, fmt.Errorf("expected key=value: %s", arg)
		}
		if _, found := ports[key]; found {
			port, err := strconv.Atoi(value)
			if err != nil || port < 1 || port > 65535 {
				return "", false, nil, fmt.Errorf("invalid port for %s", key)
			}
			ports[key] = port
			continue
		}
		switch key {
		case "rebuild":
			if value != "0" && value != "1" {
				return "", false, nil, fmt.Errorf("rebuild must be 0 or 1")
			}
		case "out", "plan", "plugins":
			outputArgs = append(outputArgs, arg)
		default:
			return "", false, nil, fmt.Errorf("unsupported development option: %s", key)
		}
	}
	_, output, plan, err := deliveryOptions(root, outputArgs)
	if err != nil {
		return "", false, nil, err
	}
	if ports["website_port"] > 65533 {
		return "", false, nil, fmt.Errorf("website_port requires two following ports for template renderers")
	}
	backend := fmt.Sprintf("http://127.0.0.1:%d", ports["backend_port"])
	admin := fmt.Sprintf("http://127.0.0.1:%d", ports["frontend_port"])
	customer := fmt.Sprintf("http://localhost:%d", ports["customer_port"])
	website := fmt.Sprintf("http://localhost:%d", ports["website_port"])
	common := replaceEnv(env, "LINAPRO_IDC_CUSTOMER_PUBLIC_ORIGIN", customer)
	common = replaceEnv(common, "GF_PLUGIN_DYNAMIC_STORAGEPATH", output)
	binary := filepath.Join(output, "lina")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	backendEnv := replaceEnv(common, "LINAPRO_SERVER_ADDRESS", fmt.Sprintf(":%d", ports["backend_port"]))
	backendEnv = replaceEnv(backendEnv, "LINAPRO_FRONTEND_DEV_SERVER_URL", admin)
	adminEnv := replaceEnv(common, "ADMIN_PORT", strconv.Itoa(ports["frontend_port"]))
	adminEnv = replaceEnv(adminEnv, "LINAPRO_BACKEND_PROXY_TARGET", backend)
	customerEnv := replaceEnv(common, "CUSTOMER_PORT", strconv.Itoa(ports["customer_port"]))
	customerEnv = replaceEnv(customerEnv, "WEBSITE_PUBLIC_ORIGIN", website)
	customerEnv = replaceEnv(customerEnv, "LINAPRO_BACKEND_PROXY_TARGET", backend)
	services := []developmentService{
		{name: "Backend", program: binary, dir: output, env: backendEnv, port: ports["backend_port"]},
		{name: "Admin", program: "node", args: []string{filepath.Join(output, "admin", "server.mjs")}, dir: output, env: adminEnv, port: ports["frontend_port"]},
		{name: "Customer", program: "node", args: []string{filepath.Join(output, "customer", "server.mjs")}, dir: output, env: customerEnv, port: ports["customer_port"]},
	}
	origins := make(map[string]string)
	for index, id := range []string{"website-default", "website-minimal"} {
		port := ports["website_port"] + index + 1
		origins[id] = fmt.Sprintf("http://127.0.0.1:%d", port)
		renderEnv := replaceEnv(common, "PORT", strconv.Itoa(port))
		renderEnv = replaceEnv(renderEnv, "HOST", "127.0.0.1")
		renderEnv = replaceEnv(renderEnv, "NUXT_API_ORIGIN", backend)
		renderEnv = replaceEnv(renderEnv, "NUXT_PUBLIC_CUSTOMER_URL", customer)
		renderEnv = replaceEnv(renderEnv, "NUXT_PUBLIC_SITE_URL", website)
		services = append(services, developmentService{name: id + " renderer", program: "node", args: []string{filepath.Join(output, "websites", id, "renderer", "server", "index.mjs")}, dir: output, env: renderEnv, port: port})
	}
	mapping, err := json.Marshal(origins)
	if err != nil {
		return "", false, nil, err
	}
	websiteEnv := replaceEnv(common, "PORT", strconv.Itoa(ports["website_port"]))
	websiteEnv = replaceEnv(websiteEnv, "HOST", "127.0.0.1")
	websiteEnv = replaceEnv(websiteEnv, "WEBSITE_API_ORIGIN", backend)
	websiteEnv = replaceEnv(websiteEnv, "WEBSITE_CUSTOMER_URL", customer)
	websiteEnv = replaceEnv(websiteEnv, "WEBSITE_SSR_ORIGINS", string(mapping))
	services = append(services, developmentService{name: "Website", program: "node", args: []string{filepath.Join(output, "websites", "website-default", "gateway", "server.mjs")}, dir: output, env: websiteEnv, port: ports["website_port"]})
	seen := make(map[int]bool)
	for _, service := range services {
		if seen[service.port] {
			return "", false, nil, fmt.Errorf("development services cannot share port %d", service.port)
		}
		seen[service.port] = true
	}
	return output, plan, services, nil
}

// checkDevelopmentPorts refuses occupied ports without stopping unrelated instances.
func checkDevelopmentPorts(services []developmentService) error {
	listeners := make([]net.Listener, 0, len(services))
	defer func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}()
	for _, service := range services {
		listenConfig := net.ListenConfig{}
		listener, err := listenConfig.Listen(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", service.port))
		if err != nil {
			return fmt.Errorf("%s port %d is unavailable; existing processes were not changed: %w", service.name, service.port, err)
		}
		listeners = append(listeners, listener)
	}
	return nil
}

// runDevelopment builds and supervises only this invocation's direct child processes.
func runDevelopment(root, state string, args []string) error {
	output, plan, services, err := developmentPlan(root, args, os.Environ())
	if err != nil {
		return err
	}
	if plan {
		fmt.Println("Development delivery:", output)
		for _, service := range services {
			fmt.Printf("%s: http://127.0.0.1:%d\n", service.name, service.port)
		}
		return nil
	}
	if err = checkDevelopmentPorts(services); err != nil {
		return err
	}
	if !contains(args, "rebuild=0") {
		if err = buildDelivery(root, state, []string{"plugins=1", "out=" + output}); err != nil {
			return err
		}
	}
	// Refuse an incomplete delivery before starting any service, including reuse mode.
	for _, service := range services {
		entry := service.program
		if len(service.args) > 0 {
			entry = service.args[0]
		}
		if info, statErr := os.Stat(entry); statErr != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("%s delivery entry is unavailable: %s; run make dev to rebuild", service.name, entry)
		}
	}
	env, err := prepare(root, state, os.Environ())
	if err != nil {
		return err
	}
	_, _, services, err = developmentPlan(root, args, env)
	if err != nil {
		return err
	}
	if err = checkDevelopmentPorts(services); err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return superviseDevelopment(ctx, services)
}

// superviseDevelopment owns every child, restarts failed services independently, and reaps all on exit.
func superviseDevelopment(ctx context.Context, services []developmentService) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan error, len(services))
	started := 0
	defer func() {
		cancel()
		for started > 0 {
			<-results
			started--
		}
	}()
	for _, service := range services {
		command, err := startDevelopmentService(ctx, service)
		if err != nil {
			return fmt.Errorf("start %s: %w", service.name, err)
		}
		started++
		go func(service developmentService, command *exec.Cmd) {
			restartDevelopmentService(ctx, service, command)
			results <- nil
		}(service, command)
		if service.name == "Backend" {
			consumed, err := waitDevelopmentBackend(ctx, service.port, results)
			if consumed {
				started--
			}
			if err != nil {
				return err
			}
		}
		fmt.Printf("%s: http://127.0.0.1:%d\n", service.name, service.port)
	}
	select {
	case <-ctx.Done():
		return nil
	case err := <-results:
		started--
		return err
	}
}

// startDevelopmentService launches a direct child with the same explicit runtime inputs.
func startDevelopmentService(ctx context.Context, service developmentService) (*exec.Cmd, error) {
	program, args := executionCommand(runtime.GOOS, service.program, service.args)
	command := exec.CommandContext(ctx, program, args...)
	command.Dir, command.Env = service.dir, service.env
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	command.WaitDelay = 5 * time.Second
	if err := command.Start(); err != nil {
		return nil, err
	}
	return command, nil
}

// restartDevelopmentService isolates exits and retries with a cancellable, capped backoff.
// A stable minute resets the delay. Cancellation never launches a replacement child.
func restartDevelopmentService(ctx context.Context, service developmentService, command *exec.Cmd) {
	delay := time.Second
	for {
		started := time.Now()
		err := command.Wait()
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			err = fmt.Errorf("service exited")
		}
		if time.Since(started) >= time.Minute {
			delay = time.Second
		}
		for {
			fmt.Fprintf(os.Stderr, "%s stopped: %v; restarting in %s\n", service.name, err, delay)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			if ctx.Err() != nil {
				return
			}
			command, err = startDevelopmentService(ctx, service)
			delay = min(delay*2, 30*time.Second)
			if err == nil {
				fmt.Fprintf(os.Stderr, "%s restarted (PID %d)\n", service.name, command.Process.Pid)
				break
			}
		}
	}
}

// waitDevelopmentBackend requires the real HTTP API before starting dependent entries.
func waitDevelopmentBackend(ctx context.Context, port int, results <-chan error) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 2 * time.Second}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/api.json", port), nil)
		if err != nil {
			return false, err
		}
		response, err := client.Do(request)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return false, nil
			}
		}
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("backend HTTP API did not become ready: %w", ctx.Err())
		case err := <-results:
			return true, err
		case <-ticker.C:
		}
	}
}
