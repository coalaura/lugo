package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/coalaura/lugo/ast"
	"github.com/coalaura/lugo/parser"
	"github.com/coalaura/plain"
)

const MaxWorkspaceResults = 100

type Server struct {
	Reader *bufio.Reader
	Writer io.Writer
	Log    *plain.Plain

	// Workspace State
	Documents           map[string]*Document
	OpenFiles           map[string]bool
	activeURIs          map[string]bool
	visitedDirs         map[string]bool
	FiveMResources      map[string]*FiveMResource
	FiveMResourceByName map[string]*FiveMResource

	// Global Index & Resolution
	GlobalIndex       map[GlobalKey][]GlobalSymbol
	KnownGlobals      map[string]bool
	KnownGlobalGlobs  []string
	LibraryPaths      []string
	lowerLibraryPaths []string
	IgnoreGlobs       []string
	compiledIgnores   []IgnorePattern

	// Shared Buffers & Parsers
	sharedParser     *parser.Parser
	diagBuf          []Diagnostic
	semTokensBuf     []SemanticToken
	semDataBuf       []uint32
	actualReadsBuf   []int
	depCache         map[ast.NodeID]DepInfo
	seenKeysBuf      map[uint64]ast.NodeID
	unusedDefsBuf    []bool
	deadStoresBuf    map[ast.NodeID]*DeadStoreInfo
	suggestCache     map[string]string
	visibilityCache  map[*Document]bool
	sharedCommentBuf []byte
	sharedDepBuf     []byte

	Version               string
	RootURI               string
	lowerRootPath         string
	WorkspaceFolders      []string
	lowerWorkspaceFolders []string

	MaxParseErrors int

	// Diagnostics & Features
	IsIndexing               bool
	DiagUndefinedGlobals     bool
	DiagImplicitGlobals      bool
	DiagUnusedLocal          bool
	DiagUnusedFunction       bool
	DiagUnusedParameter      bool
	DiagUnusedLoopVar        bool
	DiagShadowing            bool
	DiagUnreachableCode      bool
	DiagAmbiguousReturns     bool
	DiagDeprecated           bool
	DiagDuplicateField       bool
	DiagUnbalancedAssignment bool
	DiagDuplicateLocal       bool
	DiagSelfAssignment       bool
	DiagEmptyBlock           bool
	DiagFormatString         bool
	DiagTypeCheck            bool
	DiagRedundantParameter   bool
	DiagRedundantValue       bool
	DiagRedundantReturn      bool
	DiagLoopVarMutation      bool
	DiagIncorrectVararg      bool
	DiagShadowingLoopVar     bool
	DiagUnreachableElse      bool
	DiagUsedIgnoredVar       bool

	InlayParamHints    bool
	InlaySuppressMatch bool
	InlayImplicitSelf  bool

	FeatureDocHighlight   bool
	FeatureHoverEval      bool
	FeatureCodeLens       bool
	FeatureFormatting     bool
	FormatOpinionated     bool
	SuggestFunctionParams bool

	FeatureFiveM             bool
	DiagFiveMUnaccountedFile bool
}

func NewServer(version string) *Server {
	return &Server{
		Version: version,
		Reader:  bufio.NewReader(os.Stdin),
		Writer:  os.Stdout,

		// Workspace State
		Documents:           make(map[string]*Document),
		OpenFiles:           make(map[string]bool),
		IsIndexing:          true,
		FiveMResources:      make(map[string]*FiveMResource),
		FiveMResourceByName: make(map[string]*FiveMResource),

		// Global Index
		GlobalIndex: make(map[GlobalKey][]GlobalSymbol),

		// Shared Buffers
		sharedParser:     parser.New(nil, ast.NewTree(nil), 50),
		diagBuf:          make([]Diagnostic, 0, 1024),
		semTokensBuf:     make([]SemanticToken, 0, 4096),
		semDataBuf:       make([]uint32, 0, 4096*5),
		actualReadsBuf:   make([]int, 0, 4096),
		sharedCommentBuf: make([]byte, 0, 1024),
		sharedDepBuf:     make([]byte, 0, 128),

		// Configuration Defaults
		MaxParseErrors: 50,
	}
}

func (s *Server) Start() error {
	s.Log = plain.New(
		plain.WithTarget(os.Stderr),
		plain.WithDate(plain.RFC3339Local),
	)

	s.Log.Printf("Lugo LSP %s Started\n", s.Version)

	for {
		msg, err := ReadMessage(s.Reader)
		if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "closed") {
				s.Log.Println("Input stream closed, stopping server.")

				break
			}

			s.Log.Errorf("Error reading message: %v\n", err)

			continue
		}

		var req Request

		err = json.Unmarshal(msg, &req)
		if err != nil {
			s.Log.Errorf("Failed to unmarshal request: %v\n", err)

			continue
		}

		s.handleMessage(req)
	}

	return nil
}

