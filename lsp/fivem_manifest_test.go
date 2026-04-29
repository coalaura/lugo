package lsp

import (
	"path/filepath"
	"testing"
)

func TestFiveMManifestModel(t *testing.T) {
	s, root := newFiveMProfileTestServer(t)

	manifestDoc := addFiveMTestDocument(t, s, filepath.Join(root, "resource", "fxmanifest.lua"), `
fx_version 'cerulean'
client_scripts { 'client.lua', '@common/shared.lua' }
exports { 'ping', 'pong' }
widget('value')('kind', { nested = true })
is_cfxv2 'false'
`)

	res := s.parseFiveMManifest(manifestDoc)
	if res.Manifest == nil {
		t.Fatal("manifest metadata was not populated")
	}

	if res.Manifest.SourceKind != FiveMManifestSourceFXManifest {
		t.Fatalf("manifest source kind = %s, want %s", res.Manifest.SourceKind.String(), FiveMManifestSourceFXManifest.String())
	}

	entries := res.Manifest.Entries
	if len(entries) != 9 {
		t.Fatalf("manifest entry count = %d, want 9", len(entries))
	}

	if entries[0].EmittedName != "fx_version" || entries[0].Value != "cerulean" || entries[0].Shape != FiveMManifestEntryScalar {
		t.Fatalf("first manifest entry = %+v, want fx_version scalar cerulean", entries[0])
	}

	if entries[1].EmittedName != "client_script" || entries[1].NormalizedName != "client_script" || entries[1].Value != "client.lua" || entries[1].Shape != FiveMManifestEntryTableValue {
		t.Fatalf("second manifest entry = %+v, want first normalized client_script table value", entries[1])
	}

	if entries[2].EmittedName != "client_script" || entries[2].Value != "@common/shared.lua" {
		t.Fatalf("third manifest entry = %+v, want second client_script value", entries[2])
	}

	if entries[3].EmittedName != "export" || entries[3].Value != "ping" || entries[4].EmittedName != "export" || entries[4].Value != "pong" {
		t.Fatalf("export entries = %+v / %+v, want ping then pong", entries[3], entries[4])
	}

	if entries[5].EmittedName != "widget" || entries[5].Value != "value" || entries[5].Shape != FiveMManifestEntryScalar {
		t.Fatalf("widget entry = %+v, want scalar widget metadata", entries[5])
	}

	if entries[6].EmittedName != "widget_extra" || entries[6].ExtraKey != "kind" || entries[6].RawValue != "{ nested = true }" || entries[6].Shape != FiveMManifestEntryExtra {
		t.Fatalf("widget extra entry = %+v, want preserved _extra payload", entries[6])
	}

	if entries[7].EmittedName != "is_cfxv2" || entries[7].Value != "false" || !entries[7].ReservedKey || entries[7].LoaderInjected {
		t.Fatalf("reserved user entry = %+v, want preserved rejected is_cfxv2 metadata attempt", entries[7])
	}

	if entries[0].Range.Start.Line != 1 || entries[1].Range.Start.Line != 2 || entries[6].Range.Start.Line != 4 {
		t.Fatalf("manifest entry source lines = [%d %d %d], want [1 2 4]", entries[0].Range.Start.Line, entries[1].Range.Start.Line, entries[6].Range.Start.Line)
	}

	if !entries[8].LoaderInjected || entries[8].EmittedName != "is_cfxv2" || entries[8].Value != "true" || !entries[8].ReservedKey {
		t.Fatalf("loader entry = %+v, want injected is_cfxv2=true", entries[8])
	}

	if res.ClientGlobs[0] != "client.lua" || res.ClientCrossIncludes[0] != "@common/shared.lua" || res.ClientExports[0] != "ping" || res.ClientExports[1] != "pong" {
		t.Fatalf("derived manifest projections = %+v, want client globs/cross-includes/exports derived from canonical metadata", res)
	}
}

