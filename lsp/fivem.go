package lsp

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/coalaura/lugo/ast"
)

type FileEnv int

const (
	EnvUnknown FileEnv = iota
	EnvShared
	EnvClient
	EnvServer
)

type FiveMExecutionProfileKind int

const (
	FiveMProfilePlainLua FiveMExecutionProfileKind = iota
	FiveMProfileManifest
	FiveMProfileClient
	FiveMProfileServer
	FiveMProfileShared
	FiveMProfileExportBridge
)

type FiveMExecutionProfile struct {
	Kind         FiveMExecutionProfileKind
	ResourceRoot string
	ResourceName string
}

type FiveMManifestSourceKind int

const (
	FiveMManifestSourceUnknown FiveMManifestSourceKind = iota
	FiveMManifestSourceFXManifest
	FiveMManifestSourceResourceLua
)

func (kind FiveMManifestSourceKind) String() string {
	switch kind {
	case FiveMManifestSourceFXManifest:
		return "fxmanifest.lua"
	case FiveMManifestSourceResourceLua:
		return "__resource.lua"
	default:
		return ""
	}
}

func (kind FiveMManifestSourceKind) Precedence() int {
	switch kind {
	case FiveMManifestSourceFXManifest:
		return 2
	case FiveMManifestSourceResourceLua:
		return 1
	default:
		return 0
	}
}

type FiveMManifestEntryShape int

const (
	FiveMManifestEntryScalar FiveMManifestEntryShape = iota
	FiveMManifestEntryTableValue
	FiveMManifestEntryExtra
	FiveMManifestEntryInjected
)

type FiveMManifestEntry struct {
	SourceURI      string
	RawName        string
	EmittedName    string
	NormalizedName string
	Shape          FiveMManifestEntryShape
	Range          Range
	ValueRange     Range
	RawValue       string
	Value          string
	ReservedKey    bool
	LoaderInjected bool
	ExtraKey       string
	RawExtraKey    string
}

func (entry FiveMManifestEntry) Equal(other FiveMManifestEntry) bool {
	return entry.SourceURI == other.SourceURI &&
		entry.RawName == other.RawName &&
		entry.EmittedName == other.EmittedName &&
		entry.NormalizedName == other.NormalizedName &&
		entry.Shape == other.Shape &&
		entry.Range == other.Range &&
		entry.ValueRange == other.ValueRange &&
		entry.RawValue == other.RawValue &&
		entry.Value == other.Value &&
		entry.ReservedKey == other.ReservedKey &&
		entry.LoaderInjected == other.LoaderInjected &&
		entry.ExtraKey == other.ExtraKey &&
		entry.RawExtraKey == other.RawExtraKey
}

type FiveMManifest struct {
	ResourceName string
	ResourceRoot string
	SourceURI    string
	SourceKind   FiveMManifestSourceKind
	IsCfxV2      bool
	Entries      []FiveMManifestEntry
}

func (manifest *FiveMManifest) Equal(other *FiveMManifest) bool {
	if manifest == other {
		return true
	}

	if manifest == nil || other == nil {
		return false
	}

	if manifest.ResourceName != other.ResourceName || manifest.ResourceRoot != other.ResourceRoot || manifest.SourceURI != other.SourceURI || manifest.SourceKind != other.SourceKind || manifest.IsCfxV2 != other.IsCfxV2 {
		return false
	}

	return slices.EqualFunc(manifest.Entries, other.Entries, func(a, b FiveMManifestEntry) bool {
		return a.Equal(b)
	})
}

func (kind FiveMExecutionProfileKind) String() string {
	switch kind {
	case FiveMProfileManifest:
		return "fivem-manifest"
	case FiveMProfileClient:
		return "fivem-client"
	case FiveMProfileServer:
		return "fivem-server"
	case FiveMProfileShared:
		return "fivem-shared"
	case FiveMProfileExportBridge:
		return "fivem-export-bridge"
	default:
		return "plain-lua"
	}
}

func (profile FiveMExecutionProfile) Env() FileEnv {
	switch profile.Kind {
	case FiveMProfileClient:
		return EnvClient
	case FiveMProfileServer:
		return EnvServer
	case FiveMProfileShared:
		return EnvShared
	default:
		return EnvUnknown
	}
}

func (profile FiveMExecutionProfile) HasResource() bool {
	return profile.ResourceRoot != ""
}

func (profile FiveMExecutionProfile) IsFiveMActive() bool {
	switch profile.Kind {
	case FiveMProfileManifest, FiveMProfileClient, FiveMProfileServer, FiveMProfileShared, FiveMProfileExportBridge:
		return true
	default:
		return false
	}
}

func (profile FiveMExecutionProfile) AllowsExportBridge() bool {
	switch profile.Kind {
	case FiveMProfileClient, FiveMProfileServer, FiveMProfileShared, FiveMProfileExportBridge:
		return true
	default:
		return false
	}
}

func (profile FiveMExecutionProfile) AllowsFiveMGlobal(name string) bool {
	switch name {
	case "exports":
		return profile.AllowsExportBridge()
	case "source":
		return profile.Kind == FiveMProfileServer
	default:
		return false
	}
}

func (profile FiveMExecutionProfile) AllowsRuntimeLibrary() bool {
	switch profile.Kind {
	case FiveMProfileClient, FiveMProfileServer, FiveMProfileShared:
		return true
	default:
		return false
	}
}

func (profile FiveMExecutionProfile) ExportBridge() FiveMExecutionProfile {
	if !profile.AllowsExportBridge() {
		return FiveMExecutionProfile{
			Kind:         FiveMProfilePlainLua,
			ResourceRoot: profile.ResourceRoot,
			ResourceName: profile.ResourceName,
		}
	}

	return FiveMExecutionProfile{
		Kind:         FiveMProfileExportBridge,
		ResourceRoot: profile.ResourceRoot,
		ResourceName: profile.ResourceName,
	}
}

