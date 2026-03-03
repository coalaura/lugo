package lsp

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/coalaura/lugo/ast"
	"github.com/coalaura/lugo/parser"
	"github.com/coalaura/lugo/semantic"
	"github.com/coalaura/plain"
)

var luaKeywords = []string{
	"and", "break", "do", "else", "elseif", "end", "false", "for", "function",
	"goto", "if", "in", "local", "nil", "not", "or", "repeat", "return",
	"then", "true", "until", "while",
}

const MaxWorkspaceResults = 100

type GlobalSymbol struct {
	URI    string
	NodeID ast.NodeID
	Depth  int
	Name   string
}

type GlobalKey struct {
	ReceiverHash uint64 // 0 if it's a root global
	PropHash     uint64
}

type GlobalReference struct {
	Doc    *Document
	URI    string
	NodeID ast.NodeID
}

type CallerKey struct {
	URI string
	Def ast.NodeID
}

type TargetKey struct {
	URI string
	Def ast.NodeID
}

type SymbolContext struct {
	TargetDoc   *Document
	IdentName   string
	DisplayName string
	TargetURI   string
	GKey        GlobalKey
	IdentNodeID ast.NodeID
	TargetDefID ast.NodeID
	RecDefID    ast.NodeID
	IsProp      bool
	IsGlobal    bool
}

type SemanticToken struct {
	Start     uint32
	End       uint32
	TokenType uint32
	Modifiers uint32
}

type IgnorePattern struct {
	MatchFallback string
	HasSuffix     string
	HasPrefix     string
	ContainsPath  string
	SuffixPath    string
}

