package tool

import "testing"

func schemaProps(t *testing.T, params map[string]interface{}) map[string]interface{} {
	t.Helper()
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties missing")
	}
	return props
}

func TestGlobSchemaProvider(t *testing.T) {
	var _ SchemaProvider = GlobTool{}
	props := schemaProps(t, GlobTool{}.Parameters())
	if props["pattern"].(map[string]interface{})["type"] != "string" {
		t.Fatal("pattern type wrong")
	}
	if props["path"].(map[string]interface{})["type"] != "string" {
		t.Fatal("path type wrong")
	}
	req, _ := GlobTool{}.Parameters()["required"].([]string)
	if len(req) != 1 || req[0] != "pattern" {
		t.Fatalf("required = %v, want [pattern]", GlobTool{}.Parameters()["required"])
	}
}

func TestLSSchemaProvider(t *testing.T) {
	var _ SchemaProvider = LSTool{}
	props := schemaProps(t, LSTool{}.Parameters())
	ignore, ok := props["ignore"].(map[string]interface{})
	if !ok || ignore["type"] != "array" {
		t.Fatalf("ignore prop = %v, want array type", props["ignore"])
	}
	items, ok := ignore["items"].(map[string]interface{})
	if !ok || items["type"] != "string" {
		t.Fatalf("ignore items = %v, want string type", ignore["items"])
	}
	_, hasRequired := LSTool{}.Parameters()["required"]
	if hasRequired {
		t.Fatalf("LS has no required fields, got %v", LSTool{}.Parameters()["required"])
	}
}

func TestGrepSchemaProvider(t *testing.T) {
	var _ SchemaProvider = GrepTool{}
	props := schemaProps(t, GrepTool{}.Parameters())
	for _, f := range []string{"pattern", "path", "include"} {
		if props[f].(map[string]interface{})["type"] != "string" {
			t.Fatalf("%s type wrong", f)
		}
	}
	req, _ := GrepTool{}.Parameters()["required"].([]string)
	if len(req) != 1 || req[0] != "pattern" {
		t.Fatalf("required = %v, want [pattern]", GrepTool{}.Parameters()["required"])
	}
}

func TestSkillSchemaProvider(t *testing.T) {
	var _ SchemaProvider = SkillTool{}
	props := schemaProps(t, SkillTool{}.Parameters())
	if props["skill"].(map[string]interface{})["type"] != "string" {
		t.Fatal("skill type wrong")
	}
	_, hasRequired := SkillTool{}.Parameters()["required"]
	if hasRequired {
		t.Fatalf("Skill has no required fields, got %v", SkillTool{}.Parameters()["required"])
	}
}

func TestScreenshotSchemaProvider(t *testing.T) {
	var _ SchemaProvider = ScreenshotTool{}
	props := schemaProps(t, ScreenshotTool{}.Parameters())
	if props["url"].(map[string]interface{})["type"] != "string" {
		t.Fatal("url type wrong")
	}
	if props["width"].(map[string]interface{})["type"] != "number" {
		t.Fatal("width type wrong")
	}
	req, _ := ScreenshotTool{}.Parameters()["required"].([]string)
	if len(req) != 1 || req[0] != "url" {
		t.Fatalf("required = %v, want [url]", ScreenshotTool{}.Parameters()["required"])
	}
}
