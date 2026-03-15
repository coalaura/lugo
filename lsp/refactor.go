package lsp

import (
	"bytes"
	"strings"

	"github.com/coalaura/lugo/ast"
	"github.com/coalaura/lugo/token"
)

type SafeFix struct {
	Coverage []ast.NodeID
	Edits    []TextEdit
	Title    string
}

type DeadStoreInfo struct {
	CanRemoveAll bool
	Edits        []TextEdit
	Coverage     []ast.NodeID
}

func (s *Server) getSafeFixesForDocument(doc *Document, actualReads []int) []SafeFix {
	var fixes []SafeFix

	if doc.IsMeta {
		return fixes
	}

	if actualReads == nil {
		actualReads = make([]int, len(doc.Tree.Nodes))

		for refID, defID := range doc.Resolver.References {
			if defID != ast.InvalidNode && ast.NodeID(refID) != defID {
				if s.isActualRead(doc, ast.NodeID(refID), defID) {
					actualReads[defID]++
				}
			}
		}
	}

	unusedDefs := make([]bool, len(doc.Tree.Nodes))
	deadStores := make(map[ast.NodeID]*DeadStoreInfo)

	for _, defID := range doc.Resolver.LocalDefs {
		if actualReads[defID] == 0 {
			if ast.Attr(doc.Tree.Nodes[defID].Extra) == ast.AttrClose {
				continue
			}

			name := doc.Source[doc.Tree.Nodes[defID].Start:doc.Tree.Nodes[defID].End]
			if len(name) > 0 && name[0] != '_' {
				unusedDefs[defID] = true

				deadStores[defID] = &DeadStoreInfo{CanRemoveAll: true}
			}
		}
	}

	// PASS 1: Collect dead mutations
	for i := 1; i < len(doc.Tree.Nodes); i++ {
		nodeID := ast.NodeID(i)
		node := doc.Tree.Nodes[nodeID]

		switch node.Kind {
		case ast.KindAssign:
			lhsList := doc.Tree.Nodes[node.Left]
			allUnused := true

			var coverage []ast.NodeID

			for j := uint16(0); j < lhsList.Count; j++ {
				lhsID := doc.Tree.ExtraList[lhsList.Extra+uint32(j)]
				defID := s.getRootDef(doc, lhsID)

				if defID == ast.InvalidNode || !unusedDefs[defID] {
					allUnused = false

					break
				}

				coverage = append(coverage, defID)
			}

			if allUnused && lhsList.Count > 0 {
				rhsSafe := true

				if node.Right != ast.InvalidNode {
					exprList := doc.Tree.Nodes[node.Right]

					for j := uint16(0); j < exprList.Count; j++ {
						if !s.isSideEffectFree(doc, doc.Tree.ExtraList[exprList.Extra+uint32(j)]) {
							rhsSafe = false

							break
						}
					}
				}

				if rhsSafe {
					edit := TextEdit{
						Range:   s.getStatementRemovalRange(doc, nodeID),
						NewText: "",
					}

					for _, defID := range coverage {
						ds := deadStores[defID]

						ds.Edits = append(ds.Edits, edit)
						ds.Coverage = append(ds.Coverage, nodeID)
					}
				} else if node.Right != ast.InvalidNode {
					exprList := doc.Tree.Nodes[node.Right]

					if exprList.Count == 1 {
						exprID := doc.Tree.ExtraList[exprList.Extra]
						exprNode := doc.Tree.Nodes[exprID]

						if exprNode.Kind == ast.KindCallExpr || exprNode.Kind == ast.KindMethodCall {
							callText := doc.Source[exprNode.Start:exprNode.End]

							edit := TextEdit{
								Range:   getNodeRange(doc.Tree, nodeID),
								NewText: string(callText),
							}

							for _, defID := range coverage {
								ds := deadStores[defID]

								ds.Edits = append(ds.Edits, edit)
								ds.Coverage = append(ds.Coverage, nodeID)
							}
						} else {
							for _, defID := range coverage {
								deadStores[defID].CanRemoveAll = false
							}
						}
					} else {
						for _, defID := range coverage {
							deadStores[defID].CanRemoveAll = false
						}
					}
				} else {
					for _, defID := range coverage {
						deadStores[defID].CanRemoveAll = false
					}
				}
			} else {
				for j := uint16(0); j < lhsList.Count; j++ {
					lhsID := doc.Tree.ExtraList[lhsList.Extra+uint32(j)]

					defID := s.getRootDef(doc, lhsID)
					if defID != ast.InvalidNode && unusedDefs[defID] {
						deadStores[defID].CanRemoveAll = false
					}
				}
			}
		case ast.KindCallExpr:
			fnID := node.Left
			if doc.Tree.Nodes[fnID].Kind == ast.KindMemberExpr {
				recID := doc.Tree.Nodes[fnID].Left
				propID := doc.Tree.Nodes[fnID].Right
				if recID != ast.InvalidNode && propID != ast.InvalidNode {
					recStr := doc.Source[doc.Tree.Nodes[recID].Start:doc.Tree.Nodes[recID].End]
					propStr := doc.Source[doc.Tree.Nodes[propID].Start:doc.Tree.Nodes[propID].End]

					if bytes.Equal(recStr, []byte("table")) && (bytes.Equal(propStr, []byte("insert")) || bytes.Equal(propStr, []byte("remove")) || bytes.Equal(propStr, []byte("sort"))) {
						if node.Count > 0 {
							firstArgID := doc.Tree.ExtraList[node.Extra]
							defID := s.getRootDef(doc, firstArgID)

							if defID != ast.InvalidNode && unusedDefs[defID] {
								argsSafe := true

								for j := uint16(1); j < node.Count; j++ {
									if !s.isSideEffectFree(doc, doc.Tree.ExtraList[node.Extra+uint32(j)]) {
										argsSafe = false

										break
									}
								}

								if argsSafe {
									edit := TextEdit{
										Range:   s.getStatementRemovalRange(doc, nodeID),
										NewText: "",
									}

									ds := deadStores[defID]

									ds.Edits = append(ds.Edits, edit)
									ds.Coverage = append(ds.Coverage, nodeID)
								} else {
									deadStores[defID].CanRemoveAll = false
								}
							}
						}
					}
				}
			}
		}
	}

	// PASS 2: Generate SafeFixes
	for i := 1; i < len(doc.Tree.Nodes); i++ {
		nodeID := ast.NodeID(i)
		node := doc.Tree.Nodes[nodeID]

		switch node.Kind {
		case ast.KindLocalAssign:
			s.processListForFixes(doc, node.Left, node.Right, unusedDefs, deadStores, &fixes, true)
		case ast.KindForIn:
			s.processListForFixes(doc, node.Left, ast.InvalidNode, unusedDefs, deadStores, &fixes, false)
		case ast.KindLocalFunction:
			if unusedDefs[node.Left] {
				fixes = append(fixes, SafeFix{
					Coverage: []ast.NodeID{node.Left},
					Edits: []TextEdit{{
						Range:   s.getStatementRemovalRange(doc, nodeID),
						NewText: "",
					}},
					Title: "Remove unused local function",
				})

				unusedDefs[node.Left] = false

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

				unusedDefs[node.Left] = false
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
							Range:   s.expandRemovalRange(doc, doc.Tree.Nodes[stmtID].Start, doc.Tree.Nodes[lastStmtID].End),
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

func (s *Server) processListForFixes(doc *Document, nameListID, exprListID ast.NodeID, unused []bool, deadStores map[ast.NodeID]*DeadStoreInfo, fixes *[]SafeFix, canRemoveStatement bool) {
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

			unused[id] = false
		}
	}

	if suffixStart < int(nameList.Count) {
		var coverage []ast.NodeID

		canCleanlyRemove := true

		for i := suffixStart; i < int(nameList.Count); i++ {
			id := doc.Tree.ExtraList[nameList.Extra+uint32(i)]

			coverage = append(coverage, id)

			unused[id] = false

			if ds := deadStores[id]; ds != nil && !ds.CanRemoveAll {
				canCleanlyRemove = false
			}
		}

		if !canCleanlyRemove {
			for _, id := range coverage {
				*fixes = append(*fixes, s.createRenameFix(doc, id))
			}

			return
		}

		var (
			extraEdits    []TextEdit
			extraCoverage []ast.NodeID
		)

		for _, id := range coverage {
			if ds := deadStores[id]; ds != nil {
				extraEdits = append(extraEdits, ds.Edits...)
				extraCoverage = append(extraCoverage, ds.Coverage...)
			}
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

				edits := []TextEdit{{
					Range:   s.getStatementRemovalRange(doc, stmtID),
					NewText: "",
				}}

				edits = append(edits, extraEdits...)
				coverage = append(coverage, extraCoverage...)

				*fixes = append(*fixes, SafeFix{
					Coverage: coverage,
					Edits:    edits,
					Title:    "Remove unused assignment",
				})

				return
			} else if exprListID != ast.InvalidNode {
				exprList := doc.Tree.Nodes[exprListID]

				if exprList.Count == 1 {
					exprID := doc.Tree.ExtraList[exprList.Extra]
					exprNode := doc.Tree.Nodes[exprID]

					if exprNode.Kind == ast.KindCallExpr || exprNode.Kind == ast.KindMethodCall {
						stmtID := nameList.Parent
						callText := doc.Source[exprNode.Start:exprNode.End]

						edits := []TextEdit{{
							Range:   getNodeRange(doc.Tree, stmtID),
							NewText: string(callText),
						}}

						edits = append(edits, extraEdits...)
						coverage = append(coverage, extraCoverage...)

						*fixes = append(*fixes, SafeFix{
							Coverage: coverage,
							Edits:    edits,
							Title:    "Remove unused assignment (keep function call)",
						})

						return
					}
				}
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
			edits = append(edits, extraEdits...)

			coverage = append(coverage, extraCoverage...)

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

func (s *Server) processParamsForFixes(doc *Document, funcExprID ast.NodeID, unused []bool, fixes *[]SafeFix) {
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

			unused[id] = false
		}
	}

	if suffixStart < int(funcNode.Count) {
		var coverage []ast.NodeID

		for i := suffixStart; i < int(funcNode.Count); i++ {
			id := doc.Tree.ExtraList[funcNode.Extra+uint32(i)]

			coverage = append(coverage, id)

			unused[id] = false
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
	name := ast.String(doc.Source[node.Start:node.End])

	return SafeFix{
		Coverage: []ast.NodeID{id},
		Edits: []TextEdit{{
			Range:   getNodeRange(doc.Tree, id),
			NewText: "_" + name,
		}},
		Title: "Prefix with '_'",
	}
}

func (s *Server) checkSafetyAndBudget(doc *Document, targetIf ast.NodeID, budget int) ([]ast.NodeID, bool) {
	var trailing []ast.NodeID

	curr := targetIf
	isImmediateBlock := true

	for curr != ast.InvalidNode {
		pID := doc.Tree.Nodes[curr].Parent
		if pID == ast.InvalidNode {
			break
		}

		pNode := doc.Tree.Nodes[pID]

		if pNode.Kind == ast.KindBlock || pNode.Kind == ast.KindFile || pNode.Kind == ast.KindRepeat {
			idx := -1
			for i := uint16(0); i < pNode.Count; i++ {
				if doc.Tree.ExtraList[pNode.Extra+uint32(i)] == curr {
					idx = int(i)
					break
				}
			}

			if idx != -1 {
				elementsAfter := int(pNode.Count) - 1 - idx

				if isImmediateBlock {
					if elementsAfter > budget {
						return nil, false
					}
					for i := idx + 1; i < int(pNode.Count); i++ {
						trailing = append(trailing, doc.Tree.ExtraList[pNode.Extra+uint32(i)])
					}
					isImmediateBlock = false
				} else {
					// If ANY outer block has trailing statements, we cannot early return,
					// because we would accidentally skip executing them!
					if elementsAfter > 0 {
						return nil, false
					}
				}
			}
		} else if pNode.Kind == ast.KindIf || pNode.Kind == ast.KindElseIf || pNode.Kind == ast.KindElse || pNode.Kind == ast.KindDo {
			// Transparent structural blocks
		} else if pNode.Kind == ast.KindFunctionExpr || pNode.Kind == ast.KindFunctionStmt || pNode.Kind == ast.KindLocalFunction {
			return trailing, true // Safe function boundary
		} else if pNode.Kind == ast.KindWhile || pNode.Kind == ast.KindForIn || pNode.Kind == ast.KindForNum {
			return nil, false // Exiting a loop is a break, not a return. Unsafe.
		}

		curr = pID
	}

	return trailing, true
}

func (s *Server) formatStatement(doc *Document, stmtID ast.NodeID, indent string, budget int) string {
	if stmtID == ast.InvalidNode || int(stmtID) >= len(doc.Tree.Nodes) {
		return ""
	}

	node := doc.Tree.Nodes[stmtID]
	if node.Kind != ast.KindIf && node.Kind != ast.KindDo {
		return reindentNodeText(doc, stmtID, indent)
	}

	innerIndent := indent + "\t"

	if strings.Contains(indent, "    ") {
		innerIndent = indent + "    "
	} else if strings.Contains(indent, "  ") {
		innerIndent = indent + "  "
	}

	if node.Kind == ast.KindDo {
		var out strings.Builder

		out.WriteString("do\n")
		out.WriteString(s.flattenBlock(doc, node.Left, innerIndent, budget))
		out.WriteString("\n")
		out.WriteString(indent)
		out.WriteString("end")

		return out.String()
	}

	var (
		hasElseIf bool
		elseBlock ast.NodeID = ast.InvalidNode
	)

	for i := uint16(0); i < node.Count; i++ {
		if node.Extra+uint32(i) >= uint32(len(doc.Tree.ExtraList)) {
			continue
		}

		childID := doc.Tree.ExtraList[node.Extra+uint32(i)]
		if int(childID) >= len(doc.Tree.Nodes) {
			continue
		}

		child := doc.Tree.Nodes[childID]

		switch child.Kind {
		case ast.KindElseIf:
			hasElseIf = true
		case ast.KindElse:
			elseBlock = childID
		}
	}

	trailing, isSafe := s.checkSafetyAndBudget(doc, stmtID, budget)

	// Wrap string extraction block to prevent slicing errors
	getCondStr := func(condID ast.NodeID) string {
		if int(condID) >= len(doc.Tree.Nodes) {
			return ""
		}

		cNode := doc.Tree.Nodes[condID]
		if cNode.Start <= cNode.End && cNode.End <= uint32(len(doc.Source)) {
			return ast.String(doc.Source[cNode.Start:cNode.End])
		}

		return ""
	}

	if !hasElseIf && isSafe {
		var out strings.Builder

		out.WriteString("if ")
		out.WriteString(s.invertCondition(doc, node.Left))
		out.WriteString(" then\n")

		var lastEmitted ast.NodeID = ast.InvalidNode

		if elseBlock != ast.InvalidNode {
			elseNode := doc.Tree.Nodes[elseBlock]

			if int(elseNode.Left) < len(doc.Tree.Nodes) {
				elseBlockNode := doc.Tree.Nodes[elseNode.Left]

				for i := uint16(0); i < elseBlockNode.Count; i++ {
					if elseBlockNode.Extra+uint32(i) >= uint32(len(doc.Tree.ExtraList)) {
						continue
					}

					childID := doc.Tree.ExtraList[elseBlockNode.Extra+uint32(i)]

					if lastEmitted != ast.InvalidNode {
						out.WriteString("\n")
					}

					out.WriteString(innerIndent)
					out.WriteString(reindentNodeText(doc, childID, innerIndent))
					out.WriteString("\n")

					lastEmitted = childID
				}
			}
		}

		if !isTerminal(doc.Tree, lastEmitted) {
			for _, trailID := range trailing {
				if lastEmitted != ast.InvalidNode {
					out.WriteString("\n")
				}

				out.WriteString(innerIndent)
				out.WriteString(reindentNodeText(doc, trailID, innerIndent))
				out.WriteString("\n")

				lastEmitted = trailID
			}

			if !isTerminal(doc.Tree, lastEmitted) {
				if lastEmitted != ast.InvalidNode {
					out.WriteString("\n")
				}

				out.WriteString(innerIndent)
				out.WriteString("return\n")
			}
		}

		out.WriteString(indent)
		out.WriteString("end\n\n")

		out.WriteString(indent)
		out.WriteString(s.flattenBlock(doc, node.Right, indent, budget))

		return out.String()
	}

	var out strings.Builder

	out.WriteString("if ")
	out.WriteString(getCondStr(node.Left))
	out.WriteString(" then\n")

	thenText := s.flattenBlock(doc, node.Right, innerIndent, budget)
	if thenText != "" {
		out.WriteString(innerIndent)
		out.WriteString(thenText)
		out.WriteString("\n")
	}

	for i := uint16(0); i < node.Count; i++ {
		if node.Extra+uint32(i) >= uint32(len(doc.Tree.ExtraList)) {
			continue
		}

		childID := doc.Tree.ExtraList[node.Extra+uint32(i)]
		if int(childID) >= len(doc.Tree.Nodes) {
			continue
		}

		child := doc.Tree.Nodes[childID]

		switch child.Kind {
		case ast.KindElseIf:
			out.WriteString(indent)
			out.WriteString("elseif ")
			out.WriteString(getCondStr(child.Left))
			out.WriteString(" then\n")

			eiText := s.flattenBlock(doc, child.Right, innerIndent, budget)
			if eiText != "" {
				out.WriteString(innerIndent)
				out.WriteString(eiText)
				out.WriteString("\n")
			}
		case ast.KindElse:
			out.WriteString(indent)
			out.WriteString("else\n")

			eText := s.flattenBlock(doc, child.Left, innerIndent, budget)
			if eText != "" {
				out.WriteString(innerIndent)
				out.WriteString(eText)
				out.WriteString("\n")
			}
		}
	}

	out.WriteString(indent)
	out.WriteString("end")

	return out.String()
}

func (s *Server) flattenBlock(doc *Document, blockID ast.NodeID, indent string, budget int) string {
	if blockID == ast.InvalidNode || int(blockID) >= len(doc.Tree.Nodes) {
		return ""
	}

	var out strings.Builder

	blockNode := doc.Tree.Nodes[blockID]

	for i := uint16(0); i < blockNode.Count; i++ {
		if blockNode.Extra+uint32(i) >= uint32(len(doc.Tree.ExtraList)) {
			continue
		}

		childID := doc.Tree.ExtraList[blockNode.Extra+uint32(i)]
		stmtStr := s.formatStatement(doc, childID, indent, budget)

		if i > 0 {
			prevID := doc.Tree.ExtraList[blockNode.Extra+uint32(i-1)]

			var gap []byte

			if int(prevID) < len(doc.Tree.Nodes) && int(childID) < len(doc.Tree.Nodes) {
				prevNode := doc.Tree.Nodes[prevID]
				currNode := doc.Tree.Nodes[childID]

				if prevNode.End <= currNode.Start && currNode.Start <= uint32(len(doc.Source)) {
					gap = doc.Source[prevNode.End:currNode.Start]
				}
			}

			out.WriteString("\n")

			if len(gap) > 0 && bytes.Count(gap, []byte{'\n'}) > 1 {
				out.WriteString("\n")
				out.WriteString(indent)
			} else if int(childID) < len(doc.Tree.Nodes) {
				currNode := doc.Tree.Nodes[childID]
				if currNode.Kind == ast.KindIf || currNode.Kind == ast.KindDo || currNode.Kind == ast.KindForNum || currNode.Kind == ast.KindForIn || currNode.Kind == ast.KindWhile {
					out.WriteString("\n")
					out.WriteString(indent)
				} else {
					out.WriteString(indent)
				}
			} else {
				out.WriteString(indent)
			}
		}

		out.WriteString(stmtStr)
	}

	return out.String()
}

func (s *Server) invertCondition(doc *Document, condID ast.NodeID) string {
	if condID == ast.InvalidNode || int(condID) >= len(doc.Tree.Nodes) {
		return "true"
	}

	condNode := doc.Tree.Nodes[condID]

	var condStr string

	if condNode.Start <= condNode.End && condNode.End <= uint32(len(doc.Source)) {
		condStr = ast.String(doc.Source[condNode.Start:condNode.End])
	}

	switch condNode.Kind {
	case ast.KindParenExpr:
		return "(" + s.invertCondition(doc, condNode.Left) + ")"
	case ast.KindUnaryExpr:
		if condNode.Right != ast.InvalidNode && int(condNode.Right) < len(doc.Tree.Nodes) {
			if bytes.HasPrefix([]byte(condStr), []byte("not")) {
				rightNode := doc.Tree.Nodes[condNode.Right]

				if rightNode.Start <= rightNode.End && rightNode.End <= uint32(len(doc.Source)) {
					return ast.String(doc.Source[rightNode.Start:rightNode.End])
				}
			}
		}
	case ast.KindBinaryExpr:
		op := token.Kind(condNode.Extra)

		if op == token.And || op == token.Or {
			leftInverted := s.invertCondition(doc, condNode.Left)
			rightInverted := s.invertCondition(doc, condNode.Right)

			if op == token.And {
				return leftInverted + " or " + rightInverted
			}

			if int(condNode.Left) < len(doc.Tree.Nodes) {
				leftNode := doc.Tree.Nodes[condNode.Left]
				if leftNode.Kind == ast.KindBinaryExpr && token.Kind(leftNode.Extra) == token.And {
					leftInverted = "(" + leftInverted + ")"
				}
			}

			if int(condNode.Right) < len(doc.Tree.Nodes) {
				rightNode := doc.Tree.Nodes[condNode.Right]
				if rightNode.Kind == ast.KindBinaryExpr && token.Kind(rightNode.Extra) == token.And {
					rightInverted = "(" + rightInverted + ")"
				}
			}

			return leftInverted + " and " + rightInverted
		}

		var leftStr, rightStr string

		if int(condNode.Left) < len(doc.Tree.Nodes) {
			lNode := doc.Tree.Nodes[condNode.Left]
			if lNode.Start <= lNode.End && lNode.End <= uint32(len(doc.Source)) {
				leftStr = ast.String(doc.Source[lNode.Start:lNode.End])
			}
		}

		if int(condNode.Right) < len(doc.Tree.Nodes) {
			rNode := doc.Tree.Nodes[condNode.Right]
			if rNode.Start <= rNode.End && rNode.End <= uint32(len(doc.Source)) {
				rightStr = ast.String(doc.Source[rNode.Start:rNode.End])
			}
		}

		switch op {
		case token.Eq:
			return leftStr + " ~= " + rightStr
		case token.NotEq:
			return leftStr + " == " + rightStr
		case token.Less:
			return leftStr + " >= " + rightStr
		case token.LessEq:
			return leftStr + " > " + rightStr
		case token.Greater:
			return leftStr + " <= " + rightStr
		case token.GreaterEq:
			return leftStr + " < " + rightStr
		}
	}

	if condNode.Kind == ast.KindIdent || condNode.Kind == ast.KindMemberExpr || condNode.Kind == ast.KindCallExpr || condNode.Kind == ast.KindMethodCall || condNode.Kind == ast.KindIndexExpr {
		return "not " + condStr
	}

	return "not (" + condStr + ")"
}

func (s *Server) expandRemovalRange(doc *Document, start, end uint32) Range {
	// Consume leading spaces/tabs on the same line
	for start > 0 {
		c := doc.Source[start-1]

		if c == ' ' || c == '\t' {
			start--
		} else {
			break
		}
	}

	// Consume optional trailing semicolon
	for end < uint32(len(doc.Source)) {
		c := doc.Source[end]

		if c == ' ' || c == '\t' {
			end++
		} else if c == ';' {
			end++

			break
		} else {
			break
		}
	}

	// Consume trailing spaces/tabs and exactly ONE newline
	var hadNewline bool

	for end < uint32(len(doc.Source)) {
		c := doc.Source[end]

		if c == ' ' || c == '\t' || c == '\r' {
			end++
		} else if c == '\n' {
			end++

			hadNewline = true

			break
		} else {
			break
		}
	}

	if hadNewline {
		prevLineEmpty := start == 0

		if !prevLineEmpty && doc.Source[start-1] == '\n' {
			i := start - 1
			if i > 0 && doc.Source[i-1] == '\r' {
				i--
			}

			isEmpty := true

			for i > 0 {
				c := doc.Source[i-1]

				if c == ' ' || c == '\t' {
					i--
				} else if c == '\n' {
					break
				} else {
					isEmpty = false

					break
				}
			}

			prevLineEmpty = isEmpty || i == 0
		}

		// Consume one extra trailing empty line, to prevent leaving behind double blank lines.
		if prevLineEmpty {
			tempEnd := end

			for tempEnd < uint32(len(doc.Source)) {
				c := doc.Source[tempEnd]

				if c == ' ' || c == '\t' || c == '\r' {
					tempEnd++
				} else if c == '\n' {
					tempEnd++

					end = tempEnd

					break
				} else {
					break
				}
			}
		}
	}

	return getRange(doc.Tree, start, end)
}

func (s *Server) getStatementRemovalRange(doc *Document, nodeID ast.NodeID) Range {
	node := doc.Tree.Nodes[nodeID]

	return s.expandRemovalRange(doc, node.Start, node.End)
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
