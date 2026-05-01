package lsp

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	fiveMNativeGTACatalogURL    = "https://static.cfx.re/natives/natives.json"
	fiveMNativeCFXCatalogURL    = "https://static.cfx.re/natives/natives_cfx.json"
	fiveMNativeRDR3CatalogURL   = "https://raw.githubusercontent.com/VORPCORE/RDR3natives/main/rdr3natives.json"
	fiveMNativeDocsURL          = "https://docs.fivem.net/natives/?_"
	fiveMNativeRDR3DocsURL      = "https://rdr3natives.com/?_"
	fiveMNativeCacheVersion     = "v1"
	fiveMNativeGeneratorUA      = "lugo-fivem-native-runtime/1.0"
	fiveMNativeHTTPTimeout      = 2 * time.Minute
	fiveMNativeCacheFolderName  = "fivem-native-bundles"
	fiveMNativeSnapshotFilePath = "fivem_native_snapshot/catalog.json"
)

//go:embed fivem_native_snapshot/catalog.json
var fiveMNativeSnapshotFS embed.FS

var fiveMNativeRuntimeCache = &runtimeFiveMNativeCache{}

type runtimeFiveMNativeCache struct {
	mu          sync.Mutex
	initialized bool
	cacheDir    string
}

type runtimeSourceCatalog map[string]map[string]runtimeSourceNative

type runtimeSourceNative struct {
	Name        string               `json:"name"`
	Params      []runtimeSourceParam `json:"params"`
	Results     string               `json:"results"`
	ReturnType  string               `json:"return_type"`
	Description string               `json:"description"`
	Comment     string               `json:"comment"`
	Hash        string               `json:"hash"`
	Namespace   string               `json:"ns"`
	APISet      string               `json:"apiset"`
	Game        string               `json:"game"`
	Aliases     []string             `json:"aliases"`
	OldNames    []string             `json:"old_names"`
}

type runtimeSourceParam struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type runtimeGeneratedBundle struct {
	Name        string
	Input       string
	Family      string
	Runtime     string
	Description string
	Natives     []runtimeGeneratedNative
	Fallback    bool
}

type runtimeGeneratedNative struct {
	Name        string
	Description string
	Params      []runtimeGeneratedParam
	Returns     []string
	Aliases     []string
}

type runtimeGeneratedParam struct {
	Name string
	Type string
}

type runtimeLegacySnapshot struct {
	Bundles []runtimeLegacyBundle `json:"bundles"`
}

type runtimeLegacyBundle struct {
	Name        string                `json:"name"`
	Input       string                `json:"input"`
	Family      string                `json:"family"`
	Runtime     string                `json:"runtime"`
	Description string                `json:"description"`
	Natives     []runtimeLegacyNative `json:"natives"`
}

type runtimeLegacyNative struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Params      []runtimeLegacyParam `json:"params"`
	Returns     string               `json:"returns"`
}

type runtimeLegacyParam struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func loadRuntimeFiveMNativeBundle(name string) ([]byte, error) {
	if !isFiveMNativeBundleName(name) {
		return nil, fmt.Errorf("unknown FiveM native bundle %q", name)
	}

	cacheDir, err := fiveMNativeRuntimeCache.ensureCacheDir()
	if err != nil {
		return nil, err
	}

	b, err := os.ReadFile(filepath.Join(cacheDir, name))
	if err != nil {
		return nil, fmt.Errorf("read runtime FiveM native bundle %s: %w", name, err)
	}

	return b, nil
}

func (c *runtimeFiveMNativeCache) ensureCacheDir() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cacheDir, err := resolveFiveMNativeCacheDir()
	if err != nil {
		return "", err
	}

	if c.initialized && c.cacheDir == cacheDir {
		return cacheDir, nil
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("create FiveM native cache dir: %w", err)
	}

	bundles, err := buildRuntimeFiveMNativeBundles()
	if err != nil {
		if runtimeFiveMNativeBundlesPresent(cacheDir) {
			c.initialized = true
			c.cacheDir = cacheDir
			return cacheDir, nil
		}
		return "", err
	}

	for _, bundle := range bundles {
		path := filepath.Join(cacheDir, bundle.Name)
		if err := writeRuntimeFiveMNativeBundle(path, []byte(renderRuntimeFiveMNativeBundle(bundle))); err != nil {
			return "", err
		}
	}

	c.initialized = true
	c.cacheDir = cacheDir

	return cacheDir, nil
}

func resolveFiveMNativeCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(base) == "" {
		base = os.TempDir()
	}

	if strings.TrimSpace(base) == "" {
		return "", errors.New("resolve FiveM native cache dir: no cache or temp directory available")
	}

	return filepath.Join(base, "lugo", fiveMNativeCacheFolderName, fiveMNativeCacheVersion), nil
}

func runtimeFiveMNativeBundlesPresent(cacheDir string) bool {
	for name := range fiveMNativeBundleNames {
		info, err := os.Stat(filepath.Join(cacheDir, name))
		if err != nil || info.IsDir() {
			return false
		}
	}

	return true
}

func writeRuntimeFiveMNativeBundle(path string, content []byte) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0o644); err != nil {
		return fmt.Errorf("write FiveM native temp bundle %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("publish FiveM native bundle %s: %w", filepath.Base(path), err)
	}
	return nil
}

func buildRuntimeFiveMNativeBundles() ([]runtimeGeneratedBundle, error) {
	client := &http.Client{Timeout: fiveMNativeHTTPTimeout}

	gtaCatalog, err := fetchRuntimeFiveMNativeCatalog(client, fiveMNativeGTACatalogURL)
	if err != nil {
		return nil, err
	}

	cfxCatalog, err := fetchRuntimeFiveMNativeCatalog(client, fiveMNativeCFXCatalogURL)
	if err != nil {
		return nil, err
	}

	rdr3Catalog, err := fetchRuntimeFiveMNativeCatalog(client, fiveMNativeRDR3CatalogURL)
	if err != nil {
		return nil, err
	}

	bundles := []runtimeGeneratedBundle{
		buildRuntimeGTABundle("natives_21e43a33.lua", gtaCatalog, cfxCatalog),
		buildRuntimeGTABundle("natives_0193d0af.lua", gtaCatalog, cfxCatalog),
		buildRuntimeGTABundle("natives_universal.lua", gtaCatalog, cfxCatalog),
		buildRuntimeRDR3Bundle(rdr3Catalog, cfxCatalog),
		buildRuntimeServerBundle(cfxCatalog),
	}

	nyBundle, err := buildRuntimeNYBundleFromEmbeddedSnapshot()
	if err != nil {
		return nil, err
	}
	bundles = append(bundles, nyBundle)

	for i := range bundles {
		sort.Slice(bundles[i].Natives, func(a, b int) bool { return bundles[i].Natives[a].Name < bundles[i].Natives[b].Name })
	}

	sort.Slice(bundles, func(i, j int) bool { return bundles[i].Name < bundles[j].Name })

	return bundles, nil
}

func fetchRuntimeFiveMNativeCatalog(client *http.Client, url string) (runtimeSourceCatalog, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", fiveMNativeGeneratorUA)

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

	var catalog runtimeSourceCatalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, fmt.Errorf("decode %s: %w", url, err)
	}

	return catalog, nil
}

func buildRuntimeGTABundle(name string, gtaCatalog, cfxCatalog runtimeSourceCatalog) runtimeGeneratedBundle {
	bundle := runtimeGeneratedBundle{
		Name:        name,
		Input:       "natives.json + natives_cfx.json",
		Family:      "gta5",
		Runtime:     "client",
		Description: "Generated from live FiveM GTA V native metadata plus compatible CFX client/shared natives.",
	}
	bundle.Natives = mergeRuntimeGeneratedNatives(
		collectRuntimeGeneratedNatives(gtaCatalog, fiveMNativeDocsURL, func(runtimeSourceNative) bool { return true }),
		collectRuntimeGeneratedNatives(cfxCatalog, fiveMNativeDocsURL, func(n runtimeSourceNative) bool {
			if !matchesRuntimeFiveMGame(n.Game, "gta5") {
				return false
			}
			return allowsRuntimeFiveMClientAPISet(n.APISet)
		}),
	)
	return bundle
}

