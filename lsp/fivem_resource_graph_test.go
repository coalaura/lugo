package lsp

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestFiveMResourceGraph(t *testing.T) {
	s, root := newFiveMProfileTestServer(t)

	manifestDoc := addFiveMTestDocument(t, s, filepath.Join(root, "graph_resource", "fxmanifest.lua"), `
fx_version 'cerulean'
dependency 'base'
client_script 'client.lua'
provide 'inventory'
shared_script '@shared_lib/bootstrap.lua'
file 'web/index.html'
server_script '@bridge/server.lua'
shared_script 'shared.lua'
dependencies { 'dep-two', 'dep-three' }
`)

	rootURI := s.pathToURI(filepath.Join(root, "graph_resource"))
	node := s.getFiveMResourceGraphNode(rootURI)
	if node == nil {
		t.Fatal("resource graph node was not registered")
	}

	if node.Name != "graph_resource" {
		t.Fatalf("graph node name = %q, want graph_resource", node.Name)
	}

	if node.ManifestURI != manifestDoc.URI || node.ManifestSource != FiveMManifestSourceFXManifest {
		t.Fatalf("graph node manifest winner = (%s, %s), want (%s, %s)", node.ManifestSource.String(), node.ManifestURI, FiveMManifestSourceFXManifest.String(), manifestDoc.URI)
	}

	if !slices.Equal(node.Dependencies, []string{"base", "dep-two", "dep-three"}) {
		t.Fatalf("graph dependencies = %#v, want ordered base/dep-two/dep-three", node.Dependencies)
	}

	if !slices.Equal(node.Provides, []string{"inventory"}) {
		t.Fatalf("graph provides = %#v, want inventory", node.Provides)
	}

	if len(node.ClientEntries) != 1 || node.ClientEntries[0].Kind != FiveMResourceGraphExpansionLocal || node.ClientEntries[0].Pattern != "client.lua" {
		t.Fatalf("client graph entries = %#v, want single local client.lua expansion", node.ClientEntries)
	}

	if len(node.ServerEntries) != 1 || node.ServerEntries[0].Kind != FiveMResourceGraphExpansionInclude || node.ServerEntries[0].IncludeResource != "bridge" || node.ServerEntries[0].IncludePath != "server.lua" {
		t.Fatalf("server graph entries = %#v, want include @bridge/server.lua", node.ServerEntries)
	}

	if len(node.SharedEntries) != 3 {
		t.Fatalf("shared graph entries len = %d, want 3", len(node.SharedEntries))
	}

	if node.SharedEntries[0].Kind != FiveMResourceGraphExpansionInclude || node.SharedEntries[0].Include != "@shared_lib/bootstrap.lua" || node.SharedEntries[0].IncludeResource != "shared_lib" || node.SharedEntries[0].IncludePath != "bootstrap.lua" {
		t.Fatalf("first shared graph entry = %#v, want include @shared_lib/bootstrap.lua", node.SharedEntries[0])
	}

	if node.SharedEntries[1].Kind != FiveMResourceGraphExpansionLocal || node.SharedEntries[1].Pattern != "web/index.html" {
		t.Fatalf("second shared graph entry = %#v, want local web/index.html", node.SharedEntries[1])
	}

	if node.SharedEntries[2].Kind != FiveMResourceGraphExpansionLocal || node.SharedEntries[2].Pattern != "shared.lua" {
		t.Fatalf("third shared graph entry = %#v, want local shared.lua", node.SharedEntries[2])
	}

	if got := s.resolveFiveMResource("graph_resource"); got == nil || got.RootURI != rootURI {
		t.Fatalf("canonical graph lookup = %+v, want graph_resource at %s", got, rootURI)
	}

	if got := s.resolveFiveMResource("inventory"); got == nil || got.RootURI != rootURI {
		t.Fatalf("provide alias graph lookup = %+v, want graph_resource at %s", got, rootURI)
	}

	if names := s.getFiveMResourceNames(); !slices.Equal(names, []string{"graph_resource", "inventory"}) {
		t.Fatalf("public graph names = %#v, want graph_resource and inventory", names)
	}
}

func TestFiveMResourceGraphIncludes(t *testing.T) {
	h := newFiveMFixtureHarness(t, "resource_graph_includes")

	h.requireSingleDefinitionAt("graph_include_alias_ref", "graph_include_alias_definition")

	consumerDoc := h.docForMarker("graph_include_alias_ref")
	providerDoc := h.docForMarker("graph_include_alias_definition")
	if !h.server.canSeeSymbol(consumerDoc, providerDoc) {
		t.Fatal("consumer should see provider document through graph-backed include alias resolution")
	}

	providerRoot := h.server.getDocResourceRoot(providerDoc)
	if got := h.server.resolveFiveMResource("inventory"); got == nil || got.RootURI != providerRoot {
		t.Fatalf("include alias resolution = %+v, want provider root %s", got, providerRoot)
	}

	consumerNode := h.server.getFiveMResourceGraphNode(h.server.getDocResourceRoot(consumerDoc))
	if consumerNode == nil {
		t.Fatal("consumer resource graph node missing")
	}

	var include *FiveMResourceGraphExpansion
	for i := range consumerNode.SharedEntries {
		entry := &consumerNode.SharedEntries[i]
		if entry.Kind == FiveMResourceGraphExpansionInclude {
			include = entry
			break
		}
	}

	if include == nil {
		t.Fatal("consumer graph did not preserve the shared include entry")
	}

	if include.Include != "@inventory/shared.lua" || include.IncludeResource != "inventory" || include.IncludePath != "shared.lua" {
		t.Fatalf("shared include entry = %#v, want graph-backed alias target for @inventory/shared.lua", *include)
	}
}

func TestFiveMResourceGraphProvideFallback(t *testing.T) {
	h := newFiveMFixtureHarness(t, "resource_provides")

	h.requireSingleDefinitionAt("provide_fallback_ping_call", "provide_canonical_ping_definition")

	canonicalDoc := h.docForMarker("provide_canonical_ping_definition")
	providerDoc := h.docForMarker("provide_alias_ping_definition")
	resolved := h.server.resolveFiveMResource("inventory")
	if resolved == nil {
		t.Fatal("provide fallback alias should resolve through the resource graph")
	}

	canonicalRoot := h.server.getDocResourceRoot(canonicalDoc)
	if resolved.RootURI != canonicalRoot {
		t.Fatalf("inventory lookup root = %s, want canonical inventory root %s", resolved.RootURI, canonicalRoot)
	}

	providerRoot := h.server.getDocResourceRoot(providerDoc)
	if providerRoot == canonicalRoot {
		t.Fatal("fixture setup error: alias provider and canonical inventory must be different resources")
	}

	providerNode := h.server.getFiveMResourceGraphNode(providerRoot)
	if providerNode == nil || !slices.Contains(providerNode.Provides, "inventory") {
		t.Fatalf("provider graph node = %#v, want provide alias inventory", providerNode)
	}

	if names := h.server.getFiveMResourceNames(); !slices.Equal(names, []string{"inventory", "provide_consumer", "provide_resource"}) {
		t.Fatalf("public names = %#v, want canonical inventory plus other canonical resources without duplicate alias shadowing", names)
	}
}
