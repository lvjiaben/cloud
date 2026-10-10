// This file enforces the IDC management permission namespace across all suite plugins.

package backend

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/idccap"
)

var (
	menuPermsPattern         = regexp.MustCompile(`(?m)^\s*perms:\s*["']?([^\r\n"']+)["']?\s*$`)
	dynamicPluginTypePattern = regexp.MustCompile(`(?m)^\s*type:\s*["']?dynamic["']?\s*$`)
)

type apiMetaInspection struct {
	permissionTags []string
	violations     []string
}

// TestIDCPluginAPIPermissionsUseAdminNamespace rejects an unprotected source
// plugin admin API and requires the API, assignable menu/button, and static
// catalog sets to remain identical. Dynamic plugins use their mandatory
// plugin-local permission namespace instead. Customer/public APIs intentionally
// have no admin tag.
func TestIDCPluginAPIPermissionsUseAdminNamespace(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file failed")
	}
	pluginsRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	pluginDirs, err := filepath.Glob(filepath.Join(pluginsRoot, "idc-*"))
	if err != nil {
		t.Fatalf("enumerate IDC plugins failed: %v", err)
	}
	apiReferenced := make(map[string]string)
	menuReferenced := make(map[string]string)
	for _, pluginDir := range pluginDirs {
		manifestPath := filepath.Join(pluginDir, "plugin.yaml")
		manifestContent, readErr := os.ReadFile(manifestPath)
		if readErr != nil {
			t.Fatalf("read %s failed: %v", manifestPath, readErr)
		}
		if dynamicPluginTypePattern.Match(manifestContent) {
			continue
		}
		apiDir := filepath.Join(pluginDir, "backend", "api")
		if _, err := os.Stat(apiDir); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatalf("inspect %s failed: %v", apiDir, err)
		}
		err = filepath.WalkDir(apiDir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			inspection := inspectIDCAPIFile(path, content)
			for _, violation := range inspection.violations {
				t.Errorf("%s: %s", path, violation)
			}
			for _, raw := range inspection.permissionTags {
				recordIDCAdminPermissions(t, path, raw, apiReferenced)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s failed: %v", apiDir, err)
		}

		for _, match := range menuPermsPattern.FindAllSubmatch(manifestContent, -1) {
			recordIDCAdminPermissions(t, manifestPath, string(match[1]), menuReferenced)
		}
	}

	for permission, path := range apiReferenced {
		if _, ok := menuReferenced[permission]; !ok {
			t.Errorf("%s uses IDC admin permission %q that is not assignable by any menu/button", path, permission)
		}
	}
	for permission, path := range menuReferenced {
		if _, ok := apiReferenced[permission]; !ok {
			t.Errorf("%s exposes IDC admin permission %q without a protected API", path, permission)
		}
	}
	for _, permission := range idccap.AdminPermissionCatalog() {
		if _, ok := apiReferenced[permission]; !ok {
			t.Errorf("cataloged IDC permission %q is not referenced by an admin API", permission)
		}
		if _, ok := menuReferenced[permission]; !ok {
			t.Errorf("cataloged IDC permission %q is not assignable by a menu/button", permission)
		}
	}
}

// TestInspectIDCAPIFileClassifiesAdminContracts proves a management request
// cannot evade the permission catalog by omitting a g.Meta permission tag.
func TestInspectIDCAPIFileClassifiesAdminContracts(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		meta           string
		wantViolation  bool
		wantPermission string
	}{
		{
			name:          "admin missing permission",
			path:          "backend/api/admin/v1/customer.go",
			meta:          `path:"/admin/customers" method:"get"`,
			wantViolation: true,
		},
		{
			name:          "admin path and directory must agree",
			path:          "backend/api/account/v1/customer.go",
			meta:          `path:"/admin/customers" method:"get" permission:"idc:admin:customer:list"`,
			wantViolation: true,
		},
		{
			name:           "directory resource is explicitly registered",
			path:           "backend/api/directoryadmin/v1/directory.go",
			meta:           `path:"/admin/product-directories" method:"get" permission:"idc:admin:product-directory:list"`,
			wantPermission: "idc:admin:product-directory:list",
		},
		{
			name:           "server resource is explicitly registered",
			path:           "backend/api/adminserver/v1/server_list.go",
			meta:           `path:"/admin/resource/zjmfcloud-servers" method:"get" permission:"idc:admin:resource:server-batch:view"`,
			wantPermission: "idc:admin:resource:server-batch:view",
		},
		{
			name:          "registered resource cannot contain customer route",
			path:          "backend/api/adminserver/v1/server_list.go",
			meta:          `path:"/account/servers" method:"get" permission:"idc:admin:resource:server-batch:view"`,
			wantViolation: true,
		},
		{
			name:          "registered resource cannot omit permission",
			path:          "backend/api/directoryadmin/v1/directory.go",
			meta:          `path:"/admin/product-directories" method:"get"`,
			wantViolation: true,
		},
		{
			name:          "admin prefix is not an implicit registration",
			path:          "backend/api/adminunregistered/v1/resource.go",
			meta:          `path:"/admin/resource" method:"get" permission:"idc:admin:resource:server-batch:view"`,
			wantViolation: true,
		},
		{
			name: "public contract has no admin permission",
			path: "backend/api/account/v1/profile.go",
			meta: `path:"/account/profile" method:"get"`,
		},
		{
			name:           "valid admin contract",
			path:           "backend/api/admin/v1/customer.go",
			meta:           `path:"/admin/customers" method:"get" permission:"idc:admin:customer:list"`,
			wantPermission: "idc:admin:customer:list",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content := []byte("package v1\nimport \"github.com/gogf/gf/v2/frame/g\"\ntype Request struct { g.Meta `" + test.meta + "` }\n")
			inspection := inspectIDCAPIFile(test.path, content)
			if (len(inspection.violations) > 0) != test.wantViolation {
				t.Fatalf("violations=%v, wantViolation=%v", inspection.violations, test.wantViolation)
			}
			if test.wantPermission != "" &&
				(len(inspection.permissionTags) != 1 || inspection.permissionTags[0] != test.wantPermission) {
				t.Fatalf("permissions=%v, want %q", inspection.permissionTags, test.wantPermission)
			}
		})
	}
}