func (s *Server) applyInitializationOptions(opts InitializationOptions) (needsReindex bool, needsRepublish bool) {
	if s.setLibraryPaths(opts.LibraryPaths) {
		needsReindex = true
	}

	if s.setIgnoreGlobs(opts.IgnoreGlobs) {
		needsReindex = true
	}

	if s.setKnownGlobals(opts.KnownGlobals) {
		needsReindex = true
	}

	setCfg(&s.MaxParseErrors, opts.ParserMaxErrors, &needsRepublish)

	setCfg(&s.DiagUndefinedGlobals, opts.DiagUndefinedGlobals, &needsRepublish)
	setCfg(&s.DiagImplicitGlobals, opts.DiagImplicitGlobals, &needsRepublish)
	setCfg(&s.DiagUnusedLocal, opts.DiagUnusedLocal, &needsRepublish)
	setCfg(&s.DiagUnusedFunction, opts.DiagUnusedFunction, &needsRepublish)
	setCfg(&s.DiagUnusedParameter, opts.DiagUnusedParameter, &needsRepublish)
	setCfg(&s.DiagUnusedLoopVar, opts.DiagUnusedLoopVar, &needsRepublish)
	setCfg(&s.DiagShadowing, opts.DiagShadowing, &needsRepublish)
	setCfg(&s.DiagUnreachableCode, opts.DiagUnreachableCode, &needsRepublish)
	setCfg(&s.DiagAmbiguousReturns, opts.DiagAmbiguousReturns, &needsRepublish)
	setCfg(&s.DiagDeprecated, opts.DiagDeprecated, &needsRepublish)
	setCfg(&s.DiagDuplicateField, opts.DiagDuplicateField, &needsRepublish)
	setCfg(&s.DiagUnbalancedAssignment, opts.DiagUnbalancedAssignment, &needsRepublish)
	setCfg(&s.DiagDuplicateLocal, opts.DiagDuplicateLocal, &needsRepublish)
	setCfg(&s.DiagSelfAssignment, opts.DiagSelfAssignment, &needsRepublish)
	setCfg(&s.DiagEmptyBlock, opts.DiagEmptyBlock, &needsRepublish)
	setCfg(&s.DiagFormatString, opts.DiagFormatString, &needsRepublish)
	setCfg(&s.DiagTypeCheck, opts.DiagTypeCheck, &needsRepublish)
	setCfg(&s.DiagRedundantParameter, opts.DiagRedundantParameter, &needsRepublish)
	setCfg(&s.DiagRedundantValue, opts.DiagRedundantValue, &needsRepublish)
	setCfg(&s.DiagRedundantReturn, opts.DiagRedundantReturn, &needsRepublish)
	setCfg(&s.DiagLoopVarMutation, opts.DiagLoopVarMutation, &needsRepublish)
	setCfg(&s.DiagIncorrectVararg, opts.DiagIncorrectVararg, &needsRepublish)
	setCfg(&s.DiagShadowingLoopVar, opts.DiagShadowingLoopVar, &needsRepublish)
	setCfg(&s.DiagUnreachableElse, opts.DiagUnreachableElse, &needsRepublish)
	setCfg(&s.DiagUsedIgnoredVar, opts.DiagUsedIgnoredVar, &needsRepublish)

	setCfg(&s.InlayParamHints, opts.InlayParamHints, nil)
	setCfg(&s.InlaySuppressMatch, opts.InlaySuppressMatch, nil)
	setCfg(&s.InlayImplicitSelf, opts.InlayImplicitSelf, nil)

	setCfg(&s.FeatureDocHighlight, opts.FeatureDocHighlight, nil)
	setCfg(&s.FeatureHoverEval, opts.FeatureHoverEval, nil)
	setCfg(&s.FeatureCodeLens, opts.FeatureCodeLens, nil)
	setCfg(&s.FeatureFormatting, opts.FeatureFormatting, nil)
	setCfg(&s.FormatOpinionated, opts.FormatOpinionated, nil)
	setCfg(&s.SuggestFunctionParams, opts.SuggestFunctionParams, nil)

	setCfg(&s.FeatureFiveM, opts.FeatureFiveM, &needsReindex)
	setCfg(&s.DiagFiveMUnaccountedFile, opts.DiagFiveMUnaccountedFile, &needsRepublish)

	return needsReindex, needsRepublish
}

