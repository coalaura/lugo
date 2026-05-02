package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/coalaura/lugo/ast"
)

func TestFiveMNativeHelpers(t *testing.T) {
	s, root := newFiveMProfileTestServer(t)
	indexEmbeddedStdlibForTest(t, s)

	addFiveMTestDocument(t, s, filepath.Join(root, "native_resource", "fxmanifest.lua"), `
fx_version 'cerulean'
game 'gta5'
use_experimental_fxv2_oal 'yes'

client_script 'client.lua'
server_script 'server.lua'
shared_script 'shared.lua'
`)

	clientDoc := addFiveMTestDocument(t, s, filepath.Join(root, "native_resource", "client.lua"), `
local invokeResult = Citizen.InvokeNative(0x12345678)
local invokeNative2 = Citizen.InvokeNative2(0x12345678)
local getNativeBinding = Citizen.GetNative(0x12345678)
local loadNativeBinding = Citizen.LoadNative('PlayerPedId')
local pointerValue = Citizen.PointerValueInt()
local pointerValueInitialized = Citizen.PointerValueFloatInitialized()
local resultHint = Citizen.ResultAsInteger()
local resultObjectHint = Citizen.ResultAsObject2()

return invokeResult, invokeNative2, getNativeBinding, loadNativeBinding, pointerValue, pointerValueInitialized, resultHint, resultObjectHint
`)
	serverDoc := addFiveMTestDocument(t, s, filepath.Join(root, "native_resource", "server.lua"), `
local invokeResult = Citizen.InvokeNative(0x12345678)
local pointerValue = Citizen.PointerValueVector()
local resultHint = Citizen.ResultAsString()

return invokeResult, pointerValue, resultHint
`)
	sharedDoc := addFiveMTestDocument(t, s, filepath.Join(root, "native_resource", "shared.lua"), `
local invokeResult = Citizen.InvokeNative(0x12345678)
local pointerValue = Citizen.PointerValueFloat()
local resultHint = Citizen.ResultAsVector()
local maybeGetNative = Citizen.GetNative
local maybeInvokeNative2 = Citizen.InvokeNative2

return invokeResult, pointerValue, resultHint, maybeGetNative, maybeInvokeNative2
`)

	assertResolvedGlobalTarget(t, s, clientDoc, "InvokeNative", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "InvokeNative2", "std:///fivem/client.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "GetNative", "std:///fivem/client.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "LoadNative", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "PointerValueInt", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "PointerValueFloatInitialized", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "ResultAsInteger", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "ResultAsObject2", "std:///fivem/shared.lua")

	assertResolvedGlobalTarget(t, s, serverDoc, "InvokeNative", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "PointerValueVector", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "ResultAsString", "std:///fivem/shared.lua")

	assertResolvedGlobalTarget(t, s, sharedDoc, "InvokeNative", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, sharedDoc, "PointerValueFloat", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, sharedDoc, "ResultAsVector", "std:///fivem/shared.lua")
	assertUnresolvedGlobal(t, s, sharedDoc, "GetNative")
	assertUnresolvedGlobal(t, s, sharedDoc, "InvokeNative2")

	if got := clientDoc.InferType(mustFindIdentNode(t, clientDoc, "invokeResult")); got.Basics&TypeAny == 0 {
		t.Fatalf("invokeResult type = %+v, want any-returning helper metadata", got)
	}
	if got := clientDoc.InferType(mustFindIdentNode(t, clientDoc, "invokeNative2")); got.Basics&TypeAny == 0 {
		t.Fatalf("invokeNative2 type = %+v, want any-returning helper metadata", got)
	}
	if got := clientDoc.InferType(mustFindIdentNode(t, clientDoc, "getNativeBinding")); got.Basics&TypeFunction == 0 || got.Basics&TypeNil == 0 {
		t.Fatalf("getNativeBinding type = %+v, want function|nil", got)
	}
	if got := clientDoc.InferType(mustFindIdentNode(t, clientDoc, "loadNativeBinding")); got.Basics&TypeFunction == 0 || got.Basics&TypeString == 0 || got.Basics&TypeNil == 0 {
		t.Fatalf("loadNativeBinding type = %+v, want function|string|nil", got)
	}
	if got := clientDoc.InferType(mustFindIdentNode(t, clientDoc, "pointerValue")); got.Basics&TypeUserdata == 0 {
		t.Fatalf("pointerValue type = %+v, want userdata", got)
	}
	if got := clientDoc.InferType(mustFindIdentNode(t, clientDoc, "pointerValueInitialized")); got.Basics&TypeUserdata == 0 {
		t.Fatalf("pointerValueInitialized type = %+v, want userdata", got)
	}
	if got := clientDoc.InferType(mustFindIdentNode(t, clientDoc, "resultHint")); got.Basics&TypeUserdata == 0 {
		t.Fatalf("resultHint type = %+v, want userdata", got)
	}
	if got := clientDoc.InferType(mustFindIdentNode(t, clientDoc, "resultObjectHint")); got.Basics&TypeUserdata == 0 {
		t.Fatalf("resultObjectHint type = %+v, want userdata", got)
	}

	invokeCtx := s.resolveSymbolNode(clientDoc.URI, clientDoc, mustFindIdentNode(t, clientDoc, "InvokeNative"))
	if invokeCtx == nil || invokeCtx.TargetDoc == nil || invokeCtx.TargetDefID == 0 {
		t.Fatal("InvokeNative did not resolve to helper metadata")
	}
	invokeLuaDoc := invokeCtx.TargetDoc.GetLuaDoc(invokeCtx.TargetDefID)
	if invokeLuaDoc == nil || !strings.Contains(invokeLuaDoc.Description, "natives_universal.lua") {
		t.Fatalf("InvokeNative LuaDoc = %+v, want native build-selection docs", invokeLuaDoc)
	}

	loadCtx := s.resolveSymbolNode(clientDoc.URI, clientDoc, mustFindIdentNode(t, clientDoc, "LoadNative"))
	if loadCtx == nil || loadCtx.TargetDoc == nil || loadCtx.TargetDefID == 0 {
		t.Fatal("LoadNative did not resolve to helper metadata")
	}
	loadLuaDoc := loadCtx.TargetDoc.GetLuaDoc(loadCtx.TargetDefID)
	if loadLuaDoc == nil || !strings.Contains(loadLuaDoc.Description, "use_experimental_fxv2_oal") {
		t.Fatalf("LoadNative LuaDoc = %+v, want OAL helper docs", loadLuaDoc)
	}
}

func TestFiveMNativeBuildSelection(t *testing.T) {
	tests := []struct {
		name         string
		resourceDir  string
		manifestName string
		manifest     string
		docName      string
		source       string
		wantBuild    string
		wantFamily   FiveMNativeFamily
	}{
		{
			name:         "modern gta client",
			resourceDir:  "modern_gta",
			manifestName: "fxmanifest.lua",
			manifest:     "fx_version 'cerulean'\ngame 'gta5'\nclient_script 'client.lua'\n",
			docName:      "client.lua",
			source:       "return PlayerPedId()",
			wantBuild:    "natives_universal.lua",
			wantFamily:   FiveMNativeFamilyGTA5,
		},
		{
			name:         "modern gta server",
			resourceDir:  "modern_server",
			manifestName: "fxmanifest.lua",
			manifest:     "fx_version 'cerulean'\ngame 'gta5'\nserver_script 'server.lua'\n",
			docName:      "server.lua",
			source:       "return GetInvokingResource()",
			wantBuild:    "natives_server.lua",
			wantFamily:   FiveMNativeFamilyServer,
		},
		{
			name:         "legacy manifest gta client",
			resourceDir:  "legacy_manifest",
			manifestName: "__resource.lua",
			manifest:     "resource_manifest_version 'f15e72ec-3972-4fe4-9c7d-afc5394ae207'\nclient_script 'client.lua'\n",
			docName:      "client.lua",
			source:       "return GetVehicleMaxNumberOfPassengers(0)",
			wantBuild:    "natives_0193d0af.lua",
			wantFamily:   FiveMNativeFamilyGTA5,
		},
		{
			name:         "legacy baseline gta client",
			resourceDir:  "legacy_base",
			manifestName: "__resource.lua",
			manifest:     "client_script 'client.lua'\n",
			docName:      "client.lua",
			source:       "return GetDisplayNameFromVehicleModel(0)",
			wantBuild:    "natives_21e43a33.lua",
			wantFamily:   FiveMNativeFamilyGTA5,
		},
		{
			name:         "rdr3 client",
			resourceDir:  "rdr_resource",
			manifestName: "fxmanifest.lua",
			manifest:     "fx_version 'cerulean'\ngame 'rdr3'\nclient_script 'client.lua'\n",
			docName:      "client.lua",
			source:       "return GetPlayerPed(0)",
			wantBuild:    "rdr3_universal.lua",
			wantFamily:   FiveMNativeFamilyRDR3,
		},
		{
			name:         "ny client",
			resourceDir:  "ny_resource",
			manifestName: "fxmanifest.lua",
			manifest:     "fx_version 'cerulean'\ngame 'ny'\nclient_script 'client.lua'\n",
			docName:      "client.lua",
			source:       "return GetCharCoordinates(0)",
			wantBuild:    "ny_universal.lua",
			wantFamily:   FiveMNativeFamilyNY,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, root := newFiveMProfileTestServer(t)
			indexEmbeddedStdlibForTest(t, s)

			addFiveMTestDocument(t, s, filepath.Join(root, tc.resourceDir, tc.manifestName), tc.manifest)
			doc := addFiveMTestDocument(t, s, filepath.Join(root, tc.resourceDir, tc.docName), tc.source)

			selection := s.getFiveMNativeSelection(doc)
			if selection.Family != tc.wantFamily || selection.Build != tc.wantBuild {
				t.Fatalf("selection = %+v, want family=%v build=%s", selection, tc.wantFamily, tc.wantBuild)
			}
		})
	}
}

