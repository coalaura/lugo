package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	gtaNativesURL  = "https://static.cfx.re/natives/natives.json"
	cfxNativesURL  = "https://static.cfx.re/natives/natives_cfx.json"
	rdr3NativesURL = "https://raw.githubusercontent.com/VORPCORE/RDR3natives/main/rdr3natives.json"

	fiveMDocsURL = "https://docs.fivem.net/natives/?_"
	rdr3DocsURL  = "https://rdr3natives.com/?_"
)

type sourceCatalog map[string]map[string]sourceNative

type sourceNative struct {
	Name        string            `json:"name"`
	Params      []sourceParam     `json:"params"`
	Results     string            `json:"results"`
	ReturnType  string            `json:"return_type"`
	Description string            `json:"description"`
	Comment     string            `json:"comment"`
	Examples    []json.RawMessage `json:"examples"`
	Hash        string            `json:"hash"`
	Namespace   string            `json:"ns"`
	APISet      string            `json:"apiset"`
	Game        string            `json:"game"`
	Aliases     []string          `json:"aliases"`
	OldNames    []string          `json:"old_names"`
}

type sourceParam struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type generatedBundle struct {
	Name        string
	Input       string
	Family      string
	Runtime     string
	Description string
	Natives     []generatedNative
	Fallback    bool
}

type generatedNative struct {
	Name        string
	Description string
	Params      []generatedParam
	Returns     []string
	Aliases     []string
}

type generatedParam struct {
	Name string
	Type string
}

type legacySnapshot struct {
	Bundles []legacyBundle `json:"bundles"`
}

type legacyBundle struct {
	Name        string         `json:"name"`
	Input       string         `json:"input"`
	Family      string         `json:"family"`
	Runtime     string         `json:"runtime"`
	Description string         `json:"description"`
	Natives     []legacyNative `json:"natives"`
}

type legacyNative struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Params      []legacyParam `json:"params"`
	Returns     string        `json:"returns"`
}

type legacyParam struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fatal(err)
	}

	client := &http.Client{Timeout: 2 * time.Minute}

	gtaCatalog, err := fetchCatalog(client, gtaNativesURL)
	if err != nil {
		fatal(err)
	}

	cfxCatalog, err := fetchCatalog(client, cfxNativesURL)
	if err != nil {
		fatal(err)
	}

	rdr3Catalog, err := fetchCatalog(client, rdr3NativesURL)
	if err != nil {
		fatal(err)
	}

	bundles := []generatedBundle{
		buildGTABundle("natives_21e43a33.lua", gtaCatalog, cfxCatalog),
		buildGTABundle("natives_0193d0af.lua", gtaCatalog, cfxCatalog),
		buildGTABundle("natives_universal.lua", gtaCatalog, cfxCatalog),
		buildRDR3Bundle(rdr3Catalog, cfxCatalog),
		buildServerBundle(cfxCatalog),
	}

	legacyPath := filepath.Join(root, "scripts", "fivem_native_catalog", "snapshot", "catalog.json")
	nyBundle, err := buildNYBundleFromLegacySnapshot(legacyPath)
	if err != nil {
		fatal(err)
	}
	bundles = append(bundles, nyBundle)

	sort.Slice(bundles, func(i, j int) bool { return bundles[i].Name < bundles[j].Name })

	outputDir := filepath.Join(root, "lsp", "stdlib", "fivem")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fatal(err)
	}

	for _, bundle := range bundles {
		sort.Slice(bundle.Natives, func(i, j int) bool { return bundle.Natives[i].Name < bundle.Natives[j].Name })
		content := renderBundle(bundle)
		outputPath := filepath.Join(outputDir, bundle.Name)
		if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
			fatal(err)
		}
	}
}

func fetchCatalog(client *http.Client, url string) (sourceCatalog, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "lugo-fivem-native-catalog/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: unexpected status %s", url, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}

	var catalog sourceCatalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, fmt.Errorf("decode %s: %w", url, err)
	}
	return catalog, nil
}

func buildGTABundle(name string, gtaCatalog, cfxCatalog sourceCatalog) generatedBundle {
	bundle := generatedBundle{
		Name:        name,
		Input:       "natives.json + natives_cfx.json",
		Family:      "gta5",
		Runtime:     "client",
		Description: "Generated from live FiveM GTA V native metadata plus compatible CFX client/shared natives.",
	}
	bundle.Natives = mergeGeneratedNatives(
		collectGeneratedNatives(gtaCatalog, fiveMDocsURL, func(sourceNative) bool { return true }),
		collectGeneratedNatives(cfxCatalog, fiveMDocsURL, func(n sourceNative) bool {
			if !matchesGame(n.Game, "gta5") {
				return false
			}
			return allowsClientAPISet(n.APISet)
		}),
	)
	return bundle
}