// inspectIDCAPIFile parses g.Meta declarations and enforces that management
// contracts use an explicitly registered admin API directory and /admin routing, with exactly one
// non-empty permission attribute. This makes the admin classification explicit.
func inspectIDCAPIFile(path string, content []byte) apiMetaInspection {
	inspection := apiMetaInspection{}
	parsed, err := parser.ParseFile(token.NewFileSet(), path, content, 0)
	if err != nil {
		inspection.violations = append(inspection.violations, "parse API source: "+err.Error())
		return inspection
	}
	adminFile := isAdminAPIFile(path)
	ast.Inspect(parsed, func(node ast.Node) bool {
		structType, ok := node.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range structType.Fields.List {
			selector, ok := field.Type.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Meta" {
				continue
			}
			if field.Tag == nil {
				if adminFile {
					inspection.violations = append(inspection.violations, "admin g.Meta is missing its struct tag")
				}
				continue
			}
			tagValue, err := strconv.Unquote(field.Tag.Value)
			if err != nil {
				inspection.violations = append(inspection.violations, "decode g.Meta struct tag: "+err.Error())
				continue
			}
			tag := reflect.StructTag(tagValue)
			route := strings.TrimSpace(tag.Get("path"))
			adminRoute := route == "/admin" || strings.HasPrefix(route, "/admin/")
			isAdmin := adminFile || adminRoute
			if adminFile != adminRoute {
				inspection.violations = append(inspection.violations, "admin API must use a registered admin directory and an /admin route")
			}
			permission, hasPermission := tag.Lookup("permission")
			permissionCount := strings.Count(tagValue, `permission:"`)
			if isAdmin && (!hasPermission || strings.TrimSpace(permission) == "" || permissionCount != 1) {
				inspection.violations = append(inspection.violations, "admin g.Meta must declare exactly one non-empty permission attribute")
			}
			if !isAdmin && hasPermission {
				inspection.violations = append(inspection.violations, "customer/public g.Meta must not declare an IDC admin permission")
			}
			if hasPermission && strings.TrimSpace(permission) != "" {
				inspection.permissionTags = append(inspection.permissionTags, permission)
			}
		}
		return true
	})
	return inspection
}

func isAdminAPIFile(path string) bool {
	normalized := "/" + strings.Trim(filepath.ToSlash(path), "/") + "/"
	// Resource-specific namespaces keep generated controllers independent while
	// retaining an explicit, closed set of management API directories.
	for _, directory := range []string{"admin", "directoryadmin", "adminserver"} {
		if strings.Contains(normalized, "/backend/api/"+directory+"/") {
			return true
		}
	}
	return false
}

// recordIDCAdminPermissions validates comma-separated static permission tags
// and records their source for API/menu/catalog equality checks.
func recordIDCAdminPermissions(t *testing.T, path string, raw string, referenced map[string]string) {
	t.Helper()
	permissions := strings.Split(raw, ",")
	for _, permission := range permissions {
		if !idccap.IsAdminPermission(permission) {
			t.Errorf("%s declares IDC permission %q outside %s*", path, permission, idccap.AdminPermissionPrefix)
			continue
		}
		if !idccap.IsCatalogedAdminPermission(permission) {
			t.Errorf("%s declares uncataloged IDC permission %q", path, permission)
		}
		referenced[permission] = path
	}
}