func TestFiveMNativeCatalogSelection(t *testing.T) {
	tests := []struct {
		name         string
		resourceDir  string
		manifestName string
		manifest     string
		docName      string
		source       string
		symbol       string
		wantBuild    string
		wantFamily   FiveMNativeFamily
	}{
		{
			name:         "modern gta client",
			resourceDir:  "modern_gta",
			manifestName: "fxmanifest.lua",
			manifest:     "fx_version 'cerulean'\ngame 'gta5'\nclient_script 'client.lua'\n",
			docName:      "client.lua",
			source:       "return PlayerPedId()",
			symbol:       "PlayerPedId",
			wantBuild:    "natives_universal.lua",
			wantFamily:   FiveMNativeFamilyGTA5,
		},
		{
			name:         "modern gta server",
			resourceDir:  "modern_server",
			manifestName: "fxmanifest.lua",
			manifest:     "fx_version 'cerulean'\ngame 'gta5'\nserver_script 'server.lua'\n",
			docName:      "server.lua",
			source:       "return GetInvokingResource()",
			symbol:       "GetInvokingResource",
			wantBuild:    "natives_server.lua",
			wantFamily:   FiveMNativeFamilyServer,
		},
		{
			name:         "legacy manifest gta client",
			resourceDir:  "legacy_manifest",
			manifestName: "__resource.lua",
			manifest:     "resource_manifest_version 'f15e72ec-3972-4fe4-9c7d-afc5394ae207'\nclient_script 'client.lua'\n",
			docName:      "client.lua",
			source:       "return GetVehicleMaxNumberOfPassengers(0)",
			symbol:       "GetVehicleMaxNumberOfPassengers",
			wantBuild:    "natives_0193d0af.lua",
			wantFamily:   FiveMNativeFamilyGTA5,
		},
		{
			name:         "legacy baseline gta client",
			resourceDir:  "legacy_base",
			manifestName: "__resource.lua",
			manifest:     "client_script 'client.lua'\n",
			docName:      "client.lua",
			source:       "return GetDisplayNameFromVehicleModel(0)",
			symbol:       "GetDisplayNameFromVehicleModel",
			wantBuild:    "natives_21e43a33.lua",
			wantFamily:   FiveMNativeFamilyGTA5,
		},
		{
			name:         "rdr3 client",
			resourceDir:  "rdr_resource",
			manifestName: "fxmanifest.lua",
			manifest:     "fx_version 'cerulean'\ngame 'rdr3'\nclient_script 'client.lua'\n",
			docName:      "client.lua",
			source:       "return GetPlayerPed(0)",
			symbol:       "GetPlayerPed",
			wantBuild:    "rdr3_universal.lua",
			wantFamily:   FiveMNativeFamilyRDR3,
		},
		{
			name:         "ny client",
			resourceDir:  "ny_resource",
			manifestName: "fxmanifest.lua",
			manifest:     "fx_version 'cerulean'\ngame 'ny'\nclient_script 'client.lua'\n",
			docName:      "client.lua",
			source:       "return GetCharCoordinates(0)",
			symbol:       "GetCharCoordinates",
			wantBuild:    "ny_universal.lua",
			wantFamily:   FiveMNativeFamilyNY,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, root := newFiveMProfileTestServer(t)
			indexEmbeddedStdlibForTest(t, s)

			addFiveMTestDocument(t, s, filepath.Join(root, tc.resourceDir, tc.manifestName), tc.manifest)
			doc := addFiveMTestDocument(t, s, filepath.Join(root, tc.resourceDir, tc.docName), tc.source)
			wantURI := requireFiveMNativeBundleURI(t, s, tc.wantBuild)

			selection := s.getFiveMNativeSelection(doc)
			if selection.Family != tc.wantFamily || selection.Build != tc.wantBuild {
				t.Fatalf("selection = %+v, want family=%v build=%s", selection, tc.wantFamily, tc.wantBuild)
			}

			if got := countLoadedFiveMNativeBundles(s); got != len(fiveMNativeBundleNames) {
				t.Fatalf("loaded native bundles before resolution = %d, want %d pre-indexed bundles", got, len(fiveMNativeBundleNames))
			}

			assertResolvedGlobalTarget(t, s, doc, tc.symbol, wantURI)

			if got := countLoadedFiveMNativeBundles(s); got != len(fiveMNativeBundleNames) {
				t.Fatalf("loaded native bundles after resolution = %d, want %d pre-indexed bundles", got, len(fiveMNativeBundleNames))
			}

			if _, ok := s.Documents[wantURI]; !ok {
				t.Fatalf("expected selected bundle %s to be indexed", wantURI)
			}
		})
	}
}

