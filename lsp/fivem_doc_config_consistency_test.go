package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFiveMDocConfigConsistency(t *testing.T) {
	readme := mustReadRepoFile(t, "README.md")
	for _, needle := range []string{
		"lugo.fivem.enabled",
		"featureFiveM",
		"diagFiveMUnaccountedFile",
		"diagFiveMUnknownExport",
		"diagFiveMUnknownResource",
		"Manifest authoring:",
		"Callable proxies & bridge metadata:",
		"There is no separate FiveM setting for native bundle selection.",
	} {
		if !strings.Contains(readme, needle) {
			t.Fatalf("README.md missing %q", needle)
		}
	}

	settings := mustReadVSCodeSettings(t)
	assertVSCodeFiveMSetting(t, settings, "lugo.fivem.enabled", false,
		"manifest authoring", "profile-scoped runtime globals", "native helper selection")
	assertVSCodeFiveMSetting(t, settings, "lugo.fivem.diagnostics.unaccountedFile", true,
		"resource root", "plain Lua")
	assertVSCodeFiveMSetting(t, settings, "lugo.fivem.diagnostics.unknownExport", true,
		"missing exports", "`exports` bridge")
	assertVSCodeFiveMSetting(t, settings, "lugo.fivem.diagnostics.unknownResource", true,
		"unknown resource names", "`exports` bridge")

	ciYAML := mustReadRepoFile(t, "example.ci.yml")
	if !strings.Contains(ciYAML, "--ci example.ci.json") {
		t.Fatalf("example.ci.yml missing CI config invocation: %q", ciYAML)
	}
	if !strings.Contains(ciYAML, "example.ci.json") {
		t.Fatalf("example.ci.yml missing example.ci.json reference")
	}
	if !strings.Contains(readme, "[**`example.ci.json`**](example.ci.json)") {
		t.Fatalf("README.md missing example.ci.json link")
	}
}

type vscodePackage struct {
	Contributes struct {
		Configuration []struct {
			Properties map[string]struct {
				Default             any    `json:"default"`
				MarkdownDescription string `json:"markdownDescription"`
			} `json:"properties"`
		} `json:"configuration"`
	} `json:"contributes"`
}

type vscodeSetting struct {
	Default             any
	MarkdownDescription string
}

func mustReadVSCodeSettings(t *testing.T) map[string]vscodeSetting {
	t.Helper()

	var pkg vscodePackage
	if err := json.Unmarshal([]byte(mustReadRepoFile(t, filepath.Join("vscode", "package.json"))), &pkg); err != nil {
		t.Fatalf("unmarshal vscode/package.json: %v", err)
	}

	settings := map[string]vscodeSetting{}
	for _, block := range pkg.Contributes.Configuration {
		for key, value := range block.Properties {
			settings[key] = vscodeSetting{
				Default:             value.Default,
				MarkdownDescription: value.MarkdownDescription,
			}
		}
	}

	return settings
}

func assertVSCodeFiveMSetting(t *testing.T, settings map[string]vscodeSetting, key string, wantDefault bool, wantDesc ...string) {
	t.Helper()

	setting, ok := settings[key]
	if !ok {
		t.Fatalf("vscode/package.json missing %s", key)
	}

	gotDefault, ok := setting.Default.(bool)
	if !ok {
		t.Fatalf("%s default type = %T, want bool", key, setting.Default)
	}
	if gotDefault != wantDefault {
		t.Fatalf("%s default = %v, want %v", key, gotDefault, wantDefault)
	}

	for _, needle := range wantDesc {
		if !strings.Contains(setting.MarkdownDescription, needle) {
			t.Fatalf("%s markdownDescription = %q, want substring %q", key, setting.MarkdownDescription, needle)
		}
	}
}

func mustReadRepoFile(t *testing.T, parts ...string) string {
	t.Helper()

	path := filepath.Join(append([]string{".."}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}