type FiveMResource struct {
	Name                string
	RootURI             string
	ManifestURI         string
	ManifestSource      FiveMManifestSourceKind
	Manifest            *FiveMManifest
	ManifestVersion     string
	FXVersion           string
	Games               []string
	UseExperimentalOAL  bool
	Dependencies        []string
	Provides            []string
	UIPage              string
	ServerOnly          bool
	IsCfxV2             bool
	ClientGlobs         []string
	ServerGlobs         []string
	SharedGlobs         []string
	ClientCrossIncludes []string
	ServerCrossIncludes []string
	SharedCrossIncludes []string
	ClientExports       []string
	ServerExports       []string
}

type FiveMNativeFamily int

const (
	FiveMNativeFamilyUnknown FiveMNativeFamily = iota
	FiveMNativeFamilyGTA5
	FiveMNativeFamilyRDR3
	FiveMNativeFamilyNY
	FiveMNativeFamilyServer
)

func (family FiveMNativeFamily) String() string {
	switch family {
	case FiveMNativeFamilyGTA5:
		return "gta5"
	case FiveMNativeFamilyRDR3:
		return "rdr3"
	case FiveMNativeFamilyNY:
		return "ny"
	case FiveMNativeFamilyServer:
		return "server"
	default:
		return ""
	}
}

type FiveMNativeSelection struct {
	Family             FiveMNativeFamily
	Build              string
	UseExperimentalOAL bool
}

func (selection FiveMNativeSelection) Active() bool {
	return selection.Build != ""
}

func normalizeFiveMGameName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "gta5", "gtaiv", "gta4", "gta-ny", "ny":
		if strings.Contains(strings.ToLower(strings.TrimSpace(name)), "ny") || strings.Contains(strings.ToLower(strings.TrimSpace(name)), "iv") || strings.Contains(strings.ToLower(strings.TrimSpace(name)), "4") {
			return "ny"
		}

		return "gta5"
	case "rdr3", "redm", "reddeadredemption2", "red dead redemption 2":
		return "rdr3"
	default:
		return strings.ToLower(strings.TrimSpace(name))
	}
}

func isFiveMManifestVersionAtLeastAdamant(fxVersion string) bool {
	switch strings.ToLower(strings.TrimSpace(fxVersion)) {
	case "adamant", "bodacious", "cerulean":
		return true
	default:
		return false
	}
}

func (res *FiveMResource) clientNativeFamily() FiveMNativeFamily {
	for _, game := range res.Games {
		switch normalizeFiveMGameName(game) {
		case "rdr3":
			return FiveMNativeFamilyRDR3
		case "ny":
			return FiveMNativeFamilyNY
		case "gta5":
			return FiveMNativeFamilyGTA5
		}
	}

	return FiveMNativeFamilyGTA5
}

func (res *FiveMResource) clientNativeBuild() string {
	switch res.clientNativeFamily() {
	case FiveMNativeFamilyRDR3:
		return "rdr3_universal.lua"
	case FiveMNativeFamilyNY:
		return "ny_universal.lua"
	case FiveMNativeFamilyGTA5:
		if res.IsCfxV2 && isFiveMManifestVersionAtLeastAdamant(res.FXVersion) {
			return "natives_universal.lua"
		}

		switch strings.ToLower(strings.TrimSpace(res.ManifestVersion)) {
		case "44febabe-d386-4d18-afbe-5e627f4af937":
			return "natives_universal.lua"
		case "f15e72ec-3972-4fe4-9c7d-afc5394ae207":
			return "natives_0193d0af.lua"
		default:
			return "natives_21e43a33.lua"
		}
	default:
		return ""
	}
}

func (res *FiveMResource) NativeSelection(profile FiveMExecutionProfile) FiveMNativeSelection {
	if res == nil {
		return FiveMNativeSelection{}
	}

	switch profile.Kind {
	case FiveMProfileClient:
		return FiveMNativeSelection{
			Family:             res.clientNativeFamily(),
			Build:              res.clientNativeBuild(),
			UseExperimentalOAL: res.UseExperimentalOAL,
		}
	case FiveMProfileServer:
		return FiveMNativeSelection{
			Family:             FiveMNativeFamilyServer,
			Build:              "natives_server.lua",
			UseExperimentalOAL: false,
		}
	default:
		return FiveMNativeSelection{}
	}
}

type FiveMResourceGraphExpansionKind int

const (
	FiveMResourceGraphExpansionLocal FiveMResourceGraphExpansionKind = iota
	FiveMResourceGraphExpansionInclude
)

type FiveMResourceGraphExpansion struct {
	Entry           FiveMManifestEntry
	Kind            FiveMResourceGraphExpansionKind
	Pattern         string
	Include         string
	IncludeResource string
	IncludePath     string
}

type FiveMResourceGraphNode struct {
	Resource       *FiveMResource
	Name           string
	RootURI        string
	ManifestURI    string
	ManifestSource FiveMManifestSourceKind
	Provides       []string
	Dependencies   []string
	ClientEntries  []FiveMResourceGraphExpansion
	ServerEntries  []FiveMResourceGraphExpansion
	SharedEntries  []FiveMResourceGraphExpansion
}

type FiveMResourceGraph struct {
	ByRoot    map[string]*FiveMResourceGraphNode
	ByName    map[string]*FiveMResourceGraphNode
	ByProvide map[string][]*FiveMResourceGraphNode
}

func NewFiveMResourceGraph() *FiveMResourceGraph {
	return &FiveMResourceGraph{
		ByRoot:    make(map[string]*FiveMResourceGraphNode),
		ByName:    make(map[string]*FiveMResourceGraphNode),
		ByProvide: make(map[string][]*FiveMResourceGraphNode),
	}
}

func (g *FiveMResourceGraph) Clear() {
	if g == nil {
		return
	}

	clear(g.ByRoot)
	clear(g.ByName)
	clear(g.ByProvide)
}

