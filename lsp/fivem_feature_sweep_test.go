package lsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFiveMFeatureSweep(t *testing.T) {
	h := newFiveMFixtureHarness(t, "resource_exports", "resource_provides")

	h.requireSingleDefinitionAt("export_scoped_ping_call", "export_ping_definition")
	h.requireSingleDefinitionAt("provide_fallback_ping_call", "provide_canonical_ping_definition")

	refs := h.references("export_scoped_ping_call", true)
	if len(refs) == 0 {
		t.Fatal("scoped export references = 0, want definition and consumer references")
	}

	for _, ref := range refs {
		if strings.Contains(ref.URI, "/unrelated_resource/") || strings.Contains(ref.URI, "\\unrelated_resource\\") {
			t.Fatalf("scoped export references leaked unrelated resource: %+v", refs)
		}
	}

	bridge := newFiveMFixtureHarness(t, "resource_bridges", "resource_runtime_abi")

	hover := bridge.hover("bridge_proxy_hover")
	if hover == nil || !strings.Contains(hover.Contents.Value, "FiveM callable proxy") || !strings.Contains(hover.Contents.Value, "ping(count: integer)") {
		t.Fatalf("bridge proxy hover = %+v, want callable proxy hover with stable signature", hover)
	}

	completionHarness := newFiveMFixtureHarness(t, "resource_export_completion")

	valid := completionItemByLabel(completionHarness.completion("export_resource_completion"), "ping")
	if valid == nil || !strings.Contains(valid.InsertText, "ping(") {
		t.Fatalf("export_resource completion = %+v, want ping snippet from scoped export target", valid)
	}

	if completionHasLabel(completionHarness.completion("export_empty_resource_completion"), "ping") {
		t.Fatal("empty_resource completion unexpectedly included ping from unrelated resource")
	}

	sig := bridge.signatureHelp("runtime_nui_reply_signature")
	if sig == nil || len(sig.Signatures) == 0 || (!strings.Contains(sig.Signatures[0].Label, "reply(response?: any)") && !strings.Contains(sig.Signatures[0].Label, "reply(response: any?)")) {
		t.Fatalf("runtime reply signature help = %+v, want callable proxy signature", sig)
	}

	diagnosticsHarness := newFiveMFixtureHarness(t, "resource_export_missing")
	if diags := diagnosticsHarness.diagnostics("export_consumer/missing.lua"); !hasDiagnosticCode(diags, "fivem-unknown-export") {
		t.Fatalf("missing export diagnostics = %+v, want fivem-unknown-export", diags)
	}

	token := bridge.semanticTokenAt("bridge_ping_call")
	if token == nil || token.TokenType != 4 {
		t.Fatalf("bridge export semantic token = %+v, want method token", token)
	}
}

func TestFiveMQuickFixes(t *testing.T) {
	h := newFiveMFixtureHarness(t, "resource_export_completion")
	h.writeWorkspaceFile("export_consumer/completion.lua", `local valid = exports.export_resource.--[[@export_resource_completion]]p
local invalid = exports.empty_resource.--[[@export_empty_resource_completion]]p
local typo = exports.--[[@export_resource_typo]]export_resorce.ping

return valid, invalid, typo
`)
	h.reindex()

	var target Diagnostic
	for _, diag := range h.diagnostics("export_consumer/completion.lua") {
		if diag.Code == "fivem-unknown-resource" {
			target = diag
			break
		}
	}

	if target.Code == "" {
		t.Fatal("unknown resource diagnostic missing from typo completion file")
	}

	actions := h.codeActions("export_consumer/completion.lua", target.Range, []Diagnostic{target})
	uri := h.server.pathToURI(filepath.Join(h.root, "export_consumer", "completion.lua"))
	for _, action := range actions {
		if action.Title == "Change to 'export_resource'" && action.Edit != nil {
			edits := action.Edit.Changes[uri]
			if len(edits) == 1 && edits[0].NewText == "export_resource" {
				return
			}
		}
	}

	t.Fatalf("quick fixes = %+v, want Change to 'export_resource'", actions)
}

func TestPlainLuaNonRegression(t *testing.T) {
	h := newFiveMFixtureHarness(t, "plain_lua", "resource_export_completion")
	h.writeWorkspaceFile("plain_completion.lua", `local plainCompletion = exports.--[[@plain_completion]]
return TriggerServerEvent(--[[@plain_signature]]'plain:event'), plainCompletion
`)
	h.reindex()

	for _, marker := range []string{"plain_exports", "plain_source"} {
		if hover := h.hover(marker); hover != nil {
			if strings.Contains(hover.Contents.Value, "Standard Library (`fivem/") || strings.Contains(hover.Contents.Value, "cross-resource export bridge") || strings.Contains(hover.Contents.Value, "source: integer") {
				t.Fatalf("plain hover for %s = %+v, want no FiveM docs", marker, hover)
			}
		}

		if defs := h.definition(marker); len(defs) != 0 {
			t.Fatalf("plain definitions for %s = %+v, want none", marker, defs)
		}

	}

	completion := h.completion("plain_completion")
	for _, label := range []string{"export_resource", "empty_resource", "unrelated_resource"} {
		if completionHasLabel(completion, label) {
			t.Fatalf("plain completion unexpectedly included %q: %#v", label, completion.Items)
		}
	}

	if sig := h.signatureHelp("plain_signature"); sig != nil {
		t.Fatalf("plain signature help = %+v, want nil", sig)
	}

	token := h.semanticTokenAt("plain_exports")
	if token == nil || token.TokenType != 0 || token.Modifiers&(1<<3) != 0 {
		t.Fatalf("plain exports semantic token = %+v, want plain variable without defaultLibrary modifier", token)
	}
}
