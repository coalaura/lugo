package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coalaura/plain"
)

const fiveMFixtureMarkerPrefix = "--[[@"

type fiveMFixtureMarker struct {
	Name     string
	RelPath  string
	URI      string
	Offset   int
	Position Position
}

type fiveMFixtureHarness struct {
	t       testing.TB
	server  *Server
	root    string
	rpcOut  *bytes.Buffer
	markers map[string]fiveMFixtureMarker
}

type semanticTokenHit struct {
	Range     Range
	TokenType uint32
	Modifiers uint32
}

func TestFiveMFixtureHarness(t *testing.T) {
	h := newFiveMFixtureHarness(t,
		"plain_lua",
		"manifest_restricted",
		"resource_client_server_shared",
		"resource_dual_listed",
		"resource_exports",
		"resource_provides",
		"resource_bridges",
		"resource_natives",
		"mixed_workspace",
	)

	h.requireMarker("mixed_bridge_ping_call")
	h.requireMarker("mixed_bridge_ping_definition")
	h.requireMarker("mixed_bridge_resource_completion")
	h.requireMarker("mixed_plain_exports")

	consumerDoc := h.docForMarker("mixed_bridge_ping_call")
	if got := h.server.getDocumentFiveMProfile(consumerDoc).Kind; got != FiveMProfileClient {
		t.Fatalf("bridge consumer profile = %s, want %s", got.String(), FiveMProfileClient.String())
	}

	h.requireSingleDefinitionAt("mixed_bridge_ping_call", "mixed_bridge_ping_definition")

	hover := h.hover("mixed_bridge_ping_call")
	if hover == nil || !strings.Contains(hover.Contents.Value, "ping") {
		t.Fatalf("hover for mixed_bridge_ping_call = %#v, want ping details", hover)
	}

	completion := h.completion("mixed_bridge_resource_completion")
	if !completionHasLabel(completion, "mixed_bridge_provider") {
		t.Fatalf("completion labels %#v do not include mixed_bridge_provider", completion.Items)
	}

	if !hasDiagnosticCode(h.diagnostics("mixed_surface/stray.lua"), "unaccounted-file") {
		t.Fatalf("mixed_surface/stray.lua should report unaccounted-file diagnostics")
	}
}

func TestFiveMFixtureWarmReindex(t *testing.T) {
	h := newFiveMFixtureHarness(t, "mixed_workspace")

	if !hasDiagnosticCode(h.diagnostics("mixed_surface/stray.lua"), "unaccounted-file") {
		t.Fatalf("mixed_surface/stray.lua should start as an unaccounted file")
	}

	h.writeWorkspaceFile("mixed_surface/fxmanifest.lua", `
fx_version 'cerulean'
game 'gta5'

client_scripts {
	'client.lua',
	'client_consumer.lua',
	'stray.lua'
}

server_script 'server.lua'
shared_script 'shared.lua'
`)

	h.reindex()

	if hasDiagnosticCode(h.diagnostics("mixed_surface/stray.lua"), "unaccounted-file") {
		t.Fatalf("mixed_surface/stray.lua should stop reporting unaccounted-file after warm reindex")
	}

	h.requireSingleDefinitionAt("mixed_bridge_ping_call", "mixed_bridge_ping_definition")
}

func TestFiveMFixtureNonFiveMControl(t *testing.T) {
	h := newFiveMFixtureHarness(t, "plain_lua", "resource_client_server_shared")

	plainDoc := h.docForMarker("plain_exports")
	if profile := h.server.getDocumentFiveMProfile(plainDoc); profile.Kind != FiveMProfilePlainLua || profile.ResourceRoot != "" {
		t.Fatalf("plain control profile = %+v, want plain-lua with no resource root", profile)
	}

	if h.server.isKnownGlobal(plainDoc, []byte("exports")) {
		t.Fatal("plain control unexpectedly exposes exports")
	}

	if h.server.isKnownGlobal(plainDoc, []byte("source")) {
		t.Fatal("plain control unexpectedly exposes source")
	}

	if h.server.getFiveMExportBridgeProfile(plainDoc).Kind != FiveMProfilePlainLua {
		t.Fatal("plain control unexpectedly activates the FiveM export bridge")
	}

	if h.server.canSeeSymbol(plainDoc, h.docForMarker("surface_server_definition")) {
		t.Fatal("plain control unexpectedly sees FiveM resource symbols")
	}

	for _, markerName := range []string{"plain_exports", "plain_source"} {
		ctx := h.resolve(markerName)
		if ctx != nil && (ctx.TargetDefID != 0 || len(ctx.GlobalDefs) > 0) {
			t.Fatalf("plain control marker %s unexpectedly resolved: %+v", markerName, ctx)
		}
	}

	if hasDiagnosticCode(h.diagnostics("plain.lua"), "unaccounted-file") {
		t.Fatal("plain control unexpectedly reported a FiveM unaccounted-file diagnostic")
	}
}

