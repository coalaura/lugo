package lsp

import (
	"strings"
	"testing"
)

func TestFiveMManifestCompletion(t *testing.T) {
	h := newFiveMFixtureHarness(t, "manifest_authoring")

	completion := h.completion("manifest_authoring_completion")
	for _, label := range []string{"fx_version", "game", "client_script", "client_scripts", "server_script", "shared_script", "export", "dependency", "dependencies"} {
		if !completionHasLabel(completion, label) {
			t.Fatalf("manifest completion missing %q: %#v", label, completion.Items)
		}
	}

	for _, label := range []string{"Citizen", "Wait", "source", "TriggerServerEvent"} {
		if completionHasLabel(completion, label) {
			t.Fatalf("manifest completion unexpectedly included runtime symbol %q: %#v", label, completion.Items)
		}
	}
}

func TestFiveMManifestDiagnostics(t *testing.T) {
	h := newFiveMFixtureHarness(t, "manifest_authoring")

	diags := h.diagnostics("authoring_resource/fxmanifest.lua")
	for _, tc := range []struct {
		marker string
		code   string
	}{
		{marker: "manifest_reserved", code: "fivem-manifest-reserved-directive"},
		{marker: "manifest_invalid_local", code: "fivem-manifest-invalid-construct"},
		{marker: "manifest_unknown_runtime", code: "fivem-manifest-unknown-directive"},
	} {
		if !hasDiagnosticAtMarker(h, diags, tc.marker, tc.code) {
			t.Fatalf("manifest diagnostics missing %s at %s: %+v", tc.code, tc.marker, diags)
		}
	}

	if hasAnyDiagnosticAtMarker(h, diags, "manifest_custom_metadata") {
		t.Fatalf("custom manifest metadata should not be diagnosed: %+v", diags)
	}
}

func TestFiveMManifestHoverDocs(t *testing.T) {
	h := newFiveMFixtureHarness(t, "manifest_authoring")

	fxHover := h.hover("manifest_hover_fx_version")
	if fxHover == nil || !containsAll(fxHover.Contents.Value, "fxv2 manifest version", "Standard Library (`fivem/manifest.lua`)") {
		t.Fatalf("fx_version hover = %+v, want manifest docs from std:///fivem/manifest.lua", fxHover)
	}

	clientHover := h.hover("manifest_hover_client_scripts")
	if clientHover == nil || !containsAll(clientHover.Contents.Value, "client-side Lua files or globs", "Standard Library (`fivem/manifest.lua`)") {
		t.Fatalf("client_scripts hover = %+v, want manifest docs from std:///fivem/manifest.lua", clientHover)
	}

	defs := h.definition("manifest_hover_client_scripts")
	if len(defs) != 1 || defs[0].URI != "std:///fivem/manifest.lua" {
		t.Fatalf("client_scripts definitions = %+v, want std:///fivem/manifest.lua", defs)
	}
}

func hasDiagnosticAtMarker(h *fiveMFixtureHarness, diags []Diagnostic, markerName, code string) bool {
	marker := h.requireMarker(markerName)

	for _, diag := range diags {
		if diag.Code == code && positionInRange(marker.Position, diag.Range) {
			return true
		}
	}

	return false
}

func hasAnyDiagnosticAtMarker(h *fiveMFixtureHarness, diags []Diagnostic, markerName string) bool {
	marker := h.requireMarker(markerName)

	for _, diag := range diags {
		if positionInRange(marker.Position, diag.Range) {
			return true
		}
	}

	return false
}

func positionInRange(pos Position, r Range) bool {
	if pos.Line < r.Start.Line || pos.Line > r.End.Line {
		return false
	}

	if pos.Line == r.Start.Line && pos.Character < r.Start.Character {
		return false
	}

	if pos.Line == r.End.Line && pos.Character >= r.End.Character {
		return false
	}

	return true
}

func containsAll(value string, want ...string) bool {
	for _, needle := range want {
		if !strings.Contains(value, needle) {
			return false
		}
	}

	return true
}