func normalizeFiveMResourceAlias(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func parseFiveMResourceInclude(value string) (resourceName, path string, ok bool) {
	if !strings.HasPrefix(value, "@") {
		return "", "", false
	}

	trimmed := value[1:]
	sep := strings.IndexByte(trimmed, '/')
	if sep <= 0 || sep == len(trimmed)-1 {
		return "", "", false
	}

	resourceName = normalizeFiveMResourceAlias(trimmed[:sep])
	path = trimmed[sep+1:]

	return resourceName, path, resourceName != "" && path != ""
}

func newFiveMResourceGraphExpansion(entry FiveMManifestEntry) FiveMResourceGraphExpansion {
	expansion := FiveMResourceGraphExpansion{Entry: entry}

	if strings.HasPrefix(entry.Value, "@") {
		expansion.Kind = FiveMResourceGraphExpansionInclude
		expansion.Include = entry.Value
		expansion.IncludeResource, expansion.IncludePath, _ = parseFiveMResourceInclude(entry.Value)

		return expansion
	}

	expansion.Kind = FiveMResourceGraphExpansionLocal
	expansion.Pattern = entry.Value

	return expansion
}

func newFiveMResourceGraphNode(res *FiveMResource) *FiveMResourceGraphNode {
	node := &FiveMResourceGraphNode{
		Resource:       res,
		Name:           normalizeFiveMResourceAlias(res.Name),
		RootURI:        res.RootURI,
		ManifestURI:    res.ManifestURI,
		ManifestSource: res.ManifestSource,
	}

	if res.Manifest == nil {
		node.Provides = append(node.Provides, res.Provides...)
		node.Dependencies = append(node.Dependencies, res.Dependencies...)

		for _, pattern := range res.ClientGlobs {
			node.ClientEntries = append(node.ClientEntries, FiveMResourceGraphExpansion{Kind: FiveMResourceGraphExpansionLocal, Pattern: pattern})
		}

		for _, include := range res.ClientCrossIncludes {
			expansion := FiveMResourceGraphExpansion{Kind: FiveMResourceGraphExpansionInclude, Include: include}
			expansion.IncludeResource, expansion.IncludePath, _ = parseFiveMResourceInclude(include)
			node.ClientEntries = append(node.ClientEntries, expansion)
		}

		for _, pattern := range res.ServerGlobs {
			node.ServerEntries = append(node.ServerEntries, FiveMResourceGraphExpansion{Kind: FiveMResourceGraphExpansionLocal, Pattern: pattern})
		}

		for _, include := range res.ServerCrossIncludes {
			expansion := FiveMResourceGraphExpansion{Kind: FiveMResourceGraphExpansionInclude, Include: include}
			expansion.IncludeResource, expansion.IncludePath, _ = parseFiveMResourceInclude(include)
			node.ServerEntries = append(node.ServerEntries, expansion)
		}

		for _, pattern := range res.SharedGlobs {
			node.SharedEntries = append(node.SharedEntries, FiveMResourceGraphExpansion{Kind: FiveMResourceGraphExpansionLocal, Pattern: pattern})
		}

		for _, include := range res.SharedCrossIncludes {
			expansion := FiveMResourceGraphExpansion{Kind: FiveMResourceGraphExpansionInclude, Include: include}
			expansion.IncludeResource, expansion.IncludePath, _ = parseFiveMResourceInclude(include)
			node.SharedEntries = append(node.SharedEntries, expansion)
		}

		return node
	}

	for _, entry := range res.Manifest.Entries {
		if entry.LoaderInjected || entry.ReservedKey {
			continue
		}

		switch entry.EmittedName {
		case "dependency", "dependencie":
			if entry.Value != "" {
				node.Dependencies = append(node.Dependencies, entry.Value)
			}
		case "provide":
			alias := normalizeFiveMResourceAlias(entry.Value)
			if alias != "" {
				node.Provides = append(node.Provides, alias)
			}
		case "client_script":
			node.ClientEntries = append(node.ClientEntries, newFiveMResourceGraphExpansion(entry))
		case "server_script":
			node.ServerEntries = append(node.ServerEntries, newFiveMResourceGraphExpansion(entry))
		case "shared_script", "file":
			node.SharedEntries = append(node.SharedEntries, newFiveMResourceGraphExpansion(entry))
		}
	}

	return node
}

func (node *FiveMResourceGraphNode) allowedIncludes(env FileEnv) []FiveMResourceGraphExpansion {
	if node == nil {
		return nil
	}

	entries := make([]FiveMResourceGraphExpansion, 0, len(node.SharedEntries)+len(node.ClientEntries)+len(node.ServerEntries))

	for _, entry := range node.SharedEntries {
		if entry.Kind == FiveMResourceGraphExpansionInclude {
			entries = append(entries, entry)
		}
	}

	switch env {
	case EnvClient:
		for _, entry := range node.ClientEntries {
			if entry.Kind == FiveMResourceGraphExpansionInclude {
				entries = append(entries, entry)
			}
		}
	case EnvServer:
		for _, entry := range node.ServerEntries {
			if entry.Kind == FiveMResourceGraphExpansionInclude {
				entries = append(entries, entry)
			}
		}
	}

	return entries
}

func compareFiveMResourceGraphNodes(a, b *FiveMResourceGraphNode) int {
	if a == nil && b == nil {
		return 0
	}

	if a == nil {
		return 1
	}

	if b == nil {
		return -1
	}

	if a.Name != b.Name {
		if a.Name < b.Name {
			return -1
		}

		return 1
	}

	if a.RootURI != b.RootURI {
		if a.RootURI < b.RootURI {
			return -1
		}

		return 1
	}

	return 0
}

func removeFiveMResourceGraphNode(nodes []*FiveMResourceGraphNode, target *FiveMResourceGraphNode) []*FiveMResourceGraphNode {
	for i, node := range nodes {
		if node == target {
			copy(nodes[i:], nodes[i+1:])
			nodes = nodes[:len(nodes)-1]

			break
		}
	}

	return nodes
}

func (g *FiveMResourceGraph) removeNode(node *FiveMResourceGraphNode) {
	if g == nil || node == nil {
		return
	}

	delete(g.ByRoot, node.RootURI)
	if node.Name != "" && g.ByName[node.Name] == node {
		delete(g.ByName, node.Name)
	}

	for _, alias := range node.Provides {
		nodes := removeFiveMResourceGraphNode(g.ByProvide[alias], node)
		if len(nodes) == 0 {
			delete(g.ByProvide, alias)
		} else {
			g.ByProvide[alias] = nodes
		}
	}
}

func (g *FiveMResourceGraph) Register(res *FiveMResource) *FiveMResource {
	if g == nil || res == nil {
		return nil
	}

	current := g.ResourceByRoot(res.RootURI)
	if current != nil && current.ManifestSource.Precedence() > res.ManifestSource.Precedence() {
		return current
	}

	if currentNode := g.ByRoot[res.RootURI]; currentNode != nil {
		g.removeNode(currentNode)
	}

	node := newFiveMResourceGraphNode(res)
	g.ByRoot[node.RootURI] = node
	if node.Name != "" {
		g.ByName[node.Name] = node
	}

	for _, alias := range node.Provides {
		if alias == "" {
			continue
		}

		nodes := append(g.ByProvide[alias], node)
		if len(nodes) > 1 {
			sort.SliceStable(nodes, func(i, j int) bool {
				return compareFiveMResourceGraphNodes(nodes[i], nodes[j]) < 0
			})
		}

		g.ByProvide[alias] = nodes
	}

	return res
}

func (g *FiveMResourceGraph) ResourceByRoot(root string) *FiveMResource {
	if g == nil {
		return nil
	}

	if node := g.ByRoot[root]; node != nil {
		return node.Resource
	}

	return nil
}

func (g *FiveMResourceGraph) NodeByRoot(root string) *FiveMResourceGraphNode {
	if g == nil {
		return nil
	}

	return g.ByRoot[root]
}

func (g *FiveMResourceGraph) Resolve(name string) *FiveMResource {
	if g == nil {
		return nil
	}

	alias := normalizeFiveMResourceAlias(name)
	if alias == "" {
		return nil
	}

	if node := g.ByName[alias]; node != nil {
		return node.Resource
	}

	if nodes := g.ByProvide[alias]; len(nodes) > 0 && nodes[0] != nil {
		return nodes[0].Resource
	}

	return nil
}

func (g *FiveMResourceGraph) PublicNames() []string {
	if g == nil {
		return nil
	}

	names := make([]string, 0, len(g.ByName)+len(g.ByProvide))
	for name := range g.ByName {
		names = append(names, name)
	}

	for alias := range g.ByProvide {
		if _, exists := g.ByName[alias]; exists {
			continue
		}

		names = append(names, alias)
	}

	sort.Strings(names)

	return names
}

func (r *FiveMResource) Equal(other *FiveMResource) bool {
	if r == other {
		return true
	}

	if r == nil || other == nil {
		return false
	}

	if r.Name != other.Name || r.RootURI != other.RootURI || r.ManifestURI != other.ManifestURI || r.ManifestSource != other.ManifestSource || r.ManifestVersion != other.ManifestVersion || r.FXVersion != other.FXVersion || r.UseExperimentalOAL != other.UseExperimentalOAL || r.UIPage != other.UIPage || r.ServerOnly != other.ServerOnly || r.IsCfxV2 != other.IsCfxV2 {
		return false
	}

	if !r.Manifest.Equal(other.Manifest) {
		return false
	}

	if !slices.Equal(r.Dependencies, other.Dependencies) {
		return false
	}

	if !slices.Equal(r.Games, other.Games) {
		return false
	}

	if !slices.Equal(r.Provides, other.Provides) {
		return false
	}

	if !slices.Equal(r.ClientGlobs, other.ClientGlobs) {
		return false
	}

	if !slices.Equal(r.ServerGlobs, other.ServerGlobs) {
		return false
	}

	if !slices.Equal(r.SharedGlobs, other.SharedGlobs) {
		return false
	}

	if !slices.Equal(r.ClientCrossIncludes, other.ClientCrossIncludes) {
		return false
	}

	if !slices.Equal(r.ServerCrossIncludes, other.ServerCrossIncludes) {
		return false
	}

	if !slices.Equal(r.SharedCrossIncludes, other.SharedCrossIncludes) {
		return false
	}

	if !slices.Equal(r.ClientExports, other.ClientExports) {
		return false
	}

	if !slices.Equal(r.ServerExports, other.ServerExports) {
		return false
	}

	return true
}

func unquoteLuaString(s string) string {
	s = strings.TrimSpace(s)

	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') {
		if s[len(s)-1] == s[0] {
			return s[1 : len(s)-1]
		}

		return s[1:]
	}

	if strings.HasPrefix(s, "[") {
		idx := strings.IndexByte(s[1:], '[')
		if idx != -1 {
			start := 2 + idx
			if start < len(s) && s[start] == '\n' {
				start++
			}

			end := len(s) - (2 + idx)
			if start <= end {
				return s[start:end]
			}
		}
	}

	return s
}

