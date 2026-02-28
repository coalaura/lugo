package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
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

type SymbolContext struct {
	TargetDoc   *Document
	IdentName   string
	DisplayName string
	TargetURI   string
	GKey        GlobalKey
	IdentNodeID ast.NodeID
	TargetDefID ast.NodeID
	IsProp      bool
	IsGlobal    bool
}

type SemanticToken struct {
	Start     uint32
	End       uint32
	TokenType uint32
	Modifiers uint32
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
	IgnoreGlobs       []string
	IsIndexing        bool

	semTokensBuf []SemanticToken
	semDataBuf   []uint32

	DiagUndefinedGlobals bool
	DiagImplicitGlobals  bool
	DiagUnusedVariables  bool
	DiagShadowing        bool
	DiagUnreachableCode  bool
	DiagAmbiguousReturns bool
	DiagDeprecated       bool
	InlayParamHints      bool
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
		IsIndexing:   true,
	}
}

func (s *Server) Start() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	logPath := filepath.Join(filepath.Dir(exePath), "lsp.log")

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	defer logFile.Close()

	s.Log = plain.New(
		plain.WithTarget(logFile),
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
			s.DiagUnusedVariables = params.InitializationOptions.DiagnosticsUnusedVariables
			s.DiagShadowing = params.InitializationOptions.DiagnosticsShadowing
			s.DiagUnreachableCode = params.InitializationOptions.DiagnosticsUnreachableCode
			s.DiagAmbiguousReturns = params.InitializationOptions.DiagnosticsAmbiguousReturns
			s.DiagDeprecated = params.InitializationOptions.DiagnosticsDeprecated
			s.InlayParamHints = params.InitializationOptions.InlayHintsParameterNames
		}

		result := InitializeResult{
			Capabilities: ServerCapabilities{
				TextDocumentSync:        1,
				DefinitionProvider:      true,
				HoverProvider:           true,
				RenameProvider:          true,
				ReferencesProvider:      true,
				DocumentSymbolProvider:  true,
				WorkspaceSymbolProvider: true,
				InlayHintProvider:       true,
				CodeActionProvider:      true,
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

		s.IsIndexing = true

		start := time.Now()

		openDocs := make(map[string]*Document)

		for uri, doc := range s.Documents {
			if s.OpenFiles[uri] {
				openDocs[uri] = doc
			}
		}

		s.Documents = make(map[string]*Document)
		s.GlobalIndex = make(map[GlobalKey]GlobalSymbol)

		s.indexEmbeddedStdlib()

		for _, libPath := range s.LibraryPaths {
			s.Log.Printf("Indexing external library: %s\n", libPath)

			s.indexWorkspace(libPath)
		}

		if s.RootURI != "" {
			s.Log.Printf("Indexing workspace: %s", s.RootURI)

			s.indexWorkspace(s.RootURI)
		}

		for uri, doc := range openDocs {
			s.updateDocument(uri, doc.Source)
		}

		took := time.Since(start)

		s.Log.Printf("Re-indexed workspace in %s\n", took)

		s.IsIndexing = false

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
			defNode := ctx.TargetDoc.Tree.Nodes[ctx.TargetDefID]

			startLine, startCol := ctx.TargetDoc.Tree.Position(defNode.Start)
			endLine, endCol := ctx.TargetDoc.Tree.Position(defNode.End)

			loc := Location{
				URI: ctx.TargetURI,
				Range: Range{
					Start: Position{Line: startLine, Character: startCol},
					End:   Position{Line: endLine, Character: endCol},
				},
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

		if ctx == nil {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

			return
		}

		var (
			hoverText string
			fromFile  string
		)

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
				} else if ctx.IsProp {
					code = ctx.DisplayName + valStr
				} else if ctx.TargetURI == uri && ctx.TargetDefID == doc.Resolver.References[ctx.IdentNodeID] {
					var attrStr string

					if ast.Attr(ctx.TargetDoc.Tree.Nodes[ctx.TargetDefID].Extra) == ast.AttrConst {
						attrStr = " <const>"
					} else if ast.Attr(ctx.TargetDoc.Tree.Nodes[ctx.TargetDefID].Extra) == ast.AttrClose {
						attrStr = " <close>"
					}

					code = "local " + ctx.DisplayName + attrStr + valStr
				} else {
					code = "global " + ctx.DisplayName + valStr
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
				for _, r := range luadoc.Returns {
					docBuilder.WriteString("* `@return` `" + r.Type + "`")

					if r.Desc != "" {
						docBuilder.WriteString(" - " + r.Desc)
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

				for _, s := range luadoc.See {
					docBuilder.WriteString("* `" + s + "`\n")
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
			if ctx.IsProp {
				hoverText = "```lua\n" + ctx.DisplayName + " (field)\n```"
			} else {
				hoverText = "```lua\nglobal " + ctx.DisplayName + "\n```"
			}
		}

		identNode := doc.Tree.Nodes[ctx.IdentNodeID]

		startLine, startCol := doc.Tree.Position(identNode.Start)
		endLine, endCol := doc.Tree.Position(identNode.End)

		result := Hover{
			Contents: MarkupContent{Kind: "markdown", Value: hoverText},
			Range: &Range{
				Start: Position{Line: startLine, Character: startCol},
				End:   Position{Line: endLine, Character: endCol},
			},
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
	case "textDocument/codeLens":
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

				identNode := doc.Tree.Nodes[identNodeID]

				startLine, startCol := doc.Tree.Position(identNode.Start)
				endLine, endCol := doc.Tree.Position(identNode.End)

				lenses = append(lenses, CodeLens{
					Range: Range{
						Start: Position{Line: startLine, Character: startCol},
						End:   Position{Line: endLine, Character: endCol},
					},
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

		getRange := func(start, end uint32) Range {
			sLine, sCol := doc.Tree.Position(start)
			eLine, eCol := doc.Tree.Position(end)

			return Range{
				Start: Position{Line: sLine, Character: sCol},
				End:   Position{Line: eLine, Character: eCol},
			}
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
						Range:          getRange(fieldNode.Start, fieldNode.End),
						SelectionRange: getRange(keyNode.Start, keyNode.End),
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
					Range:          getRange(node.Start, node.End),
					SelectionRange: getRange(nameNode.Start, nameNode.End),
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
								Range:          getRange(node.Start, node.End),
								SelectionRange: getRange(lNode.Start, lNode.End),
							})
						} else if rNode.Kind == ast.KindTableExpr {
							syms = append(syms, DocumentSymbol{
								Name:           name,
								Kind:           SymbolKindClass,
								Range:          getRange(node.Start, node.End),
								SelectionRange: getRange(lNode.Start, lNode.End),
								Children:       walkTable(rID),
							})
						} else if node.Kind == ast.KindLocalAssign {
							syms = append(syms, DocumentSymbol{
								Name:           name,
								Kind:           SymbolKindVariable,
								Range:          getRange(lNode.Start, lNode.End),
								SelectionRange: getRange(lNode.Start, lNode.End),
							})
						}
					}
				}
			}

			return syms
		}

		var rootID ast.NodeID = ast.InvalidNode

		for i := len(doc.Tree.Nodes) - 1; i >= 1; i-- {
			if doc.Tree.Nodes[i].Kind == ast.KindFile {
				rootID = ast.NodeID(i)

				break
			}
		}

		symbols := walk(rootID)

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

			node := doc.Tree.Nodes[sym.NodeID]
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

			startLine, startCol := doc.Tree.Position(node.Start)
			endLine, endCol := doc.Tree.Position(node.End)

			results = append(results, SymbolInformation{
				Name: sym.Name,
				Kind: kind,
				Location: Location{
					URI: sym.URI,
					Range: Range{
						Start: Position{Line: startLine, Character: startCol},
						End:   Position{Line: endLine, Character: endCol},
					},
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

		addEdit := func(dDoc *Document, dUri string, nodeID ast.NodeID) {
			node := dDoc.Tree.Nodes[nodeID]

			startLine, startCol := dDoc.Tree.Position(node.Start)
			endLine, endCol := dDoc.Tree.Position(node.End)

			changes[dUri] = append(changes[dUri], TextEdit{
				Range: Range{
					Start: Position{Line: startLine, Character: startCol},
					End:   Position{Line: endLine, Character: endCol},
				},
				NewText: params.NewName,
			})
		}

		if !ctx.IsGlobal && ctx.TargetDefID != ast.InvalidNode {
			for i, def := range ctx.TargetDoc.Resolver.References {
				if def == ctx.TargetDefID {
					addEdit(ctx.TargetDoc, ctx.TargetURI, ast.NodeID(i))
				}
			}
		} else {
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

		var actions []CodeAction

		uri := s.normalizeURI(params.TextDocument.URI)

		for _, diag := range params.Context.Diagnostics {
			switch diag.Code {
			case "unused-local":
				actions = append(actions, CodeAction{
					Title:       "Prefix unused variable with '_'",
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
									NewText: "_",
								},
							},
						},
					},
				})
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

		if actions == nil {
			actions = []CodeAction{}
		}

		WriteMessage(s.Writer, Response{
			RPC:    "2.0",
			ID:     req.ID,
			Result: actions,
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
			if a.Start < b.Start {
				return -1
			}

			if a.Start > b.Start {
				return 1
			}

			return 0
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

func (s *Server) indexWorkspace(rootPathOrURI string) {
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

	var (
		indexed uint64
		failed  uint64
		skipped uint64
	)

	visited := make(map[string]bool)

	var walk func(dir string)

	walk = func(dir string) {
		if visited[dir] {
			return
		}

		visited[dir] = true

		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		for _, e := range entries {
			fullPath := filepath.Join(dir, e.Name())

			isDir := e.IsDir()
			name := e.Name()

			if s.isIgnored(fullPath, name) {
				if !isDir {
					skipped++
				}

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
						failed++

						continue
					}
				} else {
					failed++

					continue // Broken symlink
				}
			}

			if isDir {
				walk(fullPath)
			} else if strings.HasSuffix(name, ".lua") {
				b, fsErr := os.ReadFile(fullPath)
				if fsErr == nil {
					uri := s.pathToURI(fullPath)

					if runtime.GOOS == "windows" {
						uri = strings.ToLower(uri)
					}

					s.updateDocument(uri, b)

					indexed++
				} else {
					failed++
				}
			} else {
				skipped++
			}
		}
	}

	walk(path)

	s.Log.Printf("Workspace indexing complete (indexed=%d, failed=%d, skipped=%d)\n", indexed, failed, skipped)
}

func (s *Server) updateDocument(uri string, source []byte) {
	var (
		tree *ast.Tree
		doc  *Document
	)

	if existing, exists := s.Documents[uri]; exists {
		doc = existing
		doc.Source = source
		doc.ExportedGlobals = make(map[ast.NodeID]GlobalKey)

		tree = existing.Tree
		tree.Reset(source)
	} else {
		tree = ast.NewTree(source)

		doc = &Document{
			Source:          source,
			Tree:            tree,
			Resolver:        semantic.New(tree),
			ExportedGlobals: make(map[ast.NodeID]GlobalKey),
		}

		s.Documents[uri] = doc
	}

	p := parser.New(source, tree)

	rootID := p.Parse()

	doc.Errors = p.Errors

	res := doc.Resolver

	res.Reset()
	res.Resolve(rootID)

	for key, sym := range s.GlobalIndex {
		if sym.URI == uri {
			delete(s.GlobalIndex, key)
		}
	}

	for _, defID := range res.GlobalDefs {
		node := tree.Nodes[defID]
		identBytes := tree.Source[node.Start:node.End]
		hash := ast.HashBytes(tree.Source[node.Start:node.End])

		depth := getASTDepth(tree, defID)

		s.setGlobalSymbol(GlobalKey{ReceiverHash: 0, PropHash: hash}, uri, defID, depth, string(identBytes))

		doc.ExtractLuaDocFields(defID, func(name []byte) {
			fieldHash := ast.HashBytes(name)

			s.setGlobalSymbol(GlobalKey{ReceiverHash: hash, PropHash: fieldHash}, uri, defID, depth, string(identBytes)+"."+string(name))
		})

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

							s.setGlobalSymbol(GlobalKey{ReceiverHash: hash, PropHash: fd.PropHash}, uri, fd.NodeID, depth, string(identBytes)+"."+string(propBytes))
						} else if len(fd.ReceiverName) > len(localName) && bytes.HasPrefix(fd.ReceiverName, localName) && fd.ReceiverName[len(localName)] == '.' {
							suffix := fd.ReceiverName[len(localName)+1:]

							newRec := make([]byte, 0, len(globalBytes)+1+len(suffix))
							newRec = append(newRec, globalBytes...)
							newRec = append(newRec, '.')
							newRec = append(newRec, suffix...)

							newRecHash := ast.HashBytes(newRec)

							propBytes := doc.Source[doc.Tree.Nodes[fd.NodeID].Start:doc.Tree.Nodes[fd.NodeID].End]

							s.setGlobalSymbol(GlobalKey{ReceiverHash: newRecHash, PropHash: fd.PropHash}, uri, fd.NodeID, depth, string(identBytes)+"."+string(suffix)+"."+string(propBytes))
						}
					}
				}
			}
		}
	}

	// Index global table fields
	for _, fd := range res.FieldDefs {
		if fd.ReceiverDef == ast.InvalidNode {
			if bytes.Equal(fd.ReceiverName, []byte("self")) {
				continue
			}

			depth := getASTDepth(tree, fd.NodeID)

			propBytes := doc.Source[doc.Tree.Nodes[fd.NodeID].Start:doc.Tree.Nodes[fd.NodeID].End]

			sep := "."

			if doc.Tree.Nodes[doc.Tree.Nodes[fd.NodeID].Parent].Kind == ast.KindMethodName {
				sep = ":"
			}

			s.setGlobalSymbol(GlobalKey{ReceiverHash: fd.ReceiverHash, PropHash: fd.PropHash}, uri, fd.NodeID, depth, string(fd.ReceiverName)+sep+string(propBytes))
		}
	}

	s.Documents[uri] = doc
}