func newFiveMFixtureHarness(t testing.TB, fixtureNames ...string) *fiveMFixtureHarness {
	t.Helper()

	h := newFiveMFixtureHarnessWithoutIndex(t, fixtureNames...)
	h.reindex()

	return h
}

func newFiveMFixtureHarnessWithoutIndex(t testing.TB, fixtureNames ...string) *fiveMFixtureHarness {
	t.Helper()

	root := t.TempDir()
	rpcOut := new(bytes.Buffer)
	s := NewServer("test")
	s.Writer = rpcOut
	s.Log = plain.New(plain.WithTarget(io.Discard))
	s.FeatureFiveM = true
	s.DiagFiveMUnaccountedFile = true
	s.DiagFiveMUnknownExport = true
	s.DiagFiveMUnknownResource = true
	s.SuggestFunctionParams = true

	rootURI := s.pathToURI(root)
	rootPath := strings.ToLower(filepath.Clean(root))
	s.RootURI = rootURI
	s.WorkspaceFolders = []string{rootURI}
	s.lowerRootPath = rootPath
	s.lowerWorkspaceFolders = []string{rootPath}

	h := &fiveMFixtureHarness{
		t:       t,
		server:  s,
		root:    root,
		rpcOut:  rpcOut,
		markers: make(map[string]fiveMFixtureMarker),
	}

	for _, fixtureName := range fixtureNames {
		h.copyFixture(fixtureName)
	}

	return h
}

func (h *fiveMFixtureHarness) copyFixture(fixtureName string) {
	h.t.Helper()

	srcRoot := filepath.Join("testdata", "fivem", fixtureName)
	if _, err := os.Stat(srcRoot); err != nil {
		h.t.Fatalf("fixture %s missing: %v", fixtureName, err)
	}

	err := filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}

		if rel == "." {
			return nil
		}

		dstPath := filepath.Join(h.root, rel)
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return h.writeProcessedWorkspaceFile(filepath.ToSlash(rel), string(b))
	})
	if err != nil {
		h.t.Fatalf("copy fixture %s: %v", fixtureName, err)
	}
}

func (h *fiveMFixtureHarness) writeWorkspaceFile(relPath, source string) {
	h.t.Helper()

	if err := h.writeProcessedWorkspaceFile(filepath.ToSlash(relPath), source); err != nil {
		h.t.Fatalf("write workspace file %s: %v", relPath, err)
	}
}

func (h *fiveMFixtureHarness) writeProcessedWorkspaceFile(relPath, source string) error {
	clean, markers, err := stripFiveMFixtureMarkers(source)
	if err != nil {
		return fmt.Errorf("strip markers from %s: %w", relPath, err)
	}

	for name, marker := range h.markers {
		if marker.RelPath == relPath {
			delete(h.markers, name)
		}
	}

	for name, offset := range markers {
		if _, exists := h.markers[name]; exists {
			return fmt.Errorf("duplicate fixture marker %q", name)
		}

		uri := h.server.pathToURI(filepath.Join(h.root, filepath.FromSlash(relPath)))
		h.markers[name] = fiveMFixtureMarker{
			Name:     name,
			RelPath:  relPath,
			URI:      uri,
			Offset:   offset,
			Position: fixturePositionForOffset(clean, offset),
		}
	}

	fullPath := filepath.Join(h.root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}

	return os.WriteFile(fullPath, []byte(clean), 0o644)
}

