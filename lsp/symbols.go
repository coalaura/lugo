package lsp

import (
	"encoding/json"
	"iter"
	"os"
	"path/filepath"
	"strings"

	"github.com/coalaura/lugo/ast"
)

var luaKeywords = []string{
	"and", "break", "do", "else", "elseif", "end", "false", "for", "function",
	"goto", "if", "in", "local", "nil", "not", "or", "repeat", "return",
	"then", "true", "until", "while",
}

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

type RefKey struct {
	URI string
	ID  ast.NodeID
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
	GlobalDefs  []GlobalSymbol
}

func (s *Server) handleDefinition(req Request) {
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

	if ctx != nil {
		var locs []Location

		if len(ctx.GlobalDefs) > 0 {
			for _, def := range ctx.GlobalDefs {
				if tDoc, ok := s.Documents[def.URI]; ok {
					locs = append(locs, Location{
						URI:   def.URI,
						Range: getNodeRange(tDoc.Tree, def.NodeID),
					})
				}
			}
		} else if ctx.TargetDefID != ast.InvalidNode {
			locs = append(locs, Location{
				URI:   ctx.TargetURI,
				Range: getNodeRange(ctx.TargetDoc.Tree, ctx.TargetDefID),
			})
		}

		if len(locs) > 0 {
			WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: locs})

			return
		}
	}

	WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})
}

func (s *Server) handleReferences(req Request) {
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
}