func buildRDR3Bundle(rdr3Catalog, cfxCatalog sourceCatalog) generatedBundle {
	bundle := generatedBundle{
		Name:        "rdr3_universal.lua",
		Input:       "rdr3natives.json + natives_cfx.json",
		Family:      "rdr3",
		Runtime:     "client",
		Description: "Generated from live RDR3 native metadata plus compatible CFX client/shared natives.",
	}
	bundle.Natives = mergeGeneratedNatives(
		collectGeneratedNatives(rdr3Catalog, rdr3DocsURL, func(sourceNative) bool { return true }),
		collectGeneratedNatives(cfxCatalog, fiveMDocsURL, func(n sourceNative) bool {
			if !matchesGame(n.Game, "rdr3") {
				return false
			}
			return allowsClientAPISet(n.APISet)
		}),
	)
	return bundle
}

func buildServerBundle(cfxCatalog sourceCatalog) generatedBundle {
	bundle := generatedBundle{
		Name:        "natives_server.lua",
		Input:       "natives_cfx.json",
		Family:      "server",
		Runtime:     "server",
		Description: "Generated from live CFX server/shared native metadata.",
	}
	bundle.Natives = collectGeneratedNatives(cfxCatalog, fiveMDocsURL, func(n sourceNative) bool {
		return allowsServerAPISet(n.APISet)
	})
	return bundle
}

func buildNYBundleFromLegacySnapshot(snapshotPath string) (generatedBundle, error) {
	b, err := os.ReadFile(snapshotPath)
	if err != nil {
		return generatedBundle{}, fmt.Errorf("read fallback snapshot: %w", err)
	}

	var snap legacySnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return generatedBundle{}, fmt.Errorf("decode fallback snapshot: %w", err)
	}

	for _, bundle := range snap.Bundles {
		if bundle.Name != "ny_universal.lua" {
			continue
		}
		out := generatedBundle{
			Name:        bundle.Name,
			Input:       bundle.Input,
			Family:      bundle.Family,
			Runtime:     bundle.Runtime,
			Description: "Generated from the legacy Lugo native snapshot because rage-lua-natives does not expose a dedicated NY source.",
			Fallback:    true,
		}
		for _, native := range bundle.Natives {
			params := make([]generatedParam, 0, len(native.Params))
			for _, param := range native.Params {
				params = append(params, generatedParam{Name: fieldToReplace(param.Name), Type: param.Type})
			}
			returns := []string{}
			if native.Returns != "" && native.Returns != "void" {
				returns = append(returns, native.Returns)
			}
			out.Natives = append(out.Natives, generatedNative{
				Name:        native.Name,
				Description: "---" + native.Description,
				Params:      params,
				Returns:     returns,
			})
		}
		return out, nil
	}

	return generatedBundle{}, fmt.Errorf("fallback snapshot missing ny_universal.lua")
}

func collectGeneratedNatives(catalog sourceCatalog, docsBase string, include func(sourceNative) bool) []generatedNative {
	result := make([]generatedNative, 0)
	for namespace, natives := range catalog {
		for hash, native := range natives {
			if !include(native) {
				continue
			}
			result = append(result, buildGeneratedNative(namespace, hash, native, docsBase))
		}
	}
	return result
}

func buildGeneratedNative(namespace, hash string, native sourceNative, docsBase string) generatedNative {
	desc := native.Description
	if desc == "" {
		desc = native.Comment
	}
	results := native.Results
	if results == "" {
		results = native.ReturnType
	}

	convertedReturns, convertedParams := convertOutParams(nativeNameValue(native, hash), results, native.Params)
	params := make([]generatedParam, 0, len(convertedParams))
	paramNames := make(map[string]int)
	for _, param := range convertedParams {
		paramType := luaNativeType(param.Type, true)
		name := fieldToReplace(param.Name)
		if name == "" {
			name = "arg"
		}
		if count := paramNames[name]; count > 0 {
			name = fmt.Sprintf("%s%d", name, count+1)
		}
		paramNames[fieldToReplace(param.Name)]++
		params = append(params, generatedParam{Name: name, Type: paramType})
	}

	returns := make([]string, 0, len(convertedReturns))
	for _, ret := range convertedReturns {
		mapped := luaNativeType(ret, false)
		if mapped == "void" {
			continue
		}
		returns = append(returns, mapped)
	}

	name := normalizeNativeName(nativeNameValue(native, hash))
	aliases := buildAliases(name, firstNonEmptyStrings(native.Aliases, native.OldNames))

	apiSet := native.APISet
	if apiSet == "" {
		apiSet = "client"
	}

	return generatedNative{
		Name:        name,
		Description: nativeDescription(desc, hash, namespace, apiSet, docsBase),
		Params:      params,
		Returns:     returns,
		Aliases:     aliases,
	}
}