func fiveMManifestSourceFromURI(uri string) FiveMManifestSourceKind {
	switch {
	case strings.HasSuffix(uri, "/fxmanifest.lua"):
		return FiveMManifestSourceFXManifest
	case strings.HasSuffix(uri, "/__resource.lua"):
		return FiveMManifestSourceResourceLua
	default:
		return FiveMManifestSourceUnknown
	}
}

func fiveMManifestNormalizedName(name string) string {
	lower := strings.ToLower(name)

	switch lower {
	case "client_scripts":
		return "client_script"
	case "server_scripts":
		return "server_script"
	case "shared_scripts":
		return "shared_script"
	case "files":
		return "file"
	case "exports":
		return "export"
	case "client_exports":
		return "client_export"
	case "server_exports":
		return "server_export"
	case "dependencies", "dependencie":
		return "dependency"
	case "provides":
		return "provide"
	default:
		if strings.HasSuffix(lower, "s") {
			return strings.TrimSuffix(lower, "s")
		}

		return lower
	}
}

func isReservedFiveMManifestKey(name string) bool {
	return strings.EqualFold(name, "is_cfxv2")
}

func isValidFiveMManifestDirectiveIdentifier(name string) bool {
	if name == "" {
		return false
	}

	for i := 0; i < len(name); i++ {
		c := name[i]

		switch {
		case c >= 'a' && c <= 'z':
			continue
		case i > 0 && c >= '0' && c <= '9':
			continue
		case i > 0 && c == '_':
			continue
		default:
			return false
		}
	}

	return true
}