type SafeFix struct {
	Coverage []ast.NodeID
	Edits    []TextEdit
	Title    string
}

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

	DiagUndefinedGlobals bool
	DiagImplicitGlobals  bool
	DiagUnusedLocal      bool
	DiagUnusedFunction   bool
	DiagUnusedParameter  bool
	DiagUnusedLoopVar    bool
	DiagShadowing        bool
	DiagUnreachableCode  bool
	DiagAmbiguousReturns bool
	DiagDeprecated       bool
	InlayParamHints      bool
	FeatureDocHighlight  bool
	FeatureHoverEval     bool
	FeatureCodeLens      bool
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
		sharedParser: parser.New(nil, ast.NewTree(nil)),
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
	case "shutdown":
		err := WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})
		if err != nil {
			s.Log.Errorf("WriteMessage error (shutdown): %v\n", err)
		}
	case "exit":
		s.Log.Println("Received exit notification, terminating.")

		os.Exit(0)
	case "initialize":
		var params InitializeParams

		err := json.Unmarshal(req.Params, &params)
		if err == nil {
			s.RootURI = params.RootURI
			s.lowerRootPath = strings.ToLower(s.uriToPath(params.RootURI))

			s.LibraryPaths = params.InitializationOptions.LibraryPaths

			for _, lib := range s.LibraryPaths {
				s.lowerLibraryPaths = append(s.lowerLibraryPaths, strings.ToLower(filepath.Clean(filepath.FromSlash(lib))))
			}

			s.IgnoreGlobs = params.InitializationOptions.IgnoreGlobs
			s.compileIgnorePatterns()

			s.KnownGlobals = make(map[string]bool)
			s.KnownGlobalGlobs = []string{}

			for _, g := range params.InitializationOptions.KnownGlobals {
				if strings.ContainsAny(g, "*?") {
					s.KnownGlobalGlobs = append(s.KnownGlobalGlobs, g)
				} else {
					s.KnownGlobals[g] = true
				}
			}

			s.DiagUndefinedGlobals = params.InitializationOptions.DiagnosticsUndefinedGlobals
			s.DiagImplicitGlobals = params.InitializationOptions.DiagnosticsImplicitGlobals
			s.DiagUnusedLocal = params.InitializationOptions.DiagnosticsUnusedLocal
			s.DiagUnusedFunction = params.InitializationOptions.DiagnosticsUnusedFunction
			s.DiagUnusedParameter = params.InitializationOptions.DiagnosticsUnusedParameter
			s.DiagUnusedLoopVar = params.InitializationOptions.DiagnosticsUnusedLoopVar
			s.DiagShadowing = params.InitializationOptions.DiagnosticsShadowing
			s.DiagUnreachableCode = params.InitializationOptions.DiagnosticsUnreachableCode
			s.DiagAmbiguousReturns = params.InitializationOptions.DiagnosticsAmbiguousReturns
			s.DiagDeprecated = params.InitializationOptions.DiagnosticsDeprecated
			s.InlayParamHints = params.InitializationOptions.InlayHintsParameterNames
			s.FeatureDocHighlight = params.InitializationOptions.FeaturesDocumentHighlight
			s.FeatureHoverEval = params.InitializationOptions.FeaturesHoverEvaluation
			s.FeatureCodeLens = params.InitializationOptions.FeaturesCodeLens
		}

		var codeLensOptions *CodeLensOptions

		if s.FeatureCodeLens {
			codeLensOptions = &CodeLensOptions{
				ResolveProvider: true,
			}
		}

		result := InitializeResult{
			Capabilities: ServerCapabilities{
				TextDocumentSync:   1,
				DefinitionProvider: true,
				HoverProvider:      true,
				RenameProvider: map[string]bool{
					"prepareProvider": true,
				},
				ReferencesProvider:        true,
				DocumentSymbolProvider:    true,
				WorkspaceSymbolProvider:   true,
				InlayHintProvider:         s.InlayParamHints,
				CodeActionProvider:        true,
				FoldingRangeProvider:      true,
				CallHierarchyProvider:     true,
				DocumentHighlightProvider: s.FeatureDocHighlight,
				CodeLensProvider:          codeLensOptions,
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
			content = string(b)
		}

		WriteMessage(s.Writer, Response{
			RPC:    "2.0",
			ID:     req.ID,
			Result: ReadStdResult{Content: content},
		})
	case "lugo/reindex":
		s.Log.Println("Starting workspace re-index...")

		/*
			cpuFile, err := os.Create("C:\\Users\\Laura\\lugo\\lugo_cpu.prof")
			if err == nil {
				pprof.StartCPUProfile(cpuFile)
			}

			traceFile, err := os.Create("C:\\Users\\Laura\\lugo\\lugo_trace.out")
			if err == nil {
				trace.Start(traceFile)
			}
		*/

		s.IsIndexing = true

		start := time.Now()

		if s.activeURIs == nil {
			s.activeURIs = make(map[string]bool, len(s.Documents))
		} else {
			clear(s.activeURIs)
		}

		var (
			indexed   int
			unchanged int
			failed    int
		)

		s.indexEmbeddedStdlib(&indexed, &unchanged)

		for _, libPath := range s.LibraryPaths {
			s.Log.Printf("Indexing external library: %s\n", libPath)

			s.indexWorkspace(libPath, &indexed, &unchanged, &failed)
		}

		if s.RootURI != "" {
			s.Log.Printf("Indexing workspace: %s\n", s.RootURI)

			s.indexWorkspace(s.RootURI, &indexed, &unchanged, &failed)
		}

		for uri := range s.Documents {
			if !s.activeURIs[uri] && !s.OpenFiles[uri] {
				s.clearDocument(uri)
			}
		}

		for uri, doc := range s.Documents {
			if s.OpenFiles[uri] && !s.activeURIs[uri] {
				s.updateDocument(uri, doc.Source)
			}
		}

		s.activeURIs = nil

		s.IsIndexing = false

		took := time.Since(start)

		s.Log.Printf("Re-indexed workspace in %s (indexed=%d, unchanged=%d, failed=%d)\n", took, indexed, unchanged, failed)

		start = time.Now()

		var diagCount int

		for uri := range s.Documents {
			if s.isWorkspaceURI(uri) {
				s.publishDiagnostics(uri)

				diagCount++
			}
		}

		took = time.Since(start)

		s.Log.Printf("Published diagnostics for %d workspace files in %s\n", diagCount, took)

		/*
			if traceFile != nil {
				trace.Stop()

				traceFile.Close()
			}

			if cpuFile != nil {
				pprof.StopCPUProfile()

				cpuFile.Close()
			}
		*/

		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: "ok"})
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
	case "textDocument/didClose":
		var params DidCloseTextDocumentParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		uri := s.normalizeURI(params.TextDocument.URI)

		delete(s.OpenFiles, uri)

		s.Log.Debugf("Closed document: %s\n", uri)
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
					fixes := s.getSafeFixesForDocument(doc)

					for _, fix := range fixes {
						changes[targetURI] = append(changes[targetURI], fix.Edits...)
					}
				}
			} else {
				for uri, doc := range s.Documents {
					if !s.isWorkspaceURI(uri) {
						continue
					}

					fixes := s.getSafeFixesForDocument(doc)

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

				if valID != ast.InvalidNode {
					vNode := ctx.TargetDoc.Tree.Nodes[valID]

					switch vNode.Kind {
					case ast.KindNumber, ast.KindString, ast.KindTrue, ast.KindFalse, ast.KindNil:
						valStr = " = " + string(ctx.TargetDoc.Source[vNode.Start:vNode.End])
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
					n := doc.Tree.Nodes[identNodeID]

					if n.Kind == ast.KindMethodName || n.Kind == ast.KindMemberExpr {
						identNodeID = n.Right
					} else {
						break
					}
				}

				if doc.Tree.Nodes[identNodeID].Kind != ast.KindIdent {
					continue
				}

				lenses = append(lenses, CodeLens{
					Range: getNodeRange(doc.Tree, identNodeID),
					Data: map[string]any{
						"uri":    uri,
						"nodeId": identNodeID,
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
		if !ok {
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

		addCompletion := func(label string, kind CompletionItemKind, detail string, isDep bool) {
			if seen[label] {
				return
			}

			seen[label] = true

			var tags []CompletionItemTag

			if isDep {
				tags = append(tags, CompletionItemTagDeprecated)
			}

			items = append(items, CompletionItem{
				Label:  label,
				Kind:   kind,
				Detail: detail,
				Tags:   tags,
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

				if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
					i--

					continue
				}

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

		s.Log.Printf("Completion requested at offset %d. isMember=%v, recName=%s\n", offset, isMember, string(recName))

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

					isDep, _ := doc.HasDeprecatedTag(fd.NodeID)

					addCompletion(string(doc.Source[node.Start:node.End]), FieldCompletion, "field", isDep)
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

						isDep, _ := symDoc.HasDeprecatedTag(sym.NodeID)

						addCompletion(string(symDoc.Source[node.Start:node.End]), FieldCompletion, "field", isDep)
					}
				}
			}
		} else {
			doc.GetLocalsAt(offset, func(name []byte, defID ast.NodeID) bool {
				isDep, _ := doc.HasDeprecatedTag(defID)

				addCompletion(string(name), VariableCompletion, "local", isDep)

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

							addCompletion(string(symDoc.Source[node.Start:node.End]), kind, "global", isDep)
						}
					}
				}
			}

			for _, kw := range luaKeywords {
				addCompletion(kw, KeywordCompletion, "keyword", false)
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

					name := string(doc.Source[keyNode.Start:keyNode.End])

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
				name := string(doc.Source[nameNode.Start:nameNode.End])

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

						name := string(doc.Source[lNode.Start:lNode.End])

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
		if !ok {
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
				if gpID != ast.InvalidNode {
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
		if !ok {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		var root ast.NodeID

		valID := doc.getAssignedValue(defID)

		if valID != ast.InvalidNode && doc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
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
			if id == ast.InvalidNode {
				return
			}

			node := doc.Tree.Nodes[id]

			if id != root && node.Kind == ast.KindFunctionExpr {
				return
			}

			if node.Kind == ast.KindCallExpr || node.Kind == ast.KindMethodCall {
				var identID ast.NodeID

				if node.Kind == ast.KindCallExpr {
					switch doc.Tree.Nodes[node.Left].Kind {
					case ast.KindIdent:
						identID = node.Left
					case ast.KindMemberExpr:
						identID = doc.Tree.Nodes[node.Left].Right
					}
				} else {
					identID = node.Right
				}

				if identID != ast.InvalidNode {
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
				walk(doc.Tree.ExtraList[node.Extra+uint32(i)])
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
		seen := make(map[string]map[ast.NodeID]bool)

		addEdit := func(dDoc *Document, dUri string, nodeID ast.NodeID) {
			if seen[dUri] == nil {
				seen[dUri] = make(map[ast.NodeID]bool)
			}

			if seen[dUri][nodeID] {
				return
			}

			seen[dUri][nodeID] = true

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

		var callID ast.NodeID = ast.InvalidNode

		curr := doc.Tree.NodeAt(offset)

		for curr != ast.InvalidNode {
			node := doc.Tree.Nodes[curr]

			if node.Kind == ast.KindCallExpr || node.Kind == ast.KindMethodCall {
				if offset > doc.Tree.Nodes[node.Left].End {
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

		ctx := s.resolveSymbolAt(uri, doc.Tree.Nodes[funcIdentID].Start)
		if ctx == nil || ctx.TargetDoc == nil || ctx.TargetDefID == ast.InvalidNode {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		valID := ctx.TargetDoc.getAssignedValue(ctx.TargetDefID)
		if valID == ast.InvalidNode || ctx.TargetDoc.Tree.Nodes[valID].Kind != ast.KindFunctionExpr {
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
			pID := ctx.TargetDoc.Tree.ExtraList[funcNode.Extra+uint32(i)]
			pNode := ctx.TargetDoc.Tree.Nodes[pID]
			pName := string(ctx.TargetDoc.Source[pNode.Start:pNode.End])

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
			argID := doc.Tree.ExtraList[callNode.Extra+uint32(i)]
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
	case "textDocument/inlayHint":
		var params InlayHintParams

		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			return
		}

		if !s.InlayParamHints {
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
				if doc.Tree.Nodes[funcIdentID].Kind == ast.KindMemberExpr {
					funcIdentID = doc.Tree.Nodes[funcIdentID].Right
				}
			}

			if doc.Tree.Nodes[funcIdentID].Kind != ast.KindIdent {
				continue
			}

			ctx := s.resolveSymbolAt(uri, doc.Tree.Nodes[funcIdentID].Start)
			if ctx == nil || ctx.TargetDoc == nil || ctx.TargetDefID == ast.InvalidNode {
				continue
			}

			valID := ctx.TargetDoc.getAssignedValue(ctx.TargetDefID)
			if valID == ast.InvalidNode || ctx.TargetDoc.Tree.Nodes[valID].Kind != ast.KindFunctionExpr {
				continue
			}

			hasImplicitSelfCall := node.Kind == ast.KindMethodCall
			hasImplicitSelfDef := false

			pDefID := ctx.TargetDoc.Tree.Nodes[ctx.TargetDefID].Parent
			if pDefID != ast.InvalidNode && ctx.TargetDoc.Tree.Nodes[pDefID].Kind == ast.KindMethodName {
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

				argID := doc.Tree.ExtraList[node.Extra+uint32(j)]
				argNode := doc.Tree.Nodes[argID]

				pID := ctx.TargetDoc.Tree.ExtraList[funcNode.Extra+uint32(paramIdx)]
				pNode := ctx.TargetDoc.Tree.Nodes[pID]

				if pNode.Kind == ast.KindVararg {
					continue
				}

				pName := ctx.TargetDoc.Source[pNode.Start:pNode.End]
				if bytes.Equal(pName, []byte("self")) {
					continue
				}

				if argNode.Kind == ast.KindIdent {
					argName := doc.Source[argNode.Start:argNode.End]
					if bytes.Equal(pName, argName) {
						continue
					}
				}

				sLine, sCol := doc.Tree.Position(argNode.Start)
				hints = append(hints, InlayHint{
					Position:     Position{Line: sLine, Character: sCol},
					Label:        string(pName) + ":",
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
			}
		}

		if hasUnused && docOk {
			allFixes := s.getSafeFixesForDocument(doc)

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

				for _, fix := range allFixes {
					var covers bool

					if slices.Contains(fix.Coverage, defID) {
						covers = true
					}

					if covers {
						actions = append(actions, CodeAction{
							Title:       fix.Title,
							Kind:        "quickfix",
							Diagnostics: []Diagnostic{diag},
							IsPreferred: true,
							Edit: &WorkspaceEdit{
								Changes: map[string][]TextEdit{
									uri: fix.Edits,
								},
							},
						})

						break
					}
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

		if actions == nil {
			actions = []CodeAction{}
		}

		WriteMessage(s.Writer, Response{
			RPC:    "2.0",
			ID:     req.ID,
			Result: actions,
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

		var prevLine, prevCol uint32

		for _, t := range s.semTokensBuf {
			line, col := doc.Tree.Position(t.Start)
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
	}
}

func (s *Server) getSafeFixesForDocument(doc *Document) []SafeFix {
	var fixes []SafeFix

	if doc.IsMeta {
		return fixes
	}

	unusedDefs := make(map[ast.NodeID]bool)

	for _, defID := range doc.Resolver.LocalDefs {
		if doc.Resolver.UsageCount[defID] == 0 {
			name := doc.Source[doc.Tree.Nodes[defID].Start:doc.Tree.Nodes[defID].End]
			if len(name) > 0 && name[0] != '_' {
				unusedDefs[defID] = true
			}
		}
	}

	for i := 1; i < len(doc.Tree.Nodes); i++ {
		nodeID := ast.NodeID(i)
		node := doc.Tree.Nodes[nodeID]

		switch node.Kind {
		case ast.KindLocalAssign:
			s.processListForFixes(doc, node.Left, node.Right, unusedDefs, &fixes, true)
		case ast.KindForIn:
			s.processListForFixes(doc, node.Left, ast.InvalidNode, unusedDefs, &fixes, false)
		case ast.KindLocalFunction:
			if unusedDefs[node.Left] {
				fixes = append(fixes, SafeFix{
					Coverage: []ast.NodeID{node.Left},
					Edits: []TextEdit{{
						Range:   getNodeRange(doc.Tree, nodeID),
						NewText: "",
					}},
					Title: "Remove unused local function",
				})

				delete(unusedDefs, node.Left)

				continue
			}

			if node.Right != ast.InvalidNode {
				s.processParamsForFixes(doc, node.Right, unusedDefs, &fixes)
			}
		case ast.KindFunctionExpr, ast.KindFunctionStmt:
			var funcExprID ast.NodeID

			if node.Kind == ast.KindFunctionExpr {
				funcExprID = nodeID
			} else {
				funcExprID = node.Right
			}

			if funcExprID != ast.InvalidNode {
				s.processParamsForFixes(doc, funcExprID, unusedDefs, &fixes)
			}
		case ast.KindForNum:
			if unusedDefs[node.Left] {
				fixes = append(fixes, s.createRenameFix(doc, node.Left))

				delete(unusedDefs, node.Left)
			}
		case ast.KindReturn:
			if node.Left != ast.InvalidNode {
				exprList := doc.Tree.Nodes[node.Left]
				if exprList.Count > 0 {
					firstExprID := doc.Tree.ExtraList[exprList.Extra]
					firstExprNode := doc.Tree.Nodes[firstExprID]

					retLine, _ := doc.Tree.Position(node.Start)
					exprLine, _ := doc.Tree.Position(firstExprNode.Start)

					if exprLine > retLine {
						fixes = append(fixes, SafeFix{
							Coverage: []ast.NodeID{nodeID},
							Edits: []TextEdit{{
								Range:   getRange(doc.Tree, node.Start, node.Start+6),
								NewText: "return;",
							}},
							Title: "Add ';' to fix ambiguous return",
						})
					}
				}
			}
		case ast.KindBlock, ast.KindFile:
			var terminalFound bool

			for j := uint16(0); j < node.Count; j++ {
				stmtID := doc.Tree.ExtraList[node.Extra+uint32(j)]

				if terminalFound {
					lastStmtID := doc.Tree.ExtraList[node.Extra+uint32(node.Count-1)]

					fixes = append(fixes, SafeFix{
						Coverage: []ast.NodeID{stmtID},
						Edits: []TextEdit{{
							Range:   getRange(doc.Tree, doc.Tree.Nodes[stmtID].Start, doc.Tree.Nodes[lastStmtID].End),
							NewText: "",
						}},
						Title: "Remove unreachable code",
					})

					break
				}

				if isTerminal(doc.Tree, stmtID) {
					terminalFound = true
				}
			}
		}
	}

	return fixes
}

func (s *Server) processListForFixes(doc *Document, nameListID, exprListID ast.NodeID, unused map[ast.NodeID]bool, fixes *[]SafeFix, canRemoveStatement bool) {
	if nameListID == ast.InvalidNode {
		return
	}

	nameList := doc.Tree.Nodes[nameListID]

	var unusedCount int

	for i := uint16(0); i < nameList.Count; i++ {
		if unused[doc.Tree.ExtraList[nameList.Extra+uint32(i)]] {
			unusedCount++
		}
	}

	if unusedCount == 0 {
		return
	}

	suffixStart := int(nameList.Count)

	for i := int(nameList.Count) - 1; i >= 0; i-- {
		if unused[doc.Tree.ExtraList[nameList.Extra+uint32(i)]] {
			suffixStart = i
		} else {
			break
		}
	}

	for i := 0; i < suffixStart; i++ {
		id := doc.Tree.ExtraList[nameList.Extra+uint32(i)]
		if unused[id] {
			*fixes = append(*fixes, s.createRenameFix(doc, id))

			delete(unused, id)
		}
	}

	if suffixStart < int(nameList.Count) {
		var coverage []ast.NodeID

		for i := suffixStart; i < int(nameList.Count); i++ {
			id := doc.Tree.ExtraList[nameList.Extra+uint32(i)]

			coverage = append(coverage, id)
			delete(unused, id)
		}

		if suffixStart == 0 && canRemoveStatement {
			canRemove := true

			if exprListID != ast.InvalidNode {
				exprList := doc.Tree.Nodes[exprListID]

				for i := uint16(0); i < exprList.Count; i++ {
					if !s.isSideEffectFree(doc, doc.Tree.ExtraList[exprList.Extra+uint32(i)]) {
						canRemove = false

						break
					}
				}
			}

			if canRemove {
				stmtID := nameList.Parent

				*fixes = append(*fixes, SafeFix{
					Coverage: coverage,
					Edits: []TextEdit{{
						Range:   getNodeRange(doc.Tree, stmtID),
						NewText: "",
					}},
					Title: "Remove unused assignment",
				})

				return
			}
		}

		canPartialRemove := true

		var rhsEdits []TextEdit

		if exprListID != ast.InvalidNode {
			exprList := doc.Tree.Nodes[exprListID]

			if int(exprList.Count) > suffixStart {
				for i := suffixStart; i < int(exprList.Count); i++ {
					if !s.isSideEffectFree(doc, doc.Tree.ExtraList[exprList.Extra+uint32(i)]) {
						canPartialRemove = false

						break
					}
				}

				if canPartialRemove {
					firstRhsDrop := doc.Tree.ExtraList[exprList.Extra+uint32(suffixStart)]
					lastRhsDrop := doc.Tree.ExtraList[exprList.Extra+uint32(exprList.Count-1)]

					var limit uint32

					if suffixStart > 0 {
						limit = doc.Tree.Nodes[doc.Tree.ExtraList[exprList.Extra+uint32(suffixStart-1)]].End
					} else {
						limit = doc.Tree.Nodes[exprListID].Start
					}

					startOff := s.findCommaBefore(doc.Source, doc.Tree.Nodes[firstRhsDrop].Start, limit)

					rhsEdits = append(rhsEdits, TextEdit{
						Range:   getRange(doc.Tree, startOff, doc.Tree.Nodes[lastRhsDrop].End),
						NewText: "",
					})
				}
			}
		}

		if canPartialRemove && suffixStart > 0 {
			firstLhsDrop := doc.Tree.ExtraList[nameList.Extra+uint32(suffixStart)]
			lastLhsDrop := doc.Tree.ExtraList[nameList.Extra+uint32(nameList.Count-1)]
			prevLhsNode := doc.Tree.ExtraList[nameList.Extra+uint32(suffixStart-1)]

			startOff := s.findCommaBefore(doc.Source, doc.Tree.Nodes[firstLhsDrop].Start, doc.Tree.Nodes[prevLhsNode].End)

			edits := []TextEdit{{
				Range:   getRange(doc.Tree, startOff, doc.Tree.Nodes[lastLhsDrop].End),
				NewText: "",
			}}

			edits = append(edits, rhsEdits...)

			title := "Remove unused variable"

			if len(coverage) > 1 {
				title = "Remove unused variables"
			}

			*fixes = append(*fixes, SafeFix{
				Coverage: coverage,
				Edits:    edits,
				Title:    title,
			})

			return
		}

		for _, id := range coverage {
			*fixes = append(*fixes, s.createRenameFix(doc, id))
		}
	}
}

func (s *Server) processParamsForFixes(doc *Document, funcExprID ast.NodeID, unused map[ast.NodeID]bool, fixes *[]SafeFix) {
	funcNode := doc.Tree.Nodes[funcExprID]
	if funcNode.Count == 0 {
		return
	}

	suffixStart := int(funcNode.Count)

	for i := int(funcNode.Count) - 1; i >= 0; i-- {
		id := doc.Tree.ExtraList[funcNode.Extra+uint32(i)]

		if unused[id] {
			suffixStart = i
		} else {
			break
		}
	}

	for i := 0; i < suffixStart; i++ {
		id := doc.Tree.ExtraList[funcNode.Extra+uint32(i)]
		if unused[id] {
			*fixes = append(*fixes, s.createRenameFix(doc, id))

			delete(unused, id)
		}
	}

	if suffixStart < int(funcNode.Count) {
		var coverage []ast.NodeID

		for i := suffixStart; i < int(funcNode.Count); i++ {
			id := doc.Tree.ExtraList[funcNode.Extra+uint32(i)]

			coverage = append(coverage, id)
			delete(unused, id)
		}

		firstDrop := doc.Tree.ExtraList[funcNode.Extra+uint32(suffixStart)]
		lastDrop := doc.Tree.ExtraList[funcNode.Extra+uint32(funcNode.Count-1)]

		var startOff uint32

		if suffixStart == 0 {
			startOff = doc.Tree.Nodes[firstDrop].Start

			for i := startOff - 1; i < uint32(len(doc.Source)); i-- {
				if doc.Source[i] == '(' {
					startOff = i + 1

					break
				}
			}
		} else {
			prevNode := doc.Tree.ExtraList[funcNode.Extra+uint32(suffixStart-1)]
			startOff = s.findCommaBefore(doc.Source, doc.Tree.Nodes[firstDrop].Start, doc.Tree.Nodes[prevNode].End)
		}

		endOff := doc.Tree.Nodes[lastDrop].End

		title := "Remove unused parameter"

		if len(coverage) > 1 {
			title = "Remove unused parameters"
		}

		*fixes = append(*fixes, SafeFix{
			Coverage: coverage,
			Edits: []TextEdit{{
				Range:   getRange(doc.Tree, startOff, endOff),
				NewText: "",
			}},
			Title: title,
		})
	}
}

func (s *Server) createRenameFix(doc *Document, id ast.NodeID) SafeFix {
	node := doc.Tree.Nodes[id]
	name := string(doc.Source[node.Start:node.End])

	return SafeFix{
		Coverage: []ast.NodeID{id},
		Edits: []TextEdit{{
			Range:   getNodeRange(doc.Tree, id),
			NewText: "_" + name,
		}},
		Title: "Prefix with '_'",
	}
}

func (s *Server) isSideEffectFree(doc *Document, id ast.NodeID) bool {
	if id == ast.InvalidNode {
		return true
	}

	node := doc.Tree.Nodes[id]

	switch node.Kind {
	case ast.KindNumber, ast.KindString, ast.KindTrue, ast.KindFalse, ast.KindNil, ast.KindIdent, ast.KindVararg, ast.KindFunctionExpr:
		return true
	case ast.KindUnaryExpr:
		return s.isSideEffectFree(doc, node.Right)
	case ast.KindBinaryExpr:
		return s.isSideEffectFree(doc, node.Left) && s.isSideEffectFree(doc, node.Right)
	case ast.KindParenExpr:
		return s.isSideEffectFree(doc, node.Left)
	case ast.KindMemberExpr:
		return s.isSideEffectFree(doc, node.Left)
	case ast.KindIndexExpr:
		return s.isSideEffectFree(doc, node.Left) && s.isSideEffectFree(doc, node.Right)
	case ast.KindExprList:
		for i := uint16(0); i < node.Count; i++ {
			if !s.isSideEffectFree(doc, doc.Tree.ExtraList[node.Extra+uint32(i)]) {
				return false
			}
		}

		return true
	case ast.KindTableExpr:
		for i := uint16(0); i < node.Count; i++ {
			fID := doc.Tree.ExtraList[node.Extra+uint32(i)]
			fNode := doc.Tree.Nodes[fID]

			if fNode.Kind == ast.KindRecordField || fNode.Kind == ast.KindIndexField {
				if !s.isSideEffectFree(doc, fNode.Left) || !s.isSideEffectFree(doc, fNode.Right) {
					return false
				}
			} else {
				if !s.isSideEffectFree(doc, fID) {
					return false
				}
			}
		}

		return true
	case ast.KindCallExpr, ast.KindMethodCall:
		var name string

		if node.Kind == ast.KindMethodCall {
			nameNode := doc.Tree.Nodes[node.Right]
			name = string(doc.Source[nameNode.Start:nameNode.End])
		} else {
			leftNode := doc.Tree.Nodes[node.Left]

			switch leftNode.Kind {
			case ast.KindIdent:
				name = string(doc.Source[leftNode.Start:leftNode.End])
			case ast.KindMemberExpr:
				rightNode := doc.Tree.Nodes[leftNode.Right]
				name = string(doc.Source[rightNode.Start:rightNode.End])
			}
		}

		if name != "" {
			lowerName := strings.ToLower(name)

			if strings.HasPrefix(lowerName, "get") ||
				strings.HasPrefix(lowerName, "is") ||
				strings.HasPrefix(lowerName, "has") ||
				strings.HasPrefix(lowerName, "can") ||
				strings.HasPrefix(lowerName, "unpack") ||
				strings.HasPrefix(lowerName, "math.") ||
				strings.HasPrefix(lowerName, "type") ||
				strings.HasPrefix(lowerName, "tostring") ||
				strings.HasPrefix(lowerName, "tonumber") ||
				strings.HasPrefix(lowerName, "pairs") ||
				strings.HasPrefix(lowerName, "ipairs") {

				// Check args
				for i := uint16(0); i < node.Count; i++ {
					if !s.isSideEffectFree(doc, doc.Tree.ExtraList[node.Extra+uint32(i)]) {
						return false
					}
				}

				return true
			}
		}

		return false
	}

	return false
}

func (s *Server) findCommaBefore(source []byte, start, limit uint32) uint32 {
	if start <= limit || start == 0 {
		return limit
	}

	commaPos := start

	for i := start - 1; i >= limit && i < uint32(len(source)); i-- {
		if source[i] == ',' {
			commaPos = i

			break
		}
	}

	for i := commaPos - 1; i >= limit && i < uint32(len(source)); i-- {
		if source[i] == ' ' || source[i] == '\t' {
			commaPos = i
		} else {
			break
		}
	}

	return commaPos
}

func (s *Server) indexWorkspace(rootPathOrURI string, indexed, unchanged, failed *int) {
	var path string

	if strings.HasPrefix(rootPathOrURI, "file://") {
		u, err := url.Parse(rootPathOrURI)
		if err != nil {
			s.Log.Errorf("Invalid workspace URI format: %s\n", rootPathOrURI)

			return
		}

		path = u.Path

		if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
			path = path[1:]
		}
	} else {
		path = rootPathOrURI
	}

	path = strings.ReplaceAll(path, "/", string(filepath.Separator))

	if realPath, err := filepath.EvalSymlinks(path); err == nil {
		path = realPath
	}

	if s.visitedDirs == nil {
		s.visitedDirs = make(map[string]bool, 256)
	} else {
		clear(s.visitedDirs)
	}

	var walk func(dir string)

	walk = func(dir string) {
		if s.visitedDirs[dir] {
			return
		}

		s.visitedDirs[dir] = true

		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		for _, e := range entries {
			fullPath := filepath.Join(dir, e.Name())

			isDir := e.IsDir()
			name := e.Name()

			if s.isIgnored(fullPath, name) {
				continue
			}

			// Check if its a symlink
			if e.Type()&fs.ModeSymlink != 0 {
				realPath, err := filepath.EvalSymlinks(fullPath)
				if err == nil {
					stat, err := os.Stat(realPath)
					if err == nil {
						isDir = stat.IsDir()
						name = stat.Name()

						fullPath = realPath
					} else {
						*failed++

						continue
					}
				} else {
					*failed++

					continue // Broken symlink
				}
			}

			if isDir {
				walk(fullPath)
			} else if strings.HasSuffix(name, ".lua") {
				uri := s.pathToURI(fullPath)

				b, fsErr := os.ReadFile(fullPath)
				if fsErr == nil {
					if existing, ok := s.Documents[uri]; ok && bytes.Equal(existing.Source, b) {
						s.activeURIs[uri] = true

						*unchanged++

						continue
					}

					s.updateDocument(uri, b)

					if s.activeURIs != nil {
						s.activeURIs[uri] = true
					}

					*indexed++
				} else {
					*failed++
				}
			}
		}
	}

	walk(path)
}

func (s *Server) updateDocument(uri string, source []byte) {
	var (
		tree *ast.Tree
		doc  *Document
	)

	if existing, exists := s.Documents[uri]; exists {
		if bytes.Equal(existing.Source, source) {
			return
		}

		doc = existing
		doc.Source = source

		s.removeDocumentGlobals(uri, doc)

		clear(doc.ExportedGlobals)

		tree = existing.Tree
		tree.Reset(source)
	} else {
		tree = ast.NewTree(source)

		doc = &Document{
			Source:          source,
			Tree:            tree,
			Resolver:        semantic.New(tree),
			ExportedGlobals: make(map[ast.NodeID][]GlobalKey),
		}

		s.Documents[uri] = doc
	}

	p := s.sharedParser
	p.Reset(source, tree)

	rootID := p.Parse()

	if cap(doc.TypeCache) >= len(tree.Nodes) {
		doc.TypeCache = doc.TypeCache[:len(tree.Nodes)]
		clear(doc.TypeCache)

		doc.Inferring = doc.Inferring[:len(tree.Nodes)]
		clear(doc.Inferring)
	} else {
		doc.TypeCache = make([]TypeSet, len(tree.Nodes))
		doc.Inferring = make([]bool, len(tree.Nodes))
	}

	doc.IsMeta = false

	for _, c := range tree.Comments {
		if bytes.Contains(tree.Source[c.Start:c.End], []byte("@meta")) {
			doc.IsMeta = true

			break
		}
	}

	if len(p.Errors) > 0 {
		if cap(doc.Errors) >= len(p.Errors) {
			doc.Errors = doc.Errors[:len(p.Errors)]
		} else {
			doc.Errors = make([]parser.ParseError, len(p.Errors))
		}

		copy(doc.Errors, p.Errors)
	} else {
		doc.Errors = doc.Errors[:0]
	}

	res := doc.Resolver

	res.Reset()
	res.Resolve(rootID)

	for _, defID := range res.GlobalDefs {
		node := tree.Nodes[defID]
		identBytes := tree.Source[node.Start:node.End]
		hash := ast.HashBytes(tree.Source[node.Start:node.End])

		depth := getASTDepth(tree, defID)

		s.setGlobalSymbol(GlobalKey{ReceiverHash: 0, PropHash: hash}, uri, defID, depth, string(identBytes))

		for name := range doc.ExtractLuaDocFields(defID) {
			fieldHash := ast.HashBytes(name)

			buf := make([]byte, 0, len(identBytes)+1+len(name))

			buf = append(buf, identBytes...)
			buf = append(buf, '.')
			buf = append(buf, name...)

			s.setGlobalSymbol(GlobalKey{ReceiverHash: hash, PropHash: fieldHash}, uri, defID, depth, string(buf))
		}

		// Module Aliasing
		valID := doc.getAssignedValue(defID)

		if valID != ast.InvalidNode {
			valNode := tree.Nodes[valID]

			if valNode.Kind == ast.KindIdent {
				localDefID := doc.Resolver.References[valID]

				if localDefID != ast.InvalidNode {
					localName := doc.Source[doc.Tree.Nodes[localDefID].Start:doc.Tree.Nodes[localDefID].End]
					globalBytes := tree.Source[node.Start:node.End]

					for _, fd := range res.FieldDefs {
						if bytes.Equal(fd.ReceiverName, localName) {
							propBytes := doc.Source[doc.Tree.Nodes[fd.NodeID].Start:doc.Tree.Nodes[fd.NodeID].End]

							buf := make([]byte, 0, len(identBytes)+1+len(propBytes))
							buf = append(buf, identBytes...)
							buf = append(buf, '.')
							buf = append(buf, propBytes...)

							s.setGlobalSymbol(GlobalKey{ReceiverHash: hash, PropHash: fd.PropHash}, uri, fd.NodeID, depth, string(buf))
						} else if len(fd.ReceiverName) > len(localName) && bytes.HasPrefix(fd.ReceiverName, localName) && fd.ReceiverName[len(localName)] == '.' {
							suffix := fd.ReceiverName[len(localName)+1:]

							newRecHash := ast.HashBytesConcat(globalBytes, []byte{'.'}, suffix)

							propBytes := doc.Source[doc.Tree.Nodes[fd.NodeID].Start:doc.Tree.Nodes[fd.NodeID].End]

							buf := make([]byte, 0, len(identBytes)+2+len(suffix)+len(propBytes))
							buf = append(buf, identBytes...)
							buf = append(buf, '.')
							buf = append(buf, suffix...)
							buf = append(buf, '.')
							buf = append(buf, propBytes...)

							s.setGlobalSymbol(GlobalKey{ReceiverHash: newRecHash, PropHash: fd.PropHash}, uri, fd.NodeID, depth, string(buf))
						}
					}
				}
			}
		}
	}

	// Index global table fields
	for _, fd := range res.FieldDefs {
		var (
			globalRecName []byte
			globalRecHash uint64
		)

		if fd.ReceiverDef == ast.InvalidNode {
			globalRecName = fd.ReceiverName
			globalRecHash = fd.ReceiverHash
		} else {
			valID := doc.getAssignedValue(fd.ReceiverDef)
			if valID != ast.InvalidNode {
				globalRecName = s.getGlobalPath(doc, valID, 0)
				if globalRecName != nil {
					globalRecHash = ast.HashBytes(globalRecName)
				}
			}
		}

		if globalRecName != nil {
			if bytes.Equal(globalRecName, []byte("self")) {
				continue
			}

			depth := getASTDepth(tree, fd.NodeID)

			propBytes := doc.Source[doc.Tree.Nodes[fd.NodeID].Start:doc.Tree.Nodes[fd.NodeID].End]

			sep := byte('.')

			if doc.Tree.Nodes[doc.Tree.Nodes[fd.NodeID].Parent].Kind == ast.KindMethodName {
				sep = ':'
			}

			buf := make([]byte, 0, len(globalRecName)+1+len(propBytes))
			buf = append(buf, globalRecName...)
			buf = append(buf, sep)
			buf = append(buf, propBytes...)

			s.setGlobalSymbol(GlobalKey{ReceiverHash: globalRecHash, PropHash: fd.PropHash}, uri, fd.NodeID, depth, string(buf))
		}
	}

	for _, pf := range res.PendingFields {
		if res.References[pf.PropNodeID] == ast.InvalidNode {
			var recHash uint64

			if pf.ReceiverDef != ast.InvalidNode {
				valID := doc.getAssignedValue(pf.ReceiverDef)
				if valID != ast.InvalidNode {
					path := s.getGlobalPath(doc, valID, 0)
					if path != nil {
						recHash = ast.HashBytes(path)
					}
				}
			} else {
				recHash = pf.ReceiverHash
			}

			if recHash != 0 {
				key := GlobalKey{ReceiverHash: recHash, PropHash: pf.PropHash}

				actualKey := key
				currRec := recHash

				for range 10 {
					if _, exists := s.GlobalIndex[actualKey]; exists {
						break
					}

					nextRec := s.getGlobalAlias(currRec)
					if nextRec == 0 {
						break
					}

					currRec = nextRec
					actualKey = GlobalKey{ReceiverHash: currRec, PropHash: pf.PropHash}
				}
			}
		}
	}

	s.Documents[uri] = doc
}

func (s *Server) publishDiagnostics(uri string) {
	if s.IsIndexing {
		return
	}

	doc := s.Documents[uri]

	s.diagBuf = s.diagBuf[:0]

	// 1. Parse Errors
	for _, err := range doc.Errors {
		r := getRange(doc.Tree, err.Start, err.End)

		if r.Start == r.End {
			r.End.Character++
		}

		s.diagBuf = append(s.diagBuf, Diagnostic{
			Range:    r,
			Severity: SeverityError,
			Code:     "parse-error",
			Message:  err.Message,
		})
	}

	// 2. Undefined Globals
	if s.DiagUndefinedGlobals {
		for _, refID := range doc.Resolver.GlobalRefs {
			node := doc.Tree.Nodes[refID]
			if node.Start == node.End {
				continue
			}

			identBytes := doc.Source[node.Start:node.End]

			if s.isKnownGlobal(identBytes) {
				continue
			}

			hash := ast.HashBytes(identBytes)
			key := GlobalKey{ReceiverHash: 0, PropHash: hash}

			if _, exists := s.GlobalIndex[key]; !exists {
				identStr := string(identBytes)
				msg := fmt.Sprintf("Undefined global '%s'.", identStr)

				suggestion := s.suggestGlobal(identStr)

				var diagData any

				if suggestion != "" {
					msg = fmt.Sprintf("Undefined global '%s'. Did you mean '%s'?", identStr, suggestion)
					diagData = suggestion
				}

				s.diagBuf = append(s.diagBuf, Diagnostic{
					Range:    getNodeRange(doc.Tree, refID),
					Severity: SeverityWarning,
					Code:     "undefined-global",
					Message:  msg,
					Data:     diagData,
				})
			}
		}
	}

	// 3. Implicit Globals
	if s.DiagImplicitGlobals && !doc.IsMeta {
		for _, defID := range doc.Resolver.GlobalDefs {
			node := doc.Tree.Nodes[defID]

			if node.Start == node.End {
				continue
			}

			identBytes := doc.Source[node.Start:node.End]

			if s.isKnownGlobal(identBytes) {
				continue
			}

			if isRootLevel(doc.Tree, defID) {
				continue
			}

			hash := ast.HashBytes(identBytes)
			key := GlobalKey{ReceiverHash: 0, PropHash: hash}

			if sym, ok := s.GlobalIndex[key]; ok {
				if symDoc, docOk := s.Documents[sym.URI]; docOk {
					if isRootLevel(symDoc.Tree, sym.NodeID) {
						continue
					}
				}
			}

			s.diagBuf = append(s.diagBuf, Diagnostic{
				Range:    getNodeRange(doc.Tree, defID),
				Severity: SeverityWarning,
				Code:     "implicit-global",
				Message:  fmt.Sprintf("Implicit global creation '%s'. Did you forget the 'local' keyword?", string(identBytes)),
			})
		}
	}

	// 4. Shadowing & Unused Variables
	if s.DiagShadowing || s.DiagUnusedLocal || s.DiagUnusedFunction || s.DiagUnusedParameter || s.DiagUnusedLoopVar {
		fixes := s.getSafeFixesForDocument(doc)
		fixMap := make(map[ast.NodeID]string)

		for _, f := range fixes {
			for _, id := range f.Coverage {
				fixMap[id] = f.Title
			}
		}

		for _, defID := range doc.Resolver.LocalDefs {
			node := doc.Tree.Nodes[defID]
			nameBytes := doc.Source[node.Start:node.End]

			if len(nameBytes) > 0 && nameBytes[0] == '_' {
				continue
			}

			r := getNodeRange(doc.Tree, defID)

			if doc.Resolver.UsageCount[defID] == 0 {
				category := "local"
				code := "unused-local"

				pID := doc.Tree.Nodes[defID].Parent

				if pID != ast.InvalidNode {
					pNode := doc.Tree.Nodes[pID]

					switch pNode.Kind {
					case ast.KindFunctionExpr:
						category = "parameter"
						code = "unused-parameter"
					case ast.KindForNum:
						category = "loop variable"
						code = "unused-loop-var"
					case ast.KindLocalFunction:
						category = "function"
						code = "unused-function"
					case ast.KindNameList:
						gpID := pNode.Parent
						if gpID != ast.InvalidNode && doc.Tree.Nodes[gpID].Kind == ast.KindForIn {
							category = "loop variable"
							code = "unused-loop-var"
						}
					}
				}

				var (
					msg          string
					shouldReport bool
				)

				if bytes.Equal(nameBytes, []byte("...")) {
					category = "parameter"
					msg = "Unused vararg '...'. Remove it from the parameter list if it is not needed."
					code = "unused-vararg"
				} else {
					msg = fmt.Sprintf("Unused %s '%s'.", category, string(nameBytes))

					if fixTitle, ok := fixMap[defID]; ok {
						if fixTitle == "Prefix with '_'" {
							msg += " Prefix with '_' to ignore."
						} else {
							msg += " It can be safely removed."
						}
					} else {
						msg += " Prefix with '_' to ignore."
					}
				}

				switch category {
				case "local":
					shouldReport = s.DiagUnusedLocal
				case "function":
					shouldReport = s.DiagUnusedFunction
				case "parameter":
					shouldReport = s.DiagUnusedParameter
				case "loop variable":
					shouldReport = s.DiagUnusedLoopVar
				}

				if doc.IsMeta {
					shouldReport = false
				}

				if shouldReport {
					s.diagBuf = append(s.diagBuf, Diagnostic{
						Range:    r,
						Severity: SeverityWarning,
						Code:     code,
						Tags:     []DiagnosticTag{Unnecessary},
						Message:  msg,
						Data:     float64(defID),
					})
				}
			}

			if s.DiagShadowing {
				if s.isKnownGlobal(nameBytes) {
					s.diagBuf = append(s.diagBuf, Diagnostic{
						Range:    r,
						Severity: SeverityWarning,
						Code:     "shadow-global",
						Message:  fmt.Sprintf("Local variable '%s' shadows a known global.", string(nameBytes)),
					})
				} else {
					hash := ast.HashBytes(nameBytes)

					if sym, exists := s.GlobalIndex[GlobalKey{ReceiverHash: 0, PropHash: hash}]; exists {
						var related []DiagnosticRelatedInformation

						if symDoc, ok := s.Documents[sym.URI]; ok {
							var fromFile string

							if sym.URI != uri {
								fromFile = " in " + filepath.Base(s.uriToPath(sym.URI))
							}

							related = append(related, DiagnosticRelatedInformation{
								Location: Location{
									URI:   sym.URI,
									Range: getNodeRange(symDoc.Tree, sym.NodeID),
								},
								Message: fmt.Sprintf("Global '%s' defined here%s", string(nameBytes), fromFile),
							})
						}

						s.diagBuf = append(s.diagBuf, Diagnostic{
							Range:              r,
							Severity:           SeverityWarning,
							Code:               "shadow-global",
							Message:            fmt.Sprintf("Local variable '%s' shadows a global definition.", string(nameBytes)),
							RelatedInformation: related,
						})
					}
				}
			}
		}
	}

	// 5. Shadowing Outer Locals
	if s.DiagShadowing {
		for _, pair := range doc.Resolver.ShadowedOuter {
			node := doc.Tree.Nodes[pair.Shadowing]
			nameBytes := doc.Source[node.Start:node.End]

			var related []DiagnosticRelatedInformation

			related = append(related, DiagnosticRelatedInformation{
				Location: Location{
					URI:   uri,
					Range: getNodeRange(doc.Tree, pair.Shadowed),
				},
				Message: fmt.Sprintf("Outer local '%s' defined here", string(nameBytes)),
			})

			s.diagBuf = append(s.diagBuf, Diagnostic{
				Range:              getNodeRange(doc.Tree, pair.Shadowing),
				Severity:           SeverityWarning,
				Code:               "shadow-outer",
				Message:            fmt.Sprintf("Local variable '%s' shadows a variable from an outer scope.", string(nameBytes)),
				RelatedInformation: related,
			})
		}
	}

	// 6. Unreachable Code & Ambiguous Returns
	if s.DiagUnreachableCode || s.DiagAmbiguousReturns {
		for i := 1; i < len(doc.Tree.Nodes); i++ {
			node := doc.Tree.Nodes[i]

			if s.DiagAmbiguousReturns && node.Kind == ast.KindReturn && node.Left != ast.InvalidNode {
				exprList := doc.Tree.Nodes[node.Left]
				if exprList.Count > 0 {
					firstExprID := doc.Tree.ExtraList[exprList.Extra]
					firstExprNode := doc.Tree.Nodes[firstExprID]

					retLine, _ := doc.Tree.Position(node.Start)
					exprLine, _ := doc.Tree.Position(firstExprNode.Start)

					if exprLine > retLine {
						lastExprID := doc.Tree.ExtraList[exprList.Extra+uint32(exprList.Count-1)]

						s.diagBuf = append(s.diagBuf, Diagnostic{
							Range:    getRange(doc.Tree, firstExprNode.Start, doc.Tree.Nodes[lastExprID].End),
							Severity: SeverityWarning,
							Code:     "ambiguous-return",
							Message:  "Ambiguous return: expression on the next line is executed as the return value. Use 'return;' to separate statements.",
							Data:     float64(i),
						})
					}
				}
			}

			if s.DiagUnreachableCode && (node.Kind == ast.KindBlock || node.Kind == ast.KindFile) {
				var terminalFound bool

				for j := uint16(0); j < node.Count; j++ {
					stmtID := doc.Tree.ExtraList[node.Extra+uint32(j)]

					if terminalFound {
						lastStmtID := doc.Tree.ExtraList[node.Extra+uint32(node.Count-1)]

						s.diagBuf = append(s.diagBuf, Diagnostic{
							Range:    getRange(doc.Tree, doc.Tree.Nodes[stmtID].Start, doc.Tree.Nodes[lastStmtID].End),
							Severity: SeverityWarning,
							Code:     "unreachable-code",
							Tags:     []DiagnosticTag{Unnecessary},
							Message:  "Unreachable code detected.",
							Data:     float64(stmtID),
						})

						break
					}

					if isTerminal(doc.Tree, stmtID) {
						terminalFound = true
					}
				}
			}
		}
	}

	// 7. Deprecated
	if s.DiagDeprecated {
		for i := 1; i < len(doc.Tree.Nodes); i++ {
			node := doc.Tree.Nodes[i]

			if node.Kind != ast.KindIdent {
				continue
			}

			ctx := s.resolveSymbolNode(uri, doc, ast.NodeID(i))
			if ctx != nil && ctx.TargetDefID != ast.InvalidNode && ctx.TargetDefID != ast.NodeID(i) {
				isDep, msg := ctx.TargetDoc.HasDeprecatedTag(ctx.TargetDefID)
				if isDep {
					diagMsg := fmt.Sprintf("Use of deprecated symbol '%s'", ctx.DisplayName)

					if msg != "" {
						diagMsg += ": " + msg
					} else {
						diagMsg += "."
					}

					s.diagBuf = append(s.diagBuf, Diagnostic{
						Range:    getNodeRange(doc.Tree, ast.NodeID(i)),
						Severity: SeverityHint,
						Code:     "deprecated",
						Tags:     []DiagnosticTag{Deprecated},
						Message:  diagMsg,
					})
				}
			}
		}
	}

	WriteMessage(s.Writer, OutgoingNotification{
		RPC:    "2.0",
		Method: "textDocument/publishDiagnostics",
		Params: PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: s.diagBuf,
		},
	})
}

func (s *Server) resolveSymbolAt(uri string, offset uint32) *SymbolContext {
	doc, ok := s.Documents[uri]
	if !ok {
		return nil
	}

	nodeID := doc.Tree.NodeAt(offset)

	return s.resolveSymbolNode(uri, doc, nodeID)
}

func (s *Server) resolveSymbolNode(uri string, doc *Document, nodeID ast.NodeID) *SymbolContext {
	if nodeID == ast.InvalidNode || doc.Tree.Nodes[nodeID].Kind != ast.KindIdent {
		return nil
	}

	identNode := doc.Tree.Nodes[nodeID]
	identBytes := doc.Source[identNode.Start:identNode.End]
	identName := string(identBytes)
	displayName := identName

	defID := doc.Resolver.References[nodeID]
	parentID := identNode.Parent

	var (
		gKey   GlobalKey
		isProp bool
		recDef ast.NodeID = ast.InvalidNode
	)

	if parentID != ast.InvalidNode {
		pNode := doc.Tree.Nodes[parentID]

		isProp = (pNode.Kind == ast.KindMemberExpr || pNode.Kind == ast.KindMethodCall || pNode.Kind == ast.KindMethodName) && pNode.Right == nodeID
		isRecordKey := pNode.Kind == ast.KindRecordField && pNode.Left == nodeID

		if isProp {
			recID := pNode.Left

			displayName = string(doc.Source[doc.Tree.Nodes[recID].Start:identNode.End])
			recBytes := doc.Source[doc.Tree.Nodes[recID].Start:doc.Tree.Nodes[recID].End]

			curr := recID

			for curr != ast.InvalidNode {
				n := doc.Tree.Nodes[curr]
				if n.Kind == ast.KindIdent {
					recDef = doc.Resolver.References[curr]

					break
				} else if n.Kind == ast.KindMemberExpr {
					curr = n.Left
				} else {
					break
				}
			}

			gKey = GlobalKey{ReceiverHash: ast.HashBytes(recBytes), PropHash: ast.HashBytes(identBytes)}
		} else if isRecordKey {
			isProp = true

			gKey = GlobalKey{ReceiverHash: 0, PropHash: 0}
		} else {
			gKey = GlobalKey{ReceiverHash: 0, PropHash: ast.HashBytes(identBytes)}

			if defID == ast.InvalidNode && (pNode.Kind == ast.KindNameList || pNode.Kind == ast.KindFunctionExpr || pNode.Kind == ast.KindForNum || pNode.Kind == ast.KindLocalFunction || pNode.Kind == ast.KindFunctionStmt) {
				defID = nodeID
			}
		}
	} else {
		gKey = GlobalKey{ReceiverHash: 0, PropHash: ast.HashBytes(identBytes)}
	}

	isGlobal := defID == ast.InvalidNode && recDef == ast.InvalidNode && (!isProp || gKey.ReceiverHash != 0)

	ctx := &SymbolContext{
		TargetDoc:   doc,
		TargetURI:   uri,
		IdentNodeID: nodeID,
		IdentName:   identName,
		DisplayName: displayName,
		IsProp:      isProp,
		GKey:        gKey,
		IsGlobal:    isGlobal,
		RecDefID:    recDef,
	}

	if defID != ast.InvalidNode {
		ctx.TargetDefID = defID

		if !ctx.IsGlobal && ctx.TargetDoc != nil && ctx.TargetDoc.ExportedGlobals != nil {
			if keys, exported := ctx.TargetDoc.ExportedGlobals[defID]; exported && len(keys) > 0 {
				ctx.IsGlobal = true
				ctx.GKey = keys[0]
			}
		}
	} else if gKey.PropHash != 0 {
		if gSym, ok := s.getGlobalSymbol(gKey.ReceiverHash, gKey.PropHash); ok {
			if gDoc, docOk := s.Documents[gSym.URI]; docOk {
				ctx.TargetDoc = gDoc
				ctx.TargetDefID = gSym.NodeID
				ctx.TargetURI = gSym.URI
			}
		}
	}

	return ctx
}

func (s *Server) getReferences(ctx *SymbolContext, includeDeclaration bool) []Location {
	var locations []Location

	seen := make(map[string]map[ast.NodeID]bool)

	addRef := func(dDoc *Document, dUri string, nodeID ast.NodeID) {
		if !includeDeclaration && dUri == ctx.TargetURI && nodeID == ctx.TargetDefID {
			return
		}

		if seen[dUri] == nil {
			seen[dUri] = make(map[ast.NodeID]bool)
		}

		if seen[dUri][nodeID] {
			return
		}

		seen[dUri][nodeID] = true

		node := dDoc.Tree.Nodes[nodeID]

		startLine, startCol := dDoc.Tree.Position(node.Start)
		endLine, endCol := dDoc.Tree.Position(node.End)

		locations = append(locations, Location{
			URI: dUri,
			Range: Range{
				Start: Position{Line: startLine, Character: startCol},
				End:   Position{Line: endLine, Character: endCol},
			},
		})
	}

	if ctx.TargetDefID != ast.InvalidNode {
		for i, def := range ctx.TargetDoc.Resolver.References {
			if def == ctx.TargetDefID {
				addRef(ctx.TargetDoc, ctx.TargetURI, ast.NodeID(i))
			}
		}
	}

	for ref := range s.iterateGlobalReferences(ctx) {
		addRef(ref.Doc, ref.URI, ref.NodeID)
	}

	if locations == nil {
		locations = []Location{}
	}

	return locations
}

func (s *Server) getDocumentHighlights(uri string, doc *Document, ctx *SymbolContext) []DocumentHighlight {
	var highlights []DocumentHighlight

	addHighlight := func(nodeID ast.NodeID, kind DocumentHighlightKind) {
		node := doc.Tree.Nodes[nodeID]

		sLine, sCol := doc.Tree.Position(node.Start)
		eLine, eCol := doc.Tree.Position(node.End)

		highlights = append(highlights, DocumentHighlight{
			Range: Range{
				Start: Position{Line: sLine, Character: sCol},
				End:   Position{Line: eLine, Character: eCol},
			},
			Kind: kind,
		})
	}

	if ctx.TargetDefID != ast.InvalidNode && ctx.TargetURI == uri {
		for i, def := range doc.Resolver.References {
			if def == ctx.TargetDefID {
				kind := ReadHighlight

				if ast.NodeID(i) == ctx.TargetDefID || isWriteAccess(doc.Tree, ast.NodeID(i)) {
					kind = WriteHighlight
				}

				addHighlight(ast.NodeID(i), kind)
			}
		}
	} else if ctx.RecDefID != ast.InvalidNode && ctx.TargetURI == uri {
		for _, fd := range doc.Resolver.FieldDefs {
			if fd.ReceiverDef == ctx.RecDefID && fd.PropHash == ctx.GKey.PropHash && fd.ReceiverHash == ctx.GKey.ReceiverHash {
				addHighlight(fd.NodeID, WriteHighlight)
			}
		}

		for _, pf := range doc.Resolver.PendingFields {
			if pf.ReceiverDef == ctx.RecDefID && pf.PropHash == ctx.GKey.PropHash && pf.ReceiverHash == ctx.GKey.ReceiverHash {
				kind := ReadHighlight

				if isWriteAccess(doc.Tree, pf.PropNodeID) {
					kind = WriteHighlight
				}

				addHighlight(pf.PropNodeID, kind)
			}
		}
	}

	for ref := range s.iterateGlobalReferences(ctx) {
		if ref.URI == uri {
			kind := ReadHighlight

			if isWriteAccess(ref.Doc.Tree, ref.NodeID) {
				kind = WriteHighlight
			} else {
				pNode := ref.Doc.Tree.Nodes[ref.Doc.Tree.Nodes[ref.NodeID].Parent]
				if pNode.Kind == ast.KindFunctionStmt || pNode.Kind == ast.KindLocalFunction {
					kind = WriteHighlight
				}
			}

			addHighlight(ref.NodeID, kind)
		}
	}

	if len(highlights) == 0 {
		addHighlight(ctx.IdentNodeID, WriteHighlight)
	}

	slices.SortFunc(highlights, func(a, b DocumentHighlight) int {
		return cmp.Or(
			cmp.Compare(a.Range.Start.Line, b.Range.Start.Line),
			cmp.Compare(a.Range.Start.Character, b.Range.Start.Character),
		)
	})

	return slices.CompactFunc(highlights, func(a, b DocumentHighlight) bool {
		return a.Range.Start == b.Range.Start && a.Range.End == b.Range.End
	})
}

func (s *Server) getGlobalAlias(hash uint64) uint64 {
	sym, ok := s.GlobalIndex[GlobalKey{ReceiverHash: 0, PropHash: hash}]
	if !ok {
		return 0
	}

	doc, ok := s.Documents[sym.URI]
	if !ok {
		return 0
	}

	valID := doc.getAssignedValue(sym.NodeID)
	if valID == ast.InvalidNode {
		return 0
	}

	node := doc.Tree.Nodes[valID]

	if node.Kind == ast.KindIdent || node.Kind == ast.KindMemberExpr {
		if node.Kind == ast.KindIdent && doc.Resolver.References[valID] != ast.InvalidNode {
			return 0
		}

		return ast.HashBytes(doc.Source[node.Start:node.End])
	}

	return 0
}

func (s *Server) getGlobalSymbol(recHash, propHash uint64) (GlobalSymbol, bool) {
	currRec := recHash

	for range 10 {
		key := GlobalKey{ReceiverHash: currRec, PropHash: propHash}
		if sym, exists := s.GlobalIndex[key]; exists {
			return sym, true
		}

		if currRec == 0 {
			break
		}

		nextRec := s.getGlobalAlias(currRec)
		if nextRec == 0 {
			break
		}

		currRec = nextRec
	}

	return GlobalSymbol{}, false
}

func (s *Server) setGlobalSymbol(key GlobalKey, uri string, nodeID ast.NodeID, depth int, name string) {
	if doc, ok := s.Documents[uri]; ok {
		if doc.ExportedGlobals == nil {
			doc.ExportedGlobals = make(map[ast.NodeID][]GlobalKey)
		}

		doc.ExportedGlobals[nodeID] = append(doc.ExportedGlobals[nodeID], key)
	}

	if existing, exists := s.GlobalIndex[key]; exists {
		if depth > existing.Depth {
			return
		}

		// Prefer standard library definitions if depths are tied
		if depth == existing.Depth && strings.HasPrefix(existing.URI, "std://") && !strings.HasPrefix(uri, "std://") {
			return
		}
	}

	s.GlobalIndex[key] = GlobalSymbol{
		URI:    uri,
		NodeID: nodeID,
		Depth:  depth,
		Name:   name,
	}
}

func (s *Server) removeDocumentGlobals(uri string, doc *Document) {
	if doc.ExportedGlobals == nil {
		return
	}

	for _, keys := range doc.ExportedGlobals {
		for _, key := range keys {
			if sym, ok := s.GlobalIndex[key]; ok && sym.URI == uri {
				delete(s.GlobalIndex, key)

				var (
					bestSym GlobalSymbol
					found   bool
				)

				for otherURI, otherDoc := range s.Documents {
					if otherURI == uri {
						continue
					}

					for nodeID, otherKeys := range otherDoc.ExportedGlobals {
						for _, k := range otherKeys {
							if k == key {
								d := getASTDepth(otherDoc.Tree, nodeID)

								if !found || d < bestSym.Depth || (d == bestSym.Depth && strings.HasPrefix(otherURI, "std://")) {
									bestSym = GlobalSymbol{
										URI:    otherURI,
										NodeID: nodeID,
										Depth:  d,
										Name:   sym.Name,
									}

									found = true
								}
							}
						}
					}
				}

				if found {
					s.GlobalIndex[key] = bestSym
				}
			}
		}
	}
}

func (s *Server) indexEmbeddedStdlib(indexed, unchanged *int) {
	entries, err := stdlibFS.ReadDir("stdlib")
	if err != nil {
		return
	}

	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".lua") {
			b, err := stdlibFS.ReadFile("stdlib/" + e.Name())
			if err == nil {
				uri := "std:///" + e.Name()

				if existing, ok := s.Documents[uri]; ok && bytes.Equal(existing.Source, b) {
					s.activeURIs[uri] = true

					*unchanged++

					continue
				}

				s.updateDocument(uri, b)

				s.activeURIs[uri] = true

				*indexed++
			}
		}
	}
}

func (s *Server) isIgnored(fullPath, name string) bool {
	for _, p := range s.compiledIgnores {
		if p.HasSuffix != "" && strings.HasSuffix(name, p.HasSuffix) {
			return true
		}

		if p.HasPrefix != "" && strings.HasPrefix(name, p.HasPrefix) {
			return true
		}

		if p.ContainsPath != "" && strings.Contains(fullPath, p.ContainsPath) {
			return true
		}

		if p.SuffixPath != "" && strings.HasSuffix(fullPath, p.SuffixPath) {
			return true
		}

		if p.MatchFallback != "" {
			if matched, _ := filepath.Match(p.MatchFallback, name); matched {
				return true
			}
		}
	}

	return false
}

func (s *Server) clearDocument(uri string) {
	if doc, ok := s.Documents[uri]; ok {
		s.removeDocumentGlobals(uri, doc)
	}

	delete(s.Documents, uri)

	WriteMessage(s.Writer, OutgoingNotification{
		RPC:    "2.0",
		Method: "textDocument/publishDiagnostics",
		Params: PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: []Diagnostic{},
		},
	})
}

func (s *Server) uriToPath(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return ""
	}

	path := uri[7:]

	if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}

	return filepath.Clean(filepath.FromSlash(path))
}

func (s *Server) pathToURI(pathStr string) string {
	cleanPath := filepath.Clean(pathStr)

	if runtime.GOOS == "windows" {
		if len(cleanPath) > 1 && cleanPath[1] == ':' {
			cleanPath = strings.ToLower(cleanPath[:1]) + cleanPath[1:]
		}

		return "file:///" + filepath.ToSlash(cleanPath)
	}

	return "file://" + filepath.ToSlash(cleanPath)
}

func (s *Server) normalizeURI(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return uri
	}

	return s.pathToURI(s.uriToPath(uri))
}

func (s *Server) isIgnoredURI(uri string) bool {
	path := s.uriToPath(uri)

	if path == "" {
		return false
	}

	return s.isIgnored(path, filepath.Base(path))
}

func (s *Server) isWorkspaceURI(uri string) bool {
	if strings.HasPrefix(uri, "std:///") {
		return false
	}

	path := s.uriToPath(uri)

	if path == "" {
		return false
	}

	lowerPath := strings.ToLower(path)

	for _, libPath := range s.lowerLibraryPaths {
		if strings.HasPrefix(lowerPath, libPath) {
			return false
		}
	}

	if s.RootURI == "" {
		return true
	}

	return strings.HasPrefix(lowerPath, s.lowerRootPath)
}

func (s *Server) isKnownGlobal(name []byte) bool {
	if s.KnownGlobals[string(name)] {
		return true
	}

	if len(s.KnownGlobalGlobs) == 0 {
		return false
	}

	strName := string(name)

	for _, glob := range s.KnownGlobalGlobs {
		if matched, _ := filepath.Match(glob, strName); matched {
			return true
		}
	}

	return false
}

func (s *Server) buildCallHierarchyItemFromDef(uri string, doc *Document, defID ast.NodeID) CallHierarchyItem {
	valID := doc.getAssignedValue(defID)
	isFunc := valID != ast.InvalidNode && doc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr

	node := doc.Tree.Nodes[defID]
	name := string(doc.Source[node.Start:node.End])
	kind := SymbolKindVariable

	if isFunc {
		kind = SymbolKindFunction
		if doc.Tree.Nodes[node.Parent].Kind == ast.KindMethodName || doc.Tree.Nodes[node.Parent].Kind == ast.KindRecordField {
			kind = SymbolKindMethod
		}
	}

	switch node.Kind {
	case ast.KindFile:
		name = "(main)"
		kind = SymbolKindFile
	case ast.KindFunctionExpr:
		name = "(anonymous function)"
		kind = SymbolKindFunction
	}

	selRange := getNodeRange(doc.Tree, defID)
	fullRange := selRange

	if isFunc {
		fullRange = getNodeRange(doc.Tree, valID)
	} else if node.Kind == ast.KindFile {
		fullRange = Range{
			Start: Position{Line: 0, Character: 0},
			End:   selRange.Start,
		}

		if len(doc.Tree.LineOffsets) > 0 {
			lastLine := uint32(len(doc.Tree.LineOffsets) - 1)
			lastCol := uint32(len(doc.Source)) - doc.Tree.LineOffsets[lastLine]

			fullRange.End = Position{Line: lastLine, Character: lastCol}
		}
	}

	var detail string

	if uri != "" {
		detail = filepath.Base(s.uriToPath(uri))
	}

	var tags []SymbolTag

	if isDep, _ := doc.HasDeprecatedTag(defID); isDep {
		tags = append(tags, SymbolTagDeprecated)
	}

	return CallHierarchyItem{
		Name:           name,
		Kind:           kind,
		Tags:           tags,
		Detail:         detail,
		URI:            uri,
		Range:          fullRange,
		SelectionRange: selRange,
		Data: map[string]any{
			"uri":   uri,
			"defId": float64(defID),
		},
	}
}

func (s *Server) getGlobalPath(doc *Document, id ast.NodeID, depth int) []byte {
	if id == ast.InvalidNode || depth > 10 {
		return nil
	}

	node := doc.Tree.Nodes[id]

	switch node.Kind {
	case ast.KindIdent:
		defID := doc.Resolver.References[id]
		if defID == ast.InvalidNode {
			return doc.Source[node.Start:node.End]
		}

		valID := doc.getAssignedValue(defID)
		if valID != ast.InvalidNode && valID != id {
			return s.getGlobalPath(doc, valID, depth+1)
		}

		return nil
	case ast.KindMemberExpr:
		leftPath := s.getGlobalPath(doc, node.Left, depth+1)
		if leftPath != nil {
			rightBytes := doc.Source[doc.Tree.Nodes[node.Right].Start:doc.Tree.Nodes[node.Right].End]

			buf := make([]byte, 0, len(leftPath)+1+len(rightBytes))
			buf = append(buf, leftPath...)
			buf = append(buf, '.')
			buf = append(buf, rightBytes...)

			return buf
		}
	}

	return nil
}

func (s *Server) getEnclosingFunctionDef(doc *Document, id ast.NodeID) ast.NodeID {
	curr := doc.Tree.Nodes[id].Parent

	for curr != ast.InvalidNode {
		node := doc.Tree.Nodes[curr]
		if node.Kind == ast.KindFunctionExpr {
			pID := node.Parent
			if pID != ast.InvalidNode {
				pNode := doc.Tree.Nodes[pID]
				if pNode.Kind == ast.KindLocalFunction || pNode.Kind == ast.KindFunctionStmt {
					return pNode.Left
				} else if pNode.Kind == ast.KindRecordField {
					return pNode.Left
				} else if pNode.Kind == ast.KindExprList {
					gpID := pNode.Parent
					if gpID != ast.InvalidNode {
						gpNode := doc.Tree.Nodes[gpID]
						if (gpNode.Kind == ast.KindAssign || gpNode.Kind == ast.KindLocalAssign) && gpNode.Right == pID {
							idx := -1

							for i := uint16(0); i < pNode.Count; i++ {
								if doc.Tree.ExtraList[pNode.Extra+uint32(i)] == curr {
									idx = int(i)

									break
								}
							}

							if idx != -1 {
								lhs := doc.Tree.Nodes[gpNode.Left]
								if uint16(idx) < lhs.Count {
									return doc.Tree.ExtraList[lhs.Extra+uint32(idx)]
								}
							}
						}
					}
				}
			}

			return curr
		} else if node.Kind == ast.KindFile {
			return curr
		}

		curr = node.Parent
	}

	return doc.Tree.Root
}

func (s *Server) iterateGlobalReferences(ctx *SymbolContext) iter.Seq[GlobalReference] {
	return func(yield func(GlobalReference) bool) {
		if !ctx.IsGlobal {
			return
		}

		for dUri, dDoc := range s.Documents {
			if ctx.GKey.ReceiverHash == 0 {
				for _, id := range dDoc.Resolver.GlobalDefs {
					if ast.HashBytes(dDoc.Source[dDoc.Tree.Nodes[id].Start:dDoc.Tree.Nodes[id].End]) == ctx.GKey.PropHash {
						if !yield(GlobalReference{Doc: dDoc, URI: dUri, NodeID: id}) {
							return
						}
					}
				}

				for _, id := range dDoc.Resolver.GlobalRefs {
					if ast.HashBytes(dDoc.Source[dDoc.Tree.Nodes[id].Start:dDoc.Tree.Nodes[id].End]) == ctx.GKey.PropHash {
						if dDoc.Resolver.References[id] == ast.InvalidNode {
							if !yield(GlobalReference{Doc: dDoc, URI: dUri, NodeID: id}) {
								return
							}
						}
					}
				}
			} else {
				for _, fd := range dDoc.Resolver.FieldDefs {
					if fd.ReceiverHash == ctx.GKey.ReceiverHash && fd.PropHash == ctx.GKey.PropHash {
						if !yield(GlobalReference{Doc: dDoc, URI: dUri, NodeID: fd.NodeID}) {
							return
						}
					}
				}

				for _, pf := range dDoc.Resolver.PendingFields {
					if pf.ReceiverHash == ctx.GKey.ReceiverHash && pf.PropHash == ctx.GKey.PropHash {
						if dDoc.Resolver.References[pf.PropNodeID] == ast.InvalidNode {
							if !yield(GlobalReference{Doc: dDoc, URI: dUri, NodeID: pf.PropNodeID}) {
								return
							}
						}
					}
				}
			}
		}
	}
}

func (s *Server) compileIgnorePatterns() {
	s.compiledIgnores = make([]IgnorePattern, 0, len(s.IgnoreGlobs))

	for _, g := range s.IgnoreGlobs {
		cleanGlob := strings.TrimPrefix(strings.TrimPrefix(g, "**/"), "*/")
		cleanGlob = strings.TrimSuffix(strings.TrimSuffix(cleanGlob, "/**"), "/*")

		if cleanGlob == "" {
			continue
		}

		if !strings.ContainsAny(cleanGlob, "*?[") {
			cleanPath := filepath.FromSlash(cleanGlob)

			s.compiledIgnores = append(s.compiledIgnores, IgnorePattern{
				ContainsPath: string(filepath.Separator) + cleanPath + string(filepath.Separator),
				SuffixPath:   string(filepath.Separator) + cleanPath,
				HasSuffix:    cleanGlob,
			})
		} else if strings.HasPrefix(cleanGlob, "*") && !strings.ContainsAny(cleanGlob[1:], "*?[") {
			s.compiledIgnores = append(s.compiledIgnores, IgnorePattern{HasSuffix: cleanGlob[1:]})
		} else if strings.HasSuffix(cleanGlob, "*") && !strings.ContainsAny(cleanGlob[:len(cleanGlob)-1], "*?[") {
			s.compiledIgnores = append(s.compiledIgnores, IgnorePattern{HasPrefix: cleanGlob[:len(cleanGlob)-1]})
		} else {
			s.compiledIgnores = append(s.compiledIgnores, IgnorePattern{MatchFallback: g})
		}
	}
}

func (s *Server) suggestGlobal(name string) string {
	var (
		bestMatch string
		minDist   = 3
	)

	check := func(candidate string) {
		d := levenshteinFast(name, candidate, minDist-1)
		if d < minDist {
			minDist = d
			bestMatch = candidate
		}
	}

	// Prioritize known globals
	for k := range s.KnownGlobals {
		check(k)
	}

	// Then check workspace globals
	for key, sym := range s.GlobalIndex {
		if key.ReceiverHash == 0 {
			check(sym.Name)
		}
	}

	return bestMatch
}

func getRange(tree *ast.Tree, start, end uint32) Range {
	sLine, sCol := tree.Position(start)
	eLine, eCol := tree.Position(end)

	return Range{
		Start: Position{Line: sLine, Character: sCol},
		End:   Position{Line: eLine, Character: eCol},
	}
}

func getNodeRange(tree *ast.Tree, nodeID ast.NodeID) Range {
	node := tree.Nodes[nodeID]

	return getRange(tree, node.Start, node.End)
}

func isWriteAccess(tree *ast.Tree, nodeID ast.NodeID) bool {
	pID := tree.Nodes[nodeID].Parent
	if pID == ast.InvalidNode {
		return false
	}

	pNode := tree.Nodes[pID]

	switch pNode.Kind {
	case ast.KindNameList:
		gpID := pNode.Parent
		if gpID != ast.InvalidNode {
			gpNode := tree.Nodes[gpID]

			return gpNode.Kind == ast.KindLocalAssign || gpNode.Kind == ast.KindForIn
		}
	case ast.KindExprList:
		gpID := pNode.Parent
		if gpID != ast.InvalidNode {
			gpNode := tree.Nodes[gpID]

			return gpNode.Kind == ast.KindAssign && gpNode.Left == pID
		}
	case ast.KindForNum, ast.KindLocalFunction, ast.KindFunctionStmt, ast.KindRecordField:
		return pNode.Left == nodeID
	case ast.KindMethodName:
		return pNode.Right == nodeID
	case ast.KindMemberExpr:
		if pNode.Right == nodeID {
			gpID := pNode.Parent
			if gpID != ast.InvalidNode {
				gpNode := tree.Nodes[gpID]
				if gpNode.Kind == ast.KindExprList {
					ggpID := gpNode.Parent
					if ggpID != ast.InvalidNode {
						ggpNode := tree.Nodes[ggpID]

						return ggpNode.Kind == ast.KindAssign && ggpNode.Left == gpID
					}
				}
			}
		}
	}

	return false
}

func isTerminal(tree *ast.Tree, id ast.NodeID) bool {
	if id == ast.InvalidNode {
		return false
	}

	node := tree.Nodes[id]

	switch node.Kind {
	case ast.KindReturn, ast.KindBreak, ast.KindGoto:
		return true
	case ast.KindDo:
		return isTerminal(tree, node.Left)
	case ast.KindBlock:
		for i := uint16(0); i < node.Count; i++ {
			if isTerminal(tree, tree.ExtraList[node.Extra+uint32(i)]) {
				return true
			}
		}

		return false
	case ast.KindIf:
		if !isTerminal(tree, node.Right) {
			return false
		}

		var hasElse bool

		for i := uint16(0); i < node.Count; i++ {
			childID := tree.ExtraList[node.Extra+uint32(i)]
			childNode := tree.Nodes[childID]

			switch childNode.Kind {
			case ast.KindElseIf:
				if !isTerminal(tree, childNode.Right) {
					return false
				}
			case ast.KindElse:
				hasElse = true

				if !isTerminal(tree, childNode.Left) {
					return false
				}
			}
		}

		return hasElse
	}

	return false
}

func getASTDepth(tree *ast.Tree, id ast.NodeID) int {
	var depth int

	curr := tree.Nodes[id].Parent

	for curr != ast.InvalidNode {
		depth++

		curr = tree.Nodes[curr].Parent
	}

	return depth
}

func containsFold(b, queryLower []byte) bool {
	if len(queryLower) == 0 {
		return true
	}

	if len(b) < len(queryLower) {
		return false
	}

	for i := 0; i <= len(b)-len(queryLower); i++ {
		match := true

		for j := range queryLower {
			cb := b[i+j]

			if cb >= 'A' && cb <= 'Z' {
				cb += 32 // fast to-lower
			}

			qb := queryLower[j]

			if cb != qb {
				if (cb == '.' && qb == ':') || (cb == ':' && qb == '.') {
					continue
				}

				match = false

				break
			}
		}

		if match {
			return true
		}
	}

	return false
}

func isRootLevel(tree *ast.Tree, id ast.NodeID) bool {
	curr := tree.Nodes[id].Parent

	for curr != ast.InvalidNode {
		n := tree.Nodes[curr]

		if n.Kind == ast.KindBlock {
			parentID := n.Parent
			if parentID == tree.Root {
				return true
			}

			pNode := tree.Nodes[parentID]
			if pNode.Kind == ast.KindIf || pNode.Kind == ast.KindElseIf || pNode.Kind == ast.KindElse || pNode.Kind == ast.KindDo {
				curr = parentID

				continue
			}

			return false
		}

		curr = n.Parent
	}

	return false
}

// Fast, zero-allocation Levenshtein distance for strings up to 63 bytes.
func levenshteinFast(s, t string, maxDist int) int {
	if len(s) > len(t) {
		s, t = t, s
	}

	ls := len(s)
	lt := len(t)

	if ls == 0 {
		return lt
	}

	if lt-ls > maxDist {
		return maxDist + 1
	}

	if lt > 63 {
		return maxDist + 1
	}

	var (
		v0 [64]int
		v1 [64]int
	)

	for i := 0; i <= ls; i++ {
		v0[i] = i
	}

	for i := range lt {
		v1[0] = i + 1

		minDistForRow := v1[0]

		for j := range ls {
			cost := 1

			if t[i] == s[j] {
				cost = 0
			}

			a := v1[j] + 1
			b := v0[j+1] + 1
			c := v0[j] + cost

			m := min(c, min(b, a))

			v1[j+1] = m

			if m < minDistForRow {
				minDistForRow = m
			}
		}

		if minDistForRow > maxDist {
			return maxDist + 1
		}

		v0 = v1
	}

	return v1[ls]
}