func (h *fiveMFixtureHarness) reindex() {
	h.t.Helper()
	h.server.refreshWorkspace()
	h.resetRPC()
}

func (h *fiveMFixtureHarness) docForMarker(markerName string) *Document {
	h.t.Helper()

	marker := h.requireMarker(markerName)
	doc := h.server.Documents[marker.URI]
	if doc == nil {
		h.t.Fatalf("document for marker %s (%s) was not indexed", markerName, marker.URI)
	}

	return doc
}

func (h *fiveMFixtureHarness) resolve(markerName string) *SymbolContext {
	h.t.Helper()

	marker := h.requireMarker(markerName)
	doc := h.docForMarker(markerName)
	return h.server.resolveSymbolAt(marker.URI, doc.Tree.Offset(marker.Position.Line, marker.Position.Character))
}

func (h *fiveMFixtureHarness) hover(markerName string) *Hover {
	h.t.Helper()

	res := h.positionResponse(markerName, func(req Request) {
		h.server.handleHover(req)
	})
	if len(res) == 0 || string(res) == "null" {
		return nil
	}

	var hover Hover
	if err := json.Unmarshal(res, &hover); err != nil {
		h.t.Fatalf("decode hover for %s: %v", markerName, err)
	}

	return &hover
}

func (h *fiveMFixtureHarness) definition(markerName string) []Location {
	h.t.Helper()

	res := h.positionResponse(markerName, func(req Request) {
		h.server.handleDefinition(req)
	})
	if len(res) == 0 || string(res) == "null" {
		return nil
	}

	var locations []Location
	if err := json.Unmarshal(res, &locations); err != nil {
		h.t.Fatalf("decode definition result for %s: %v", markerName, err)
	}

	return locations
}

func (h *fiveMFixtureHarness) completion(markerName string) CompletionList {
	h.t.Helper()

	res := h.positionResponse(markerName, func(req Request) {
		h.server.handleCompletion(req)
	})

	var completion CompletionList
	if err := json.Unmarshal(res, &completion); err != nil {
		h.t.Fatalf("decode completion result for %s: %v", markerName, err)
	}

	return completion
}

func (h *fiveMFixtureHarness) references(markerName string, includeDeclaration bool) []Location {
	h.t.Helper()

	marker := h.requireMarker(markerName)
	params, err := json.Marshal(ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: marker.URI},
		Position:     marker.Position,
		Context:      ReferenceContext{IncludeDeclaration: includeDeclaration},
	})
	if err != nil {
		h.t.Fatalf("marshal reference params for %s: %v", markerName, err)
	}

	h.resetRPC()
	h.server.handleReferences(Request{RPC: "2.0", ID: 1, Params: params})

	body := h.lastResponse(1)
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		h.t.Fatalf("decode reference response for %s: %v", markerName, err)
	}

	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return nil
	}

	var locations []Location
	if err := json.Unmarshal(envelope.Result, &locations); err != nil {
		h.t.Fatalf("decode references result for %s: %v", markerName, err)
	}

	return locations
}

func (h *fiveMFixtureHarness) signatureHelp(markerName string) *SignatureHelp {
	h.t.Helper()

	marker := h.requireMarker(markerName)
	params, err := json.Marshal(SignatureHelpParams{
		TextDocument: TextDocumentIdentifier{URI: marker.URI},
		Position:     marker.Position,
	})
	if err != nil {
		h.t.Fatalf("marshal signature help params for %s: %v", markerName, err)
	}

	h.resetRPC()
	h.server.handleSignatureHelp(Request{RPC: "2.0", ID: 1, Params: params})

	body := h.lastResponse(1)
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		h.t.Fatalf("decode signature help response for %s: %v", markerName, err)
	}

	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return nil
	}

	var help SignatureHelp
	if err := json.Unmarshal(envelope.Result, &help); err != nil {
		h.t.Fatalf("decode signature help for %s: %v", markerName, err)
	}

	return &help
}