func hasReservedFiveMManifestEntry(entries []FiveMManifestEntry) bool {
	for _, entry := range entries {
		if entry.ReservedKey && !entry.LoaderInjected {
			return true
		}
	}

	return false
}

func (s *Server) buildFiveMManifestDiagnostics(doc *Document) []Diagnostic {
	if doc == nil || !doc.IsFiveMManifest {
		return nil
	}

	res := s.parseFiveMManifest(doc)
	if res == nil || res.Manifest == nil {
		return nil
	}

	diags := make([]Diagnostic, 0, 8)
	seenReserved := make(map[Range]bool, 4)

	for _, entry := range res.Manifest.Entries {
		if !entry.ReservedKey || entry.LoaderInjected {
			continue
		}

		if seenReserved[entry.Range] {
			continue
		}

		seenReserved[entry.Range] = true

		name := entry.RawName
		if name == "" {
			name = entry.EmittedName
		}

		diags = append(diags, Diagnostic{
			Range:    entry.Range,
			Severity: SeverityWarning,
			Code:     "fivem-manifest-reserved-directive",
			Message:  fmt.Sprintf("Manifest directive '%s' is reserved by the loader and cannot be authored manually.", name),
		})
	}

	if doc.Tree.Root == ast.InvalidNode || int(doc.Tree.Root) >= len(doc.Tree.Nodes) {
		return diags
	}

	root := doc.Tree.Nodes[doc.Tree.Root]
	if root.Left == ast.InvalidNode || int(root.Left) >= len(doc.Tree.Nodes) {
		return diags
	}

	block := doc.Tree.Nodes[root.Left]
	for i := uint16(0); i < block.Count; i++ {
		if block.Extra+uint32(i) >= uint32(len(doc.Tree.ExtraList)) {
			continue
		}

		stmtID := doc.Tree.ExtraList[block.Extra+uint32(i)]
		if int(stmtID) >= len(doc.Tree.Nodes) {
			continue
		}

		stmt := doc.Tree.Nodes[stmtID]
		if stmt.Kind != ast.KindCallExpr && stmt.Kind != ast.KindMethodCall {
			diags = append(diags, Diagnostic{
				Range:    getNodeRange(doc.Tree, stmtID),
				Severity: SeverityWarning,
				Code:     "fivem-manifest-invalid-construct",
				Message:  "Invalid manifest construct. Manifest files only support top-level directive calls.",
			})

			continue
		}

		rawName, _, _, entries, ok := s.fiveMManifestEntriesForCall(doc, stmtID)
		if !ok || len(entries) == 0 {
			diags = append(diags, Diagnostic{
				Range:    getNodeRange(doc.Tree, stmtID),
				Severity: SeverityWarning,
				Code:     "fivem-manifest-invalid-construct",
				Message:  "Invalid manifest construct. Manifest directives must be authored as direct top-level calls.",
			})

			continue
		}

		if hasReservedFiveMManifestEntry(entries) {
			continue
		}

		if !isValidFiveMManifestDirectiveIdentifier(rawName) {
			diags = append(diags, Diagnostic{
				Range:    getNodeRange(doc.Tree, stmtID),
				Severity: SeverityWarning,
				Code:     "fivem-manifest-unknown-directive",
				Message:  fmt.Sprintf("Unknown manifest directive '%s'. Manifest directives should use lowercase identifiers; runtime APIs are unavailable in manifest files.", rawName),
			})
		}
	}

	return diags
}

func (s *Server) fiveMManifestNodeValue(doc *Document, nodeID ast.NodeID) (string, string, Range, bool) {
	if int(nodeID) >= len(doc.Tree.Nodes) {
		return "", "", Range{}, false
	}

	node := doc.Tree.Nodes[nodeID]
	if node.Start > node.End || node.End > uint32(len(doc.Source)) {
		return "", "", Range{}, false
	}

	raw := string(doc.Source[node.Start:node.End])
	value := raw

	if node.Kind == ast.KindString {
		value = unquoteLuaString(raw)
	}

	return raw, value, getNodeRange(doc.Tree, nodeID), true
}

