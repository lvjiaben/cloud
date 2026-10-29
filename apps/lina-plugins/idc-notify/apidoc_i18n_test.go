package idcnotify

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestAPIDocI18NCoversNotificationSourceMetadata(t *testing.T) {
	var english map[string]string
	readEmbeddedJSON(t, "manifest/i18n/en-US/apidoc/index.json", &english)
	if len(english) != 0 {
		t.Fatalf("English API-document bundle must remain an empty source-text placeholder")
	}

	chinese := make(map[string]string)
	catalogs, err := fs.Glob(EmbeddedFiles, "manifest/i18n/zh-CN/apidoc/*.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range catalogs {
		var catalog map[string]string
		readEmbeddedJSON(t, path, &catalog)
		for key, value := range catalog {
			chinese[key] = value
		}
	}
	required := notificationAPIDocKeys(t)
	var missing []string
	for _, key := range required {
		if strings.TrimSpace(chinese[key]) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("notification Chinese API-document catalog is incomplete:\n%s", strings.Join(missing, "\n"))
	}
}

func TestNotificationAPIContractMetadataIsComplete(t *testing.T) {
	var violations []string
	err := fs.WalkDir(os.DirFS("."), "backend/api", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		content, err := fs.ReadFile(os.DirFS("."), path)
		if err != nil {
			return err
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, content, 0)
		if err != nil {
			return err
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			typeSpec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			structure, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return false
			}
			for _, field := range structure.Fields.List {
				if field.Tag == nil {
					continue
				}
				raw, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					continue
				}
				tag := reflect.StructTag(raw)
				location := path + ":" + typeSpec.Name.Name
				if tag.Get("path") != "" {
					if strings.TrimSpace(tag.Get("dc")) == "" {
						violations = append(violations, location+" g.Meta is missing dc")
					}
					continue
				}
				jsonName := strings.Split(tag.Get("json"), ",")[0]
				if jsonName == "" || jsonName == "-" {
					continue
				}
				if strings.TrimSpace(tag.Get("dc")) == "" {
					violations = append(violations, location+" field "+jsonName+" is missing dc")
				}
				if strings.TrimSpace(tag.Get("eg")) == "" {
					violations = append(violations, location+" field "+jsonName+" is missing eg")
				}
				if strings.HasSuffix(jsonName, "At") && !strings.Contains(tag.Get("dc"), "Unix timestamp in milliseconds") {
					violations = append(violations, location+" time field "+jsonName+" does not declare Unix milliseconds")
				}
			}
			return false
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan notification API metadata: %v", err)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("notification API contract metadata is incomplete:\n%s", strings.Join(violations, "\n"))
	}
}

func readEmbeddedJSON(t *testing.T, path string, target any) {
	t.Helper()
	content, err := fs.ReadFile(EmbeddedFiles, path)
	if err != nil {
		t.Fatalf("read embedded JSON %s: %v", path, err)
	}
	if err = json.Unmarshal(content, target); err != nil {
		t.Fatalf("parse embedded JSON %s: %v", path, err)
	}
}

func notificationAPIDocKeys(t *testing.T) []string {
	t.Helper()
	set := make(map[string]struct{})
	sources := os.DirFS(".")
	err := fs.WalkDir(sources, "backend/api", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		content, err := fs.ReadFile(sources, path)
		if err != nil {
			return err
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, content, 0)
		if err != nil {
			return err
		}
		packagePath := strings.TrimSuffix(strings.TrimPrefix(filepath.ToSlash(path), "backend/"), "/"+filepath.Base(path))
		prefix := "plugins.idc_notify." + strings.ReplaceAll(packagePath, "/", ".")
		ast.Inspect(parsed, func(node ast.Node) bool {
			typeSpec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			structure, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return false
			}
			component := prefix + "." + typeSpec.Name.Name
			for _, field := range structure.Fields.List {
				if field.Tag == nil {
					continue
				}
				raw, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					continue
				}
				tag := reflect.StructTag(raw)
				if strings.TrimSpace(tag.Get("tags")) != "" {
					set[component+".meta.tags"] = struct{}{}
				}
				if strings.TrimSpace(tag.Get("summary")) != "" {
					set[component+".meta.summary"] = struct{}{}
				}
				if strings.TrimSpace(tag.Get("dc")) == "" && strings.TrimSpace(tag.Get("description")) == "" {
					continue
				}
				name := strings.Split(tag.Get("json"), ",")[0]
				if name == "" {
					name = strings.Split(tag.Get("p"), ",")[0]
				}
				if name != "" && name != "-" {
					set[component+".fields."+name+".dc"] = struct{}{}
				}
			}
			return false
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan embedded notification API metadata: %v", err)
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