func (s *Server) setIgnoreGlobs(globs []string) bool {
	if slices.Equal(s.IgnoreGlobs, globs) {
		return false
	}

	s.IgnoreGlobs = slices.Clone(globs)
	s.compileIgnorePatterns()

	return true
}

func (s *Server) setKnownGlobals(globals []string) bool {
	var (
		newKnownGlobals     map[string]bool
		newKnownGlobalGlobs []string
	)

	if len(globals) > 0 {
		newKnownGlobals = make(map[string]bool, len(globals))
		newKnownGlobalGlobs = make([]string, 0, len(globals))

		for _, g := range globals {
			if strings.ContainsAny(g, "*?") {
				newKnownGlobalGlobs = append(newKnownGlobalGlobs, g)
			} else {
				newKnownGlobals[g] = true
			}
		}
	}

	if mapsEqualStringBool(s.KnownGlobals, newKnownGlobals) && slices.Equal(s.KnownGlobalGlobs, newKnownGlobalGlobs) {
		return false
	}

	s.KnownGlobals = newKnownGlobals
	s.KnownGlobalGlobs = newKnownGlobalGlobs

	return true
}

func (s *Server) setLibraryPaths(paths []string) bool {
	if slices.Equal(s.LibraryPaths, paths) {
		return false
	}

	s.LibraryPaths = slices.Clone(paths)
	s.lowerLibraryPaths = s.lowerLibraryPaths[:0]

	for i, lib := range s.LibraryPaths {
		if realPath, err := filepath.EvalSymlinks(lib); err == nil {
			s.LibraryPaths[i] = realPath

			lib = realPath
		}

		s.lowerLibraryPaths = append(s.lowerLibraryPaths, strings.ToLower(filepath.Clean(filepath.FromSlash(lib))))
	}

	return true
}

func (s *Server) handleMessage(req Request) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()

			s.Log.Errorf("CRITICAL PANIC in method %s: %v\n%s\n", req.Method, r, string(stack))

			// Attempt to notify the client before we die
			if req.ID != 0 {
				WriteMessage(s.Writer, Response{
					RPC: "2.0",
					ID:  req.ID,
					Error: ResponseError{
						Code:    -32603, // InternalError
						Message: fmt.Sprintf("Lugo LSP crashed critically: %v", r),
					},
				})
			}

			// Fail-fast
			os.Exit(1)
		}
	}()

	s.Log.Debugf("Received method: %s\n", req.Method)

	switch req.Method {
	// Lifecycle
	case "initialize":
		s.handleInitialize(req)
	case "shutdown":
		s.handleShutdown(req)
	case "exit":
		s.handleExit(req)

	// Workspace
	case "workspace/didChangeConfiguration":
		s.handleDidChangeConfiguration(req)
	case "workspace/didChangeWatchedFiles":
		s.handleDidChangeWatchedFiles(req)
	case "textDocument/didOpen":
		s.handleDidOpen(req)
	case "textDocument/didChange":
		s.handleDidChange(req)
	case "textDocument/didClose":
		s.handleDidClose(req)
	case "lugo/reindex":
		s.handleReindex(req)
	case "lugo/readStd":
		s.handleReadStd(req)

	// Symbols & Navigation
	case "textDocument/definition":
		s.handleDefinition(req)
	case "textDocument/references":
		s.handleReferences(req)
	case "textDocument/documentSymbol":
		s.handleDocumentSymbol(req)
	case "workspace/symbol":
		s.handleWorkspaceSymbol(req)

	// Refactoring & Code Actions
	case "textDocument/codeAction":
		s.handleCodeAction(req)
	case "codeAction/resolve":
		s.handleCodeActionResolve(req)
	case "workspace/executeCommand":
		s.handleExecuteCommand(req)
	case "textDocument/prepareRename":
		s.handlePrepareRename(req)
	case "textDocument/rename":
		s.handleRename(req)
	case "textDocument/linkedEditingRange":
		s.handleLinkedEditingRange(req)

	// Editor Features
	case "textDocument/hover":
		s.handleHover(req)
	case "textDocument/completion":
		s.handleCompletion(req)
	case "textDocument/signatureHelp":
		s.handleSignatureHelp(req)
	case "textDocument/inlayHint":
		s.handleInlayHint(req)
	case "textDocument/documentHighlight":
		s.handleDocumentHighlight(req)
	case "textDocument/semanticTokens/full":
		s.handleSemanticTokensFull(req)
	case "textDocument/formatting":
		s.handleFormatting(req)
	case "textDocument/rangeFormatting":
		s.handleRangeFormatting(req)
	case "textDocument/foldingRange":
		s.handleFoldingRange(req)
	case "textDocument/selectionRange":
		s.handleSelectionRange(req)
	case "textDocument/codeLens":
		s.handleCodeLens(req)
	case "codeLens/resolve":
		s.handleCodeLensResolve(req)
	case "textDocument/prepareCallHierarchy":
		s.handlePrepareCallHierarchy(req)
	case "callHierarchy/incomingCalls":
		s.handleCallHierarchyIncomingCalls(req)
	case "callHierarchy/outgoingCalls":
		s.handleCallHierarchyOutgoingCalls(req)
	}
}

