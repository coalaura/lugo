package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type snapshot struct {
	Bundles []bundle `json:"bundles"`
}

type bundle struct {
	Name        string   `json:"name"`
	Input       string   `json:"input"`
	Family      string   `json:"family"`
	Runtime     string   `json:"runtime"`
	Description string   `json:"description"`
	Natives     []native `json:"natives"`
}

type native struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Params      []param `json:"params"`
	Returns     string  `json:"returns"`
}

type param struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fatal(err)
	}

	inputPath := filepath.Join(root, "scripts", "fivem_native_catalog", "snapshot", "catalog.json")
	b, err := os.ReadFile(inputPath)
	if err != nil {
		fatal(err)
	}

	var snap snapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		fatal(err)
	}

	sort.Slice(snap.Bundles, func(i, j int) bool { return snap.Bundles[i].Name < snap.Bundles[j].Name })

	outputDir := filepath.Join(root, "lsp", "stdlib", "fivem")
	for _, bundle := range snap.Bundles {
		sort.Slice(bundle.Natives, func(i, j int) bool { return bundle.Natives[i].Name < bundle.Natives[j].Name })

		content := renderBundle(bundle)
		outputPath := filepath.Join(outputDir, bundle.Name)
		if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
			fatal(err)
		}
	}
}

func renderBundle(bundle bundle) string {
	var sb strings.Builder
	sb.WriteString("---@meta\n\n")
	sb.WriteString(fmt.Sprintf("---Generated from the frozen FiveM native wrapper snapshot for `%s`.\n", bundle.Name))
	sb.WriteString(fmt.Sprintf("---Upstream-equivalent input: `%s` (%s %s subset).\n", bundle.Input, bundle.Family, bundle.Runtime))
	if bundle.Description != "" {
		sb.WriteString("---")
		sb.WriteString(bundle.Description)
		sb.WriteString("\n")
	}

	for i, native := range bundle.Natives {
		if i > 0 {
			sb.WriteString("\n")
		}

		sb.WriteString("---")
		sb.WriteString(native.Description)
		sb.WriteString("\n")
		for _, param := range native.Params {
			sb.WriteString(fmt.Sprintf("---@param %s %s\n", param.Name, param.Type))
		}
		if native.Returns != "" {
			sb.WriteString(fmt.Sprintf("---@return %s\n", native.Returns))
		}
		sb.WriteString("function ")
		sb.WriteString(native.Name)
		sb.WriteByte('(')
		for idx, param := range native.Params {
			if idx > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(param.Name)
		}
		sb.WriteString(") end\n")
	}

	return sb.String()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