func buildRuntimeRDR3Bundle(rdr3Catalog, cfxCatalog runtimeSourceCatalog) runtimeGeneratedBundle {
	bundle := runtimeGeneratedBundle{
		Name:        "rdr3_universal.lua",
		Input:       "rdr3natives.json + natives_cfx.json",
		Family:      "rdr3",
		Runtime:     "client",
		Description: "Generated from live RDR3 native metadata plus compatible CFX client/shared natives.",
	}
	bundle.Natives = mergeRuntimeGeneratedNatives(
		collectRuntimeGeneratedNatives(rdr3Catalog, fiveMNativeRDR3DocsURL, func(runtimeSourceNative) bool { return true }),
		collectRuntimeGeneratedNatives(cfxCatalog, fiveMNativeDocsURL, func(n runtimeSourceNative) bool {
			if !matchesRuntimeFiveMGame(n.Game, "rdr3") {
				return false
			}
			return allowsRuntimeFiveMClientAPISet(n.APISet)
		}),
	)
	return bundle
}

func buildRuntimeServerBundle(cfxCatalog runtimeSourceCatalog) runtimeGeneratedBundle {
	bundle := runtimeGeneratedBundle{
		Name:        "natives_server.lua",
		Input:       "natives_cfx.json",
		Family:      "server",
		Runtime:     "server",
		Description: "Generated from live CFX server/shared native metadata.",
	}
	bundle.Natives = collectRuntimeGeneratedNatives(cfxCatalog, fiveMNativeDocsURL, func(n runtimeSourceNative) bool {
		return allowsRuntimeFiveMServerAPISet(n.APISet)
	})
	return bundle
}

func buildRuntimeNYBundleFromEmbeddedSnapshot() (runtimeGeneratedBundle, error) {
	b, err := fiveMNativeSnapshotFS.ReadFile(fiveMNativeSnapshotFilePath)
	if err != nil {
		return runtimeGeneratedBundle{}, fmt.Errorf("read embedded FiveM native snapshot: %w", err)
	}

	var snap runtimeLegacySnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return runtimeGeneratedBundle{}, fmt.Errorf("decode embedded FiveM native snapshot: %w", err)
	}

	for _, bundle := range snap.Bundles {
		if bundle.Name != "ny_universal.lua" {
			continue
		}

		out := runtimeGeneratedBundle{
			Name:        bundle.Name,
			Input:       bundle.Input,
			Family:      bundle.Family,
			Runtime:     bundle.Runtime,
			Description: "Generated from Lugo's embedded NY snapshot because the upstream metadata feeds do not publish an NY catalog.",
			Fallback:    true,
		}

		for _, native := range bundle.Natives {
			params := make([]runtimeGeneratedParam, 0, len(native.Params))
			for _, param := range native.Params {
				params = append(params, runtimeGeneratedParam{Name: sanitizeRuntimeFiveMField(param.Name), Type: param.Type})
			}

			returns := []string{}
			if native.Returns != "" && native.Returns != "void" {
				returns = append(returns, native.Returns)
			}

			out.Natives = append(out.Natives, runtimeGeneratedNative{
				Name:        native.Name,
				Description: "---" + native.Description,
				Params:      params,
				Returns:     returns,
			})
		}

		return out, nil
	}

	return runtimeGeneratedBundle{}, errors.New("embedded snapshot missing ny_universal.lua")
}

func collectRuntimeGeneratedNatives(catalog runtimeSourceCatalog, docsBase string, include func(runtimeSourceNative) bool) []runtimeGeneratedNative {
	result := make([]runtimeGeneratedNative, 0)
	for namespace, natives := range catalog {
		for hash, native := range natives {
			if !include(native) {
				continue
			}
			result = append(result, buildRuntimeGeneratedNative(namespace, hash, native, docsBase))
		}
	}
	return result
}