func TestFiveMManifestProbePrecedence(t *testing.T) {
	s, root := newFiveMProfileTestServer(t)

	legacyDoc := addFiveMTestDocument(t, s, filepath.Join(root, "resource", "__resource.lua"), `client_script 'legacy.lua'`)
	legacyRes := s.parseFiveMManifest(legacyDoc)
	legacyActive := s.registerFiveMManifestResource(legacyRes)
	if legacyActive.ManifestSource != FiveMManifestSourceResourceLua {
		t.Fatalf("legacy-only manifest source = %s, want %s", legacyActive.ManifestSource.String(), FiveMManifestSourceResourceLua.String())
	}

	modernDoc := addFiveMTestDocument(t, s, filepath.Join(root, "resource", "fxmanifest.lua"), `client_script 'modern.lua'`)
	modernRes := s.parseFiveMManifest(modernDoc)
	activeRes := s.registerFiveMManifestResource(modernRes)

	if activeRes.ManifestSource != FiveMManifestSourceFXManifest {
		t.Fatalf("active manifest source = %s, want %s", activeRes.ManifestSource.String(), FiveMManifestSourceFXManifest.String())
	}

	if activeRes.ManifestURI != modernDoc.URI {
		t.Fatalf("active manifest uri = %s, want %s", activeRes.ManifestURI, modernDoc.URI)
	}

	if len(activeRes.ClientGlobs) != 1 || activeRes.ClientGlobs[0] != "modern.lua" {
		t.Fatalf("active client globs = %#v, want only modern.lua", activeRes.ClientGlobs)
	}

	rootURI := s.pathToURI(filepath.Join(root, "resource"))
	if got := s.FiveMResources[rootURI]; got == nil || got.ManifestSource != FiveMManifestSourceFXManifest {
		t.Fatalf("resource map winner = %+v, want fxmanifest.lua to win probe precedence", got)
	}
}

func TestFiveMManifestDerivedSettings(t *testing.T) {
	s, root := newFiveMProfileTestServer(t)

	manifestDoc := addFiveMTestDocument(t, s, filepath.Join(root, "resource", "fxmanifest.lua"), `
fx_version 'cerulean'
use_experimental_fxv2_oal 'yes'
dependency 'base'
dependencies { 'lib-one', 'lib-two' }
provide 'inventory'
ui_page 'html/index.html'
server_only 'yes'
client_script 'client.lua'
server_scripts { 'server.lua', '@bridge/server.lua' }
shared_scripts { 'shared.lua', '@common/shared.lua' }
files { 'html/index.html', '@common/ui.css' }
exports { 'ping' }
server_exports { 'pong' }
`)

	res := s.parseFiveMManifest(manifestDoc)

	if res.FXVersion != "cerulean" {
		t.Fatalf("fx_version = %q, want cerulean", res.FXVersion)
	}

	if !res.UseExperimentalOAL {
		t.Fatal("use_experimental_fxv2_oal should derive as true")
	}

	if len(res.Dependencies) != 3 || res.Dependencies[0] != "base" || res.Dependencies[1] != "lib-one" || res.Dependencies[2] != "lib-two" {
		t.Fatalf("dependencies = %#v, want base/lib-one/lib-two", res.Dependencies)
	}

	if len(res.Provides) != 1 || res.Provides[0] != "inventory" {
		t.Fatalf("provides = %#v, want inventory", res.Provides)
	}

	if res.UIPage != "html/index.html" {
		t.Fatalf("ui_page = %q, want html/index.html", res.UIPage)
	}

	if !res.ServerOnly {
		t.Fatal("server_only should derive as true")
	}

	if len(res.ClientGlobs) != 1 || res.ClientGlobs[0] != "client.lua" {
		t.Fatalf("client globs = %#v, want client.lua", res.ClientGlobs)
	}

	if len(res.ServerGlobs) != 1 || res.ServerGlobs[0] != "server.lua" {
		t.Fatalf("server globs = %#v, want server.lua", res.ServerGlobs)
	}

	if len(res.ServerCrossIncludes) != 1 || res.ServerCrossIncludes[0] != "@bridge/server.lua" {
		t.Fatalf("server cross includes = %#v, want @bridge/server.lua", res.ServerCrossIncludes)
	}

	if len(res.SharedGlobs) != 2 || res.SharedGlobs[0] != "shared.lua" || res.SharedGlobs[1] != "html/index.html" {
		t.Fatalf("shared globs = %#v, want shared.lua and html/index.html", res.SharedGlobs)
	}

	if len(res.SharedCrossIncludes) != 2 || res.SharedCrossIncludes[0] != "@common/shared.lua" || res.SharedCrossIncludes[1] != "@common/ui.css" {
		t.Fatalf("shared cross includes = %#v, want shared/ui cross-resource entries", res.SharedCrossIncludes)
	}

	if len(res.ClientExports) != 1 || res.ClientExports[0] != "ping" {
		t.Fatalf("client exports = %#v, want ping", res.ClientExports)
	}

	if len(res.ServerExports) != 1 || res.ServerExports[0] != "pong" {
		t.Fatalf("server exports = %#v, want pong", res.ServerExports)
	}

	if res.Manifest == nil || !res.Manifest.IsCfxV2 || res.Manifest.SourceKind != FiveMManifestSourceFXManifest {
		t.Fatalf("manifest metadata = %+v, want fxmanifest-backed cfxv2 metadata", res.Manifest)
	}
}
