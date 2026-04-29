package lsp

import (
	"strings"
	"testing"

	"github.com/coalaura/lugo/ast"
)

func TestFiveMFuncrefTypeModel(t *testing.T) {
	h := newFiveMFixtureHarness(t, "resource_bridges", "resource_runtime_abi")

	bridgeDoc := h.docForMarker("bridge_proxy_hover")
	bridgeType := bridgeDoc.InferType(nodeIDAtMarker(t, h, "bridge_proxy_hover"))
	if bridgeType.Basics&TypeTable == 0 || bridgeType.Basics&TypeFunction != 0 {
		t.Fatalf("bridge proxy type = %+v, want callable proxy table without plain function bit", bridgeType)
	}
	if !bridgeType.IsCallable() || bridgeType.CallSig == "" {
		t.Fatalf("bridge proxy callable metadata = %+v, want stable callable signature", bridgeType)
	}

	replyDoc := h.docForMarker("runtime_nui_reply_hover")
	replyType := replyDoc.InferType(nodeIDAtMarker(t, h, "runtime_nui_reply_hover"))
	if replyType.Basics&TypeTable == 0 || replyType.Basics&TypeFunction != 0 {
		t.Fatalf("nui reply type = %+v, want callable proxy table without plain function bit", replyType)
	}
	if !strings.Contains(replyType.CallSig, "response: any?") && !strings.Contains(replyType.CallSig, "response?: any") {
		t.Fatalf("nui reply call signature = %q, want response payload signature", replyType.CallSig)
	}

	bagDoc := h.docForMarker("runtime_statebag_replicated_hover")
	if bagType := bagDoc.InferType(nodeIDAtMarker(t, h, "runtime_statebag_bag_hover")); bagType.Basics&TypeString == 0 {
		t.Fatalf("state bag name type = %+v, want string from bridge callback docs", bagType)
	}
	if repType := bagDoc.InferType(nodeIDAtMarker(t, h, "runtime_statebag_replicated_hover")); repType.Basics&TypeBoolean == 0 {
		t.Fatalf("state bag replicated type = %+v, want boolean from bridge callback docs", repType)
	}
}

func TestFiveMBridgeTyping(t *testing.T) {
	h := newFiveMFixtureHarness(t, "resource_bridges", "resource_runtime_abi")

	bridgeHover := h.hover("bridge_proxy_hover")
	if bridgeHover == nil || !strings.Contains(bridgeHover.Contents.Value, "FiveM callable proxy") || !strings.Contains(bridgeHover.Contents.Value, "ping(count: integer)") {
		t.Fatalf("bridge proxy hover = %+v, want callable proxy hover with stable signature", bridgeHover)
	}

	replyHover := h.hover("runtime_nui_reply_hover")
	if replyHover == nil || !strings.Contains(replyHover.Contents.Value, "FiveM callable proxy") || (!strings.Contains(replyHover.Contents.Value, "reply(response?: any)") && !strings.Contains(replyHover.Contents.Value, "reply(response: any?)")) {
		t.Fatalf("nui reply hover = %+v, want callable proxy hover with reply signature", replyHover)
	}

	bagHover := h.hover("runtime_statebag_replicated_hover")
	if bagHover == nil || !strings.Contains(bagHover.Contents.Value, "local replicated: boolean") {
		t.Fatalf("state bag callback hover = %+v, want boolean callback parameter typing", bagHover)
	}

	proxyCompletion := h.completion("runtime_nui_reply_completion")
	if completionHasLabel(proxyCompletion, "__cfx_functionReference") {
		t.Fatalf("nui reply completion unexpectedly exposed raw proxy marker: %#v", proxyCompletion.Items)
	}

	bridgeCompletion := h.completion("bridge_proxy_completion")
	if completionHasLabel(bridgeCompletion, "__cfx_functionReference") {
		t.Fatalf("bridge proxy completion unexpectedly exposed raw proxy marker: %#v", bridgeCompletion.Items)
	}
}

func TestFiveMFuncrefSignatureHelp(t *testing.T) {
	h := newFiveMFixtureHarness(t, "resource_bridges", "resource_runtime_abi")

	replySig := h.signatureHelp("runtime_nui_reply_signature")
	if replySig == nil || len(replySig.Signatures) == 0 || (!strings.Contains(replySig.Signatures[0].Label, "reply(response?: any)") && !strings.Contains(replySig.Signatures[0].Label, "reply(response: any?)")) {
		t.Fatalf("nui reply signature help = %+v, want proxy callback signature", replySig)
	}

}

func nodeIDAtMarker(t *testing.T, h *fiveMFixtureHarness, markerName string) ast.NodeID {
	t.Helper()

	marker := h.requireMarker(markerName)
	doc := h.docForMarker(markerName)
	nodeID := doc.Tree.NodeAt(doc.Tree.Offset(marker.Position.Line, marker.Position.Character))
	if nodeID == ast.InvalidNode {
		t.Fatalf("node at marker %s was invalid", markerName)
	}

	return nodeID
}
