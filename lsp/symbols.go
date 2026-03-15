package lsp

import (
	"iter"
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

		if !ctx.IsGlobal && ctx.TargetDoc != nil && ctx.TargetDoc.ExportedGlobalDefs != nil {
			if exportedKey, exported := ctx.TargetDoc.ExportedGlobalDefs[defID]; exported {
				ctx.IsGlobal = true
				ctx.GKey = exportedKey
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
			doc.ExportedGlobals = make(map[GlobalKey]ast.NodeID)
			doc.ExportedGlobalDefs = make(map[ast.NodeID]GlobalKey)
		}

		doc.ExportedGlobals[key] = nodeID
		doc.ExportedGlobalDefs[nodeID] = key
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

	for key := range doc.ExportedGlobals {
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

				if nodeID, exists := otherDoc.ExportedGlobals[key]; exists {
					d := getASTDepth(otherDoc.Tree, nodeID)

					isStd := strings.HasPrefix(otherURI, "std://")
					bestIsStd := strings.HasPrefix(bestSym.URI, "std://")

					var take bool

					if !found {
						take = true
					} else if d < bestSym.Depth {
						take = true
					} else if d == bestSym.Depth {
						if isStd && !bestIsStd {
							take = true
						} else if isStd == bestIsStd {
							if otherURI > bestSym.URI || (otherURI == bestSym.URI && nodeID > bestSym.NodeID) {
								take = true
							}
						}
					}

					if take {
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

			if found {
				s.GlobalIndex[key] = bestSym
			}
		}
	}
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