func (s *Server) publishDiagnostics(uri string) {
	if s.IsIndexing {
		return
	}

	doc := s.Documents[uri]

	var diagnostics []Diagnostic

	for _, err := range doc.Errors {
		startLine, startCol := doc.Tree.Position(err.Start)
		endLine, endCol := doc.Tree.Position(err.End)

		if startLine == endLine && startCol == endCol {
			endCol++
		}

		diagnostics = append(diagnostics, Diagnostic{
			Range: Range{
				Start: Position{Line: startLine, Character: startCol},
				End:   Position{Line: endLine, Character: endCol},
			},
			Severity: SeverityError,
			Message:  err.Message,
		})

		// accidental implicit globals
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
				startLine, startCol := doc.Tree.Position(node.Start)
				endLine, endCol := doc.Tree.Position(node.End)

				diagnostics = append(diagnostics, Diagnostic{
					Range: Range{
						Start: Position{Line: startLine, Character: startCol},
						End:   Position{Line: endLine, Character: endCol},
					},
					Severity: SeverityWarning,
					Message:  "Undefined global: " + string(identBytes),
				})
			}
		}
	}

	if s.DiagImplicitGlobals {
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

			startLine, startCol := doc.Tree.Position(node.Start)
			endLine, endCol := doc.Tree.Position(node.End)

			diagnostics = append(diagnostics, Diagnostic{
				Range: Range{
					Start: Position{Line: startLine, Character: startCol},
					End:   Position{Line: endLine, Character: endCol},
				},
				Severity: SeverityWarning,
				Code:     "implicit-global",
				Message:  "Implicit global creation: '" + string(identBytes) + "'. Did you forget the 'local' keyword?",
			})
		}
	}

	if s.DiagShadowing || s.DiagUnusedVariables {
		for _, defID := range doc.Resolver.LocalDefs {
			node := doc.Tree.Nodes[defID]
			nameBytes := doc.Source[node.Start:node.End]

			if len(nameBytes) > 0 && nameBytes[0] == '_' {
				continue
			}

			startLine, startCol := doc.Tree.Position(node.Start)
			endLine, endCol := doc.Tree.Position(node.End)

			r := Range{
				Start: Position{Line: startLine, Character: startCol},
				End:   Position{Line: endLine, Character: endCol},
			}

			if s.DiagUnusedVariables && doc.Resolver.UsageCount[defID] == 0 {
				diagnostics = append(diagnostics, Diagnostic{
					Range:    r,
					Severity: SeverityHint,
					Code:     "unused-local",
					Tags:     []DiagnosticTag{Unnecessary},
					Message:  "Unused local variable: '" + string(nameBytes) + "'. If this is intentional, prefix the name with an underscore (e.g., '_" + string(nameBytes) + "').",
				})
			}

			if s.DiagShadowing {
				if s.isKnownGlobal(nameBytes) {
					diagnostics = append(diagnostics, Diagnostic{
						Range:    r,
						Severity: SeverityWarning,
						Message:  "Local variable '" + string(nameBytes) + "' shadows a known global.",
					})
				} else {
					hash := ast.HashBytes(nameBytes)

					if sym, exists := s.GlobalIndex[GlobalKey{ReceiverHash: 0, PropHash: hash}]; exists {
						var related []DiagnosticRelatedInformation

						if symDoc, ok := s.Documents[sym.URI]; ok {
							sNode := symDoc.Tree.Nodes[sym.NodeID]

							sLine, sCol := symDoc.Tree.Position(sNode.Start)
							eLine, eCol := symDoc.Tree.Position(sNode.End)

							var fromFile string

							if sym.URI != uri {
								fromFile = " in " + filepath.Base(s.uriToPath(sym.URI))
							}

							related = append(related, DiagnosticRelatedInformation{
								Location: Location{
									URI: sym.URI,
									Range: Range{
										Start: Position{Line: sLine, Character: sCol},
										End:   Position{Line: eLine, Character: eCol},
									},
								},
								Message: "Global '" + string(nameBytes) + "' defined here" + fromFile,
							})
						}

						diagnostics = append(diagnostics, Diagnostic{
							Range:              r,
							Severity:           SeverityWarning,
							Message:            "Local variable '" + string(nameBytes) + "' shadows a global definition.",
							RelatedInformation: related,
						})
					}
				}
			}
		}
	}

	if s.DiagShadowing {
		for _, pair := range doc.Resolver.ShadowedOuter {
			node := doc.Tree.Nodes[pair.Shadowing]
			nameBytes := doc.Source[node.Start:node.End]

			startLine, startCol := doc.Tree.Position(node.Start)
			endLine, endCol := doc.Tree.Position(node.End)

			var related []DiagnosticRelatedInformation

			shadowedNode := doc.Tree.Nodes[pair.Shadowed]

			sLine, sCol := doc.Tree.Position(shadowedNode.Start)
			eLine, eCol := doc.Tree.Position(shadowedNode.End)

			related = append(related, DiagnosticRelatedInformation{
				Location: Location{
					URI: uri,
					Range: Range{
						Start: Position{Line: sLine, Character: sCol},
						End:   Position{Line: eLine, Character: eCol},
					},
				},
				Message: "Outer local '" + string(nameBytes) + "' defined here",
			})

			diagnostics = append(diagnostics, Diagnostic{
				Range: Range{
					Start: Position{Line: startLine, Character: startCol},
					End:   Position{Line: endLine, Character: endCol},
				},
				Severity:           SeverityWarning,
				Message:            "Local variable '" + string(nameBytes) + "' shadows a variable from an outer scope.",
				RelatedInformation: related,
			})
		}
	}

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
						sLine, sCol := doc.Tree.Position(firstExprNode.Start)

						lastExprID := doc.Tree.ExtraList[exprList.Extra+uint32(exprList.Count-1)]
						lastExprNode := doc.Tree.Nodes[lastExprID]
						eLine, eCol := doc.Tree.Position(lastExprNode.End)

						diagnostics = append(diagnostics, Diagnostic{
							Range: Range{
								Start: Position{Line: sLine, Character: sCol},
								End:   Position{Line: eLine, Character: eCol},
							},
							Severity: SeverityWarning,
							Message:  "Ambiguous return: this executes as the return value because Lua ignores newlines. Use 'return;' if you meant to leave this as unreachable code.",
						})
					}
				}
			}

			if s.DiagUnreachableCode && node.Kind == ast.KindBlock || node.Kind == ast.KindFile {
				var terminalFound bool

				for j := uint16(0); j < node.Count; j++ {
					stmtID := doc.Tree.ExtraList[node.Extra+uint32(j)]

					if terminalFound {
						lastStmtID := doc.Tree.ExtraList[node.Extra+uint32(node.Count-1)]

						startNode := doc.Tree.Nodes[stmtID]
						endNode := doc.Tree.Nodes[lastStmtID]

						sLine, sCol := doc.Tree.Position(startNode.Start)
						eLine, eCol := doc.Tree.Position(endNode.End)

						diagnostics = append(diagnostics, Diagnostic{
							Range: Range{
								Start: Position{Line: sLine, Character: sCol},
								End:   Position{Line: eLine, Character: eCol},
							},
							Severity: SeverityHint,
							Tags:     []DiagnosticTag{Unnecessary},
							Message:  "Unreachable code detected.",
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
					startLine, startCol := doc.Tree.Position(node.Start)
					endLine, endCol := doc.Tree.Position(node.End)

					diagMsg := "Use of deprecated symbol '" + ctx.DisplayName + "'"

					if msg != "" {
						diagMsg += ": " + msg
					}

					diagnostics = append(diagnostics, Diagnostic{
						Range: Range{
							Start: Position{Line: startLine, Character: startCol},
							End:   Position{Line: endLine, Character: endCol},
						},
						Severity: SeverityHint,
						Tags:     []DiagnosticTag{Deprecated},
						Message:  diagMsg,
					})
				}
			}
		}
	}

	if diagnostics == nil {
		diagnostics = []Diagnostic{}
	}

	WriteMessage(s.Writer, OutgoingNotification{
		RPC:    "2.0",
		Method: "textDocument/publishDiagnostics",
		Params: PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: diagnostics,
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
	)

	if parentID != ast.InvalidNode {
		pNode := doc.Tree.Nodes[parentID]

		isProp = (pNode.Kind == ast.KindMemberExpr || pNode.Kind == ast.KindMethodCall || pNode.Kind == ast.KindMethodName) && pNode.Right == nodeID
		isRecordKey := pNode.Kind == ast.KindRecordField && pNode.Left == nodeID

		if isProp {
			recID := pNode.Left

			displayName = string(doc.Source[doc.Tree.Nodes[recID].Start:identNode.End])
			recBytes := doc.Source[doc.Tree.Nodes[recID].Start:doc.Tree.Nodes[recID].End]

			var recDef ast.NodeID = ast.InvalidNode

			if doc.Tree.Nodes[recID].Kind == ast.KindIdent {
				recDef = doc.Resolver.References[recID]
			}

			if recDef == ast.InvalidNode {
				gKey = GlobalKey{ReceiverHash: ast.HashBytes(recBytes), PropHash: ast.HashBytes(identBytes)}
			}
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

	isGlobal := defID == ast.InvalidNode || (isProp && gKey.ReceiverHash != 0)

	ctx := &SymbolContext{
		IdentNodeID: nodeID,
		IdentName:   identName,
		DisplayName: displayName,
		IsProp:      isProp,
		GKey:        gKey,
		IsGlobal:    isGlobal,
	}

	if defID != ast.InvalidNode {
		ctx.TargetDoc = doc
		ctx.TargetDefID = defID
		ctx.TargetURI = uri

		if !ctx.IsGlobal && ctx.TargetDoc != nil && ctx.TargetDoc.ExportedGlobals != nil {
			if gKey, exported := ctx.TargetDoc.ExportedGlobals[defID]; exported {
				ctx.IsGlobal = true
				ctx.GKey = gKey
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

	addRef := func(dDoc *Document, dUri string, nodeID ast.NodeID) {
		if !includeDeclaration && dUri == ctx.TargetURI && nodeID == ctx.TargetDefID {
			return
		}

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

	if !ctx.IsGlobal && ctx.TargetDefID != ast.InvalidNode {
		for i, def := range ctx.TargetDoc.Resolver.References {
			if def == ctx.TargetDefID {
				addRef(ctx.TargetDoc, ctx.TargetURI, ast.NodeID(i))
			}
		}
	} else {
		for dUri, dDoc := range s.Documents {
			if ctx.GKey.ReceiverHash == 0 {
				for _, id := range dDoc.Resolver.GlobalDefs {
					node := dDoc.Tree.Nodes[id]

					if ast.HashBytes(dDoc.Source[node.Start:node.End]) == ctx.GKey.PropHash {
						addRef(dDoc, dUri, id)
					}
				}

				for _, id := range dDoc.Resolver.GlobalRefs {
					node := dDoc.Tree.Nodes[id]

					if ast.HashBytes(dDoc.Source[node.Start:node.End]) == ctx.GKey.PropHash {
						if dDoc.Resolver.References[id] == ast.InvalidNode {
							addRef(dDoc, dUri, id)
						}
					}
				}
			} else {
				for _, fd := range dDoc.Resolver.FieldDefs {
					if fd.ReceiverHash == ctx.GKey.ReceiverHash && fd.PropHash == ctx.GKey.PropHash {
						addRef(dDoc, dUri, fd.NodeID)
					}
				}

				for _, pf := range dDoc.Resolver.PendingFields {
					if pf.ReceiverHash == ctx.GKey.ReceiverHash && pf.PropHash == ctx.GKey.PropHash {
						if dDoc.Resolver.References[pf.PropNodeID] == ast.InvalidNode {
							addRef(dDoc, dUri, pf.PropNodeID)
						}
					}
				}
			}
		}
	}

	if locations == nil {
		locations = []Location{}
	}

	return locations
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
	if existing, exists := s.GlobalIndex[key]; exists {
		if depth > existing.Depth {
			return
		}
	}

	s.GlobalIndex[key] = GlobalSymbol{
		URI:    uri,
		NodeID: nodeID,
		Depth:  depth,
		Name:   name,
	}

	if doc, ok := s.Documents[uri]; ok {
		if doc.ExportedGlobals == nil {
			doc.ExportedGlobals = make(map[ast.NodeID]GlobalKey)
		}

		doc.ExportedGlobals[nodeID] = key
	}
}

func (s *Server) indexEmbeddedStdlib() {
	entries, err := stdlibFS.ReadDir("stdlib")
	if err != nil {
		s.Log.Warnln("No embedded stdlib found or stdlib folder missing")

		return
	}

	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".lua") {
			b, err := stdlibFS.ReadFile("stdlib/" + e.Name())
			if err == nil {
				uri := "std:///" + e.Name()

				s.updateDocument(uri, b)
			}
		}
	}

	s.Log.Println("Embedded stdlib indexed")
}

func (s *Server) isIgnored(fullPath, name string) bool {
	for _, g := range s.IgnoreGlobs {
		if matched, _ := filepath.Match(g, name); matched {
			return true
		}

		cleanGlob := strings.TrimPrefix(strings.TrimPrefix(g, "**/"), "*/")
		cleanGlob = strings.TrimSuffix(strings.TrimSuffix(cleanGlob, "/**"), "/*")

		if cleanGlob == "" {
			continue
		}

		cleanPath := filepath.FromSlash(cleanGlob)

		if strings.Contains(fullPath, string(filepath.Separator)+cleanPath+string(filepath.Separator)) || strings.HasSuffix(fullPath, string(filepath.Separator)+cleanPath) {
			return true
		}
	}

	return false
}

func (s *Server) clearDocument(uri string) {
	delete(s.Documents, uri)

	for key, sym := range s.GlobalIndex {
		if sym.URI == uri {
			delete(s.GlobalIndex, key)
		}
	}

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

	norm := s.pathToURI(s.uriToPath(uri))

	if runtime.GOOS == "windows" {
		return strings.ToLower(norm)
	}

	return norm
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

		hasElse := false

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
