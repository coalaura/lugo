package lsp

import (
	"strings"
	"testing"
)

func TestFiveMExportScoping(t *testing.T) {
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
}

func TestFiveMUnknownExportDiagnostics(t *testing.T) {
	h := newFiveMFixtureHarness(t, "resource_export_missing")

	diags := h.diagnostics("export_consumer/missing.lua")

	var unknown []Diagnostic
	for _, diag := range diags {
		if diag.Code == "fivem-unknown-export" {
			unknown = append(unknown, diag)
		}
	}

	if len(unknown) != 1 {
		t.Fatalf("unknown export diagnostics = %+v, want exactly one scoped unknown-export", diags)
	}

	if !strings.Contains(unknown[0].Message, "empty_resource") || !strings.Contains(unknown[0].Message, "ping") {
		t.Fatalf("unknown export diagnostic = %+v, want empty_resource/ping message", unknown[0])
	}
}

func TestFiveMExportCompletionConsistency(t *testing.T) {
	h := newFiveMFixtureHarness(t, "resource_export_completion")

	valid := completionItemByLabel(h.completion("export_resource_completion"), "ping")
	if valid == nil {
		t.Fatal("export_resource completion missing ping")
	}

	if !strings.Contains(valid.InsertText, "ping(") {
		t.Fatalf("export_resource ping insert text = %q, want snippet from scoped export target", valid.InsertText)
	}

	if completionHasLabel(h.completion("export_empty_resource_completion"), "ping") {
		t.Fatal("empty_resource completion unexpectedly included ping from unrelated resource")
	}

}

func completionItemByLabel(list CompletionList, label string) *CompletionItem {
	for i := range list.Items {
		if list.Items[i].Label == label {
			return &list.Items[i]
		}
	}

	return nil
}