func TestFiveMLazyNativeResolution(t *testing.T) {
	h := newFiveMFixtureHarness(t, "resource_natives")
	wantURI := requireFiveMNativeBundleURI(t, h.server, "natives_universal.lua")

	if got := countLoadedFiveMNativeBundles(h.server); got != len(fiveMNativeBundleNames) {
		t.Fatalf("loaded native bundles before first lookup = %d, want %d pre-indexed bundles", got, len(fiveMNativeBundleNames))
	}

	if _, ok := h.server.GlobalIndex[GlobalKey{ReceiverHash: 0, PropHash: ast.HashBytes([]byte("PlayerPedId"))}]; !ok {
		t.Fatal("PlayerPedId should be indexed from the preloaded runtime native library")
	}

	ctx := h.resolve("native_client_call")
	if ctx == nil || ctx.TargetURI != wantURI {
		t.Fatalf("native_client_call resolved to %+v, want %s", ctx, wantURI)
	}

	if got := countLoadedFiveMNativeBundles(h.server); got != len(fiveMNativeBundleNames) {
		t.Fatalf("loaded native bundles after first lookup = %d, want %d pre-indexed bundles", got, len(fiveMNativeBundleNames))
	}

	if _, ok := h.server.Documents[wantURI]; !ok {
		t.Fatalf("expected %s document to be indexed", wantURI)
	}

	if _, ok := h.server.GlobalIndex[GlobalKey{ReceiverHash: 0, PropHash: ast.HashBytes([]byte("PlayerPedId"))}]; !ok {
		t.Fatal("PlayerPedId should remain indexed after resolution")
	}

	hover := h.hover("native_client_call")
	if hover == nil || !strings.Contains(hover.Contents.Value, "Returns the entity handle for the local player ped") {
		t.Fatalf("hover for native_client_call = %#v, want generated native docs", hover)
	}

	defs := h.definition("native_client_call")
	if len(defs) != 1 || defs[0].URI != wantURI {
		t.Fatalf("definitions for native_client_call = %+v, want %s", defs, wantURI)
	}
}