func (h *fiveMFixtureHarness) semanticTokens(relPath string) []semanticTokenHit {
	h.t.Helper()

	uri := h.server.pathToURI(filepath.Join(h.root, filepath.FromSlash(relPath)))
	params, err := json.Marshal(SemanticTokensParams{TextDocument: TextDocumentIdentifier{URI: uri}})
	if err != nil {
		h.t.Fatalf("marshal semantic token params for %s: %v", relPath, err)
	}

	h.resetRPC()
	h.server.handleSemanticTokensFull(Request{RPC: "2.0", ID: 1, Params: params})

	body := h.lastResponse(1)
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		h.t.Fatalf("decode semantic token response for %s: %v", relPath, err)
	}

	var payload SemanticTokens
	if err := json.Unmarshal(envelope.Result, &payload); err != nil {
		h.t.Fatalf("decode semantic token payload for %s: %v", relPath, err)
	}

	lineOffsets := h.server.Documents[uri].Tree.LineOffsets
	tokens := make([]semanticTokenHit, 0, len(payload.Data)/5)

	var (
		line uint32
		col  uint32
	)

	for i := 0; i+4 < len(payload.Data); i += 5 {
		deltaLine := payload.Data[i]
		deltaCol := payload.Data[i+1]
		length := payload.Data[i+2]
		tokenType := payload.Data[i+3]
		modifiers := payload.Data[i+4]

		line += deltaLine
		if deltaLine == 0 {
			col += deltaCol
		} else {
			col = deltaCol
		}

		start := Position{Line: line, Character: col}
		end := Position{Line: line, Character: col + length}
		if int(line) < len(lineOffsets) {
			tokens = append(tokens, semanticTokenHit{
				Range:     Range{Start: start, End: end},
				TokenType: tokenType,
				Modifiers: modifiers,
			})
		}
	}

	return tokens
}

func (h *fiveMFixtureHarness) semanticTokenAt(markerName string) *semanticTokenHit {
	h.t.Helper()

	marker := h.requireMarker(markerName)
	for _, token := range h.semanticTokens(marker.RelPath) {
		if token.Range.Start == marker.Position {
			return &token
		}
	}

	return nil
}

func (h *fiveMFixtureHarness) diagnostics(relPath string) []Diagnostic {
	h.t.Helper()

	uri := h.server.pathToURI(filepath.Join(h.root, filepath.FromSlash(relPath)))
	h.resetRPC()
	h.server.publishDiagnostics(uri)

	body := h.lastNotification("textDocument/publishDiagnostics")
	var notification struct {
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(body, &notification); err != nil {
		h.t.Fatalf("decode diagnostics envelope for %s: %v", relPath, err)
	}

	var params PublishDiagnosticsParams
	if err := json.Unmarshal(notification.Params, &params); err != nil {
		h.t.Fatalf("decode diagnostics params for %s: %v", relPath, err)
	}

	if params.URI != uri {
		h.t.Fatalf("diagnostics uri = %s, want %s", params.URI, uri)
	}

	return params.Diagnostics
}

func (h *fiveMFixtureHarness) codeActions(relPath string, rng Range, diags []Diagnostic) []CodeAction {
	h.t.Helper()

	uri := h.server.pathToURI(filepath.Join(h.root, filepath.FromSlash(relPath)))
	params, err := json.Marshal(CodeActionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Range:        rng,
		Context:      CodeActionContext{Diagnostics: diags},
	})
	if err != nil {
		h.t.Fatalf("marshal code action params for %s: %v", relPath, err)
	}

	h.resetRPC()
	h.server.handleCodeAction(Request{RPC: "2.0", ID: 1, Params: params})

	body := h.lastResponse(1)
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		h.t.Fatalf("decode code action response for %s: %v", relPath, err)
	}

	var actions []CodeAction
	if err := json.Unmarshal(envelope.Result, &actions); err != nil {
		h.t.Fatalf("decode code actions for %s: %v", relPath, err)
	}

	return actions
}

func (h *fiveMFixtureHarness) requireSingleDefinitionAt(refMarkerName, defMarkerName string) {
	h.t.Helper()

	locations := h.definition(refMarkerName)
	if len(locations) != 1 {
		h.t.Fatalf("definition count for %s = %d, want 1", refMarkerName, len(locations))
	}

	want := h.requireMarker(defMarkerName)
	got := locations[0]
	if got.URI != want.URI {
		h.t.Fatalf("definition uri for %s = %s, want %s", refMarkerName, got.URI, want.URI)
	}

	if got.Range.Start != want.Position {
		h.t.Fatalf("definition range start for %s = %+v, want %+v", refMarkerName, got.Range.Start, want.Position)
	}
}