func (s *Server) fiveMManifestEntriesForCall(doc *Document, nodeID ast.NodeID) (rawName, emittedName, normalizedName string, entries []FiveMManifestEntry, ok bool) {
	if int(nodeID) >= len(doc.Tree.Nodes) {
		return "", "", "", nil, false
	}

	node := doc.Tree.Nodes[nodeID]
	if node.Kind != ast.KindCallExpr && node.Kind != ast.KindMethodCall {
		return "", "", "", nil, false
	}

	if int(node.Left) >= len(doc.Tree.Nodes) {
		return "", "", "", nil, false
	}

	left := doc.Tree.Nodes[node.Left]

	if left.Kind == ast.KindCallExpr || left.Kind == ast.KindMethodCall {
		rawName, emittedName, normalizedName, entries, ok = s.fiveMManifestEntriesForCall(doc, node.Left)
		if !ok || rawName == "" {
			return "", "", "", nil, false
		}

		rawExtraKey := ""
		extraKey := ""
		rawValue := ""
		value := ""
		valueRange := Range{}

		if node.Count >= 1 && node.Extra < uint32(len(doc.Tree.ExtraList)) {
			if keyRaw, keyValue, _, found := s.fiveMManifestNodeValue(doc, doc.Tree.ExtraList[node.Extra]); found {
				rawExtraKey = keyRaw
				extraKey = keyValue
			}
		}

		if node.Count >= 2 && node.Extra+1 < uint32(len(doc.Tree.ExtraList)) {
			if payloadRaw, payloadValue, payloadRange, found := s.fiveMManifestNodeValue(doc, doc.Tree.ExtraList[node.Extra+1]); found {
				rawValue = payloadRaw
				value = payloadValue
				valueRange = payloadRange
			}
		}

		entries = append(entries, FiveMManifestEntry{
			SourceURI:      doc.URI,
			RawName:        rawName,
			EmittedName:    emittedName + "_extra",
			NormalizedName: normalizedName,
			Shape:          FiveMManifestEntryExtra,
			Range:          getNodeRange(doc.Tree, nodeID),
			ValueRange:     valueRange,
			RawValue:       rawValue,
			Value:          value,
			ReservedKey:    isReservedFiveMManifestKey(emittedName),
			ExtraKey:       extraKey,
			RawExtraKey:    rawExtraKey,
		})

		return rawName, emittedName, normalizedName, entries, true
	}

	if left.Kind != ast.KindIdent || left.Start > left.End || left.End > uint32(len(doc.Source)) {
		return "", "", "", nil, false
	}

	rawName = string(doc.Source[left.Start:left.End])
	normalizedName = fiveMManifestNormalizedName(rawName)
	emittedName = strings.ToLower(rawName)
	callRange := getNodeRange(doc.Tree, nodeID)
	reserved := isReservedFiveMManifestKey(emittedName)

	if node.Count == 1 && node.Extra < uint32(len(doc.Tree.ExtraList)) {
		argID := doc.Tree.ExtraList[node.Extra]
		if int(argID) < len(doc.Tree.Nodes) && doc.Tree.Nodes[argID].Kind == ast.KindTableExpr {
			tableNode := doc.Tree.Nodes[argID]
			emittedName = normalizedName

			for i := uint16(0); i < tableNode.Count; i++ {
				if tableNode.Extra+uint32(i) >= uint32(len(doc.Tree.ExtraList)) {
					continue
				}

				valueID := doc.Tree.ExtraList[tableNode.Extra+uint32(i)]
				rawValue, value, valueRange, found := s.fiveMManifestNodeValue(doc, valueID)
				if !found {
					continue
				}

				entries = append(entries, FiveMManifestEntry{
					SourceURI:      doc.URI,
					RawName:        rawName,
					EmittedName:    emittedName,
					NormalizedName: normalizedName,
					Shape:          FiveMManifestEntryTableValue,
					Range:          callRange,
					ValueRange:     valueRange,
					RawValue:       rawValue,
					Value:          value,
					ReservedKey:    reserved,
				})
			}

			return rawName, emittedName, normalizedName, entries, true
		}
	}

	if node.Count == 0 {
		entries = append(entries, FiveMManifestEntry{
			SourceURI:      doc.URI,
			RawName:        rawName,
			EmittedName:    emittedName,
			NormalizedName: normalizedName,
			Shape:          FiveMManifestEntryScalar,
			Range:          callRange,
			ReservedKey:    reserved,
		})

		return rawName, emittedName, normalizedName, entries, true
	}

	for i := uint16(0); i < node.Count; i++ {
		if node.Extra+uint32(i) >= uint32(len(doc.Tree.ExtraList)) {
			continue
		}

		valueID := doc.Tree.ExtraList[node.Extra+uint32(i)]
		rawValue, value, valueRange, found := s.fiveMManifestNodeValue(doc, valueID)
		if !found {
			continue
		}

		entries = append(entries, FiveMManifestEntry{
			SourceURI:      doc.URI,
			RawName:        rawName,
			EmittedName:    emittedName,
			NormalizedName: normalizedName,
			Shape:          FiveMManifestEntryScalar,
			Range:          callRange,
			ValueRange:     valueRange,
			RawValue:       rawValue,
			Value:          value,
			ReservedKey:    reserved,
		})
	}

	return rawName, emittedName, normalizedName, entries, true
}

func parseFiveMBoolSetting(value string) bool {
	if value == "" {
		return true
	}

	switch strings.ToLower(strings.TrimSpace(value)) {
	case "false", "0", "off", "no", "nil":
		return false
	default:
		return true
	}
}