func TestFiveMNativeCatalogIsolation(t *testing.T) {
	h := newFiveMFixtureHarness(t, "resource_natives")
	clientURI := requireFiveMNativeBundleURI(t, h.server, "natives_universal.lua")
	serverURI := requireFiveMNativeBundleURI(t, h.server, "natives_server.lua")

	assertResolvedGlobalTarget(t, h.server, h.docForMarker("native_client_call"), "PlayerPedId", clientURI)
	assertResolvedGlobalTarget(t, h.server, h.docForMarker("native_client_legacy_hidden"), "GetVehicleMaxNumberOfPassengers", clientURI)
	assertResolvedGlobalTarget(t, h.server, h.docForMarker("native_client_server_hidden"), "GetInvokingResource", clientURI)

	assertResolvedGlobalTarget(t, h.server, h.docForMarker("native_server_call"), "GetInvokingResource", serverURI)
	assertUnresolvedGlobal(t, h.server, h.docForMarker("native_server_client_hidden"), "PlayerPedId")

	assertUnresolvedGlobal(t, h.server, h.docForMarker("native_plain_call"), "PlayerPedId")
	assertUnresolvedGlobal(t, h.server, h.docForMarker("native_plain_server_call"), "GetInvokingResource")
}

func TestFiveMNativeOALSelection(t *testing.T) {
	s, root := newFiveMProfileTestServer(t)
	indexEmbeddedStdlibForTest(t, s)

	addFiveMTestDocument(t, s, filepath.Join(root, "oal_enabled", "fxmanifest.lua"), `
fx_version 'cerulean'
game 'gta5'
use_experimental_fxv2_oal 'yes'
client_script 'client.lua'
`)
	enabledDoc := addFiveMTestDocument(t, s, filepath.Join(root, "oal_enabled", "client.lua"), `return Citizen.LoadNative, Citizen.GetNative`)

	addFiveMTestDocument(t, s, filepath.Join(root, "oal_disabled", "fxmanifest.lua"), `
fx_version 'cerulean'
game 'gta5'
client_script 'client.lua'
`)
	disabledDoc := addFiveMTestDocument(t, s, filepath.Join(root, "oal_disabled", "client.lua"), `return Citizen.LoadNative, Citizen.GetNative`)

	enabledSel := s.getFiveMNativeSelection(enabledDoc)
	if !enabledSel.UseExperimentalOAL || enabledSel.Build != "natives_universal.lua" {
		t.Fatalf("enabled OAL selection = %+v, want universal build with OAL enabled", enabledSel)
	}

	disabledSel := s.getFiveMNativeSelection(disabledDoc)
	if disabledSel.UseExperimentalOAL {
		t.Fatalf("disabled OAL selection = %+v, want OAL disabled", disabledSel)
	}

	loadCtx := s.resolveSymbolNode(enabledDoc.URI, enabledDoc, mustFindIdentNode(t, enabledDoc, "LoadNative"))
	if loadCtx == nil || loadCtx.TargetDoc == nil || loadCtx.TargetDefID == 0 {
		t.Fatal("LoadNative did not resolve in OAL-enabled resource")
	}
	loadLuaDoc := loadCtx.TargetDoc.GetLuaDoc(loadCtx.TargetDefID)
	if loadLuaDoc == nil || !strings.Contains(loadLuaDoc.Description, "direct OAL-style callable path") {
		t.Fatalf("LoadNative LuaDoc = %+v, want OAL path description", loadLuaDoc)
	}

	getNativeCtx := s.resolveSymbolNode(enabledDoc.URI, enabledDoc, mustFindIdentNode(t, enabledDoc, "GetNative"))
	if getNativeCtx == nil || getNativeCtx.TargetDoc == nil || getNativeCtx.TargetDefID == 0 {
		t.Fatal("GetNative did not resolve in OAL-enabled resource")
	}
	getNativeLuaDoc := getNativeCtx.TargetDoc.GetLuaDoc(getNativeCtx.TargetDefID)
	if getNativeLuaDoc == nil || !strings.Contains(getNativeLuaDoc.Description, "direct native callable binding") {
		t.Fatalf("GetNative LuaDoc = %+v, want direct binding description", getNativeLuaDoc)
	}
}