func (s *Server) handleDocumentSymbol(req Request) {
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

	var (
		walkTable func(tableID ast.NodeID) []DocumentSymbol
		walk      func(nodeID ast.NodeID) []DocumentSymbol
	)

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
					children = walk(valNode.Right)
				case ast.KindTableExpr:
					kind = SymbolKindClass
					children = walkTable(fieldNode.Right)
				case ast.KindCallExpr, ast.KindMethodCall:
					children = walk(fieldNode.Right)
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

			var children []DocumentSymbol

			if node.Right != ast.InvalidNode {
				funcExpr := doc.Tree.Nodes[node.Right]
				if funcExpr.Kind == ast.KindFunctionExpr {
					children = walk(funcExpr.Right)
				}
			}

			syms = append(syms, DocumentSymbol{
				Name:           name,
				Kind:           kind,
				Range:          getNodeRange(doc.Tree, nodeID),
				SelectionRange: getNodeRange(doc.Tree, node.Left),
				Children:       children,
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

					switch rNode.Kind {
					case ast.KindFunctionExpr:
						syms = append(syms, DocumentSymbol{
							Name:           name,
							Kind:           SymbolKindFunction,
							Range:          getNodeRange(doc.Tree, nodeID),
							SelectionRange: getNodeRange(doc.Tree, lID),
							Children:       walk(rNode.Right),
						})
					case ast.KindTableExpr:
						syms = append(syms, DocumentSymbol{
							Name:           name,
							Kind:           SymbolKindClass,
							Range:          getNodeRange(doc.Tree, nodeID),
							SelectionRange: getNodeRange(doc.Tree, lID),
							Children:       walkTable(rID),
						})
					default:
						if node.Kind == ast.KindLocalAssign {
							syms = append(syms, DocumentSymbol{
								Name:           name,
								Kind:           SymbolKindVariable,
								Range:          getNodeRange(doc.Tree, lID),
								SelectionRange: getNodeRange(doc.Tree, lID),
							})
						}

						if rNode.Kind == ast.KindCallExpr || rNode.Kind == ast.KindMethodCall {
							syms = append(syms, walk(rID)...)
						}
					}
				}
			}
		case ast.KindCallExpr, ast.KindMethodCall:
			var (
				funcName    string
				funcIdentID ast.NodeID
			)

			if node.Kind == ast.KindMethodCall {
				funcIdentID = node.Right
				if int(node.Left) < len(doc.Tree.Nodes) && int(node.Right) < len(doc.Tree.Nodes) {
					leftNode := doc.Tree.Nodes[node.Left]
					rightNode := doc.Tree.Nodes[node.Right]

					if leftNode.Start <= rightNode.End && rightNode.End <= uint32(len(doc.Source)) {
						funcName = ast.String(doc.Source[leftNode.Start:rightNode.End])
					}
				}
			} else {
				if int(node.Left) < len(doc.Tree.Nodes) {
					leftNode := doc.Tree.Nodes[node.Left]
					if leftNode.Start <= leftNode.End && leftNode.End <= uint32(len(doc.Source)) {
						funcName = ast.String(doc.Source[leftNode.Start:leftNode.End])
					}

					if leftNode.Kind == ast.KindMemberExpr {
						funcIdentID = leftNode.Right
					} else {
						funcIdentID = node.Left
					}
				}
			}

			var (
				targetFuncID ast.NodeID = ast.InvalidNode
				targetDoc    *Document
				paramOffset  int
			)

			if funcIdentID != ast.InvalidNode && int(funcIdentID) < len(doc.Tree.Nodes) {
				ctx := s.resolveSymbolNode(uri, doc, funcIdentID)
				if ctx != nil && ctx.TargetDefID != ast.InvalidNode && ctx.TargetDoc != nil {
					valID := ctx.TargetDoc.getAssignedValue(ctx.TargetDefID)
					if valID != ast.InvalidNode && int(valID) < len(ctx.TargetDoc.Tree.Nodes) {
						if ctx.TargetDoc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
							targetFuncID = valID
							targetDoc = ctx.TargetDoc

							hasImplicitSelfCall := node.Kind == ast.KindMethodCall

							var hasImplicitSelfDef bool

							pDefID := ctx.TargetDoc.Tree.Nodes[ctx.TargetDefID].Parent
							if pDefID != ast.InvalidNode && int(pDefID) < len(ctx.TargetDoc.Tree.Nodes) {
								if ctx.TargetDoc.Tree.Nodes[pDefID].Kind == ast.KindMethodName {
									hasImplicitSelfDef = true
								}
							}

							if hasImplicitSelfCall && !hasImplicitSelfDef {
								paramOffset = 1
							} else if !hasImplicitSelfCall && hasImplicitSelfDef {
								paramOffset = -1
							}
						}
					}
				}
			}

			for i := uint16(0); i < node.Count; i++ {
				argID := doc.Tree.ExtraList[node.Extra+uint32(i)]
				argNode := doc.Tree.Nodes[argID]

				switch argNode.Kind {
				case ast.KindFunctionExpr:
					paramName := "callback"

					// Attempt to map the argument back to the function's parameter list
					if targetFuncID != ast.InvalidNode && targetDoc != nil {
						targetFuncNode := targetDoc.Tree.Nodes[targetFuncID]
						paramIdx := int(i) + paramOffset
						if paramIdx >= 0 && paramIdx < int(targetFuncNode.Count) {
							if targetFuncNode.Extra+uint32(paramIdx) < uint32(len(targetDoc.Tree.ExtraList)) {
								pID := targetDoc.Tree.ExtraList[targetFuncNode.Extra+uint32(paramIdx)]
								if int(pID) < len(targetDoc.Tree.Nodes) {
									pNode := targetDoc.Tree.Nodes[pID]
									if pNode.Start <= pNode.End && pNode.End <= uint32(len(targetDoc.Source)) {
										pNameStr := ast.String(targetDoc.Source[pNode.Start:pNode.End])
										if pNameStr != "" && pNameStr != "..." {
											paramName = pNameStr
										}
									}
								}
							}
						}
					}

					name := "(anonymous function)"
					if funcName != "" {
						name = paramName + " in " + funcName
					}

					var selRange Range

					if argNode.Start+8 <= argNode.End {
						selRange = getRange(doc.Tree, argNode.Start, argNode.Start+8)
					} else {
						selRange = getNodeRange(doc.Tree, argID)
					}

					syms = append(syms, DocumentSymbol{
						Name:           name,
						Kind:           SymbolKindFunction,
						Range:          getNodeRange(doc.Tree, argID),
						SelectionRange: selRange,
						Children:       walk(argNode.Right),
					})
				case ast.KindCallExpr, ast.KindMethodCall:
					syms = append(syms, walk(argID)...)
				case ast.KindTableExpr:
					syms = append(syms, walkTable(argID)...)
				}
			}
		case ast.KindReturn:
			if node.Left != ast.InvalidNode {
				exprList := doc.Tree.Nodes[node.Left]

				for i := uint16(0); i < exprList.Count; i++ {
					exprID := doc.Tree.ExtraList[exprList.Extra+uint32(i)]
					exprNode := doc.Tree.Nodes[exprID]

					switch exprNode.Kind {
					case ast.KindFunctionExpr:
						var selRange Range

						if exprNode.Start+8 <= exprNode.End {
							selRange = getRange(doc.Tree, exprNode.Start, exprNode.Start+8)
						} else {
							selRange = getNodeRange(doc.Tree, exprID)
						}

						syms = append(syms, DocumentSymbol{
							Name:           "(return function)",
							Kind:           SymbolKindFunction,
							Range:          getNodeRange(doc.Tree, exprID),
							SelectionRange: selRange,
							Children:       walk(exprNode.Right),
						})
					case ast.KindCallExpr, ast.KindMethodCall:
						syms = append(syms, walk(exprID)...)
					}
				}
			}
		case ast.KindIf:
			syms = append(syms, walk(node.Right)...)

			for i := uint16(0); i < node.Count; i++ {
				syms = append(syms, walk(doc.Tree.ExtraList[node.Extra+uint32(i)])...)
			}
		case ast.KindElseIf, ast.KindWhile, ast.KindForIn, ast.KindForNum:
			syms = append(syms, walk(node.Right)...)
		case ast.KindElse, ast.KindRepeat, ast.KindDo:
			syms = append(syms, walk(node.Left)...)
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
}