func buildRuntimeGeneratedNative(namespace, hash string, native runtimeSourceNative, docsBase string) runtimeGeneratedNative {
	desc := native.Description
	if desc == "" {
		desc = native.Comment
	}

	results := native.Results
	if results == "" {
		results = native.ReturnType
	}

	convertedReturns, convertedParams := convertRuntimeFiveMOutParams(runtimeFiveMNativeNameValue(native, hash), results, native.Params)
	params := make([]runtimeGeneratedParam, 0, len(convertedParams))
	paramNames := make(map[string]int)
	for _, param := range convertedParams {
		paramType := mapRuntimeFiveMType(param.Type, true)
		baseName := sanitizeRuntimeFiveMField(param.Name)
		if baseName == "" {
			baseName = "arg"
		}
		name := baseName
		if count := paramNames[baseName]; count > 0 {
			name = fmt.Sprintf("%s%d", baseName, count+1)
		}
		paramNames[baseName]++
		params = append(params, runtimeGeneratedParam{Name: name, Type: paramType})
	}

	returns := make([]string, 0, len(convertedReturns))
	for _, ret := range convertedReturns {
		mapped := mapRuntimeFiveMType(ret, false)
		if mapped == "void" {
			continue
		}
		returns = append(returns, mapped)
	}

	name := normalizeRuntimeFiveMNativeName(runtimeFiveMNativeNameValue(native, hash))
	aliases := buildRuntimeFiveMAliases(name, firstNonEmptyRuntimeStringSlice(native.Aliases, native.OldNames))

	apiSet := native.APISet
	if apiSet == "" {
		apiSet = "client"
	}

	return runtimeGeneratedNative{
		Name:        name,
		Description: buildRuntimeFiveMDescription(desc, hash, namespace, apiSet, docsBase),
		Params:      params,
		Returns:     returns,
		Aliases:     aliases,
	}
}

func mergeRuntimeGeneratedNatives(sets ...[]runtimeGeneratedNative) []runtimeGeneratedNative {
	merged := make(map[string]runtimeGeneratedNative)
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
			existing.Aliases = mergeRuntimeFiveMAliases(existing.Aliases, native.Aliases)
			merged[native.Name] = existing
		}
	}

	result := make([]runtimeGeneratedNative, 0, len(merged))
	for _, native := range merged {
		result = append(result, native)
	}
	return result
}

