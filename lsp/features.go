package lsp

import (
	"cmp"
	"path/filepath"
	"slices"

	"github.com/coalaura/lugo/ast"
)

type SemanticToken struct {
	Start     uint32
	End       uint32
	TokenType uint32
	Modifiers uint32
}

func (s *Server) buildCallHierarchyItemFromDef(uri string, doc *Document, defID ast.NodeID) CallHierarchyItem {
	if defID == ast.InvalidNode || int(defID) >= len(doc.Tree.Nodes) {
		return CallHierarchyItem{}
	}

	valID := doc.getAssignedValue(defID)
	isFunc := valID != ast.InvalidNode && int(valID) < len(doc.Tree.Nodes) && doc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr

	node := doc.Tree.Nodes[defID]

	var name string

	if node.Start <= node.End && node.End <= uint32(len(doc.Source)) {
		name = ast.String(doc.Source[node.Start:node.End])
	}

	if name == "" {
		name = "<error>"
	}

	kind := SymbolKindVariable

	if isFunc {
		kind = SymbolKindFunction

		if node.Parent != ast.InvalidNode && int(node.Parent) < len(doc.Tree.Nodes) {
			if doc.Tree.Nodes[node.Parent].Kind == ast.KindMethodName || doc.Tree.Nodes[node.Parent].Kind == ast.KindRecordField {
				kind = SymbolKindMethod
			}
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

func (s *Server) getDocumentHighlights(uri string, doc *Document, ctx *SymbolContext) []DocumentHighlight {
	var highlights []DocumentHighlight

	addHighlight := func(nodeID ast.NodeID, kind DocumentHighlightKind) {
		if nodeID == ast.InvalidNode || int(nodeID) >= len(doc.Tree.Nodes) {
			return
		}

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
				if int(ref.NodeID) < len(ref.Doc.Tree.Nodes) {
					pID := ref.Doc.Tree.Nodes[ref.NodeID].Parent

					if pID != ast.InvalidNode && int(pID) < len(ref.Doc.Tree.Nodes) {
						pNode := ref.Doc.Tree.Nodes[pID]
						if pNode.Kind == ast.KindFunctionStmt || pNode.Kind == ast.KindLocalFunction {
							kind = WriteHighlight
						}
					}
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

func (s *Server) getEnclosingFunctionDef(doc *Document, id ast.NodeID) ast.NodeID {
	if id == ast.InvalidNode || int(id) >= len(doc.Tree.Nodes) {
		return ast.InvalidNode
	}

	curr := doc.Tree.Nodes[id].Parent

	for curr != ast.InvalidNode && int(curr) < len(doc.Tree.Nodes) {
		node := doc.Tree.Nodes[curr]
		if node.Kind == ast.KindFunctionExpr {
			pID := node.Parent
			if pID != ast.InvalidNode && int(pID) < len(doc.Tree.Nodes) {
				pNode := doc.Tree.Nodes[pID]
				if pNode.Kind == ast.KindLocalFunction || pNode.Kind == ast.KindFunctionStmt {
					return pNode.Left
				} else if pNode.Kind == ast.KindRecordField {
					return pNode.Left
				} else if pNode.Kind == ast.KindExprList {
					gpID := pNode.Parent
					if gpID != ast.InvalidNode && int(gpID) < len(doc.Tree.Nodes) {
						gpNode := doc.Tree.Nodes[gpID]
						if (gpNode.Kind == ast.KindAssign || gpNode.Kind == ast.KindLocalAssign) && gpNode.Right == pID {
							idx := -1

							for i := uint16(0); i < pNode.Count; i++ {
								if pNode.Extra+uint32(i) < uint32(len(doc.Tree.ExtraList)) && doc.Tree.ExtraList[pNode.Extra+uint32(i)] == curr {
									idx = int(i)

									break
								}
							}

							if idx != -1 {
								lhs := doc.Tree.Nodes[gpNode.Left]
								if uint16(idx) < lhs.Count && lhs.Extra+uint32(idx) < uint32(len(doc.Tree.ExtraList)) {
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
