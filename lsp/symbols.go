package lsp

import (
	"bytes"
	"encoding/json"
	"iter"
	"strings"

	"github.com/coalaura/lugo/ast"
)

var luaKeywords = []string{
	"and", "break", "do", "else", "elseif", "end", "false", "for", "function",
	"goto", "if", "in", "local", "nil", "not", "or", "repeat", "return",
	"then", "true", "until", "while",
}

type GlobalSymbol struct {
	URI           string
	Name          string
	Parent        string
	NodeID        ast.NodeID
	IsRoot        bool
	IsDeprecated  bool
	DeprecatedMsg string
}

type ExportedSymbol struct {
	NodeID        ast.NodeID
	Key           GlobalKey
	IsRoot        bool
	IsDeprecated  bool
	DeprecatedMsg string
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
	GlobalDefs  []GlobalSymbol
	GKey        GlobalKey
	IdentNodeID ast.NodeID
	TargetDefID ast.NodeID
	RecDefID    ast.NodeID
	IsProp      bool
	IsGlobal    bool
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
		walkTable func(tableID ast.NodeID, out *[]DocumentSymbol)
		walk      func(nodeID ast.NodeID, out *[]DocumentSymbol)
	)

	walkTable = func(tableID ast.NodeID, out *[]DocumentSymbol) {
		node := doc.Tree.Nodes[tableID]

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
					walk(valNode.Right, &children)
				case ast.KindTableExpr:
					kind = SymbolKindClass
					walkTable(fieldNode.Right, &children)
				case ast.KindCallExpr, ast.KindMethodCall:
					walk(fieldNode.Right, &children)
				}

				*out = append(*out, DocumentSymbol{
					Name:           name,
					Kind:           kind,
					Range:          getNodeRange(doc.Tree, fieldID),
					SelectionRange: getNodeRange(doc.Tree, fieldNode.Left),
					Children:       children,
				})
			}
		}
	}

	walk = func(nodeID ast.NodeID, out *[]DocumentSymbol) {
		if nodeID == ast.InvalidNode {
			return
		}

		node := doc.Tree.Nodes[nodeID]

		switch node.Kind {
		case ast.KindFile:
			walk(node.Left, out)
		case ast.KindBlock:
			for i := uint16(0); i < node.Count; i++ {
				walk(doc.Tree.ExtraList[node.Extra+uint32(i)], out)
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
					walk(funcExpr.Right, &children)
				}
			}

			*out = append(*out, DocumentSymbol{
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
						var children []DocumentSymbol

						walk(rNode.Right, &children)

						*out = append(*out, DocumentSymbol{
							Name:           name,
							Kind:           SymbolKindFunction,
							Range:          getNodeRange(doc.Tree, nodeID),
							SelectionRange: getNodeRange(doc.Tree, lID),
							Children:       children,
						})
					case ast.KindTableExpr:
						var children []DocumentSymbol

						walkTable(rID, &children)

						*out = append(*out, DocumentSymbol{
							Name:           name,
							Kind:           SymbolKindClass,
							Range:          getNodeRange(doc.Tree, nodeID),
							SelectionRange: getNodeRange(doc.Tree, lID),
							Children:       children,
						})
					default:
						if node.Kind == ast.KindLocalAssign {
							*out = append(*out, DocumentSymbol{
								Name:           name,
								Kind:           SymbolKindVariable,
								Range:          getNodeRange(doc.Tree, lID),
								SelectionRange: getNodeRange(doc.Tree, lID),
							})
						}

						if rNode.Kind == ast.KindCallExpr || rNode.Kind == ast.KindMethodCall {
							walk(rID, out)
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

							paramOffset = getImplicitSelfOffset(node, ctx.TargetDoc, ctx.TargetDefID)
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

					var children []DocumentSymbol

					walk(argNode.Right, &children)

					*out = append(*out, DocumentSymbol{
						Name:           name,
						Kind:           SymbolKindFunction,
						Range:          getNodeRange(doc.Tree, argID),
						SelectionRange: selRange,
						Children:       children,
					})
				case ast.KindCallExpr, ast.KindMethodCall:
					walk(argID, out)
				case ast.KindTableExpr:
					walkTable(argID, out)
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

						var children []DocumentSymbol

						walk(exprNode.Right, &children)

						*out = append(*out, DocumentSymbol{
							Name:           "(return function)",
							Kind:           SymbolKindFunction,
							Range:          getNodeRange(doc.Tree, exprID),
							SelectionRange: selRange,
							Children:       children,
						})
					case ast.KindCallExpr, ast.KindMethodCall:
						walk(exprID, out)
					}
				}
			}
		case ast.KindIf:
			walk(node.Right, out)

			for i := uint16(0); i < node.Count; i++ {
				walk(doc.Tree.ExtraList[node.Extra+uint32(i)], out)
			}
		case ast.KindElseIf, ast.KindWhile, ast.KindForIn, ast.KindForNum:
			walk(node.Right, out)
		case ast.KindElse, ast.KindRepeat, ast.KindDo:
			walk(node.Left, out)
		}
	}

	var symbols []DocumentSymbol

	walk(doc.Tree.Root, &symbols)

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

	queryLower := strings.ToLower(params.Query)

	var (
		results []SymbolInformation
		count   int
	)

	for key, syms := range s.GlobalIndex {
		for _, sym := range syms {
			if !containsFold(sym.Name, queryLower) {
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

	if defID == ast.InvalidNode && identName == "self" {
		isGlobal = false

		for name, id := range doc.LocalsAt(identNode.Start) {
			if bytes.Equal(name, []byte("self")) {
				defID = id

				break
			}
		}
	}

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

		if !ctx.IsGlobal && ctx.TargetDoc != nil {
			for _, exp := range ctx.TargetDoc.ExportedGlobalDefs {
				if exp.NodeID == defID {
					ctx.IsGlobal = true
					ctx.GKey = exp.Key

					if gSyms, ok := s.getGlobalSymbols(doc, ctx.GKey.ReceiverHash, ctx.GKey.PropHash); ok {
						bestDefs := s.getBestDefsForContext(doc, nodeID, gSyms)
						ctx.GlobalDefs = bestDefs
					}
					break
				}
			}
		}
	} else if isProp {
		pID := doc.Tree.Nodes[nodeID].Parent
		if pID != ast.InvalidNode && int(pID) < len(doc.Tree.Nodes) {
			pNode := doc.Tree.Nodes[pID]

			var propType TypeSet

			switch pNode.Kind {
			case ast.KindMemberExpr:
				propType = doc.InferType(pID)
			case ast.KindMethodCall, ast.KindMethodName:
				propType = doc.inferMemberExpr(pNode)
			}

			if propType.DeclNode != ast.InvalidNode && propType.DeclURI != "" {
				if targetDoc, ok := s.Documents[propType.DeclURI]; ok {
					ctx.TargetDoc = targetDoc
					ctx.TargetURI = propType.DeclURI

					defForVal := targetDoc.getDefForValue(propType.DeclNode)
					if defForVal != ast.InvalidNode {
						ctx.TargetDefID = defForVal
					} else {
						ctx.TargetDefID = propType.DeclNode
					}
				}
			}
		}
	}

	if ctx.TargetDefID == ast.InvalidNode && gKey.PropHash != 0 {
		if gSyms, ok := s.getGlobalSymbols(doc, gKey.ReceiverHash, gKey.PropHash); ok {
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

	parentID := doc.Tree.Nodes[identNodeID].Parent
	if parentID != ast.InvalidNode && int(parentID) < len(doc.Tree.Nodes) {
		parentNode := doc.Tree.Nodes[parentID]
		if parentNode.Kind == ast.KindCallExpr && parentNode.Left == identNodeID {
			activeCallArgs = int(parentNode.Count)
		} else if parentNode.Kind == ast.KindMethodCall && parentNode.Right == identNodeID {
			activeCallArgs = int(parentNode.Count)
			isMethodCall = true
		} else if parentNode.Kind == ast.KindMemberExpr {
			grandParentID := parentNode.Parent
			if grandParentID != ast.InvalidNode && int(grandParentID) < len(doc.Tree.Nodes) {
				grandParentNode := doc.Tree.Nodes[grandParentID]
				if grandParentNode.Kind == ast.KindCallExpr && grandParentNode.Left == parentID {
					activeCallArgs = int(grandParentNode.Count)
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

				var paramOffset int

				if isMethodCall {
					paramOffset = getImplicitSelfOffset(ast.Node{Kind: ast.KindMethodCall}, tDoc, def.NodeID)
				} else {
					paramOffset = getImplicitSelfOffset(ast.Node{Kind: ast.KindCallExpr}, tDoc, def.NodeID)
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
			if !s.canSeeSymbol(dDoc, ctx.TargetDoc) {
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

func (s *Server) getGlobalSymbols(srcDoc *Document, recHash, propHash uint64) ([]GlobalSymbol, bool) {
	currRec := recHash

	for range 10 {
		key := GlobalKey{ReceiverHash: currRec, PropHash: propHash}
		if syms, exists := s.GlobalIndex[key]; exists && len(syms) > 0 {
			var filtered []GlobalSymbol

			for _, sym := range syms {
				if tgtDoc, ok := s.Documents[sym.URI]; ok && s.canSeeSymbol(srcDoc, tgtDoc) {
					filtered = append(filtered, sym)
					if len(filtered) >= 10 {
						break
					}
				}
			}

			if len(filtered) > 0 {
				return filtered, true
			}
		}

		if currRec == 0 {
			break
		}

		classSyms, ok := s.GlobalIndex[GlobalKey{ReceiverHash: 0, PropHash: currRec}]
		if ok && len(classSyms) > 0 && classSyms[0].Parent != "" {
			currRec = ast.HashBytes([]byte(classSyms[0].Parent))

			continue
		}

		nextRec := s.getGlobalAlias(currRec)
		if nextRec == 0 {
			break
		}

		currRec = nextRec
	}

	return nil, false
}

func (s *Server) setGlobalSymbol(key GlobalKey, uri string, nodeID ast.NodeID, name, parent string, isRoot bool, isDep bool, depMsg string) {
	if doc, ok := s.Documents[uri]; ok {
		doc.ExportedGlobalDefs = append(doc.ExportedGlobalDefs, ExportedSymbol{
			NodeID:        nodeID,
			Key:           key,
			IsRoot:        isRoot,
			IsDeprecated:  isDep,
			DeprecatedMsg: depMsg,
		})
	}

	s.GlobalIndex[key] = append(s.GlobalIndex[key], GlobalSymbol{
		URI:           uri,
		NodeID:        nodeID,
		Name:          name,
		Parent:        parent,
		IsRoot:        isRoot,
		IsDeprecated:  isDep,
		DeprecatedMsg: depMsg,
	})
}

func (s *Server) removeDocumentGlobals(uri string, doc *Document) {
	for _, exp := range doc.ExportedGlobalDefs {
		if syms, ok := s.GlobalIndex[exp.Key]; ok {
			var n int

			for _, sym := range syms {
				if sym.URI != uri || sym.NodeID != exp.NodeID {
					syms[n] = sym

					n++
				}
			}

			if n > 0 {
				s.GlobalIndex[exp.Key] = syms[:n]
			} else {
				delete(s.GlobalIndex, exp.Key)
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

func (s *Server) suggestGlobal(srcDoc *Document, name string) string {
	var (
		bestMatch string
		minDist   = 3
	)

	nameLen := len(name)

	check := func(candidate string) {
		if candidate == "" {
			return
		}

		candLen := len(candidate)

		diff := candLen - nameLen
		if diff < 0 {
			diff = -diff
		}

		if diff > 3 {
			return
		}

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
				if tgtDoc, ok := s.Documents[sym.URI]; ok && s.canSeeSymbol(srcDoc, tgtDoc) {
					check(sym.Name)

					break
				}
			}
		}
	}

	return bestMatch
}

func (s *Server) getDocResourceRoot(doc *Document) string {
	if !s.FeatureFiveM {
		return ""
	}

	if doc.IsLibrary {
		return "std"
	}

	if doc.FiveMResolved {
		return doc.FiveMRoot
	}

	var bestRoot string

	for root := range s.FiveMResources {
		if strings.HasPrefix(doc.URI, root+"/") || doc.URI == root {
			if len(root) > len(bestRoot) {
				bestRoot = root
			}
		}
	}

	doc.FiveMRoot = bestRoot
	doc.FiveMResolved = true

	return bestRoot
}

func (s *Server) canSeeSymbol(srcDoc, tgtDoc *Document) bool {
	if !s.FeatureFiveM {
		return true
	}

	if srcDoc == tgtDoc {
		return true
	}

	if tgtDoc.IsLibrary {
		return true
	}

	srcRoot := s.getDocResourceRoot(srcDoc)
	tgtRoot := s.getDocResourceRoot(tgtDoc)

	if srcRoot == "" && tgtRoot == "" {
		return true
	}

	if srcRoot == tgtRoot {
		srcRes := s.FiveMResources[srcRoot]
		if srcRes != nil {
			srcEnv := s.getDocFileEnv(srcRes, srcDoc)
			tgtEnv := s.getDocFileEnv(srcRes, tgtDoc)

			srcIsManifest := srcDoc.IsFiveMManifest
			tgtIsManifest := tgtDoc.IsFiveMManifest

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
		if len(tgtDoc.URI) > len(tgtRes.RootURI) {
			relPath = tgtDoc.URI[len(tgtRes.RootURI)+1:]
		}
	} else if tgtRoot != "" {
		idx := strings.LastIndexByte(tgtRoot, '/')
		if idx != -1 {
			tgtName = tgtRoot[idx+1:]
		} else {
			tgtName = tgtRoot
		}

		if len(tgtDoc.URI) > len(tgtRoot) {
			relPath = tgtDoc.URI[len(tgtRoot)+1:]
		}
	}

	if tgtName != "" && relPath != "" {
		expectedInclude := "@" + tgtName + "/" + relPath

		srcEnv := s.getDocFileEnv(srcRes, srcDoc)

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

					if matchGlob(globPattern, relPath) {
						return true
					}
				}
			}
		}
	}

	return false
}

func (s *Server) isKnownGlobal(name []byte) bool {
	strName := ast.String(name)

	if s.KnownGlobals[strName] {
		return true
	}

	for _, glob := range s.KnownGlobalGlobs {
		if matchGlob(glob, strName) {
			return true
		}
	}

	return false
}