func renderRuntimeFiveMNativeBundle(bundle runtimeGeneratedBundle) string {
	var sb strings.Builder
	sb.WriteString("---@meta\n\n")
	sb.WriteString(fmt.Sprintf("---Generated from live rage-lua-natives-compatible metadata for `%s`.\n", bundle.Name))
	sb.WriteString(fmt.Sprintf("---Source input: `%s` (%s %s).\n", bundle.Input, bundle.Family, bundle.Runtime))
	if bundle.Fallback {
		sb.WriteString("---This bundle currently falls back to Lugo's embedded snapshot because no upstream rage-lua-natives source exists for this runtime.\n")
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

func runtimeFiveMNativeNameValue(native runtimeSourceNative, hash string) string {
	if native.Name != "" {
		return native.Name
	}
	if native.Hash != "" {
		return native.Hash
	}
	return hash
}

func normalizeRuntimeFiveMNativeName(name string) string {
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

func buildRuntimeFiveMAliases(nativeName string, aliases []string) []string {
	result := make([]string, 0, len(aliases))
	seen := map[string]struct{}{}
	for _, alias := range aliases {
		if alias == "" || strings.HasPrefix(alias, "0") {
			continue
		}
		normalized := normalizeRuntimeFiveMNativeName(alias)
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

func convertRuntimeFiveMOutParams(nativeName, results string, params []runtimeSourceParam) ([]string, []runtimeSourceParam) {
	returnTypes := []string{separateRuntimeFiveMObjectTypes(firstNonEmptyRuntimeString(results, "void"))}
	keptParams := make([]runtimeSourceParam, 0, len(params))
	for _, param := range params {
		typeName := strings.ToLower(separateRuntimeFiveMObjectTypes(param.Type))
		if !strings.Contains(typeName, "*") {
			keptParams = append(keptParams, runtimeSourceParam{Name: param.Name, Type: typeName})
			continue
		}

		trimmed := strings.TrimSuffix(typeName, "*")
		trimmed = strings.TrimPrefix(trimmed, "const ")
		trimmed = strings.TrimSpace(trimmed)

		if isRuntimeFiveMNonReturnPointerNative(nativeName) || trimmed == "char" {
			keptParams = append(keptParams, runtimeSourceParam{Name: param.Name, Type: trimmed})
			continue
		}

		if len(returnTypes) == 1 && returnTypes[0] == "void" {
			returnTypes = returnTypes[:0]
		}
		returnTypes = append(returnTypes, trimmed)
	}
	return returnTypes, keptParams
}

func firstNonEmptyRuntimeString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyRuntimeStringSlice(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

func mergeRuntimeFiveMAliases(a, b []string) []string {
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

func buildRuntimeFiveMDescription(description, hash, namespace, apiset, docsBase string) string {
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
	return fmt.Sprintf("---**`%s` `%s`**  \n---[Native Documentation](%s%s)  \n%s", namespace, firstNonEmptyRuntimeString(apiset, "client"), docsBase, hash, strings.Join(lines, "\n"))
}

func sanitizeRuntimeFiveMField(field string) string {
	switch field {
	case "end", "repeat", "local":
		return "_" + field
	default:
		return field
	}
}

func mapRuntimeFiveMType(typeName string, input bool) string {
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

func separateRuntimeFiveMObjectTypes(typeName string) string {
	if strings.Contains(typeName, "Object") {
		return strings.ReplaceAll(typeName, "Object", "object_1")
	}
	return typeName
}

func allowsRuntimeFiveMClientAPISet(apiSet string) bool {
	apiSet = strings.ToLower(strings.TrimSpace(apiSet))
	switch apiSet {
	case "", "client", "shared", "all":
		return true
	default:
		return false
	}
}

func allowsRuntimeFiveMServerAPISet(apiSet string) bool {
	apiSet = strings.ToLower(strings.TrimSpace(apiSet))
	switch apiSet {
	case "", "server", "shared", "all":
		return true
	default:
		return false
	}
}

func matchesRuntimeFiveMGame(value, target string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	target = strings.ToLower(strings.TrimSpace(target))
	if value == "" || value == target || value == "common" || value == "all" {
		return true
	}
	return false
}

func isRuntimeFiveMNonReturnPointerNative(name string) bool {
	_, ok := runtimeFiveMNonReturnPointerNatives[strings.ToUpper(strings.TrimSpace(name))]
	return ok
}

var runtimeFiveMNonReturnPointerNatives = map[string]struct{}{
	"DELETE_ENTITY":                           {},
	"SET_ENTITY_AS_NO_LONGER_NEEDED":          {},
	"SET_PED_AS_NO_LONGER_NEEDED":             {},
	"DELETE_PED":                              {},
	"REMOVE_PED_ELEGANTLY":                    {},
	"SET_VEHICLE_AS_NO_LONGER_NEEDED":         {},
	"DELETE_MISSION_TRAIN":                    {},
	"DELETE_VEHICLE":                          {},
	"SET_MISSION_TRAIN_AS_NO_LONGER_NEEDED":   {},
	"DELETE_OBJECT":                           {},
	"SET_OBJECT_AS_NO_LONGER_NEEDED":          {},
	"SET_PLAYER_WANTED_CENTRE_POSITION":       {},
	"_START_SHAPE_TEST_SURROUNDING_COORDS":    {},
	"REMOVE_BLIP":                             {},
	"SET_BIT":                                 {},
	"CLEAR_BIT":                               {},
	"SET_SCALEFORM_MOVIE_AS_NO_LONGER_NEEDED": {},
	"DELETE_ROPE":                             {},
	"DOES_ROPE_EXIST":                         {},
	"CLEAR_SEQUENCE_TASK":                     {},
}