func (s *Server) handleInitialize(req Request) {
	var params InitializeParams

	err := json.Unmarshal(req.Params, &params)
	if err == nil {
		if len(params.WorkspaceFolders) > 0 {
			for _, folder := range params.WorkspaceFolders {
				uri := s.normalizeURI(folder.URI)

				s.WorkspaceFolders = append(s.WorkspaceFolders, uri)
				s.lowerWorkspaceFolders = append(s.lowerWorkspaceFolders, strings.ToLower(s.uriToPath(uri)))
			}

			s.RootURI = s.WorkspaceFolders[0]
			s.lowerRootPath = s.lowerWorkspaceFolders[0]
		} else if params.RootURI != "" {
			s.RootURI = s.normalizeURI(params.RootURI)
			s.lowerRootPath = strings.ToLower(s.uriToPath(s.RootURI))

			s.WorkspaceFolders = []string{s.RootURI}
			s.lowerWorkspaceFolders = []string{s.lowerRootPath}
		}

		s.applyInitializationOptions(params.InitializationOptions)
	}

	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync:   1,
			DefinitionProvider: true,
			HoverProvider:      true,
			RenameProvider: map[string]bool{
				"prepareProvider": true,
			},
			ReferencesProvider:              true,
			DocumentSymbolProvider:          true,
			WorkspaceSymbolProvider:         true,
			InlayHintProvider:               true,
			FoldingRangeProvider:            true,
			SelectionRangeProvider:          true,
			CallHierarchyProvider:           true,
			DocumentHighlightProvider:       true,
			DocumentFormattingProvider:      true,
			DocumentRangeFormattingProvider: true,
			CodeActionProvider: map[string]any{
				"codeActionKinds": []string{"quickfix", "refactor.rewrite"},
				"resolveProvider": true,
			},
			CodeLensProvider: &CodeLensOptions{
				ResolveProvider: true,
			},
			SignatureHelpProvider: &SignatureHelpOptions{
				TriggerCharacters: []string{"(", ","},
			},
			CompletionProvider: &CompletionOptions{
				TriggerCharacters: []string{".", ":"},
			},
			SemanticTokensProvider: &SemanticTokensOptions{
				Legend: SemanticTokensLegend{
					TokenTypes:     []string{"variable", "property", "parameter", "function", "method", "class", "number", "string", "keyword"},
					TokenModifiers: []string{"declaration", "readonly", "deprecated", "defaultLibrary"},
				},
				Full: true,
			},
			ExecuteCommandProvider: &ExecuteCommandOptions{
				Commands: []string{"lugo.applySafeFixes"},
			},
		},
	}

	err = WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: result})
	if err != nil {
		s.Log.Errorf("WriteMessage error: %v\n", err)
	}
}

func (s *Server) handleShutdown(req Request) {
	err := WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})
	if err != nil {
		s.Log.Errorf("WriteMessage error (shutdown): %v\n", err)
	}
}

func (s *Server) handleExit(req Request) {
	s.Log.Println("Received exit notification, terminating.")

	os.Exit(0)
}

func (s *Server) getRequireModName(doc *Document, callID ast.NodeID) string {
	if callID == ast.InvalidNode || int(callID) >= len(doc.Tree.Nodes) {
		return ""
	}

	node := doc.Tree.Nodes[callID]
	if node.Kind != ast.KindCallExpr {
		return ""
	}

	if int(node.Left) >= len(doc.Tree.Nodes) {
		return ""
	}

	funcNode := doc.Tree.Nodes[node.Left]
	if funcNode.Kind != ast.KindIdent {
		return ""
	}

	funcName := doc.Source[funcNode.Start:funcNode.End]
	if !bytes.Equal(funcName, []byte("require")) {
		return ""
	}

	if node.Count == 0 || node.Extra >= uint32(len(doc.Tree.ExtraList)) {
		return ""
	}

	argID := doc.Tree.ExtraList[node.Extra]

	res, ok := doc.evalNode(argID, 0)
	if ok && res.kind == ast.KindString {
		return res.str
	}

	return ""
}
