package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFiveMRuntimeABIMetadata(t *testing.T) {
	s, root := newFiveMProfileTestServer(t)
	indexEmbeddedStdlibForTest(t, s)

	addFiveMTestDocument(t, s, filepath.Join(root, "runtime_resource", "fxmanifest.lua"), `
client_script 'client.lua'
server_script 'server.lua'
shared_script 'shared.lua'
`)

	clientDoc := addFiveMTestDocument(t, s, filepath.Join(root, "runtime_resource", "client.lua"), `
	return Citizen, Wait, CreateThread, SetTimeout, ClearTimeout, TriggerServerEvent, RegisterNUICallback, GlobalState, msgpack, json, debug, exports
	`)
	serverDoc := addFiveMTestDocument(t, s, filepath.Join(root, "runtime_resource", "server.lua"), `
	return Citizen, TriggerClientEvent, RegisterServerEvent, source, msgpack, json, debug, os, io, PerformHttpRequest, PerformHttpRequestAwait, GetPlayers, GetPlayerIdentifiers, GetPlayerTokens, exports
	`)
	sharedDoc := addFiveMTestDocument(t, s, filepath.Join(root, "runtime_resource", "shared.lua"), `
	return Citizen, AddEventHandler, RegisterNetEvent, TriggerEvent, GlobalState, msgpack, json, debug, exports
	`)

	assertResolvedGlobalTarget(t, s, clientDoc, "Citizen", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "Wait", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "CreateThread", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "SetTimeout", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "ClearTimeout", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "TriggerServerEvent", "std:///fivem/client.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "RegisterNUICallback", "std:///fivem/client.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "GlobalState", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "msgpack", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "json", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "debug", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "exports", "std:///fivem/export_bridge.lua")

	assertResolvedGlobalTarget(t, s, serverDoc, "Citizen", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "TriggerClientEvent", "std:///fivem/server.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "RegisterServerEvent", "std:///fivem/server.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "source", "std:///fivem/server.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "msgpack", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "json", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "debug", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "os", "std:///fivem/server.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "io", "std:///fivem/server.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "PerformHttpRequest", "std:///fivem/server.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "PerformHttpRequestAwait", "std:///fivem/server.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "GetPlayers", "std:///fivem/server.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "GetPlayerIdentifiers", "std:///fivem/server.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "GetPlayerTokens", "std:///fivem/server.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "exports", "std:///fivem/export_bridge.lua")

	assertResolvedGlobalTarget(t, s, sharedDoc, "Citizen", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, sharedDoc, "AddEventHandler", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, sharedDoc, "RegisterNetEvent", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, sharedDoc, "TriggerEvent", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, sharedDoc, "GlobalState", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, sharedDoc, "msgpack", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, sharedDoc, "json", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, sharedDoc, "debug", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, sharedDoc, "exports", "std:///fivem/export_bridge.lua")

	ctx := s.resolveSymbolNode(clientDoc.URI, clientDoc, mustFindIdentNode(t, clientDoc, "Wait"))
	if ctx == nil || ctx.TargetDoc == nil || ctx.TargetDefID == 0 {
		t.Fatal("Wait did not resolve to runtime metadata")
	}

	if got := ctx.TargetDoc.GetLuaDoc(ctx.TargetDefID); got == nil || !strings.Contains(strings.ToLower(got.Description), "scheduler coroutine") {
		t.Fatalf("Wait LuaDoc = %+v, want runtime scheduler description", got)
	}
}

func TestFiveMRuntimeABIScoping(t *testing.T) {
	s, root := newFiveMProfileTestServer(t)
	indexEmbeddedStdlibForTest(t, s)

	manifestDoc := addFiveMTestDocument(t, s, filepath.Join(root, "runtime_resource", "fxmanifest.lua"), `
fx_version 'cerulean'
client_script 'client.lua'
server_script 'server.lua'
shared_script 'shared.lua'

local manifestCitizen = Citizen
local manifestWait = Wait
local manifestMsgpack = msgpack
local manifestJson = json
local manifestDebug = debug
local manifestOs = os
local manifestIo = io
`)
	clientDoc := addFiveMTestDocument(t, s, filepath.Join(root, "runtime_resource", "client.lua"), `
	return TriggerClientEvent, RegisterServerEvent, source, Citizen, Wait, GlobalState, json, debug, PerformHttpRequest, GetPlayers, os, io
	`)
	serverDoc := addFiveMTestDocument(t, s, filepath.Join(root, "runtime_resource", "server.lua"), `
	return TriggerServerEvent, RegisterNUICallback, SendNUIMessage, Citizen, source, json, debug, PerformHttpRequest, GetPlayers, os, io
	`)
	sharedDoc := addFiveMTestDocument(t, s, filepath.Join(root, "runtime_resource", "shared.lua"), `
	return Citizen, Wait, AddEventHandler, TriggerClientEvent, TriggerServerEvent, RegisterNUICallback, source, GlobalState, json, debug, PerformHttpRequest, GetPlayers, os, io
	`)

	assertUnresolvedGlobal(t, s, manifestDoc, "Citizen")
	assertUnresolvedGlobal(t, s, manifestDoc, "Wait")
	assertUnresolvedGlobal(t, s, manifestDoc, "msgpack")
	assertResolvedGlobalTarget(t, s, manifestDoc, "json", "std:///fivem/manifest.lua")
	assertResolvedGlobalTarget(t, s, manifestDoc, "debug", "std:///fivem/manifest.lua")
	assertUnresolvedGlobal(t, s, manifestDoc, "os")
	assertUnresolvedGlobal(t, s, manifestDoc, "io")

	assertUnresolvedGlobal(t, s, clientDoc, "TriggerClientEvent")
	assertUnresolvedGlobal(t, s, clientDoc, "RegisterServerEvent")
	assertUnresolvedGlobal(t, s, clientDoc, "source")
	assertUnresolvedGlobal(t, s, clientDoc, "PerformHttpRequest")
	assertUnresolvedGlobal(t, s, clientDoc, "GetPlayers")
	assertUnresolvedGlobal(t, s, clientDoc, "os")
	assertUnresolvedGlobal(t, s, clientDoc, "io")
	assertResolvedGlobalTarget(t, s, clientDoc, "Citizen", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "Wait", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "GlobalState", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "json", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, clientDoc, "debug", "std:///fivem/shared.lua")

	assertUnresolvedGlobal(t, s, serverDoc, "TriggerServerEvent")
	assertUnresolvedGlobal(t, s, serverDoc, "RegisterNUICallback")
	assertUnresolvedGlobal(t, s, serverDoc, "SendNUIMessage")
	assertResolvedGlobalTarget(t, s, serverDoc, "Citizen", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "source", "std:///fivem/server.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "json", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "debug", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "os", "std:///fivem/server.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "io", "std:///fivem/server.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "PerformHttpRequest", "std:///fivem/server.lua")
	assertResolvedGlobalTarget(t, s, serverDoc, "GetPlayers", "std:///fivem/server.lua")

	assertResolvedGlobalTarget(t, s, sharedDoc, "Citizen", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, sharedDoc, "Wait", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, sharedDoc, "AddEventHandler", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, sharedDoc, "GlobalState", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, sharedDoc, "json", "std:///fivem/shared.lua")
	assertResolvedGlobalTarget(t, s, sharedDoc, "debug", "std:///fivem/shared.lua")
	assertUnresolvedGlobal(t, s, sharedDoc, "TriggerClientEvent")
	assertUnresolvedGlobal(t, s, sharedDoc, "TriggerServerEvent")
	assertUnresolvedGlobal(t, s, sharedDoc, "RegisterNUICallback")
	assertUnresolvedGlobal(t, s, sharedDoc, "source")
	assertUnresolvedGlobal(t, s, sharedDoc, "PerformHttpRequest")
	assertUnresolvedGlobal(t, s, sharedDoc, "GetPlayers")
	assertUnresolvedGlobal(t, s, sharedDoc, "os")
	assertUnresolvedGlobal(t, s, sharedDoc, "io")
}

func TestFiveMRuntimeABIHoverDocs(t *testing.T) {
	h := newFiveMFixtureHarness(t, "resource_runtime_abi")

	waitHover := h.hover("runtime_citizen_wait_hover")
	if waitHover == nil {
		t.Fatal("Citizen.Wait hover was nil")
	}

	if !strings.Contains(waitHover.Contents.Value, "function Citizen.Wait(milliseconds: integer?)") {
		t.Fatalf("Citizen.Wait hover = %q, want runtime signature", waitHover.Contents.Value)
	}

	if !strings.Contains(waitHover.Contents.Value, "Yields the current scheduler coroutine") {
		t.Fatalf("Citizen.Wait hover = %q, want scheduler description", waitHover.Contents.Value)
	}

	waitDefs := h.definition("runtime_citizen_wait_hover")
	if len(waitDefs) != 1 || waitDefs[0].URI != "std:///fivem/shared.lua" {
		t.Fatalf("Citizen.Wait definition = %+v, want std:///fivem/shared.lua", waitDefs)
	}

	nuiHover := h.hover("runtime_nui_callback_hover")
	if nuiHover == nil || !strings.Contains(nuiHover.Contents.Value, "legacy event-backed NUI callback bridge") {
		t.Fatalf("RegisterNUICallback hover = %+v, want client bridge docs", nuiHover)
	}

	exportsHover := h.hover("runtime_exports_hover")
	if exportsHover == nil || !strings.Contains(exportsHover.Contents.Value, "cross-resource export bridge") {
		t.Fatalf("exports hover = %+v, want export bridge docs", exportsHover)
	}

	exportsDefs := h.definition("runtime_exports_hover")
	if len(exportsDefs) != 1 || exportsDefs[0].URI != "std:///fivem/export_bridge.lua" {
		t.Fatalf("exports definition = %+v, want std:///fivem/export_bridge.lua", exportsDefs)
	}

	sourceHover := h.hover("runtime_source_hover")
	if sourceHover == nil || !strings.Contains(sourceHover.Contents.Value, "source: integer") {
		t.Fatalf("source hover = %+v, want integer runtime docs", sourceHover)
	}

	sourceDefs := h.definition("runtime_source_hover")
	if len(sourceDefs) != 1 || sourceDefs[0].URI != "std:///fivem/server.lua" {
		t.Fatalf("source definition = %+v, want std:///fivem/server.lua", sourceDefs)
	}

	triggerDoc := h.docForMarker("runtime_trigger_server_signature")
	triggerCtx := h.server.resolveSymbolNode(triggerDoc.URI, triggerDoc, mustFindIdentNode(t, triggerDoc, "TriggerServerEvent"))
	if triggerCtx == nil || triggerCtx.TargetDoc == nil || triggerCtx.TargetDefID == 0 {
		t.Fatal("TriggerServerEvent did not resolve to runtime metadata")
	}

	triggerLuaDoc := triggerCtx.TargetDoc.GetLuaDoc(triggerCtx.TargetDefID)
	if triggerLuaDoc == nil || len(triggerLuaDoc.Params) < 2 {
		t.Fatalf("TriggerServerEvent LuaDoc = %+v, want parameter metadata from std docs", triggerLuaDoc)
	}

	triggerValueID := triggerCtx.TargetDoc.getAssignedValue(triggerCtx.TargetDefID)
	triggerSignature := triggerCtx.TargetDoc.getFunctionParams(triggerValueID, triggerLuaDoc)
	if !strings.Contains(triggerSignature, "eventName: string") || !strings.Contains(triggerSignature, "...: any") {
		t.Fatalf("TriggerServerEvent signature = %q, want doc-derived parameter types", triggerSignature)
	}
}

func TestFiveMRuntimeVisibility(t *testing.T) {
	h := newFiveMFixtureHarness(t, "resource_runtime_abi")

	clientCompletion := h.completion("runtime_client_completion")
	for _, label := range []string{"Wait", "exports", "TriggerServerEvent", "RegisterNUICallback"} {
		if !completionHasLabel(clientCompletion, label) {
			t.Fatalf("client completion missing %q: %#v", label, clientCompletion.Items)
		}
	}
	for _, label := range []string{"source", "TriggerClientEvent", "RegisterServerEvent"} {
		if completionHasLabel(clientCompletion, label) {
			t.Fatalf("client completion unexpectedly included %q: %#v", label, clientCompletion.Items)
		}
	}

	sharedCompletion := h.completion("runtime_shared_completion")
	for _, label := range []string{"Wait", "exports", "AddEventHandler"} {
		if !completionHasLabel(sharedCompletion, label) {
			t.Fatalf("shared completion missing %q: %#v", label, sharedCompletion.Items)
		}
	}
	for _, label := range []string{"source", "TriggerClientEvent", "TriggerServerEvent", "RegisterNUICallback"} {
		if completionHasLabel(sharedCompletion, label) {
			t.Fatalf("shared completion unexpectedly included %q: %#v", label, sharedCompletion.Items)
		}
	}

	manifestCompletion := h.completion("runtime_manifest_completion")
	for _, label := range []string{"Citizen", "Wait", "exports", "source", "TriggerServerEvent"} {
		if completionHasLabel(manifestCompletion, label) {
			t.Fatalf("manifest completion unexpectedly included %q: %#v", label, manifestCompletion.Items)
		}
	}

	if hover := h.hover("runtime_citizen_wait_hover"); hover == nil || !strings.Contains(hover.Contents.Value, "Citizen.Wait") {
		t.Fatalf("runtime wait hover = %+v, want runtime docs", hover)
	}

	waitDefs := h.definition("runtime_citizen_wait_hover")
	if len(waitDefs) != 1 || waitDefs[0].URI != "std:///fivem/shared.lua" {
		t.Fatalf("runtime wait definition = %+v, want std:///fivem/shared.lua", waitDefs)
	}

	sourceRefs := h.references("runtime_source_hover", false)
	if len(sourceRefs) != 1 || !strings.HasSuffix(sourceRefs[0].URI, "/server.lua") {
		t.Fatalf("runtime source references = %+v, want one visible server.lua reference", sourceRefs)
	}

	triggerSig := h.signatureHelp("runtime_trigger_server_signature")
	if triggerSig == nil || len(triggerSig.Signatures) == 0 || !strings.Contains(triggerSig.Signatures[0].Label, "TriggerServerEvent(eventName: string") {
		t.Fatalf("runtime trigger signature help = %+v, want client runtime signature", triggerSig)
	}

	if hiddenSig := h.signatureHelp("runtime_shared_signature"); hiddenSig != nil {
		t.Fatalf("shared runtime signature help = %+v, want nil for hidden server-only helper", hiddenSig)
	}

	if hiddenSig := h.signatureHelp("runtime_manifest_signature"); hiddenSig != nil {
		t.Fatalf("manifest runtime signature help = %+v, want nil for restricted manifest surface", hiddenSig)
	}
}

func TestFiveMInferenceGlobals(t *testing.T) {
	s, root := newFiveMProfileTestServer(t)
	indexEmbeddedStdlibForTest(t, s)

	manifestDoc := addFiveMTestDocument(t, s, filepath.Join(root, "runtime_resource", "fxmanifest.lua"), `
fx_version 'cerulean'
client_script 'client.lua'
server_script 'server.lua'
shared_script 'shared.lua'

local manifestWait = Wait
local manifestSource = source
local manifestExports = exports
`)
	clientDoc := addFiveMTestDocument(t, s, filepath.Join(root, "runtime_resource", "client.lua"), `
return Citizen, Wait, TriggerServerEvent, RegisterNUICallback, exports, source
`)
	serverDoc := addFiveMTestDocument(t, s, filepath.Join(root, "runtime_resource", "server.lua"), `
return Citizen, TriggerClientEvent, RegisterServerEvent, source, exports, RegisterNUICallback
`)
	sharedDoc := addFiveMTestDocument(t, s, filepath.Join(root, "runtime_resource", "shared.lua"), `
return Citizen, Wait, AddEventHandler, exports, source, TriggerServerEvent, TriggerClientEvent
`)

	if got := manifestDoc.InferType(mustFindIdentNode(t, manifestDoc, "Wait")); got.Basics != TypeUnknown || got.CustomName != "" {
		t.Fatalf("manifest Wait type = %+v, want unknown", got)
	}
	if got := manifestDoc.InferType(mustFindIdentNode(t, manifestDoc, "source")); got.Basics != TypeUnknown || got.CustomName != "" {
		t.Fatalf("manifest source type = %+v, want unknown", got)
	}
	if got := manifestDoc.InferType(mustFindIdentNode(t, manifestDoc, "exports")); got.Basics != TypeUnknown || got.CustomName != "" {
		t.Fatalf("manifest exports type = %+v, want unknown", got)
	}

	if got := clientDoc.InferType(mustFindIdentNode(t, clientDoc, "Citizen")); got.Basics&TypeTable == 0 {
		t.Fatalf("client Citizen type = %+v, want table-like runtime metadata", got)
	}
	if got := clientDoc.InferType(mustFindIdentNode(t, clientDoc, "Wait")); got.Basics&TypeFunction == 0 {
		t.Fatalf("client Wait type = %+v, want function", got)
	}
	if got := clientDoc.InferType(mustFindIdentNode(t, clientDoc, "TriggerServerEvent")); got.Basics&TypeFunction == 0 {
		t.Fatalf("client TriggerServerEvent type = %+v, want function", got)
	}
	if got := clientDoc.InferType(mustFindIdentNode(t, clientDoc, "exports")); got.Basics&TypeTable == 0 {
		t.Fatalf("client exports type = %+v, want table", got)
	}
	if got := clientDoc.InferType(mustFindIdentNode(t, clientDoc, "source")); got.Basics != TypeUnknown || got.CustomName != "" {
		t.Fatalf("client source type = %+v, want unknown", got)
	}

	if got := serverDoc.InferType(mustFindIdentNode(t, serverDoc, "TriggerClientEvent")); got.Basics&TypeFunction == 0 {
		t.Fatalf("server TriggerClientEvent type = %+v, want function", got)
	}
	if got := serverDoc.InferType(mustFindIdentNode(t, serverDoc, "RegisterServerEvent")); got.Basics&TypeFunction == 0 {
		t.Fatalf("server RegisterServerEvent type = %+v, want function", got)
	}
	if got := serverDoc.InferType(mustFindIdentNode(t, serverDoc, "source")); got.Basics&TypeNumber == 0 {
		t.Fatalf("server source type = %+v, want number", got)
	}
	if got := serverDoc.InferType(mustFindIdentNode(t, serverDoc, "exports")); got.Basics&TypeTable == 0 {
		t.Fatalf("server exports type = %+v, want table", got)
	}
	if got := serverDoc.InferType(mustFindIdentNode(t, serverDoc, "RegisterNUICallback")); got.Basics != TypeUnknown || got.CustomName != "" {
		t.Fatalf("server RegisterNUICallback type = %+v, want unknown", got)
	}

	if got := sharedDoc.InferType(mustFindIdentNode(t, sharedDoc, "Citizen")); got.Basics&TypeTable == 0 {
		t.Fatalf("shared Citizen type = %+v, want table-like runtime metadata", got)
	}
	if got := sharedDoc.InferType(mustFindIdentNode(t, sharedDoc, "Wait")); got.Basics&TypeFunction == 0 {
		t.Fatalf("shared Wait type = %+v, want function", got)
	}
	if got := sharedDoc.InferType(mustFindIdentNode(t, sharedDoc, "AddEventHandler")); got.Basics&TypeFunction == 0 {
		t.Fatalf("shared AddEventHandler type = %+v, want function", got)
	}
	if got := sharedDoc.InferType(mustFindIdentNode(t, sharedDoc, "exports")); got.Basics&TypeTable == 0 {
		t.Fatalf("shared exports type = %+v, want table", got)
	}
	for _, name := range []string{"source", "TriggerServerEvent", "TriggerClientEvent"} {
		if got := sharedDoc.InferType(mustFindIdentNode(t, sharedDoc, name)); got.Basics != TypeUnknown || got.CustomName != "" {
			t.Fatalf("shared %s type = %+v, want unknown", name, got)
		}
	}
}

func TestFiveMManifestRestrictedSurface(t *testing.T) {
	h := newFiveMFixtureHarness(t, "manifest_restricted")

	for _, marker := range []string{"restricted_manifest_wait", "restricted_manifest_source"} {
		if hover := h.hover(marker); hover != nil {
			if strings.Contains(hover.Contents.Value, "Standard Library (`fivem/") || strings.Contains(hover.Contents.Value, "function Wait(") || strings.Contains(hover.Contents.Value, "source: integer") || strings.Contains(hover.Contents.Value, "cross-resource export bridge") {
				t.Fatalf("manifest hover for %s = %+v, want no runtime FiveM docs", marker, hover)
			}
		}
		if defs := h.definition(marker); len(defs) != 0 {
			t.Fatalf("manifest definitions for %s = %+v, want none", marker, defs)
		}
		if refs := h.references(marker, false); len(refs) > 0 {
			for _, ref := range refs {
				if strings.HasPrefix(ref.URI, "std:///fivem/") {
					t.Fatalf("manifest references for %s = %+v, want no runtime FiveM references", marker, refs)
				}
			}
		}
	}

	if hover := h.hover("restricted_manifest_exports"); hover != nil {
		if strings.Contains(hover.Contents.Value, "cross-resource export bridge") || strings.Contains(hover.Contents.Value, "Standard Library (`fivem/export_bridge.lua`)") {
			t.Fatalf("manifest hover for restricted_manifest_exports = %+v, want manifest docs only", hover)
		}
	}

	if defs := h.definition("restricted_manifest_exports"); len(defs) > 0 {
		for _, def := range defs {
			if def.URI == "std:///fivem/export_bridge.lua" {
				t.Fatalf("manifest definitions for restricted_manifest_exports = %+v, want no runtime export bridge docs", defs)
			}
		}
	}

	completion := h.completion("restricted_manifest_completion")
	for _, label := range []string{"Citizen", "Wait", "source", "TriggerServerEvent", "RESTRICTED_CLIENT", "RESTRICTED_SERVER", "RESTRICTED_SHARED"} {
		if completionHasLabel(completion, label) {
			t.Fatalf("manifest completion unexpectedly included %q: %#v", label, completion.Items)
		}
	}

	if sig := h.signatureHelp("restricted_manifest_signature"); sig != nil {
		t.Fatalf("manifest signature help = %+v, want nil for restricted manifest surface", sig)
	}
}