func (s *Server) handleWorkspaceSymbol(req Request) {
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

	for key, syms := range s.GlobalIndex {
		for _, sym := range syms {
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
	if nodeID == ast.InvalidNode || int(nodeID) >= len(doc.Tree.Nodes) {
		return nil
	}

	identNode := doc.Tree.Nodes[nodeID]

	if identNode.Kind != ast.KindIdent && identNode.Kind != ast.KindVararg {
		return nil
	}

	if identNode.Start > identNode.End || identNode.End > uint32(len(doc.Source)) {
		return nil
	}

	identBytes := doc.Source[identNode.Start:identNode.End]
	identName := ast.String(identBytes)

	displayName := identName
	if displayName == "" {
		displayName = "<error>"
	}

	defID := doc.Resolver.References[nodeID]
	parentID := identNode.Parent

	var (
		gKey   GlobalKey
		isProp bool
		recDef ast.NodeID = ast.InvalidNode
	)

	if parentID != ast.InvalidNode && int(parentID) < len(doc.Tree.Nodes) {
		pNode := doc.Tree.Nodes[parentID]

		isProp = (pNode.Kind == ast.KindMemberExpr || pNode.Kind == ast.KindMethodCall || pNode.Kind == ast.KindMethodName) && pNode.Right == nodeID
		isRecordKey := pNode.Kind == ast.KindRecordField && pNode.Left == nodeID

		if isProp {
			recID := pNode.Left

			if recID != ast.InvalidNode && int(recID) < len(doc.Tree.Nodes) {
				recNode := doc.Tree.Nodes[recID]

				if recNode.Start <= identNode.End && identNode.End <= uint32(len(doc.Source)) {
					displayName = ast.String(doc.Source[recNode.Start:identNode.End])
				}

				if recNode.Start <= recNode.End && recNode.End <= uint32(len(doc.Source)) {
					recBytes := doc.Source[recNode.Start:recNode.End]
					gKey = GlobalKey{ReceiverHash: ast.HashBytes(recBytes), PropHash: ast.HashBytes(identBytes)}
				}

				curr := recID

				for curr != ast.InvalidNode && int(curr) < len(doc.Tree.Nodes) {
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

				var modName string

				if recDef != ast.InvalidNode {
					valID := doc.getAssignedValue(recDef)
					modName = s.getRequireModName(doc, valID)
				} else {
					modName = s.getRequireModName(doc, recID)
				}

				if modName != "" {
					targetDoc := s.resolveModule(uri, modName)
					if targetDoc != nil {
						gKey.ReceiverHash = ast.HashBytesConcat([]byte("module:"), nil, []byte(targetDoc.URI))
					}
				}
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

	var isModuleAccess bool

	if gKey.ReceiverHash != 0 {
		if recDef != ast.InvalidNode {
			valID := doc.getAssignedValue(recDef)

			if s.getRequireModName(doc, valID) != "" {
				isModuleAccess = true
			}
		} else if isProp {
			if s.getRequireModName(doc, doc.Tree.Nodes[parentID].Left) != "" {
				isModuleAccess = true
			}
		}
	}

	isGlobal := (defID == ast.InvalidNode && recDef == ast.InvalidNode && (!isProp || gKey.ReceiverHash != 0)) || isModuleAccess

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

		if !ctx.IsGlobal && ctx.TargetDoc != nil && ctx.TargetDoc.ExportedGlobalDefs != nil {
			if exportedKey, exported := ctx.TargetDoc.ExportedGlobalDefs[defID]; exported {
				ctx.IsGlobal = true
				ctx.GKey = exportedKey

				if gSyms, ok := s.getGlobalSymbols(uri, ctx.GKey.ReceiverHash, ctx.GKey.PropHash); ok {
					bestDefs := s.getBestDefsForContext(doc, nodeID, gSyms)
					ctx.GlobalDefs = bestDefs
				}
			}
		}
	} else if gKey.PropHash != 0 {
		if gSyms, ok := s.getGlobalSymbols(uri, gKey.ReceiverHash, gKey.PropHash); ok {
			bestDefs := s.getBestDefsForContext(doc, nodeID, gSyms)

			ctx.GlobalDefs = bestDefs

			if len(bestDefs) > 0 {
				if gDoc, docOk := s.Documents[bestDefs[0].URI]; docOk {
					ctx.TargetDoc = gDoc
					ctx.TargetDefID = bestDefs[0].NodeID
					ctx.TargetURI = bestDefs[0].URI
				}
			}
		}
	}

	return ctx
}

func (s *Server) getBestDefsForContext(doc *Document, identNodeID ast.NodeID, defs []GlobalSymbol) []GlobalSymbol {
	if len(defs) <= 1 {
		return defs
	}

	var (
		activeCallArgs int = -1
		isMethodCall   bool
	)

	pID := doc.Tree.Nodes[identNodeID].Parent
	if pID != ast.InvalidNode && int(pID) < len(doc.Tree.Nodes) {
		pNode := doc.Tree.Nodes[pID]
		if pNode.Kind == ast.KindCallExpr && pNode.Left == identNodeID {
			activeCallArgs = int(pNode.Count)
		} else if pNode.Kind == ast.KindMethodCall && pNode.Right == identNodeID {
			activeCallArgs = int(pNode.Count)
			isMethodCall = true
		} else if pNode.Kind == ast.KindMemberExpr {
			gpID := pNode.Parent
			if gpID != ast.InvalidNode && int(gpID) < len(doc.Tree.Nodes) {
				gpNode := doc.Tree.Nodes[gpID]
				if gpNode.Kind == ast.KindCallExpr && gpNode.Left == pID {
					activeCallArgs = int(gpNode.Count)
				}
			}
		}
	}

	if activeCallArgs >= 0 {
		var (
			bestDefs  []GlobalSymbol
			bestScore int = -1
		)

		for _, def := range defs {
			tDoc := s.Documents[def.URI]
			if tDoc == nil {
				continue
			}

			valID := tDoc.getAssignedValue(def.NodeID)
			if valID != ast.InvalidNode && int(valID) < len(tDoc.Tree.Nodes) && tDoc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
				funcNode := tDoc.Tree.Nodes[valID]

				var hasImplicitSelfDef bool

				pDefID := tDoc.Tree.Nodes[def.NodeID].Parent
				if pDefID != ast.InvalidNode && int(pDefID) < len(tDoc.Tree.Nodes) && tDoc.Tree.Nodes[pDefID].Kind == ast.KindMethodName {
					hasImplicitSelfDef = true
				}

				var paramOffset int

				if isMethodCall && !hasImplicitSelfDef {
					paramOffset = 1
				} else if !isMethodCall && hasImplicitSelfDef {
					paramOffset = -1
				}

				expectedArgs := int(funcNode.Count) - paramOffset
				if expectedArgs < 0 {
					expectedArgs = 0
				}

				var score int

				if expectedArgs == activeCallArgs {
					score = 2
				} else if expectedArgs > activeCallArgs {
					score = 1
				}

				if funcNode.Count > 0 {
					lastParamID := tDoc.Tree.ExtraList[funcNode.Extra+uint32(funcNode.Count-1)]
					if tDoc.Tree.Nodes[lastParamID].Kind == ast.KindVararg {
						if activeCallArgs >= expectedArgs-1 {
							score = 2
						}
					}
				}

				if score > bestScore {
					bestScore = score
					bestDefs = []GlobalSymbol{def}
				} else if score == bestScore {
					bestDefs = append(bestDefs, def)
				}
			}
		}

		if len(bestDefs) > 0 {
			return bestDefs
		}
	}

	return defs
}

func (s *Server) getReferences(ctx *SymbolContext, includeDeclaration bool) []Location {
	var locations []Location

	seen := make(map[RefKey]bool)

	addRef := func(dDoc *Document, dUri string, nodeID ast.NodeID) {
		if !includeDeclaration && dUri == ctx.TargetURI && nodeID == ctx.TargetDefID {
			return
		}

		if nodeID == ast.InvalidNode || int(nodeID) >= len(dDoc.Tree.Nodes) {
			return
		}

		rk := RefKey{URI: dUri, ID: nodeID}

		if seen[rk] {
			return
		}

		seen[rk] = true

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

func (s *Server) iterateGlobalReferences(ctx *SymbolContext) iter.Seq[GlobalReference] {
	return func(yield func(GlobalReference) bool) {
		if !ctx.IsGlobal {
			return
		}

		for dUri, dDoc := range s.Documents {
			if !s.canSeeSymbol(dUri, ctx.TargetURI) {
				continue
			}

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

func (s *Server) getGlobalSymbols(sourceURI string, recHash, propHash uint64) ([]GlobalSymbol, bool) {
	currRec := recHash

	for range 10 {
		key := GlobalKey{ReceiverHash: currRec, PropHash: propHash}
		if syms, exists := s.GlobalIndex[key]; exists && len(syms) > 0 {
			var filtered []GlobalSymbol

			for _, sym := range syms {
				if s.canSeeSymbol(sourceURI, sym.URI) {
					filtered = append(filtered, sym)
				}
			}

			if len(filtered) > 0 {
				return filtered, true
			}
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

	return nil, false
}

func (s *Server) setGlobalSymbol(key GlobalKey, uri string, nodeID ast.NodeID, depth int, name string) {
	if doc, ok := s.Documents[uri]; ok {
		if doc.ExportedGlobalDefs == nil {
			doc.ExportedGlobalDefs = make(map[ast.NodeID]GlobalKey)
		}

		doc.ExportedGlobalDefs[nodeID] = key
	}

	syms := s.GlobalIndex[key]

	for i, sym := range syms {
		if sym.URI == uri && sym.NodeID == nodeID {
			syms[i].Depth = depth
			syms[i].Name = name
			return
		}
	}

	s.GlobalIndex[key] = append(syms, GlobalSymbol{
		URI:    uri,
		NodeID: nodeID,
		Depth:  depth,
		Name:   name,
	})
}

func (s *Server) removeDocumentGlobals(uri string, doc *Document) {
	if doc.ExportedGlobalDefs == nil {
		return
	}

	for nodeID, key := range doc.ExportedGlobalDefs {
		if syms, ok := s.GlobalIndex[key]; ok {
			var newSyms []GlobalSymbol

			for _, sym := range syms {
				if sym.URI != uri || sym.NodeID != nodeID {
					newSyms = append(newSyms, sym)
				}
			}

			if len(newSyms) > 0 {
				s.GlobalIndex[key] = newSyms
			} else {
				delete(s.GlobalIndex, key)
			}
		}
	}
}

func (s *Server) getGlobalAlias(hash uint64) uint64 {
	syms, ok := s.GlobalIndex[GlobalKey{ReceiverHash: 0, PropHash: hash}]
	if !ok || len(syms) == 0 {
		return 0
	}

	sym := syms[0]

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

func (s *Server) getGlobalPath(doc *Document, id ast.NodeID, depth int) []byte {
	if id == ast.InvalidNode || int(id) >= len(doc.Tree.Nodes) || depth > 10 {
		return nil
	}

	node := doc.Tree.Nodes[id]

	switch node.Kind {
	case ast.KindIdent:
		defID := doc.Resolver.References[id]
		if defID == ast.InvalidNode {
			if node.Start <= node.End && node.End <= uint32(len(doc.Source)) {
				return doc.Source[node.Start:node.End]
			}
			return nil
		}

		valID := doc.getAssignedValue(defID)
		if valID != ast.InvalidNode && valID != id {
			return s.getGlobalPath(doc, valID, depth+1)
		}

		return nil
	case ast.KindMemberExpr:
		leftPath := s.getGlobalPath(doc, node.Left, depth+1)
		if leftPath != nil {
			if node.Right == ast.InvalidNode || int(node.Right) >= len(doc.Tree.Nodes) {
				return nil
			}

			rightNode := doc.Tree.Nodes[node.Right]

			if rightNode.Start <= rightNode.End && rightNode.End <= uint32(len(doc.Source)) {
				rightBytes := doc.Source[rightNode.Start:rightNode.End]

				buf := make([]byte, 0, len(leftPath)+1+len(rightBytes))
				buf = append(buf, leftPath...)
				buf = append(buf, '.')
				buf = append(buf, rightBytes...)

				return buf
			}
		}
	}

	return nil
}

func (s *Server) suggestGlobal(sourceURI string, name string) string {
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
	for key, syms := range s.GlobalIndex {
		if key.ReceiverHash == 0 && len(syms) > 0 {
			for _, sym := range syms {
				if s.canSeeSymbol(sourceURI, sym.URI) {
					check(sym.Name)

					break
				}
			}
		}
	}

	return bestMatch
}

func (s *Server) getResourceRoot(uri string) string {
	if !s.FeatureFiveM {
		return ""
	}

	if strings.HasPrefix(uri, "std://") {
		return "std"
	}

	if root, ok := s.resourceCache[uri]; ok {
		return root
	}

	// First check if it matches any known parsed FiveMResource
	var bestRoot string

	for root := range s.FiveMResources {
		if strings.HasPrefix(uri, root+"/") || uri == root {
			if len(root) > len(bestRoot) {
				bestRoot = root
			}
		}
	}

	if bestRoot != "" {
		s.resourceCache[uri] = bestRoot
		return bestRoot
	}

	// Fallback to disk scan (for files opened outside the indexed workspace)
	path := s.uriToPath(uri)
	if path == "" {
		return ""
	}

	dir := filepath.Dir(path)

	for {
		if _, err := os.Stat(filepath.Join(dir, "fxmanifest.lua")); err == nil {
			res := s.pathToURI(dir)
			s.resourceCache[uri] = res

			return res
		}

		if _, err := os.Stat(filepath.Join(dir, "__resource.lua")); err == nil {
			res := s.pathToURI(dir)
			s.resourceCache[uri] = res

			return res
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	s.resourceCache[uri] = ""

	return ""
}

func (s *Server) canSeeSymbol(sourceURI, targetURI string) bool {
	if !s.FeatureFiveM {
		return true
	}

	if sourceURI == targetURI {
		return true
	}

	if strings.HasPrefix(targetURI, "std://") {
		return true
	}

	targetPath := strings.ToLower(s.uriToPath(targetURI))

	for _, lib := range s.lowerLibraryPaths {
		if strings.HasPrefix(targetPath, lib) {
			return true
		}
	}

	srcRoot := s.getResourceRoot(sourceURI)
	tgtRoot := s.getResourceRoot(targetURI)

	if srcRoot == "" && tgtRoot == "" {
		return true
	}

	// Same resource check (with Client/Server separation)
	if srcRoot == tgtRoot {
		srcRes := s.FiveMResources[srcRoot]
		if srcRes != nil {
			srcEnv := s.getFileEnv(srcRes, sourceURI)
			tgtEnv := s.getFileEnv(srcRes, targetURI)

			srcIsManifest := strings.HasSuffix(sourceURI, "/fxmanifest.lua") || strings.HasSuffix(sourceURI, "/__resource.lua")
			tgtIsManifest := strings.HasSuffix(targetURI, "/fxmanifest.lua") || strings.HasSuffix(targetURI, "/__resource.lua")

			// Isolate unaccounted files
			if (!srcIsManifest && srcEnv == EnvUnknown) || (!tgtIsManifest && tgtEnv == EnvUnknown) {
				return false
			}

			if srcEnv == EnvClient && tgtEnv == EnvServer {
				return false
			}

			if srcEnv == EnvServer && tgtEnv == EnvClient {
				return false
			}
		}

		return true
	}

	// Cross-resource `@resource/file.lua` check
	srcRes := s.FiveMResources[srcRoot]
	if srcRes == nil {
		return false
	}

	tgtRes := s.FiveMResources[tgtRoot]

	var (
		tgtName string
		relPath string
	)

	if tgtRes != nil {
		tgtName = tgtRes.Name
		if len(targetURI) > len(tgtRes.RootURI) {
			relPath = targetURI[len(tgtRes.RootURI)+1:]
		}
	} else if tgtRoot != "" {
		parts := strings.Split(tgtRoot, "/")

		tgtName = parts[len(parts)-1]
		if len(targetURI) > len(tgtRoot) {
			relPath = targetURI[len(tgtRoot)+1:]
		}
	}

	if tgtName != "" && relPath != "" {
		expectedInclude := "@" + tgtName + "/" + relPath

		srcEnv := s.getFileEnv(srcRes, sourceURI)

		var allowedIncludes []string

		allowedIncludes = append(allowedIncludes, srcRes.SharedCrossIncludes...)

		if srcEnv == EnvClient || srcEnv == EnvUnknown {
			allowedIncludes = append(allowedIncludes, srcRes.ClientCrossIncludes...)
		}

		if srcEnv == EnvServer || srcEnv == EnvUnknown {
			allowedIncludes = append(allowedIncludes, srcRes.ServerCrossIncludes...)
		}

		for _, inc := range allowedIncludes {
			if strings.EqualFold(inc, expectedInclude) {
				return true
			}

			if strings.Contains(inc, "*") {
				prefix := "@" + tgtName + "/"

				if len(inc) >= len(prefix) && strings.EqualFold(inc[:len(prefix)], prefix) {
					globPattern := inc[len(prefix):]

					if matchFiveMGlob(globPattern, relPath) {
						return true
					}
				}
			}
		}
	}

	return false
}

func (s *Server) isKnownGlobal(name []byte) bool {
	if s.KnownGlobals[ast.String(name)] {
		return true
	}

	if len(s.KnownGlobalGlobs) == 0 {
		return false
	}

	strName := ast.String(name)

	for _, glob := range s.KnownGlobalGlobs {
		if matched, _ := filepath.Match(glob, strName); matched {
			return true
		}
	}

	return false
}