func (res *FiveMResource) deriveFromManifest() {
	res.ManifestVersion = ""
	res.FXVersion = ""
	res.Games = res.Games[:0]
	res.UseExperimentalOAL = false
	res.Dependencies = res.Dependencies[:0]
	res.Provides = res.Provides[:0]
	res.UIPage = ""
	res.ServerOnly = false
	res.IsCfxV2 = false
	res.ClientGlobs = res.ClientGlobs[:0]
	res.ServerGlobs = res.ServerGlobs[:0]
	res.SharedGlobs = res.SharedGlobs[:0]
	res.ClientCrossIncludes = res.ClientCrossIncludes[:0]
	res.ServerCrossIncludes = res.ServerCrossIncludes[:0]
	res.SharedCrossIncludes = res.SharedCrossIncludes[:0]
	res.ClientExports = res.ClientExports[:0]
	res.ServerExports = res.ServerExports[:0]

	if res.Manifest == nil {
		return
	}

	res.ManifestURI = res.Manifest.SourceURI
	res.ManifestSource = res.Manifest.SourceKind
	res.IsCfxV2 = res.Manifest.IsCfxV2

	for _, entry := range res.Manifest.Entries {
		if entry.LoaderInjected {
			if entry.EmittedName == "is_cfxv2" {
				res.IsCfxV2 = parseFiveMBoolSetting(entry.Value)
			}

			continue
		}

		if entry.ReservedKey {
			continue
		}

		switch entry.EmittedName {
		case "resource_manifest_version":
			if entry.Value != "" {
				res.ManifestVersion = entry.Value
			}
		case "fx_version":
			if entry.Value != "" {
				res.FXVersion = entry.Value
			}
		case "game":
			if game := normalizeFiveMGameName(entry.Value); game != "" {
				res.Games = append(res.Games, game)
			}
		case "use_experimental_fxv2_oal":
			res.UseExperimentalOAL = parseFiveMBoolSetting(entry.Value)
		case "dependency", "dependencie":
			if entry.Value != "" {
				res.Dependencies = append(res.Dependencies, entry.Value)
			}
		case "provide":
			if entry.Value != "" {
				res.Provides = append(res.Provides, entry.Value)
			}
		case "ui_page":
			if entry.Value != "" {
				res.UIPage = entry.Value
			}
		case "server_only":
			res.ServerOnly = parseFiveMBoolSetting(entry.Value)
		case "client_script":
			if strings.HasPrefix(entry.Value, "@") {
				res.ClientCrossIncludes = append(res.ClientCrossIncludes, entry.Value)
			} else if entry.Value != "" {
				res.ClientGlobs = append(res.ClientGlobs, entry.Value)
			}
		case "server_script":
			if strings.HasPrefix(entry.Value, "@") {
				res.ServerCrossIncludes = append(res.ServerCrossIncludes, entry.Value)
			} else if entry.Value != "" {
				res.ServerGlobs = append(res.ServerGlobs, entry.Value)
			}
		case "shared_script", "file":
			if strings.HasPrefix(entry.Value, "@") {
				res.SharedCrossIncludes = append(res.SharedCrossIncludes, entry.Value)
			} else if entry.Value != "" {
				res.SharedGlobs = append(res.SharedGlobs, entry.Value)
			}
		case "export", "client_export":
			if entry.Value != "" {
				res.ClientExports = append(res.ClientExports, entry.Value)
			}
		case "server_export":
			if entry.Value != "" {
				res.ServerExports = append(res.ServerExports, entry.Value)
			}
		}
	}
}

func (s *Server) parseFiveMManifest(doc *Document) *FiveMResource {
	manifest := &FiveMManifest{
		SourceURI:  doc.URI,
		SourceKind: fiveMManifestSourceFromURI(doc.URI),
	}

	manifest.ResourceRoot = doc.URI[:strings.LastIndex(doc.URI, "/")]
	parts := strings.Split(manifest.ResourceRoot, "/")
	manifest.ResourceName = strings.ToLower(parts[len(parts)-1])

	res := &FiveMResource{
		Name:           manifest.ResourceName,
		RootURI:        manifest.ResourceRoot,
		ManifestURI:    manifest.SourceURI,
		ManifestSource: manifest.SourceKind,
		Manifest:       manifest,
	}

	for i := 1; i < len(doc.Tree.Nodes); i++ {
		node := doc.Tree.Nodes[i]
		if (node.Kind == ast.KindCallExpr || node.Kind == ast.KindMethodCall) && !(node.Parent != ast.InvalidNode && int(node.Parent) < len(doc.Tree.Nodes) && (doc.Tree.Nodes[node.Parent].Kind == ast.KindCallExpr || doc.Tree.Nodes[node.Parent].Kind == ast.KindMethodCall) && doc.Tree.Nodes[node.Parent].Left == ast.NodeID(i)) {
			_, _, _, entries, ok := s.fiveMManifestEntriesForCall(doc, ast.NodeID(i))
			if ok {
				manifest.Entries = append(manifest.Entries, entries...)
			}
		}
	}

	if manifest.SourceKind == FiveMManifestSourceFXManifest {
		manifest.IsCfxV2 = true
		manifest.Entries = append(manifest.Entries, FiveMManifestEntry{
			SourceURI:      manifest.SourceURI,
			RawName:        "is_cfxv2",
			EmittedName:    "is_cfxv2",
			NormalizedName: "is_cfxv2",
			Shape:          FiveMManifestEntryInjected,
			RawValue:       "true",
			Value:          "true",
			ReservedKey:    true,
			LoaderInjected: true,
		})
	}

	res.deriveFromManifest()

	return res
}

func (s *Server) registerFiveMManifestResource(res *FiveMResource) *FiveMResource {
	if res == nil {
		return nil
	}

	if s.FiveMResourceGraph == nil {
		s.FiveMResourceGraph = NewFiveMResourceGraph()
	}

	active := s.FiveMResourceGraph.Register(res)
	if active == nil {
		return nil
	}

	clear(s.FiveMResources)
	for root, node := range s.FiveMResourceGraph.ByRoot {
		s.FiveMResources[root] = node.Resource
	}

	clear(s.FiveMResourceByName)
	for name, node := range s.FiveMResourceGraph.ByName {
		s.FiveMResourceByName[name] = node.Resource
	}

	return active
}

func (s *Server) getDocFileEnv(res *FiveMResource, doc *Document) FileEnv {
	_ = res

	return s.getDocumentFiveMProfile(doc).Env()
}

