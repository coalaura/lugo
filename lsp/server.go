package lsp

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/coalaura/lugo/ast"
	"github.com/coalaura/lugo/parser"
	"github.com/coalaura/lugo/token"
	"github.com/coalaura/plain"
)

const MaxWorkspaceResults = 100

type Server struct {
	Version           string
	Reader            *bufio.Reader
	Writer            io.Writer
	Log               *plain.Plain
	Documents         map[string]*Document
	GlobalIndex       map[GlobalKey]GlobalSymbol
	KnownGlobals      map[string]bool
	OpenFiles         map[string]bool
	RootURI           string
	lowerRootPath     string
	LibraryPaths      []string
	lowerLibraryPaths []string
	KnownGlobalGlobs  []string
	IsIndexing        bool

	activeURIs  map[string]bool
	visitedDirs map[string]bool

	IgnoreGlobs     []string
	compiledIgnores []IgnorePattern

	semTokensBuf []SemanticToken
	semDataBuf   []uint32

	sharedParser *parser.Parser
	diagBuf      []Diagnostic

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
	DiagTypeCheck            bool

	MaxParseErrors int

	InlayParamHints    bool
	InlaySuppressMatch bool
	InlayImplicitSelf  bool

	FeatureDocHighlight bool
	FeatureHoverEval    bool
	FeatureCodeLens     bool
	FeatureFormatting   bool
	FormatOpinionated   bool
}