func mergeGeneratedNatives(sets ...[]generatedNative) []generatedNative {
	merged := make(map[string]generatedNative)
	for _, set := range sets {
		for _, native := range set {
			existing, ok := merged[native.Name]
			if !ok {
				merged[native.Name] = native
				continue
			}

			if strings.TrimSpace(existing.Description) == "---This native does not have an official description." && strings.TrimSpace(native.Description) != "---This native does not have an official description." {
				existing.Description = native.Description
			}
			if len(existing.Params) == 0 && len(native.Params) > 0 {
				existing.Params = native.Params
			}
			if len(existing.Returns) == 0 && len(native.Returns) > 0 {
				existing.Returns = native.Returns
			}
			existing.Aliases = mergeAliases(existing.Aliases, native.Aliases)
			merged[native.Name] = existing
		}
	}

	result := make([]generatedNative, 0, len(merged))
	for _, native := range merged {
		result = append(result, native)
	}
	return result
}

func renderBundle(bundle generatedBundle) string {
	var sb strings.Builder
	sb.WriteString("---@meta\n\n")
	sb.WriteString(fmt.Sprintf("---Generated from live rage-lua-natives-compatible metadata for `%s`.\n", bundle.Name))
	sb.WriteString(fmt.Sprintf("---Source input: `%s` (%s %s).\n", bundle.Input, bundle.Family, bundle.Runtime))
	if bundle.Fallback {
		sb.WriteString("---This bundle currently falls back to Lugo's checked-in snapshot because no upstream rage-lua-natives source exists for this runtime.\n")
	}
	if bundle.Description != "" {
		sb.WriteString("---")
		sb.WriteString(bundle.Description)
		sb.WriteString("\n")
	}

	for i, native := range bundle.Natives {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(native.Description)
		sb.WriteString("\n")
		for _, param := range native.Params {
			sb.WriteString(fmt.Sprintf("---@param %s %s\n", param.Name, param.Type))
		}
		if len(native.Returns) > 0 {
			sb.WriteString("---@return ")
			sb.WriteString(strings.Join(native.Returns, ", "))
			sb.WriteString("\n")
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
		for _, alias := range native.Aliases {
			sb.WriteString("\n---@deprecated\n")
			sb.WriteString(alias)
			sb.WriteString(" = ")
			sb.WriteString(native.Name)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func nativeNameValue(native sourceNative, hash string) string {
	if native.Name != "" {
		return native.Name
	}
	if native.Hash != "" {
		return native.Hash
	}
	return hash
}

func normalizeNativeName(name string) string {
	name = strings.ToLower(name)
	name = strings.Replace(name, "0x", "n_0x", 1)
	var out strings.Builder
	upperNext := false
	for i, r := range name {
		switch {
		case i == 0:
			out.WriteString(strings.ToUpper(string(r)))
		case r == '_':
			upperNext = true
		case upperNext && r >= 'a' && r <= 'z':
			out.WriteRune(r - ('a' - 'A'))
			upperNext = false
		default:
			out.WriteRune(r)
			upperNext = false
		}
	}
	return out.String()
}

func buildAliases(nativeName string, aliases []string) []string {
	result := make([]string, 0, len(aliases))
	seen := map[string]struct{}{}
	for _, alias := range aliases {
		if alias == "" || strings.HasPrefix(alias, "0") {
			continue
		}
		normalized := normalizeNativeName(alias)
		if normalized == nativeName {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result
}

func convertOutParams(nativeName, results string, params []sourceParam) ([]string, []sourceParam) {
	returnTypes := []string{separateObjectTypes(firstNonEmpty(results, "void"))}
	keptParams := make([]sourceParam, 0, len(params))
	for _, param := range params {
		typeName := strings.ToLower(separateObjectTypes(param.Type))
		if !strings.Contains(typeName, "*") {
			keptParams = append(keptParams, sourceParam{Name: param.Name, Type: typeName})
			continue
		}

		trimmed := strings.TrimSuffix(typeName, "*")
		trimmed = strings.TrimPrefix(trimmed, "const ")
		trimmed = strings.TrimSpace(trimmed)

		if isNonReturnPointerNative(nativeName) || trimmed == "char" {
			keptParams = append(keptParams, sourceParam{Name: param.Name, Type: trimmed})
			continue
		}

		if len(returnTypes) == 1 && returnTypes[0] == "void" {
			returnTypes = returnTypes[:0]
		}
		returnTypes = append(returnTypes, trimmed)
	}
	return returnTypes, keptParams
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyStrings(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

func mergeAliases(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	result := make([]string, 0, len(a)+len(b))
	for _, alias := range append(append([]string{}, a...), b...) {
		if _, ok := seen[alias]; ok {
			continue
		}
		seen[alias] = struct{}{}
		result = append(result, alias)
	}
	sort.Strings(result)
	return result
}

func nativeDescription(description, hash, namespace, apiset, docsBase string) string {
	baseDesc := description
	if strings.TrimSpace(baseDesc) == "" {
		baseDesc = "This native does not have an official description."
	}
	baseDesc = strings.ReplaceAll(baseDesc, "\r\n", "\n")
	baseDesc = strings.ReplaceAll(baseDesc, "\r", "\n")
	lines := strings.Split(baseDesc, "\n")
	for i, line := range lines {
		lines[i] = "---" + line
	}
	return fmt.Sprintf("---**`%s` `%s`**  \n---[Native Documentation](%s%s)  \n%s", namespace, firstNonEmpty(apiset, "client"), docsBase, hash, strings.Join(lines, "\n"))
}

func fieldToReplace(field string) string {
	switch field {
	case "end", "repeat", "local":
		return "_" + field
	default:
		return field
	}
}

func luaNativeType(typeName string, input bool) string {
	typeName = strings.ToLower(strings.TrimSpace(typeName))
	switch typeName {
	case "vector3", "string", "void":
		return typeName
	case "char", "char*":
		return "string"
	case "hash":
		if input {
			return "integer | string"
		}
		return "integer"
	case "bool":
		return "boolean"
	case "object":
		return "table"
	case "func":
		return "function"
	case "float":
		return "number"
	case "uint", "entity", "player", "decisionmaker", "fireid", "ped", "vehicle", "cam", "cargenerator", "group", "train", "pickup", "object_1", "weapon", "interior", "blip", "texture", "texturedict", "coverpoint", "camera", "tasksequence", "sphere", "scrhandle", "int", "long", "itemset", "animscene", "perschar", "popzone", "prompt", "propset", "volume":
		return "integer"
	default:
		return "any"
	}
}

func separateObjectTypes(typeName string) string {
	if strings.Contains(typeName, "Object") {
		return strings.ReplaceAll(typeName, "Object", "object_1")
	}
	return typeName
}

func allowsClientAPISet(apiSet string) bool {
	apiSet = strings.ToLower(strings.TrimSpace(apiSet))
	switch apiSet {
	case "", "client", "shared", "all":
		return true
	default:
		return false
	}
}

func allowsServerAPISet(apiSet string) bool {
	apiSet = strings.ToLower(strings.TrimSpace(apiSet))
	switch apiSet {
	case "", "server", "shared", "all":
		return true
	default:
		return false
	}
}

func matchesGame(value, target string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	target = strings.ToLower(strings.TrimSpace(target))
	if value == "" || value == target || value == "common" || value == "all" {
		return true
	}
	return false
}

func isNonReturnPointerNative(name string) bool {
	_, ok := nonReturnPointerNatives[strings.ToUpper(strings.TrimSpace(name))]
	return ok
}

var nonReturnPointerNatives = map[string]struct{}{
	"DELETE_ENTITY":                        {},
	"SET_ENTITY_AS_NO_LONGER_NEEDED":      {},
	"SET_PED_AS_NO_LONGER_NEEDED":         {},
	"DELETE_PED":                          {},
	"REMOVE_PED_ELEGANTLY":                {},
	"SET_VEHICLE_AS_NO_LONGER_NEEDED":     {},
	"DELETE_MISSION_TRAIN":                {},
	"DELETE_VEHICLE":                      {},
	"SET_MISSION_TRAIN_AS_NO_LONGER_NEEDED": {},
	"DELETE_OBJECT":                       {},
	"SET_OBJECT_AS_NO_LONGER_NEEDED":      {},
	"SET_PLAYER_WANTED_CENTRE_POSITION":   {},
	"_START_SHAPE_TEST_SURROUNDING_COORDS": {},
	"REMOVE_BLIP":                         {},
	"SET_BIT":                             {},
	"CLEAR_BIT":                           {},
	"SET_SCALEFORM_MOVIE_AS_NO_LONGER_NEEDED": {},
	"DELETE_ROPE":                         {},
	"DOES_ROPE_EXIST":                    {},
	"CLEAR_SEQUENCE_TASK":                {},
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