func (s *Server) getDocumentFiveMProfile(doc *Document) FiveMExecutionProfile {
	if doc == nil {
		return FiveMExecutionProfile{}
	}

	if doc.FiveMProfileCached {
		return doc.FiveMProfile
	}

	profile := FiveMExecutionProfile{Kind: FiveMProfilePlainLua}

	if !s.FeatureFiveM || doc.IsLibrary {
		doc.FiveMProfile = profile
		doc.FiveMProfileCached = true

		return profile
	}

	if doc.IsFiveMManifest {
		profile.Kind = FiveMProfileManifest
		profile.ResourceRoot = doc.URI[:strings.LastIndex(doc.URI, "/")]
		profile.ResourceName = fiveMResourceNameFromRoot(profile.ResourceRoot)

		if res := s.FiveMResources[profile.ResourceRoot]; res != nil && res.Name != "" {
			profile.ResourceName = res.Name
		}

		doc.FiveMProfile = profile
		doc.FiveMProfileCached = true

		return profile
	}

	root, res := s.findFiveMResource(doc)
	if root != "" {
		profile.ResourceRoot = root
		profile.ResourceName = fiveMResourceNameFromRoot(root)

		if res != nil && res.Name != "" {
			profile.ResourceName = res.Name
		}

		switch s.classifyDocumentEnv(res, doc) {
		case EnvClient:
			profile.Kind = FiveMProfileClient
		case EnvServer:
			profile.Kind = FiveMProfileServer
		case EnvShared:
			profile.Kind = FiveMProfileShared
		}
	}

	doc.FiveMProfile = profile
	doc.FiveMProfileCached = true

	return profile
}

func (s *Server) getFiveMExportBridgeProfile(doc *Document) FiveMExecutionProfile {
	return s.getDocumentFiveMProfile(doc).ExportBridge()
}

func (s *Server) isFiveMGlobalAvailable(doc *Document, name string) bool {
	return s.getDocumentFiveMProfile(doc).AllowsFiveMGlobal(name)
}

func (s *Server) hasFiveMExportBridge(doc *Document) bool {
	return s.getFiveMExportBridgeProfile(doc).Kind == FiveMProfileExportBridge
}

func (s *Server) getFiveMNativeSelection(doc *Document) FiveMNativeSelection {
	profile := s.getDocumentFiveMProfile(doc)
	if profile.Kind != FiveMProfileClient && profile.Kind != FiveMProfileServer {
		return FiveMNativeSelection{}
	}

	res := s.resolveFiveMResourceByRoot(profile.ResourceRoot)
	if res == nil {
		return FiveMNativeSelection{}
	}

	return res.NativeSelection(profile)
}

func (s *Server) findFiveMResource(doc *Document) (string, *FiveMResource) {
	var bestRoot string

	for root := range s.FiveMResources {
		if strings.HasPrefix(doc.URI, root+"/") || doc.URI == root {
			if len(root) > len(bestRoot) {
				bestRoot = root
			}
		}
	}

	if bestRoot == "" {
		return "", nil
	}

	return bestRoot, s.resolveFiveMResourceByRoot(bestRoot)
}

func (s *Server) resolveFiveMResource(name string) *FiveMResource {
	if s == nil || s.FiveMResourceGraph == nil {
		return nil
	}

	return s.FiveMResourceGraph.Resolve(name)
}

func (s *Server) resolveFiveMResourceByRoot(root string) *FiveMResource {
	if s == nil || s.FiveMResourceGraph == nil {
		return s.FiveMResources[root]
	}

	if res := s.FiveMResourceGraph.ResourceByRoot(root); res != nil {
		return res
	}

	return s.FiveMResources[root]
}

func (s *Server) getFiveMResourceGraphNode(root string) *FiveMResourceGraphNode {
	if s == nil || s.FiveMResourceGraph == nil {
		return nil
	}

	return s.FiveMResourceGraph.NodeByRoot(root)
}

func (s *Server) getFiveMResourceNames() []string {
	if s == nil || s.FiveMResourceGraph == nil {
		return nil
	}

	return s.FiveMResourceGraph.PublicNames()
}

func (s *Server) canSeeFiveMCrossResourceInclude(srcProfile FiveMExecutionProfile, tgtDoc *Document) bool {
	if s == nil || s.FiveMResourceGraph == nil || tgtDoc == nil || srcProfile.ResourceRoot == "" {
		return false
	}

	srcNode := s.getFiveMResourceGraphNode(srcProfile.ResourceRoot)
	if srcNode == nil {
		return false
	}

	tgtProfile := s.getDocumentFiveMProfile(tgtDoc)
	tgtRoot := tgtProfile.ResourceRoot
	if tgtRoot == "" {
		return false
	}

	var relPath string
	if len(tgtDoc.URI) > len(tgtRoot) {
		relPath = tgtDoc.URI[len(tgtRoot)+1:]
	}

	if relPath == "" {
		return false
	}

	for _, include := range srcNode.allowedIncludes(srcProfile.Env()) {
		if include.IncludeResource == "" || include.IncludePath == "" {
			continue
		}

		resolved := s.resolveFiveMResource(include.IncludeResource)
		if resolved == nil || resolved.RootURI != tgtRoot {
			continue
		}

		if include.IncludePath == relPath || matchGlob(include.IncludePath, relPath) {
			return true
		}
	}

	return false
}

func (s *Server) classifyDocumentEnv(res *FiveMResource, doc *Document) FileEnv {
	if res == nil {
		return EnvUnknown
	}

	var relPath string

	if len(doc.URI) > len(res.RootURI) {
		relPath = doc.URI[len(res.RootURI)+1:]
	} else {
		relPath = ""
	}

	var env FileEnv = EnvUnknown

	for _, glob := range res.SharedGlobs {
		if matchGlob(glob, relPath) {
			env = EnvShared
			break
		}
	}

	if env == EnvUnknown {
		var (
			isClient bool
			isServer bool
		)

		for _, glob := range res.ClientGlobs {
			if matchGlob(glob, relPath) {
				isClient = true

				break
			}
		}

		for _, glob := range res.ServerGlobs {
			if matchGlob(glob, relPath) {
				isServer = true

				break
			}
		}

		if isClient && isServer {
			env = EnvShared
		} else if isClient {
			env = EnvClient
		} else if isServer {
			env = EnvServer
		}
	}

	return env
}

func fiveMResourceNameFromRoot(root string) string {
	idx := strings.LastIndexByte(root, '/')
	if idx == -1 {
		return strings.ToLower(root)
	}

	return strings.ToLower(root[idx+1:])
}