func NewServer(version string) *Server {
	return &Server{
		Version:      version,
		Reader:       bufio.NewReader(os.Stdin),
		Writer:       os.Stdout,
		Documents:    make(map[string]*Document),
		GlobalIndex:  make(map[GlobalKey]GlobalSymbol),
		OpenFiles:    make(map[string]bool),
		semTokensBuf: make([]SemanticToken, 0, 4096),
		semDataBuf:   make([]uint32, 0, 4096*5),
		sharedParser: parser.New(nil, ast.NewTree(nil), 50),
		diagBuf:      make([]Diagnostic, 0, 1024),
		IsIndexing:   true,
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
	setCfg(&s.DiagTypeCheck, opts.DiagTypeCheck, &needsRepublish)

	setCfg(&s.InlayParamHints, opts.InlayParamHints, nil)
	setCfg(&s.InlaySuppressMatch, opts.InlaySuppressMatch, nil)
	setCfg(&s.InlayImplicitSelf, opts.InlayImplicitSelf, nil)

	setCfg(&s.FeatureDocHighlight, opts.FeatureDocHighlight, nil)
	setCfg(&s.FeatureHoverEval, opts.FeatureHoverEval, nil)
	setCfg(&s.FeatureCodeLens, opts.FeatureCodeLens, nil)
	setCfg(&s.FeatureFormatting, opts.FeatureFormatting, nil)
	setCfg(&s.FormatOpinionated, opts.FormatOpinionated, nil)

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

	for _, lib := range s.LibraryPaths {
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
	case "initialize":
		var params InitializeParams

		err := json.Unmarshal(req.Params, &params)
		if err == nil {
			s.RootURI = params.RootURI
			s.lowerRootPath = strings.ToLower(s.uriToPath(params.RootURI))

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
				ReferencesProvider:         true,
				DocumentSymbolProvider:     true,
				WorkspaceSymbolProvider:    true,
				InlayHintProvider:          true,
				FoldingRangeProvider:       true,
				CallHierarchyProvider:      true,
				DocumentHighlightProvider:  true,
				DocumentFormattingProvider: true,
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
						TokenTypes:     []string{"variable", "property", "parameter", "function", "method", "class"},
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
	case "shutdown":
		err := WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})
		if err != nil {
			s.Log.Errorf("WriteMessage error (shutdown): %v\n", err)
		}
	case "exit":
		s.Log.Println("Received exit notification, terminating.")

		os.Exit(0)
	case "workspace/didChangeConfiguration":
		var params DidChangeConfigurationParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		needsReindex, needsRepublish := s.applyInitializationOptions(params.Settings)

		if needsReindex {
			s.refreshWorkspace()
		} else if needsRepublish {
			s.publishWorkspaceDiagnostics()
		}
	case "workspace/didChangeWatchedFiles":
		var params DidChangeWatchedFilesParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		for _, change := range params.Changes {
			uri := s.normalizeURI(change.URI)

			if s.isIgnoredURI(uri) {
				continue
			}

			switch change.Type {
			case 1, 2: // Created, Changed
				if !s.OpenFiles[uri] {
					path := s.uriToPath(uri)

					if b, err := os.ReadFile(path); err == nil {
						s.updateDocument(uri, b)

						if s.isWorkspaceURI(uri) {
							s.publishDiagnostics(uri)
						}
					}
				}
			case 3: // Deleted
				s.clearDocument(uri)
			}
		}
	case "workspace/executeCommand":
		var params ExecuteCommandParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		if params.Command == "lugo.applySafeFixes" {
			var targetURI string

			if len(params.Arguments) > 0 {
				if uriStr, ok := params.Arguments[0].(string); ok && uriStr != "" {
					targetURI = s.normalizeURI(uriStr)
				}
			}

			changes := make(map[string][]TextEdit)

			if targetURI != "" {
				if doc, ok := s.Documents[targetURI]; ok {
					fixes := s.getSafeFixesForDocument(doc, nil)

					for _, fix := range fixes {
						changes[targetURI] = append(changes[targetURI], fix.Edits...)
					}
				}
			} else {
				for uri, doc := range s.Documents {
					if !s.isWorkspaceURI(uri) {
						continue
					}

					fixes := s.getSafeFixesForDocument(doc, nil)

					for _, fix := range fixes {
						changes[uri] = append(changes[uri], fix.Edits...)
					}
				}
			}

			if len(changes) > 0 {
				WriteMessage(s.Writer, OutgoingRequest{
					RPC:    "2.0",
					ID:     99999, // Fire and forget request ID
					Method: "workspace/applyEdit",
					Params: ApplyWorkspaceEditParams{
						Label: "Apply safe fixes",
						Edit:  WorkspaceEdit{Changes: changes},
					},
				})
			}

			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})
		}
	case "workspace/symbol":
		var params WorkspaceSymbolParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		queryLower := []byte(strings.ToLower(params.Query))

		var (
			results []SymbolInformation
			count   int
		)

		for key, sym := range s.GlobalIndex {
			if !containsFold([]byte(sym.Name), queryLower) {
				continue
			}

			doc, ok := s.Documents[sym.URI]
			if !ok {
				continue
			}

			kind := SymbolKindVariable

			valID := doc.getAssignedValue(sym.NodeID)

			if valID != ast.InvalidNode {
				valKind := doc.Tree.Nodes[valID].Kind
				if valKind == ast.KindFunctionExpr {
					if key.ReceiverHash != 0 {
						kind = SymbolKindMethod
					} else {
						kind = SymbolKindFunction
					}
				} else if valKind == ast.KindTableExpr {
					kind = SymbolKindClass
				} else if key.ReceiverHash != 0 {
					kind = SymbolKindField
				}
			} else if key.ReceiverHash != 0 {
				kind = SymbolKindField
			}

			results = append(results, SymbolInformation{
				Name: sym.Name,
				Kind: kind,
				Location: Location{
					URI:   sym.URI,
					Range: getNodeRange(doc.Tree, sym.NodeID),
				},
			})

			count++

			if count >= MaxWorkspaceResults {
				break
			}
		}

		if results == nil {
			results = []SymbolInformation{}
		}

		WriteMessage(s.Writer, Response{
			RPC:    "2.0",
			ID:     req.ID,
			Result: results,
		})
	case "lugo/reindex":
		s.refreshWorkspace()

		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: "ok"})
	case "lugo/readStd":
		var params ReadStdParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		var content string

		filename := params.URI

		if strings.HasPrefix(filename, "std:///") {
			filename = filename[7:]
		} else if strings.HasPrefix(filename, "std:/") {
			filename = filename[5:]
		} else if strings.HasPrefix(filename, "std://") {
			filename = filename[6:]
		}

		b, err := stdlibFS.ReadFile("stdlib/" + filename)
		if err == nil {
			content = ast.String(b)
		}

		WriteMessage(s.Writer, Response{
			RPC:    "2.0",
			ID:     req.ID,
			Result: ReadStdResult{Content: content},
		})
	case "textDocument/didOpen":
		var params DidOpenTextDocumentParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		if s.isIgnoredURI(uri) {
			return
		}

		s.OpenFiles[uri] = true

		s.updateDocument(uri, []byte(params.TextDocument.Text))

		s.publishDiagnostics(uri)

		s.Log.Debugf("Opened document: %s\n", uri)
	case "textDocument/didChange":
		var params DidChangeTextDocumentParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		if s.isIgnoredURI(uri) {
			if _, ok := s.Documents[uri]; ok {
				s.clearDocument(uri)
			}

			return
		}

		if len(params.ContentChanges) > 0 {
			s.updateDocument(uri, []byte(params.ContentChanges[0].Text))

			s.publishDiagnostics(uri)

			s.Log.Debugf("Updated document: %s\n", uri)
		}
	case "textDocument/didClose":
		var params DidCloseTextDocumentParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		delete(s.OpenFiles, uri)

		path := s.uriToPath(uri)
		if path != "" {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				s.clearDocument(uri)
			}
		}

		s.Log.Debugf("Closed document: %s\n", uri)
	case "textDocument/completion":
		var params CompletionParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		offset := doc.Tree.Offset(params.Position.Line, params.Position.Character)

		items := make([]CompletionItem, 0, 64)
		seen := make(map[string]bool)

		addCompletion := func(label string, kind CompletionItemKind, detail string, isDep bool, sortText string) {
			if label == "" || seen[label] {
				return
			}

			seen[label] = true

			var tags []CompletionItemTag

			if isDep {
				tags = append(tags, CompletionItemTagDeprecated)
			}

			items = append(items, CompletionItem{
				Label:    label,
				Kind:     kind,
				Detail:   detail,
				SortText: sortText,
				Tags:     tags,
			})
		}

		var (
			recName  []byte
			isMember bool
		)

		i := int(offset) - 1

		for i >= 0 {
			c := doc.Source[i]

			isIdentChar := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'

			if !isIdentChar {
				break
			}

			i--
		}

		for i >= 0 {
			c := doc.Source[i]

			if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				i--
			} else {
				break
			}
		}

		if i >= 0 && (doc.Source[i] == '.' || doc.Source[i] == ':') {
			isMember = true

			i--

			for i >= 0 {
				c := doc.Source[i]

				if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
					i--
				} else {
					break
				}
			}

			endId := i + 1

			for i >= 0 {
				c := doc.Source[i]

				isIdentChar := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.'

				if !isIdentChar {
					break
				}

				i--
			}

			startId := i + 1

			if startId < endId {
				recName = doc.Source[startId:endId]
			}
		}

		s.Log.Printf("Completion requested at offset %d. isMember=%v, recName=%s\n", offset, isMember, ast.String(recName))

		if isMember && len(recName) > 0 {
			recHash := ast.HashBytes(recName)

			var recDef ast.NodeID = ast.InvalidNode

			doc.GetLocalsAt(offset, func(name []byte, defID ast.NodeID) bool {
				if bytes.Equal(name, recName) {
					recDef = defID

					return false
				}

				return true
			})

			for _, fd := range doc.Resolver.FieldDefs {
				if (recDef != ast.InvalidNode && fd.ReceiverDef == recDef) || (recDef == ast.InvalidNode && fd.ReceiverHash == recHash) {
					node := doc.Tree.Nodes[fd.NodeID]

					kind := FieldCompletion

					valID := doc.getAssignedValue(fd.NodeID)
					if valID != ast.InvalidNode && doc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
						kind = FunctionCompletion
					}

					isDep, _ := doc.HasDeprecatedTag(fd.NodeID)

					addCompletion(ast.String(doc.Source[node.Start:node.End]), kind, "field", isDep, "1")
				}
			}

			validRecs := make(map[uint64]bool)

			currRec := recHash

			for i := 0; i < 10 && currRec != 0; i++ {
				validRecs[currRec] = true

				currRec = s.getGlobalAlias(currRec)
			}

			for key, sym := range s.GlobalIndex {
				if validRecs[key.ReceiverHash] && key.PropHash != 0 {
					if symDoc, ok := s.Documents[sym.URI]; ok {
						node := symDoc.Tree.Nodes[sym.NodeID]

						kind := FieldCompletion

						valID := symDoc.getAssignedValue(sym.NodeID)
						if valID != ast.InvalidNode && symDoc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
							kind = FunctionCompletion
						}

						isDep, _ := symDoc.HasDeprecatedTag(sym.NodeID)

						sortGroup := "2"
						if sym.URI == uri {
							sortGroup = "1"
						}

						addCompletion(ast.String(symDoc.Source[node.Start:node.End]), kind, "field", isDep, sortGroup)
					}
				}
			}
		} else {
			doc.GetLocalsAt(offset, func(name []byte, defID ast.NodeID) bool {
				isDep, _ := doc.HasDeprecatedTag(defID)

				kind := VariableCompletion

				valID := doc.getAssignedValue(defID)
				if valID != ast.InvalidNode && doc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
					kind = FunctionCompletion
				}

				addCompletion(ast.String(name), kind, "local", isDep, "0")

				return true
			})

			for key, sym := range s.GlobalIndex {
				if key.ReceiverHash == 0 && key.PropHash != 0 {
					if symDoc, ok := s.Documents[sym.URI]; ok {
						node := symDoc.Tree.Nodes[sym.NodeID]

						if node.Kind == ast.KindIdent || node.Kind == ast.KindMethodName {
							kind := VariableCompletion

							valID := symDoc.getAssignedValue(sym.NodeID)

							if valID != ast.InvalidNode && symDoc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
								kind = FunctionCompletion
							}

							isDep, _ := symDoc.HasDeprecatedTag(sym.NodeID)

							sortGroup := "2"
							if sym.URI == uri {
								sortGroup = "1"
							}

							addCompletion(ast.String(symDoc.Source[node.Start:node.End]), kind, "global", isDep, sortGroup)
						}
					}
				}
			}

			for _, kw := range luaKeywords {
				addCompletion(kw, KeywordCompletion, "keyword", false, "3")
			}
		}

		WriteMessage(s.Writer, Response{
			RPC: "2.0",
			ID:  req.ID,
			Result: CompletionList{
				IsIncomplete: false,
				Items:        items,
			},
		})
	case "textDocument/definition":
		var params TextDocumentPositionParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		offset := doc.Tree.Offset(params.Position.Line, params.Position.Character)
		ctx := s.resolveSymbolAt(uri, offset)

		if ctx != nil && ctx.TargetDefID != ast.InvalidNode {
			loc := Location{
				URI:   ctx.TargetURI,
				Range: getNodeRange(ctx.TargetDoc.Tree, ctx.TargetDefID),
			}

			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: []Location{loc}})

			return
		}

		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})
	case "textDocument/documentHighlight":
		var params DocumentHighlightParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		if !s.FeatureDocHighlight {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		offset := doc.Tree.Offset(params.Position.Line, params.Position.Character)

		ctx := s.resolveSymbolAt(uri, offset)
		if ctx == nil {
			curr := doc.Tree.NodeAt(offset)

			for curr != ast.InvalidNode {
				node := doc.Tree.Nodes[curr]

				if node.Kind == ast.KindCallExpr || node.Kind == ast.KindMethodCall {
					var funcIdentID ast.NodeID

					if node.Kind == ast.KindMethodCall {
						funcIdentID = node.Right
					} else {
						funcIdentID = node.Left
						if doc.Tree.Nodes[funcIdentID].Kind == ast.KindMemberExpr {
							funcIdentID = doc.Tree.Nodes[funcIdentID].Right
						}
					}

					if funcIdentID != ast.InvalidNode && doc.Tree.Nodes[funcIdentID].Kind == ast.KindIdent {
						ctx = s.resolveSymbolNode(uri, doc, funcIdentID)
					}

					break
				}

				curr = node.Parent
			}
		}

		if ctx == nil {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: []DocumentHighlight{}})

			return
		}

		highlights := s.getDocumentHighlights(uri, doc, ctx)

		WriteMessage(s.Writer, Response{
			RPC:    "2.0",
			ID:     req.ID,
			Result: highlights,
		})
	case "textDocument/documentSymbol":
		var params DocumentSymbolParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		var walkTable func(tableID ast.NodeID) []DocumentSymbol

		walkTable = func(tableID ast.NodeID) []DocumentSymbol {
			node := doc.Tree.Nodes[tableID]

			var syms []DocumentSymbol

			for i := uint16(0); i < node.Count; i++ {
				fieldID := doc.Tree.ExtraList[node.Extra+uint32(i)]
				fieldNode := doc.Tree.Nodes[fieldID]

				if fieldNode.Kind == ast.KindRecordField {
					keyNode := doc.Tree.Nodes[fieldNode.Left]
					valNode := doc.Tree.Nodes[fieldNode.Right]

					name := ast.String(doc.Source[keyNode.Start:keyNode.End])
					if name == "" {
						name = "<error>"
					}

					kind := SymbolKindField

					var children []DocumentSymbol

					switch valNode.Kind {
					case ast.KindFunctionExpr:
						kind = SymbolKindMethod
					case ast.KindTableExpr:
						kind = SymbolKindClass

						children = walkTable(fieldNode.Right)
					}

					syms = append(syms, DocumentSymbol{
						Name:           name,
						Kind:           kind,
						Range:          getNodeRange(doc.Tree, fieldID),
						SelectionRange: getNodeRange(doc.Tree, fieldNode.Left),
						Children:       children,
					})
				}
			}

			return syms
		}

		var walk func(nodeID ast.NodeID) []DocumentSymbol

		walk = func(nodeID ast.NodeID) []DocumentSymbol {
			if nodeID == ast.InvalidNode {
				return nil
			}

			node := doc.Tree.Nodes[nodeID]

			var syms []DocumentSymbol

			switch node.Kind {
			case ast.KindFile:
				return walk(node.Left)
			case ast.KindBlock:
				for i := uint16(0); i < node.Count; i++ {
					syms = append(syms, walk(doc.Tree.ExtraList[node.Extra+uint32(i)])...)
				}
			case ast.KindLocalFunction, ast.KindFunctionStmt:
				nameNode := doc.Tree.Nodes[node.Left]

				name := ast.String(doc.Source[nameNode.Start:nameNode.End])
				if name == "" {
					name = "<error>"
				}

				kind := SymbolKindFunction

				if nameNode.Kind == ast.KindMethodName {
					kind = SymbolKindMethod
				}

				syms = append(syms, DocumentSymbol{
					Name:           name,
					Kind:           kind,
					Range:          getNodeRange(doc.Tree, nodeID),
					SelectionRange: getNodeRange(doc.Tree, node.Left),
				})
			case ast.KindLocalAssign, ast.KindAssign:
				lhsList := doc.Tree.Nodes[node.Left]
				rhsList := node.Right

				if rhsList != ast.InvalidNode {
					rhsNode := doc.Tree.Nodes[rhsList]

					for i := uint16(0); i < lhsList.Count && i < rhsNode.Count; i++ {
						lID := doc.Tree.ExtraList[lhsList.Extra+uint32(i)]
						lNode := doc.Tree.Nodes[lID]

						var (
							rID   ast.NodeID = ast.InvalidNode
							rNode ast.Node
						)

						if i < rhsNode.Count {
							rID = doc.Tree.ExtraList[rhsNode.Extra+uint32(i)]
							rNode = doc.Tree.Nodes[rID]
						}

						name := ast.String(doc.Source[lNode.Start:lNode.End])
						if name == "" {
							name = "<error>"
						}

						if rNode.Kind == ast.KindFunctionExpr {
							syms = append(syms, DocumentSymbol{
								Name:           name,
								Kind:           SymbolKindFunction,
								Range:          getNodeRange(doc.Tree, nodeID),
								SelectionRange: getNodeRange(doc.Tree, lID),
							})
						} else if rNode.Kind == ast.KindTableExpr {
							syms = append(syms, DocumentSymbol{
								Name:           name,
								Kind:           SymbolKindClass,
								Range:          getNodeRange(doc.Tree, nodeID),
								SelectionRange: getNodeRange(doc.Tree, lID),
								Children:       walkTable(rID),
							})
						} else if node.Kind == ast.KindLocalAssign {
							syms = append(syms, DocumentSymbol{
								Name:           name,
								Kind:           SymbolKindVariable,
								Range:          getNodeRange(doc.Tree, lID),
								SelectionRange: getNodeRange(doc.Tree, lID),
							})
						}
					}
				}
			}

			return syms
		}

		symbols := walk(doc.Tree.Root)

		if symbols == nil {
			symbols = []DocumentSymbol{}
		}

		WriteMessage(s.Writer, Response{
			RPC:    "2.0",
			ID:     req.ID,
			Result: symbols,
		})
	case "textDocument/foldingRange":
		var params FoldingRangeParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		ranges := make([]FoldingRange, 0, 64)

		for i := 1; i < len(doc.Tree.Nodes); i++ {
			node := doc.Tree.Nodes[i]

			switch node.Kind {
			case ast.KindFunctionExpr, ast.KindTableExpr, ast.KindDo, ast.KindWhile, ast.KindRepeat, ast.KindIf, ast.KindElseIf, ast.KindElse, ast.KindForNum, ast.KindForIn, ast.KindString:
				sLine, sCol := doc.Tree.Position(node.Start)
				eLine, eCol := doc.Tree.Position(node.End)

				// Only fold if it spans multiple lines
				if sLine < eLine {
					ranges = append(ranges, FoldingRange{
						StartLine:      sLine,
						StartCharacter: sCol,
						EndLine:        eLine,
						EndCharacter:   eCol,
					})
				}
			}
		}

		for _, c := range doc.Tree.Comments {
			sLine, sCol := doc.Tree.Position(c.Start)
			eLine, eCol := doc.Tree.Position(c.End)

			if sLine < eLine {
				ranges = append(ranges, FoldingRange{
					StartLine:      sLine,
					StartCharacter: sCol,
					EndLine:        eLine,
					EndCharacter:   eCol,
					Kind:           "comment",
				})
			}
		}

		WriteMessage(s.Writer, Response{
			RPC:    "2.0",
			ID:     req.ID,
			Result: ranges,
		})
	case "textDocument/hover":
		var params TextDocumentPositionParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})
			return
		}

		offset := doc.Tree.Offset(params.Position.Line, params.Position.Character)
		ctx := s.resolveSymbolAt(uri, offset)

		var (
			hoverText string
			fromFile  string
			r         *Range
		)

		if ctx != nil {
			parsedRange := getNodeRange(doc.Tree, ctx.IdentNodeID)
			r = &parsedRange

			if ctx.TargetURI != "" && ctx.TargetURI != uri {
				fromFile = filepath.Base(ctx.TargetURI)
			}

			if ctx.TargetDoc != nil && ctx.TargetDefID != ast.InvalidNode {
				rawComments := ctx.TargetDoc.getCommentsAbove(ctx.TargetDefID)
				luadoc := parseLuaDoc(rawComments)

				valID := ctx.TargetDoc.getAssignedValue(ctx.TargetDefID)
				isFunc := valID != ast.InvalidNode && ctx.TargetDoc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr

				var valStr string

				if valID != ast.InvalidNode && int(valID) < len(ctx.TargetDoc.Tree.Nodes) {
					vNode := ctx.TargetDoc.Tree.Nodes[valID]

					switch vNode.Kind {
					case ast.KindNumber, ast.KindString, ast.KindTrue, ast.KindFalse, ast.KindNil:
						if vNode.Start <= vNode.End && vNode.End <= uint32(len(ctx.TargetDoc.Source)) {
							valStr = " = " + ast.String(ctx.TargetDoc.Source[vNode.Start:vNode.End])
						}
					}
				}

				var returnStr string

				if len(luadoc.Returns) == 1 {
					returnStr = ": " + luadoc.Returns[0].Type
				} else if len(luadoc.Returns) > 1 {
					var rTypes []string

					for _, r := range luadoc.Returns {
						rTypes = append(rTypes, r.Type)
					}

					returnStr = ": (" + strings.Join(rTypes, ", ") + ")"
				}

				var (
					code         string
					matchedField *LuaDocField
				)

				if isFunc {
					paramsStr := ctx.TargetDoc.getFunctionParams(valID, luadoc)

					genericStr := ""
					if len(luadoc.Generics) > 0 {
						var gNames []string
						for _, g := range luadoc.Generics {
							gNames = append(gNames, g.Name)
						}
						genericStr = "<" + strings.Join(gNames, ", ") + ">"
					}

					if !ctx.IsProp && ctx.TargetDefID == ctx.IdentNodeID {
						code = "local function " + ctx.DisplayName + genericStr + "(" + paramsStr + ")" + returnStr
					} else {
						code = "function " + ctx.DisplayName + genericStr + "(" + paramsStr + ")" + returnStr
					}
				} else {
					if ctx.IsProp && ctx.TargetDefID != ctx.IdentNodeID {
						for i := range luadoc.Fields {
							if luadoc.Fields[i].Name == ctx.IdentName {
								matchedField = &luadoc.Fields[i]
								break
							}
						}
					}

					if matchedField != nil {
						code = ctx.DisplayName + ": " + matchedField.Type + valStr

						luadoc.Description = matchedField.Desc
						luadoc.Params = nil
						luadoc.Returns = nil
					} else if luadoc.Class != nil {
						code = "class " + luadoc.Class.Name

						if luadoc.Class.Parent != "" {
							code += " : " + luadoc.Class.Parent
						}

						if luadoc.Class.Desc != "" {
							if luadoc.Description != "" {
								luadoc.Description = luadoc.Class.Desc + "\n\n" + luadoc.Description
							} else {
								luadoc.Description = luadoc.Class.Desc
							}
						}
					} else if luadoc.Alias != nil {
						code = "alias " + luadoc.Alias.Name + " = " + luadoc.Alias.Type

						if luadoc.Alias.Desc != "" {
							if luadoc.Description != "" {
								luadoc.Description = luadoc.Alias.Desc + "\n\n" + luadoc.Description
							} else {
								luadoc.Description = luadoc.Alias.Desc
							}
						}
					} else if luadoc.Type != nil {
						code = ctx.DisplayName + ": " + luadoc.Type.Type + valStr

						if luadoc.Type.Desc != "" {
							if luadoc.Description != "" {
								luadoc.Description = luadoc.Type.Desc + "\n\n" + luadoc.Description
							} else {
								luadoc.Description = luadoc.Type.Desc
							}
						}
					} else {
						var baseType TypeSet

						if ctx.TargetDoc != nil && ctx.TargetDefID != ast.InvalidNode {
							baseType = ctx.TargetDoc.InferType(ctx.TargetDefID)
						} else if ctx.IsProp {
							pID := doc.Tree.Nodes[ctx.IdentNodeID].Parent
							if pID != ast.InvalidNode {
								pNode := doc.Tree.Nodes[pID]
								if pNode.Kind == ast.KindMemberExpr || pNode.Kind == ast.KindMethodCall {
									baseType = doc.InferType(pID)
								}
							}
						}

						if ctx.IsProp {
							inferred := doc.ContextualType(ctx.IdentNodeID, offset, baseType)

							typeStr := inferred.Format()
							if typeStr != "any" {
								code = ctx.DisplayName + ": " + typeStr + valStr
							} else {
								code = ctx.DisplayName + valStr
							}
						} else if ctx.TargetURI == uri && ctx.TargetDefID == doc.Resolver.References[ctx.IdentNodeID] {
							var attrStr string

							if ast.Attr(ctx.TargetDoc.Tree.Nodes[ctx.TargetDefID].Extra) == ast.AttrConst {
								attrStr = " <const>"
							} else if ast.Attr(ctx.TargetDoc.Tree.Nodes[ctx.TargetDefID].Extra) == ast.AttrClose {
								attrStr = " <close>"
							}

							inferred := doc.ContextualType(ctx.IdentNodeID, offset, baseType)

							typeStr := inferred.Format()
							if typeStr != "any" {
								code = "local " + ctx.DisplayName + attrStr + ": " + typeStr + valStr
							} else {
								code = "local " + ctx.DisplayName + attrStr + valStr
							}
						} else {
							inferred := doc.ContextualType(ctx.IdentNodeID, offset, baseType)

							typeStr := inferred.Format()
							if typeStr != "any" {
								code = "global " + ctx.DisplayName + ": " + typeStr + valStr
							} else {
								code = "global " + ctx.DisplayName + valStr
							}
						}
					}
				}

				var docBuilder strings.Builder

				if luadoc.IsDeprecated {
					docBuilder.WriteString("**@deprecated**")

					if luadoc.DeprecatedMsg != "" {
						docBuilder.WriteString(" - " + luadoc.DeprecatedMsg)
					}

					docBuilder.WriteString("\n\n")
				}

				if luadoc.Description != "" {
					docBuilder.WriteString(luadoc.Description + "\n\n")
				}

				if len(luadoc.Generics) > 0 {
					for _, g := range luadoc.Generics {
						docBuilder.WriteString("* `@generic` `" + g.Name + "`")

						if g.Parent != "" {
							docBuilder.WriteString(" : `" + g.Parent + "`")
						}

						docBuilder.WriteString("\n")
					}

					docBuilder.WriteString("\n")
				}

				if len(luadoc.Params) > 0 {
					for _, p := range luadoc.Params {
						docBuilder.WriteString("* `@param` `" + p.Name + "`")

						if p.Type != "" {
							docBuilder.WriteString(" `" + p.Type + "`")
						}

						if p.Desc != "" {
							docBuilder.WriteString(" - " + p.Desc)
						}

						docBuilder.WriteString("\n")
					}

					docBuilder.WriteString("\n")
				}

				if len(luadoc.Returns) > 0 {
					for _, ret := range luadoc.Returns {
						docBuilder.WriteString("* `@return` `" + ret.Type + "`")

						if ret.Desc != "" {
							docBuilder.WriteString(" - " + ret.Desc)
						}

						docBuilder.WriteString("\n")
					}

					docBuilder.WriteString("\n")
				}

				if len(luadoc.Fields) > 0 && matchedField == nil {
					docBuilder.WriteString("**Fields**\n")

					for _, f := range luadoc.Fields {
						docBuilder.WriteString("* `" + f.Name + "`")

						if f.Type != "" {
							docBuilder.WriteString(" `" + f.Type + "`")
						}

						if f.Desc != "" {
							docBuilder.WriteString(" - " + f.Desc)
						}

						docBuilder.WriteString("\n")
					}

					docBuilder.WriteString("\n")
				}

				if len(luadoc.Overloads) > 0 {
					docBuilder.WriteString("**Overloads**\n")

					for _, o := range luadoc.Overloads {
						docBuilder.WriteString("```lua\n" + o + "\n```\n")
					}

					docBuilder.WriteString("\n")
				}

				if len(luadoc.See) > 0 {
					docBuilder.WriteString("**See also**\n")

					for _, see := range luadoc.See {
						docBuilder.WriteString("* `" + see + "`\n")
					}

					docBuilder.WriteString("\n")
				}

				docString := strings.TrimSpace(docBuilder.String())

				hoverText = "```lua\n" + code + "\n```"

				if docString != "" {
					hoverText += "\n---\n" + docString
				}

				if fromFile != "" {
					if after, ok := strings.CutPrefix(ctx.TargetURI, "std:///"); ok {
						hoverText += "\n---\n*Standard Library (`" + after + "`)*"
					} else {
						hoverText += "\n---\n*Defined in `" + fromFile + "`*"
					}
				}
			} else {
				var baseType TypeSet

				if ctx.IsProp {
					pID := doc.Tree.Nodes[ctx.IdentNodeID].Parent
					if pID != ast.InvalidNode {
						baseType = doc.InferType(pID)
					}
				}

				inferred := doc.ContextualType(ctx.IdentNodeID, offset, baseType)
				typeStr := inferred.Format()

				if ctx.IsProp {
					if typeStr != "any" {
						hoverText = "```lua\n" + ctx.DisplayName + ": " + typeStr + "\n```"
					} else {
						hoverText = "```lua\n" + ctx.DisplayName + " (field)\n```"
					}
				} else {
					if typeStr != "any" {
						hoverText = "```lua\nglobal " + ctx.DisplayName + ": " + typeStr + "\n```"
					} else {
						hoverText = "```lua\nglobal " + ctx.DisplayName + "\n```"
					}
				}
			}
		}

		if s.FeatureHoverEval {
			if startOff, endOff, evalVal, ok := doc.FindEvaluableParent(offset); ok {
				evalStr := fmt.Sprintf("\n---\n*Evaluates to:*\n```lua\n%s\n```", evalVal)

				if hoverText != "" {
					hoverText += evalStr
				} else {
					hoverText = strings.TrimPrefix(evalStr, "\n---\n")
				}

				evalRange := getRange(doc.Tree, startOff, endOff)
				r = &evalRange
			}
		}

		if hoverText == "" {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		result := Hover{
			Contents: MarkupContent{Kind: "markdown", Value: hoverText},
			Range:    r,
		}

		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: result})
	case "textDocument/inlayHint":
		var params InlayHintParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		if !s.InlayParamHints && !s.InlayImplicitSelf {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: []InlayHint{}})

			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: []InlayHint{}})

			return
		}

		startOffset := doc.Tree.Offset(params.Range.Start.Line, params.Range.Start.Character)
		endOffset := doc.Tree.Offset(params.Range.End.Line, params.Range.End.Character)

		var hints []InlayHint

		for i := 1; i < len(doc.Tree.Nodes); i++ {
			node := doc.Tree.Nodes[i]

			if node.Start > endOffset || node.End < startOffset {
				continue
			}

			// 1. Implicit 'self' hint for method definitions
			if s.InlayImplicitSelf && node.Kind == ast.KindFunctionStmt {
				if int(node.Left) < len(doc.Tree.Nodes) && doc.Tree.Nodes[node.Left].Kind == ast.KindMethodName {
					nameNode := doc.Tree.Nodes[node.Left]

					var funcNode ast.Node

					if int(node.Right) < len(doc.Tree.Nodes) {
						funcNode = doc.Tree.Nodes[node.Right]
					}

					var parenOff uint32

					if nameNode.End != 0xFFFFFFFF && nameNode.End <= uint32(len(doc.Source)) {
						for j := nameNode.End; j < uint32(len(doc.Source)); j++ {
							if doc.Source[j] == '(' {
								parenOff = j + 1

								break
							}
						}
					}

					if parenOff > 0 {
						var label string

						if funcNode.Count > 0 {
							label = "self, "
						} else {
							label = "self"
						}

						sLine, sCol := doc.Tree.Position(parenOff)

						hints = append(hints, InlayHint{
							Position: Position{Line: sLine, Character: sCol},
							Label:    label,
							Kind:     ParameterHint,
							Tooltip:  "Implicit 'self' parameter from colon syntax",
						})
					}
				}

				continue
			}

			// 2. Parameter name hints for function calls
			if !s.InlayParamHints {
				continue
			}

			if node.Kind != ast.KindCallExpr && node.Kind != ast.KindMethodCall {
				continue
			}

			if node.Count == 0 {
				continue
			}

			var funcIdentID ast.NodeID

			if node.Kind == ast.KindMethodCall {
				funcIdentID = node.Right
			} else {
				funcIdentID = node.Left
				if int(funcIdentID) < len(doc.Tree.Nodes) && doc.Tree.Nodes[funcIdentID].Kind == ast.KindMemberExpr {
					funcIdentID = doc.Tree.Nodes[funcIdentID].Right
				}
			}

			if int(funcIdentID) >= len(doc.Tree.Nodes) || doc.Tree.Nodes[funcIdentID].Kind != ast.KindIdent {
				continue
			}

			ctx := s.resolveSymbolAt(uri, doc.Tree.Nodes[funcIdentID].Start)
			if ctx == nil || ctx.TargetDoc == nil || ctx.TargetDefID == ast.InvalidNode {
				continue
			}

			valID := ctx.TargetDoc.getAssignedValue(ctx.TargetDefID)
			if valID == ast.InvalidNode || int(valID) >= len(ctx.TargetDoc.Tree.Nodes) || ctx.TargetDoc.Tree.Nodes[valID].Kind != ast.KindFunctionExpr {
				continue
			}

			hasImplicitSelfCall := node.Kind == ast.KindMethodCall

			var hasImplicitSelfDef bool

			pDefID := ctx.TargetDoc.Tree.Nodes[ctx.TargetDefID].Parent
			if pDefID != ast.InvalidNode && int(pDefID) < len(ctx.TargetDoc.Tree.Nodes) && ctx.TargetDoc.Tree.Nodes[pDefID].Kind == ast.KindMethodName {
				hasImplicitSelfDef = true
			}

			paramOffset := 0
			if hasImplicitSelfCall && !hasImplicitSelfDef {
				paramOffset = 1 // e.g., table:func(arg) -> function table.func(self, arg)
			} else if !hasImplicitSelfCall && hasImplicitSelfDef {
				paramOffset = -1 // e.g., table.func(table, arg) -> function table:func(arg)
			}

			funcNode := ctx.TargetDoc.Tree.Nodes[valID]

			for j := uint16(0); j < node.Count; j++ {
				paramIdx := int(j) + paramOffset

				if paramIdx < 0 || paramIdx >= int(funcNode.Count) {
					continue
				}

				// SAFE GUARD: ExtraList and Node indexing for arguments
				if node.Extra+uint32(j) >= uint32(len(doc.Tree.ExtraList)) {
					continue
				}

				argID := doc.Tree.ExtraList[node.Extra+uint32(j)]
				if argID == ast.InvalidNode || int(argID) >= len(doc.Tree.Nodes) {
					continue
				}

				argNode := doc.Tree.Nodes[argID]

				if funcNode.Extra+uint32(paramIdx) >= uint32(len(ctx.TargetDoc.Tree.ExtraList)) {
					continue
				}

				pID := ctx.TargetDoc.Tree.ExtraList[funcNode.Extra+uint32(paramIdx)]
				if pID == ast.InvalidNode || int(pID) >= len(ctx.TargetDoc.Tree.Nodes) {
					continue
				}

				pNode := ctx.TargetDoc.Tree.Nodes[pID]
				if pNode.Kind == ast.KindVararg {
					continue
				}

				if pNode.Start > pNode.End || pNode.End > uint32(len(ctx.TargetDoc.Source)) {
					continue
				}

				pName := ctx.TargetDoc.Source[pNode.Start:pNode.End]

				if bytes.Equal(pName, []byte("self")) {
					continue
				}

				if s.InlaySuppressMatch && argNode.Kind == ast.KindIdent {
					if argNode.Start <= argNode.End && argNode.End <= uint32(len(doc.Source)) {
						argName := doc.Source[argNode.Start:argNode.End]
						if bytes.Equal(pName, argName) {
							continue
						}
					}
				}

				if argNode.Start == 0xFFFFFFFF {
					continue
				}

				sLine, sCol := doc.Tree.Position(argNode.Start)
				hints = append(hints, InlayHint{
					Position:     Position{Line: sLine, Character: sCol},
					Label:        ast.String(pName) + ":",
					Kind:         ParameterHint,
					PaddingRight: true,
				})
			}
		}

		if hints == nil {
			hints = []InlayHint{}
		}

		WriteMessage(s.Writer, Response{
			RPC:    "2.0",
			ID:     req.ID,
			Result: hints,
		})
	case "textDocument/references":
		var params ReferenceParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		offset := doc.Tree.Offset(params.Position.Line, params.Position.Character)
		ctx := s.resolveSymbolAt(uri, offset)

		if ctx == nil {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: []Location{}})

			return
		}

		locations := s.getReferences(ctx, params.Context.IncludeDeclaration)

		WriteMessage(s.Writer, Response{
			RPC:    "2.0",
			ID:     req.ID,
			Result: locations,
		})
	case "textDocument/semanticTokens/full":
		var params SemanticTokensParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		s.semTokensBuf = s.semTokensBuf[:0]

		for i := 1; i < len(doc.Tree.Nodes); i++ {
			node := doc.Tree.Nodes[i]

			if node.Kind != ast.KindIdent {
				continue
			}

			identBytes := doc.Source[node.Start:node.End]

			var (
				tokenType uint32 = 0 // 0: variable
				modifiers uint32 = 0
			)

			defID := doc.Resolver.References[i]
			isDecl := ast.NodeID(i) == defID

			if isDecl {
				modifiers |= 1 << 0 // declaration
			}

			if defID == ast.InvalidNode {
				if s.isKnownGlobal(identBytes) {
					modifiers |= 1 << 3 // defaultLibrary
				}
			} else {
				pNode := doc.Tree.Nodes[defID]
				if pNode.Parent != ast.InvalidNode {
					parentOfDef := doc.Tree.Nodes[pNode.Parent]
					if parentOfDef.Kind == ast.KindFunctionExpr || parentOfDef.Kind == ast.KindFunctionStmt {
						if parentOfDef.Left != defID && parentOfDef.Right != defID {
							tokenType = 2 // parameter
						}
					}
				}

				if ast.Attr(doc.Tree.Nodes[defID].Extra) != ast.AttrNone {
					parentOfDef := doc.Tree.Nodes[doc.Tree.Nodes[defID].Parent]
					if parentOfDef.Kind == ast.KindNameList {
						modifiers |= 1 << 1 // readonly
					}
				}
			}

			parentID := node.Parent
			if parentID != ast.InvalidNode {
				pNode := doc.Tree.Nodes[parentID]

				if pNode.Kind == ast.KindMemberExpr && pNode.Right == ast.NodeID(i) {
					tokenType = 1 // property
				} else if pNode.Kind == ast.KindMethodCall && pNode.Right == ast.NodeID(i) {
					tokenType = 4 // method
				} else if pNode.Kind == ast.KindMethodName && pNode.Right == ast.NodeID(i) {
					tokenType = 4 // method
				} else if pNode.Kind == ast.KindRecordField && pNode.Left == ast.NodeID(i) {
					tokenType = 1 // property
				}
			}

			if tokenType == 0 || tokenType == 1 {
				targetDoc := doc
				targetDef := defID

				if defID == ast.InvalidNode {
					hash := ast.HashBytes(identBytes)
					recHash := uint64(0)

					if tokenType == 1 && parentID != ast.InvalidNode {
						pNode := doc.Tree.Nodes[parentID]
						recID := pNode.Left
						recBytes := doc.Source[doc.Tree.Nodes[recID].Start:doc.Tree.Nodes[recID].End]
						recHash = ast.HashBytes(recBytes)
					}

					if sym, ok := s.getGlobalSymbol(recHash, hash); ok {
						if gDoc, ok := s.Documents[sym.URI]; ok {
							targetDoc = gDoc
							targetDef = sym.NodeID
						}
					}
				}

				if targetDef != ast.InvalidNode {
					valID := targetDoc.getAssignedValue(targetDef)
					if valID != ast.InvalidNode {
						vNode := targetDoc.Tree.Nodes[valID]
						switch vNode.Kind {
						case ast.KindFunctionExpr:
							if tokenType == 1 {
								tokenType = 4 // method
							} else {
								tokenType = 3 // function
							}
						case ast.KindTableExpr:
							tokenType = 5 // class
						}
					} else {
						pID := targetDoc.Tree.Nodes[targetDef].Parent
						if pID != ast.InvalidNode {
							pNode := targetDoc.Tree.Nodes[pID]
							if pNode.Kind == ast.KindFunctionStmt || pNode.Kind == ast.KindLocalFunction {
								if tokenType == 1 {
									tokenType = 4 // method
								} else {
									tokenType = 3 // function
								}
							}
						}
					}
				}
			}

			if defID != ast.InvalidNode {
				isDep, _ := doc.HasDeprecatedTag(defID)
				if isDep {
					modifiers |= 1 << 2 // deprecated
				}
			}

			s.semTokensBuf = append(s.semTokensBuf, SemanticToken{
				Start:     node.Start,
				End:       node.End,
				TokenType: tokenType,
				Modifiers: modifiers,
			})
		}

		slices.SortFunc(s.semTokensBuf, func(a, b SemanticToken) int {
			return cmp.Compare(a.Start, b.Start)
		})

		s.semDataBuf = s.semDataBuf[:0]

		var (
			prevLine uint32
			prevCol  uint32
			lineIdx  uint32
		)

		lineOffsets := doc.Tree.LineOffsets
		numLines := uint32(len(lineOffsets))

		for _, t := range s.semTokensBuf {
			for lineIdx+1 < numLines && lineOffsets[lineIdx+1] <= t.Start {
				lineIdx++
			}

			line := lineIdx
			col := t.Start - lineOffsets[lineIdx]

			length := t.End - t.Start

			deltaLine := line - prevLine
			deltaCol := col

			if deltaLine == 0 {
				deltaCol = col - prevCol
			}

			s.semDataBuf = append(s.semDataBuf, deltaLine, deltaCol, length, t.TokenType, t.Modifiers)

			prevLine = line
			prevCol = col
		}

		WriteMessage(s.Writer, Response{
			RPC: "2.0",
			ID:  req.ID,
			Result: SemanticTokens{
				Data: s.semDataBuf,
			},
		})
	case "textDocument/signatureHelp":
		var params SignatureHelpParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		offset := doc.Tree.Offset(params.Position.Line, params.Position.Character)

		var (
			isComment bool
			low       int
			high      = len(doc.Tree.Comments)
		)

		for low < high {
			mid := int(uint(low+high) >> 1)

			c := doc.Tree.Comments[mid]
			if c.End < offset {
				low = mid + 1
			} else if c.Start > offset {
				high = mid
			} else {
				isComment = true

				break
			}
		}

		if isComment {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		var callID ast.NodeID = ast.InvalidNode

		curr := doc.Tree.NodeAt(offset)

		for curr != ast.InvalidNode && int(curr) < len(doc.Tree.Nodes) {
			node := doc.Tree.Nodes[curr]

			if node.Kind == ast.KindBlock || node.Kind == ast.KindFunctionExpr || node.Kind == ast.KindString {
				break
			}

			if node.Kind == ast.KindCallExpr || node.Kind == ast.KindMethodCall {
				if int(node.Left) < len(doc.Tree.Nodes) && offset > doc.Tree.Nodes[node.Left].End {
					callID = curr

					break
				}
			}

			curr = node.Parent
		}

		if callID == ast.InvalidNode {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		callNode := doc.Tree.Nodes[callID]

		var funcIdentID ast.NodeID

		if callNode.Kind == ast.KindMethodCall {
			funcIdentID = callNode.Right
		} else {
			funcIdentID = callNode.Left
		}

		if funcIdentID == ast.InvalidNode || int(funcIdentID) >= len(doc.Tree.Nodes) {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		ctx := s.resolveSymbolAt(uri, doc.Tree.Nodes[funcIdentID].Start)
		if ctx == nil || ctx.TargetDoc == nil || ctx.TargetDefID == ast.InvalidNode {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		valID := ctx.TargetDoc.getAssignedValue(ctx.TargetDefID)
		if valID == ast.InvalidNode || int(valID) >= len(ctx.TargetDoc.Tree.Nodes) || ctx.TargetDoc.Tree.Nodes[valID].Kind != ast.KindFunctionExpr {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		luadoc := parseLuaDoc(ctx.TargetDoc.getCommentsAbove(ctx.TargetDefID))
		funcNode := ctx.TargetDoc.Tree.Nodes[valID]

		var (
			paramsInfo []ParameterInformation
			labels     []string
		)

		paramDocs := make(map[string]LuaDocParam)

		for _, p := range luadoc.Params {
			paramDocs[p.Name] = p
		}

		for i := uint16(0); i < funcNode.Count; i++ {
			if funcNode.Extra+uint32(i) >= uint32(len(ctx.TargetDoc.Tree.ExtraList)) {
				continue
			}

			pID := ctx.TargetDoc.Tree.ExtraList[funcNode.Extra+uint32(i)]
			if pID == ast.InvalidNode || int(pID) >= len(ctx.TargetDoc.Tree.Nodes) {
				continue
			}

			pNode := ctx.TargetDoc.Tree.Nodes[pID]
			if pNode.Start > pNode.End || pNode.End > uint32(len(ctx.TargetDoc.Source)) {
				continue
			}

			pName := ast.String(ctx.TargetDoc.Source[pNode.Start:pNode.End])

			label := pName

			var docContent *MarkupContent

			if pDoc, ok := paramDocs[pName]; ok {
				if pDoc.Type != "" {
					label += ": " + pDoc.Type
				}

				if pDoc.Desc != "" {
					docContent = &MarkupContent{Kind: "markdown", Value: pDoc.Desc}
				}
			}

			labels = append(labels, label)
			paramsInfo = append(paramsInfo, ParameterInformation{
				Label:         label,
				Documentation: docContent,
			})
		}

		var activeParam int

		for i := uint16(0); i < callNode.Count; i++ {
			if callNode.Extra+uint32(i) >= uint32(len(doc.Tree.ExtraList)) {
				continue
			}

			argID := doc.Tree.ExtraList[callNode.Extra+uint32(i)]
			if argID == ast.InvalidNode || int(argID) >= len(doc.Tree.Nodes) {
				continue
			}

			argNode := doc.Tree.Nodes[argID]

			if offset > argNode.End {
				activeParam = int(i) + 1
			} else {
				activeParam = int(i)

				break
			}
		}

		var funcDoc *MarkupContent

		if luadoc.Description != "" {
			funcDoc = &MarkupContent{Kind: "markdown", Value: luadoc.Description}
		}

		sigInfo := SignatureInformation{
			Label:         ctx.DisplayName + "(" + strings.Join(labels, ", ") + ")",
			Documentation: funcDoc,
			Parameters:    paramsInfo,
		}

		WriteMessage(s.Writer, Response{
			RPC: "2.0",
			ID:  req.ID,
			Result: SignatureHelp{
				Signatures:      []SignatureInformation{sigInfo},
				ActiveSignature: 0,
				ActiveParameter: activeParam,
			},
		})
	case "textDocument/prepareCallHierarchy":
		var params CallHierarchyPrepareParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		offset := doc.Tree.Offset(params.Position.Line, params.Position.Character)

		ctx := s.resolveSymbolAt(uri, offset)
		if ctx == nil || ctx.TargetDoc == nil || ctx.TargetDefID == ast.InvalidNode {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		item := s.buildCallHierarchyItemFromDef(ctx.TargetURI, ctx.TargetDoc, ctx.TargetDefID)

		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: []CallHierarchyItem{item}})
	case "callHierarchy/incomingCalls":
		var params CallHierarchyIncomingCallsParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		data, ok := params.Item.Data.(map[string]any)
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		uri, _ := data["uri"].(string)
		defIDFloat, _ := data["defId"].(float64)
		defID := ast.NodeID(defIDFloat)

		doc, ok := s.Documents[uri]
		if !ok || defID == ast.InvalidNode || int(defID) >= len(doc.Tree.Nodes) {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		ctx := s.resolveSymbolNode(uri, doc, defID)
		if ctx == nil {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		locations := s.getReferences(ctx, false)

		callers := make(map[CallerKey][]Range)

		for _, loc := range locations {
			refDoc := s.Documents[loc.URI]
			if refDoc == nil {
				continue
			}

			offset := refDoc.Tree.Offset(loc.Range.Start.Line, loc.Range.Start.Character)

			refID := refDoc.Tree.NodeAt(offset)
			if refID == ast.InvalidNode {
				continue
			}

			pID := refDoc.Tree.Nodes[refID].Parent
			if pID == ast.InvalidNode || int(pID) >= len(refDoc.Tree.Nodes) {
				continue
			}

			pNode := refDoc.Tree.Nodes[pID]

			isCall := false
			callNodeID := ast.InvalidNode

			if pNode.Kind == ast.KindCallExpr && pNode.Left == refID {
				isCall = true
				callNodeID = pID
			} else if pNode.Kind == ast.KindMethodCall && pNode.Right == refID {
				isCall = true
				callNodeID = pID
			} else if pNode.Kind == ast.KindMemberExpr {
				gpID := refDoc.Tree.Nodes[pID].Parent
				if gpID != ast.InvalidNode && int(gpID) < len(refDoc.Tree.Nodes) {
					gpNode := refDoc.Tree.Nodes[gpID]
					if gpNode.Kind == ast.KindCallExpr && gpNode.Left == pID {
						isCall = true
						callNodeID = gpID
					}
				}
			}

			if isCall {
				enclosingFuncDefID := s.getEnclosingFunctionDef(refDoc, callNodeID)

				cKey := CallerKey{URI: loc.URI, Def: enclosingFuncDefID}

				callers[cKey] = append(callers[cKey], getNodeRange(refDoc.Tree, callNodeID))
			}
		}

		var result []CallHierarchyIncomingCall

		for key, ranges := range callers {
			cDoc := s.Documents[key.URI]
			if cDoc == nil {
				continue
			}

			item := s.buildCallHierarchyItemFromDef(key.URI, cDoc, key.Def)

			result = append(result, CallHierarchyIncomingCall{
				From:       item,
				FromRanges: ranges,
			})
		}

		if result == nil {
			result = []CallHierarchyIncomingCall{}
		}

		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: result})
	case "callHierarchy/outgoingCalls":
		var params CallHierarchyOutgoingCallsParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		data, ok := params.Item.Data.(map[string]any)
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		uri, _ := data["uri"].(string)
		defIDFloat, _ := data["defId"].(float64)
		defID := ast.NodeID(defIDFloat)

		doc, ok := s.Documents[uri]
		if !ok || defID == ast.InvalidNode || int(defID) >= len(doc.Tree.Nodes) {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		var root ast.NodeID

		valID := doc.getAssignedValue(defID)

		if valID != ast.InvalidNode && int(valID) < len(doc.Tree.Nodes) && doc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
			root = valID
		} else if doc.Tree.Nodes[defID].Kind == ast.KindFile || doc.Tree.Nodes[defID].Kind == ast.KindFunctionExpr {
			root = defID
		} else {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: []CallHierarchyOutgoingCall{}})

			return
		}

		targets := make(map[TargetKey][]Range)

		var walk func(id ast.NodeID)

		walk = func(id ast.NodeID) {
			if id == ast.InvalidNode || int(id) >= len(doc.Tree.Nodes) {
				return
			}

			node := doc.Tree.Nodes[id]

			if id != root && node.Kind == ast.KindFunctionExpr {
				return
			}

			if node.Kind == ast.KindCallExpr || node.Kind == ast.KindMethodCall {
				var identID ast.NodeID

				if node.Kind == ast.KindCallExpr {
					if int(node.Left) < len(doc.Tree.Nodes) {
						switch doc.Tree.Nodes[node.Left].Kind {
						case ast.KindIdent:
							identID = node.Left
						case ast.KindMemberExpr:
							identID = doc.Tree.Nodes[node.Left].Right
						}
					}
				} else {
					identID = node.Right
				}

				if identID != ast.InvalidNode && int(identID) < len(doc.Tree.Nodes) {
					ctx := s.resolveSymbolNode(uri, doc, identID)
					if ctx != nil && ctx.TargetDefID != ast.InvalidNode && ctx.TargetDoc != nil {
						tKey := TargetKey{URI: ctx.TargetURI, Def: ctx.TargetDefID}

						targets[tKey] = append(targets[tKey], getNodeRange(doc.Tree, id))
					}
				}
			}

			walk(node.Left)
			walk(node.Right)

			for i := uint16(0); i < node.Count; i++ {
				if node.Extra+uint32(i) < uint32(len(doc.Tree.ExtraList)) {
					walk(doc.Tree.ExtraList[node.Extra+uint32(i)])
				}
			}
		}

		walk(root)

		var result []CallHierarchyOutgoingCall

		for key, ranges := range targets {
			tDoc := s.Documents[key.URI]
			if tDoc == nil {
				continue
			}

			item := s.buildCallHierarchyItemFromDef(key.URI, tDoc, key.Def)

			result = append(result, CallHierarchyOutgoingCall{
				To:         item,
				FromRanges: ranges,
			})
		}

		if result == nil {
			result = []CallHierarchyOutgoingCall{}
		}

		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: result})
	case "textDocument/codeAction":
		var params CodeActionParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		var (
			actions   []CodeAction
			hasUnused bool
		)

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, docOk := s.Documents[uri]
		if !docOk {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: []CodeAction{}})

			return
		}

		for _, diag := range params.Context.Diagnostics {
			switch diag.Code {
			case "unused-local", "unused-parameter", "unused-loop-var", "unused-vararg", "unused-function", "unreachable-code", "ambiguous-return":
				hasUnused = true
			case "undefined-global":
				if suggestion, ok := diag.Data.(string); ok && suggestion != "" {
					actions = append(actions, CodeAction{
						Title:       fmt.Sprintf("Change to '%s'", suggestion),
						Kind:        "quickfix",
						Diagnostics: []Diagnostic{diag},
						IsPreferred: true,
						Edit: &WorkspaceEdit{
							Changes: map[string][]TextEdit{
								uri: {
									{
										Range:   diag.Range,
										NewText: suggestion,
									},
								},
							},
						},
					})
				}
			case "implicit-global":
				actions = append(actions, CodeAction{
					Title:       "Prefix variable with 'local'",
					Kind:        "quickfix",
					Diagnostics: []Diagnostic{diag},
					IsPreferred: true,
					Edit: &WorkspaceEdit{
						Changes: map[string][]TextEdit{
							uri: {
								{
									Range: Range{
										Start: diag.Range.Start,
										End:   diag.Range.Start,
									},
									NewText: "local ",
								},
							},
						},
					},
				})
			case "self-assignment":
				if stmtIDFloat, ok := diag.Data.(float64); ok {
					stmtID := ast.NodeID(stmtIDFloat)
					stmtNode := doc.Tree.Nodes[stmtID]

					// Only provide a clean fix if it's a single assignment
					if doc.Tree.Nodes[stmtNode.Left].Count == 1 {
						actions = append(actions, CodeAction{
							Title:       "Remove self-assignment",
							Kind:        "quickfix",
							Diagnostics: []Diagnostic{diag},
							IsPreferred: true,
							Edit: &WorkspaceEdit{
								Changes: map[string][]TextEdit{
									uri: {
										{
											Range:   s.getStatementRemovalRange(doc, stmtID),
											NewText: "",
										},
									},
								},
							},
						})
					}
				}
			case "empty-block":
				if doStmtIDFloat, ok := diag.Data.(float64); ok {
					doStmtID := ast.NodeID(doStmtIDFloat)
					actions = append(actions, CodeAction{
						Title:       "Remove empty 'do' block",
						Kind:        "quickfix",
						Diagnostics: []Diagnostic{diag},
						IsPreferred: true,
						Edit: &WorkspaceEdit{
							Changes: map[string][]TextEdit{
								uri: {
									{
										Range:   s.getStatementRemovalRange(doc, doStmtID),
										NewText: "",
									},
								},
							},
						},
					})
				}
			}
		}

		if hasUnused && docOk {
			allFixes := s.getSafeFixesForDocument(doc, nil)

			var allEdits []TextEdit

			for _, diag := range params.Context.Diagnostics {
				if diag.Code != "unused-local" && diag.Code != "unused-parameter" && diag.Code != "unused-loop-var" && diag.Code != "unused-vararg" && diag.Code != "unused-function" && diag.Code != "unreachable-code" && diag.Code != "ambiguous-return" {
					continue
				}

				defIDFloat, ok := diag.Data.(float64)
				if !ok {
					continue
				}

				defID := ast.NodeID(defIDFloat)

				var (
					editsForDef []TextEdit
					title       string
				)

				for _, fix := range allFixes {
					if slices.Contains(fix.Coverage, defID) {
						editsForDef = append(editsForDef, fix.Edits...)

						if title == "" {
							title = fix.Title
						}
					}
				}

				if len(editsForDef) > 0 {
					actions = append(actions, CodeAction{
						Title:       title,
						Kind:        "quickfix",
						Diagnostics: []Diagnostic{diag},
						IsPreferred: true,
						Edit: &WorkspaceEdit{
							Changes: map[string][]TextEdit{
								uri: editsForDef,
							},
						},
					})
				}
			}

			for _, fix := range allFixes {
				allEdits = append(allEdits, fix.Edits...)
			}

			if len(allEdits) > 0 {
				actions = append(actions, CodeAction{
					Title: "Apply all safe fixes in file",
					Kind:  "source.fixAll",
					Edit: &WorkspaceEdit{
						Changes: map[string][]TextEdit{
							uri: allEdits,
						},
					},
				})
			}
		}

		var (
			targetIf          ast.NodeID = ast.InvalidNode
			targetCond        ast.NodeID = ast.InvalidNode
			condTitle         string
			targetTableInsert ast.NodeID = ast.InvalidNode
			targetMethod      ast.NodeID = ast.InvalidNode
			targetNestedIf    ast.NodeID = ast.InvalidNode
			nestedIfTitle     string
		)

		cursorLine := params.Range.Start.Line

		for i := 1; i < len(doc.Tree.Nodes); i++ {
			node := doc.Tree.Nodes[i]
			if node.Kind == ast.KindIf || node.Kind == ast.KindElseIf || node.Kind == ast.KindWhile {
				sLine, _ := doc.Tree.Position(node.Start)
				if sLine == cursorLine {
					switch node.Kind {
					case ast.KindIf:
						targetIf = ast.NodeID(i)
						condTitle = "Invert 'if' condition"
					case ast.KindElseIf:
						condTitle = "Invert 'elseif' condition"
					case ast.KindWhile:
						condTitle = "Invert 'while' condition"
					}

					targetCond = node.Left

					break
				}
			}
		}

		offset := doc.Tree.Offset(params.Range.Start.Line, params.Range.Start.Character)
		curr := doc.Tree.NodeAt(offset)

		for curr != ast.InvalidNode {
			node := doc.Tree.Nodes[curr]

			// 1. Convert Method Signature
			if node.Kind == ast.KindFunctionStmt && targetMethod == ast.InvalidNode {
				if int(node.Right) < len(doc.Tree.Nodes) {
					funcExprNode := doc.Tree.Nodes[node.Right]
					if int(funcExprNode.Right) < len(doc.Tree.Nodes) {
						blockNode := doc.Tree.Nodes[funcExprNode.Right]
						if offset <= blockNode.Start {
							if int(node.Left) < len(doc.Tree.Nodes) {
								nameNode := doc.Tree.Nodes[node.Left]
								if nameNode.Kind == ast.KindMethodName || nameNode.Kind == ast.KindMemberExpr {
									targetMethod = curr
								}
							}
						}
					}
				}
			}

			// 2. Optimize table.insert
			if node.Kind == ast.KindCallExpr && targetTableInsert == ast.InvalidNode && node.Count == 2 {
				if int(node.Left) < len(doc.Tree.Nodes) {
					leftNode := doc.Tree.Nodes[node.Left]
					if leftNode.Kind == ast.KindMemberExpr && int(leftNode.Left) < len(doc.Tree.Nodes) && int(leftNode.Right) < len(doc.Tree.Nodes) {
						recNode := doc.Tree.Nodes[leftNode.Left]
						propNode := doc.Tree.Nodes[leftNode.Right]

						if recNode.Start <= recNode.End && recNode.End <= uint32(len(doc.Source)) &&
							propNode.Start <= propNode.End && propNode.End <= uint32(len(doc.Source)) {

							recName := doc.Source[recNode.Start:recNode.End]
							propName := doc.Source[propNode.Start:propNode.End]

							if bytes.Equal(recName, []byte("table")) && bytes.Equal(propName, []byte("insert")) {
								targetTableInsert = curr
							}
						}
					}
				}
			}

			// 3. Merge Nested If
			if node.Kind == ast.KindIf && targetNestedIf == ast.InvalidNode {
				var hasElse bool

				for i := uint16(0); i < node.Count; i++ {
					if node.Extra+uint32(i) < uint32(len(doc.Tree.ExtraList)) {
						childID := doc.Tree.ExtraList[node.Extra+uint32(i)]
						if int(childID) < len(doc.Tree.Nodes) {
							if doc.Tree.Nodes[childID].Kind == ast.KindElseIf || doc.Tree.Nodes[childID].Kind == ast.KindElse {
								hasElse = true

								break
							}
						}
					}
				}

				if !hasElse {
					if int(node.Right) < len(doc.Tree.Nodes) {
						blockNode := doc.Tree.Nodes[node.Right]
						if blockNode.Count == 1 && blockNode.Extra < uint32(len(doc.Tree.ExtraList)) {
							innerStmtID := doc.Tree.ExtraList[blockNode.Extra]
							if int(innerStmtID) < len(doc.Tree.Nodes) {
								innerStmt := doc.Tree.Nodes[innerStmtID]
								if innerStmt.Kind == ast.KindIf {
									var innerHasElse bool

									for i := uint16(0); i < innerStmt.Count; i++ {
										if innerStmt.Extra+uint32(i) < uint32(len(doc.Tree.ExtraList)) {
											childID := doc.Tree.ExtraList[innerStmt.Extra+uint32(i)]
											if int(childID) < len(doc.Tree.Nodes) {
												if doc.Tree.Nodes[childID].Kind == ast.KindElseIf || doc.Tree.Nodes[childID].Kind == ast.KindElse {
													innerHasElse = true

													break
												}
											}
										}
									}

									if !innerHasElse {
										targetNestedIf = curr
										nestedIfTitle = "Merge nested 'if' statements"
									}
								}
							}
						}
					}
				}
			}

			curr = node.Parent
		}

		// 1. Condition Inverter
		if targetCond != ast.InvalidNode {
			actions = append(actions, CodeAction{
				Title:       condTitle,
				Kind:        "refactor.rewrite",
				IsPreferred: false,
				Data: map[string]any{
					"type":   "invertCondition",
					"uri":    uri,
					"nodeId": float64(targetCond),
				},
			})
		}

		// 2. Recursive Early-Return Converter
		if targetIf != ast.InvalidNode {
			ifNode := doc.Tree.Nodes[targetIf]

			var hasElseIf bool

			for i := uint16(0); i < ifNode.Count; i++ {
				child := doc.Tree.Nodes[doc.Tree.ExtraList[ifNode.Extra+uint32(i)]]
				if child.Kind == ast.KindElseIf {
					hasElseIf = true

					break
				}
			}

			_, isSafe := s.checkSafetyAndBudget(doc, targetIf, 3)

			if !hasElseIf && isSafe {
				actions = append(actions, CodeAction{
					Title:       "Convert to early returns (recursive)",
					Kind:        "refactor.rewrite",
					IsPreferred: false,
					Data: map[string]any{
						"type":   "earlyReturn",
						"uri":    uri,
						"nodeId": float64(targetIf),
					},
				})
			}
		}

		// 3. Optimize table.insert
		if targetTableInsert != ast.InvalidNode {
			actions = append(actions, CodeAction{
				Title:       "Optimize 'table.insert' to 't[#t+1]'",
				Kind:        "refactor.rewrite",
				IsPreferred: true,
				Data: map[string]any{
					"type":   "optimizeTableInsert",
					"uri":    uri,
					"nodeId": float64(targetTableInsert),
				},
			})
		}

		// 4. Convert Method Signature
		if targetMethod != ast.InvalidNode {
			methodNode := doc.Tree.Nodes[targetMethod]
			nameNode := doc.Tree.Nodes[methodNode.Left]

			title := "Convert to dot notation (.method)"

			if nameNode.Kind == ast.KindMemberExpr {
				title = "Convert to colon notation (:method)"
			}

			actions = append(actions, CodeAction{
				Title:       title,
				Kind:        "refactor.rewrite",
				IsPreferred: false,
				Data: map[string]any{
					"type":   "convertMethodSig",
					"uri":    uri,
					"nodeId": float64(targetMethod),
				},
			})
		}

		// 5. Merge Nested If
		if targetNestedIf != ast.InvalidNode {
			actions = append(actions, CodeAction{
				Title:       nestedIfTitle,
				Kind:        "refactor.rewrite",
				IsPreferred: false,
				Data: map[string]any{
					"type":   "mergeNestedIf",
					"uri":    uri,
					"nodeId": float64(targetNestedIf),
				},
			})
		}

		if actions == nil {
			actions = []CodeAction{}
		}

		WriteMessage(s.Writer, Response{
			RPC:    "2.0",
			ID:     req.ID,
			Result: actions,
		})
	case "codeAction/resolve":
		var action CodeAction

		err := json.Unmarshal(req.Params, &action)
		if err != nil {
			return
		}

		if data, ok := action.Data.(map[string]any); ok {
			actionType, _ := data["type"].(string)
			uri, _ := data["uri"].(string)
			nodeIDFloat, _ := data["nodeId"].(float64)

			nodeID := ast.NodeID(nodeIDFloat)

			if doc, ok := s.Documents[uri]; ok && nodeID != ast.InvalidNode && int(nodeID) < len(doc.Tree.Nodes) {
				if actionType == "invertCondition" {
					invertedCond := s.invertCondition(doc, nodeID)
					action.Edit = &WorkspaceEdit{
						Changes: map[string][]TextEdit{
							uri: {{
								Range:   getNodeRange(doc.Tree, nodeID),
								NewText: invertedCond,
							}},
						},
					}
				} else if actionType == "earlyReturn" {
					ifNode := doc.Tree.Nodes[nodeID]
					ifLine, _ := doc.Tree.Position(ifNode.Start)

					var lineStart uint32

					if int(ifLine) < len(doc.Tree.LineOffsets) {
						lineStart = doc.Tree.LineOffsets[ifLine]
					}

					var indentBytes []byte

					for i := lineStart; i < ifNode.Start && i < uint32(len(doc.Source)); i++ {
						if doc.Source[i] == ' ' || doc.Source[i] == '\t' {
							indentBytes = append(indentBytes, doc.Source[i])
						} else {
							break
						}
					}

					indent := string(indentBytes)

					outText := s.formatStatement(doc, nodeID, indent, 3)

					action.Edit = &WorkspaceEdit{
						Changes: map[string][]TextEdit{
							uri: {{
								Range:   getNodeRange(doc.Tree, nodeID),
								NewText: trimTrailingWhitespace(outText),
							}},
						},
					}
				} else if actionType == "optimizeTableInsert" {
					callNode := doc.Tree.Nodes[nodeID]
					if callNode.Count >= 2 && callNode.Extra+1 < uint32(len(doc.Tree.ExtraList)) {
						arg1ID := doc.Tree.ExtraList[callNode.Extra]
						arg2ID := doc.Tree.ExtraList[callNode.Extra+1]

						if int(arg1ID) < len(doc.Tree.Nodes) && int(arg2ID) < len(doc.Tree.Nodes) {
							arg1Node := doc.Tree.Nodes[arg1ID]
							arg2Node := doc.Tree.Nodes[arg2ID]

							if arg1Node.Start <= arg1Node.End && arg1Node.End <= uint32(len(doc.Source)) &&
								arg2Node.Start <= arg2Node.End && arg2Node.End <= uint32(len(doc.Source)) {

								arg1 := ast.String(doc.Source[arg1Node.Start:arg1Node.End])
								arg2 := ast.String(doc.Source[arg2Node.Start:arg2Node.End])

								newText := fmt.Sprintf("%s[#%s+1] = %s", arg1, arg1, arg2)

								action.Edit = &WorkspaceEdit{
									Changes: map[string][]TextEdit{
										uri: {{
											Range:   getNodeRange(doc.Tree, nodeID),
											NewText: newText,
										}},
									},
								}
							}
						}
					}
				} else if actionType == "convertMethodSig" {
					funcNode := doc.Tree.Nodes[nodeID]
					if int(funcNode.Left) < len(doc.Tree.Nodes) {
						nameNode := doc.Tree.Nodes[funcNode.Left]

						if int(nameNode.Left) < len(doc.Tree.Nodes) && int(nameNode.Right) < len(doc.Tree.Nodes) {
							leftNode := doc.Tree.Nodes[nameNode.Left]
							rightNode := doc.Tree.Nodes[nameNode.Right]

							if leftNode.Start <= leftNode.End && leftNode.End <= uint32(len(doc.Source)) &&
								rightNode.Start <= rightNode.End && rightNode.End <= uint32(len(doc.Source)) {

								leftStr := ast.String(doc.Source[leftNode.Start:leftNode.End])
								rightStr := ast.String(doc.Source[rightNode.Start:rightNode.End])

								var newText string

								if nameNode.Kind == ast.KindMethodName {
									newText = leftStr + "." + rightStr
								} else {
									newText = leftStr + ":" + rightStr
								}

								action.Edit = &WorkspaceEdit{
									Changes: map[string][]TextEdit{
										uri: {{
											Range:   getNodeRange(doc.Tree, funcNode.Left),
											NewText: newText,
										}},
									},
								}
							}
						}
					}
				} else if actionType == "mergeNestedIf" {
					ifNode := doc.Tree.Nodes[nodeID]
					if int(ifNode.Right) < len(doc.Tree.Nodes) {
						blockNode := doc.Tree.Nodes[ifNode.Right]

						if blockNode.Count > 0 && blockNode.Extra < uint32(len(doc.Tree.ExtraList)) {
							innerIfID := doc.Tree.ExtraList[blockNode.Extra]

							if int(innerIfID) < len(doc.Tree.Nodes) {
								innerIfNode := doc.Tree.Nodes[innerIfID]

								wrapCond := func(condID ast.NodeID) string {
									if int(condID) >= len(doc.Tree.Nodes) {
										return ""
									}

									condNode := doc.Tree.Nodes[condID]
									if condNode.Start > condNode.End || condNode.End > uint32(len(doc.Source)) {
										return ""
									}

									condStr := ast.String(doc.Source[condNode.Start:condNode.End])

									if condNode.Kind == ast.KindBinaryExpr && token.Kind(condNode.Extra) == token.Or {
										return "(" + condStr + ")"
									}

									return condStr
								}

								newCond := wrapCond(ifNode.Left) + " and " + wrapCond(innerIfNode.Left)

								ifLine, _ := doc.Tree.Position(ifNode.Start)

								var lineStart uint32

								if int(ifLine) < len(doc.Tree.LineOffsets) {
									lineStart = doc.Tree.LineOffsets[ifLine]
								}

								var indentBytes []byte

								for i := lineStart; i < ifNode.Start && i < uint32(len(doc.Source)); i++ {
									if doc.Source[i] == ' ' || doc.Source[i] == '\t' {
										indentBytes = append(indentBytes, doc.Source[i])
									} else {
										break
									}
								}

								indent := string(indentBytes)

								innerIndent := indent + "\t"
								if bytes.Contains(indentBytes, []byte("    ")) {
									innerIndent = indent + "    "
								} else if bytes.Contains(indentBytes, []byte("  ")) {
									innerIndent = indent + "  "
								}

								var newText strings.Builder

								newText.WriteString("if ")
								newText.WriteString(newCond)
								newText.WriteString(" then\n")

								innerBody := s.flattenBlock(doc, innerIfNode.Right, innerIndent, 999)
								if innerBody != "" {
									newText.WriteString(innerIndent)
									newText.WriteString(innerBody)
									newText.WriteString("\n")
								}

								newText.WriteString(indent)
								newText.WriteString("end")

								action.Edit = &WorkspaceEdit{
									Changes: map[string][]TextEdit{
										uri: {{
											Range:   getNodeRange(doc.Tree, nodeID),
											NewText: trimTrailingWhitespace(newText.String()),
										}},
									},
								}
							}
						}
					}
				}
			}
		}

		WriteMessage(s.Writer, Response{
			RPC:    "2.0",
			ID:     req.ID,
			Result: action,
		})
	case "textDocument/codeLens":
		if !s.FeatureCodeLens {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		var params CodeLensParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		var lenses []CodeLens

		for i := 1; i < len(doc.Tree.Nodes); i++ {
			node := doc.Tree.Nodes[i]

			if node.Kind == ast.KindLocalFunction || node.Kind == ast.KindFunctionStmt {
				identNodeID := node.Left

				for {
					if identNodeID == ast.InvalidNode || int(identNodeID) >= len(doc.Tree.Nodes) {
						break
					}

					n := doc.Tree.Nodes[identNodeID]

					if n.Kind == ast.KindMethodName || n.Kind == ast.KindMemberExpr {
						identNodeID = n.Right
					} else {
						break
					}
				}

				if identNodeID == ast.InvalidNode || int(identNodeID) >= len(doc.Tree.Nodes) || doc.Tree.Nodes[identNodeID].Kind != ast.KindIdent {
					continue
				}

				lenses = append(lenses, CodeLens{
					Range: getNodeRange(doc.Tree, identNodeID),
					Data: map[string]any{
						"uri":    uri,
						"nodeId": float64(identNodeID),
					},
				})
			}
		}

		if lenses == nil {
			lenses = []CodeLens{}
		}

		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: lenses})
	case "codeLens/resolve":
		var codeLens CodeLens

		err := json.Unmarshal(req.Params, &codeLens)
		if err != nil {
			return
		}

		data, ok := codeLens.Data.(map[string]any)
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: codeLens})

			return
		}

		uri, _ := data["uri"].(string)
		nodeIDFloat, _ := data["nodeId"].(float64)
		nodeID := ast.NodeID(nodeIDFloat)

		doc, ok := s.Documents[uri]
		if !ok || nodeID == ast.InvalidNode || int(nodeID) >= len(doc.Tree.Nodes) {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: codeLens})

			return
		}

		identNode := doc.Tree.Nodes[nodeID]

		ctx := s.resolveSymbolAt(uri, identNode.Start)
		if ctx == nil {
			codeLens.Command = &Command{Title: "0 references", Command: ""}

			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: codeLens})

			return
		}

		locations := s.getReferences(ctx, false)
		count := len(locations)

		var title string

		if count == 1 {
			title = "1 reference"
		} else {
			title = fmt.Sprintf("%d references", count)
		}

		var (
			cmd  string
			args []any
		)

		if count > 0 {
			cmd = "lugo.showReferences"
			args = []any{uri, codeLens.Range.Start, locations}
		}

		codeLens.Command = &Command{
			Title:     title,
			Command:   cmd,
			Arguments: args,
		}

		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: codeLens})
	case "textDocument/formatting":
		if !s.FeatureFormatting {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		var params DocumentFormattingParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		start := time.Now()

		formatter := NewFormatter(params.Options.TabSize, !params.Options.InsertSpaces, s.FormatOpinionated)
		formatted := formatter.Format(doc.Source)

		took := time.Since(start)

		s.Log.Printf("Formatted document in %s\n", took)

		if bytes.Equal(doc.Source, formatted) {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		endLine, endCol := doc.Tree.Position(uint32(len(doc.Source)))

		changes := []TextEdit{
			{
				Range: Range{
					Start: Position{Line: 0, Character: 0},
					End:   Position{Line: endLine, Character: endCol},
				},
				NewText: string(formatted),
			},
		}

		WriteMessage(s.Writer, Response{
			RPC:    "2.0",
			ID:     req.ID,
			Result: changes,
		})
	case "textDocument/linkedEditingRange":
		var params LinkedEditingRangeParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		offset := doc.Tree.Offset(params.Position.Line, params.Position.Character)

		ctx := s.resolveSymbolAt(uri, offset)
		if ctx == nil || ctx.IsGlobal || ctx.TargetDoc == nil || ctx.TargetDoc != doc || ctx.TargetDefID == ast.InvalidNode {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		var ranges []Range

		for i, def := range doc.Resolver.References {
			if def == ctx.TargetDefID {
				ranges = append(ranges, getNodeRange(doc.Tree, ast.NodeID(i)))
			}
		}

		WriteMessage(s.Writer, Response{
			RPC: "2.0",
			ID:  req.ID,
			Result: LinkedEditingRanges{
				Ranges: ranges,
			},
		})
	case "textDocument/prepareRename":
		var params PrepareRenameParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		offset := doc.Tree.Offset(params.Position.Line, params.Position.Character)

		ctx := s.resolveSymbolAt(uri, offset)
		if ctx == nil {
			WriteMessage(s.Writer, Response{
				RPC: "2.0",
				ID:  req.ID,
				Error: ResponseError{
					Code:    -32602,
					Message: "Cannot rename this element.",
				},
			})

			return
		}

		WriteMessage(s.Writer, Response{
			RPC: "2.0",
			ID:  req.ID,
			Result: PrepareRenameResult{
				Range:       getNodeRange(doc.Tree, ctx.IdentNodeID),
				Placeholder: ctx.IdentName,
			},
		})
	case "textDocument/rename":
		var params RenameParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		doc, ok := s.Documents[uri]
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		offset := doc.Tree.Offset(params.Position.Line, params.Position.Character)
		ctx := s.resolveSymbolAt(uri, offset)

		if ctx == nil {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		changes := make(map[string][]TextEdit)
		seen := make(map[RefKey]bool)

		addEdit := func(dDoc *Document, dUri string, nodeID ast.NodeID) {
			rk := RefKey{URI: dUri, ID: nodeID}

			if seen[rk] {
				return
			}

			seen[rk] = true

			changes[dUri] = append(changes[dUri], TextEdit{
				Range:   getNodeRange(dDoc.Tree, nodeID),
				NewText: params.NewName,
			})
		}

		if ctx.TargetDefID != ast.InvalidNode {
			for i, def := range ctx.TargetDoc.Resolver.References {
				if def == ctx.TargetDefID {
					addEdit(ctx.TargetDoc, ctx.TargetURI, ast.NodeID(i))
				}
			}
		}

		if ctx.IsGlobal {
			for dUri, dDoc := range s.Documents {
				if ctx.GKey.ReceiverHash == 0 {
					for _, id := range dDoc.Resolver.GlobalDefs {
						node := dDoc.Tree.Nodes[id]

						if ast.HashBytes(dDoc.Source[node.Start:node.End]) == ctx.GKey.PropHash {
							addEdit(dDoc, dUri, id)
						}
					}

					for _, id := range dDoc.Resolver.GlobalRefs {
						node := dDoc.Tree.Nodes[id]

						if ast.HashBytes(dDoc.Source[node.Start:node.End]) == ctx.GKey.PropHash {
							if dDoc.Resolver.References[id] == ast.InvalidNode {
								addEdit(dDoc, dUri, id)
							}
						}
					}
				} else {
					for _, fd := range dDoc.Resolver.FieldDefs {
						if fd.ReceiverHash == ctx.GKey.ReceiverHash && fd.PropHash == ctx.GKey.PropHash {
							addEdit(dDoc, dUri, fd.NodeID)
						}
					}

					for _, pf := range dDoc.Resolver.PendingFields {
						if pf.ReceiverHash == ctx.GKey.ReceiverHash && pf.PropHash == ctx.GKey.PropHash {
							if dDoc.Resolver.References[pf.PropNodeID] == ast.InvalidNode {
								addEdit(dDoc, dUri, pf.PropNodeID)
							}
						}
					}
				}
			}
		}

		WriteMessage(s.Writer, Response{
			RPC: "2.0",
			ID:  req.ID,
			Result: WorkspaceEdit{
				Changes: changes,
			},
		})
	}
}