func (h *fiveMFixtureHarness) requireMarker(markerName string) fiveMFixtureMarker {
	h.t.Helper()

	marker, ok := h.markers[markerName]
	if !ok {
		h.t.Fatalf("fixture marker %q not found", markerName)
	}

	return marker
}

func (h *fiveMFixtureHarness) positionResponse(markerName string, call func(Request)) json.RawMessage {
	h.t.Helper()

	marker := h.requireMarker(markerName)
	params, err := json.Marshal(TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: marker.URI},
		Position:     marker.Position,
	})
	if err != nil {
		h.t.Fatalf("marshal params for %s: %v", markerName, err)
	}

	h.resetRPC()
	call(Request{RPC: "2.0", ID: 1, Params: params})

	body := h.lastResponse(1)
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		h.t.Fatalf("decode response envelope for %s: %v", markerName, err)
	}

	return envelope.Result
}

func (h *fiveMFixtureHarness) lastResponse(id int) []byte {
	h.t.Helper()

	var last []byte
	for _, msg := range h.readRPCMessages() {
		var envelope struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(msg, &envelope); err == nil && envelope.ID == id {
			last = msg
		}
	}

	if last == nil {
		h.t.Fatalf("response with id %d not found", id)
	}

	return last
}

func (h *fiveMFixtureHarness) lastNotification(method string) []byte {
	h.t.Helper()

	var last []byte
	for _, msg := range h.readRPCMessages() {
		var envelope struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(msg, &envelope); err == nil && envelope.Method == method {
			last = msg
		}
	}

	if last == nil {
		h.t.Fatalf("notification %s not found", method)
	}

	return last
}

func (h *fiveMFixtureHarness) readRPCMessages() [][]byte {
	h.t.Helper()

	reader := bufio.NewReader(bytes.NewReader(h.rpcOut.Bytes()))
	var messages [][]byte
	for {
		msg, err := ReadMessage(reader)
		if err != nil {
			if err == io.EOF {
				break
			}

			h.t.Fatalf("read rpc message: %v", err)
		}

		copied := make([]byte, len(msg))
		copy(copied, msg)
		messages = append(messages, copied)
	}

	h.resetRPC()
	return messages
}

func (h *fiveMFixtureHarness) resetRPC() {
	h.rpcOut.Reset()
}

func stripFiveMFixtureMarkers(source string) (string, map[string]int, error) {
	markers := make(map[string]int)
	var b strings.Builder

	for i := 0; i < len(source); {
		if strings.HasPrefix(source[i:], fiveMFixtureMarkerPrefix) {
			nameStart := i + len(fiveMFixtureMarkerPrefix)
			nameEnd := strings.Index(source[nameStart:], "]]")
			if nameEnd < 0 {
				return "", nil, fmt.Errorf("unterminated fixture marker")
			}

			name := source[nameStart : nameStart+nameEnd]
			if name == "" {
				return "", nil, fmt.Errorf("empty fixture marker")
			}
			if _, exists := markers[name]; exists {
				return "", nil, fmt.Errorf("duplicate fixture marker %q", name)
			}

			markers[name] = b.Len()
			i = nameStart + nameEnd + 2
			continue
		}

		b.WriteByte(source[i])
		i++
	}

	return b.String(), markers, nil
}

func fixturePositionForOffset(source string, offset int) Position {
	var pos Position
	for i := 0; i < offset && i < len(source); i++ {
		if source[i] == '\n' {
			pos.Line++
			pos.Character = 0
			continue
		}

		pos.Character++
	}

	return pos
}

func completionHasLabel(list CompletionList, label string) bool {
	for _, item := range list.Items {
		if item.Label == label {
			return true
		}
	}

	return false
}

func hasDiagnosticCode(diags []Diagnostic, code string) bool {
	for _, diag := range diags {
		if diag.Code == code {
			return true
		}
	}

	return false
}
