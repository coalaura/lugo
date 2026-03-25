package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
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

func (s *Server) handleCodeAction(req Request) {
	var params CodeActionParams

	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		return
	}

	var (
		actions        []CodeAction
		hasSafeFixDiag bool
	)

	uri := s.normalizeURI(params.TextDocument.URI)

	doc, docOk := s.Documents[uri]
	if !docOk {
		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: []CodeAction{}})

		return
	}

	for _, diag := range params.Context.Diagnostics {
		switch diag.Code {
		case "unused-local", "unused-parameter", "unused-loop-var", "unused-vararg", "unused-function", "unreachable-code", "ambiguous-return", "self-assignment", "empty-block", "redundant-return", "redundant-value", "redundant-parameter":
			hasSafeFixDiag = true
		case "used-ignored-var":
			defIDFloat, ok := diag.Data.(float64)
			if ok {
				defID := ast.NodeID(defIDFloat)
				if int(defID) < len(doc.Tree.Nodes) {
					node := doc.Tree.Nodes[defID]
					nameBytes := doc.Source[node.Start:node.End]

					if len(nameBytes) > 1 && nameBytes[0] == '_' {
						newNameBytes := nameBytes[1:]
						newName := string(newNameBytes)

						if !s.isNameSafe(doc, defID, newNameBytes) {
							newName = s.generateSafeName(doc, defID, newName, false)
						}

						if newName != "" {
							var edits []TextEdit

							edits = append(edits, TextEdit{
								Range:   getNodeRange(doc.Tree, defID),
								NewText: newName,
							})

							for i, refDefID := range doc.Resolver.References {
								if refDefID == defID && ast.NodeID(i) != defID {
									edits = append(edits, TextEdit{
										Range:   getNodeRange(doc.Tree, ast.NodeID(i)),
										NewText: newName,
									})
								}
							}

							actions = append(actions, CodeAction{
								Title:       fmt.Sprintf("Rename to '%s'", newName),
								Kind:        "quickfix",
								Diagnostics: []Diagnostic{diag},
								IsPreferred: true,
								Edit: &WorkspaceEdit{
									Changes: map[string][]TextEdit{
										uri: edits,
									},
								},
							})
						}
					}
				}
			}
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
		case "shadow-global", "shadow-outer", "duplicate-local":
			offset := doc.Tree.Offset(diag.Range.Start.Line, diag.Range.Start.Character)
			defID := doc.Tree.NodeAt(offset)

			if defID != ast.InvalidNode && int(defID) < len(doc.Tree.Nodes) && doc.Resolver.References[defID] == defID {
				node := doc.Tree.Nodes[defID]
				nameBytes := doc.Source[node.Start:node.End]
				baseName := string(nameBytes)

				safeName := s.generateSafeName(doc, defID, baseName, false)

				if safeName != "" && safeName != baseName {
					var edits []TextEdit

					edits = append(edits, TextEdit{
						Range:   getNodeRange(doc.Tree, defID),
						NewText: safeName,
					})

					for i, refDefID := range doc.Resolver.References {
						if refDefID == defID && ast.NodeID(i) != defID {
							edits = append(edits, TextEdit{
								Range:   getNodeRange(doc.Tree, ast.NodeID(i)),
								NewText: safeName,
							})
						}
					}

					actions = append(actions, CodeAction{
						Title:       fmt.Sprintf("Rename to '%s'", safeName),
						Kind:        "quickfix",
						Diagnostics: []Diagnostic{diag},
						IsPreferred: true,
						Edit: &WorkspaceEdit{
							Changes: map[string][]TextEdit{
								uri: edits,
							},
						},
					})
				}
			}
		}
	}

	allFixes := s.getSafeFixesForDocument(doc, nil)

	var allEdits []TextEdit

	if hasSafeFixDiag {
		for _, diag := range params.Context.Diagnostics {
			switch diag.Code {
			case "unused-local", "unused-parameter", "unused-loop-var", "unused-vararg", "unused-function", "unreachable-code", "ambiguous-return", "self-assignment", "empty-block", "redundant-return", "redundant-value", "redundant-parameter":
				// Proceed
			default:
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
		targetIf            ast.NodeID = ast.InvalidNode
		targetCond          ast.NodeID = ast.InvalidNode
		condTitle           string
		targetTableInsert   ast.NodeID = ast.InvalidNode
		targetMethod        ast.NodeID = ast.InvalidNode
		targetNestedIf      ast.NodeID = ast.InvalidNode
		nestedIfTitle       string
		targetMultiAssign   ast.NodeID = ast.InvalidNode
		targetSwapIfElse    ast.NodeID = ast.InvalidNode
		targetParen         ast.NodeID = ast.InvalidNode
		targetForNum        ast.NodeID = ast.InvalidNode
		forNumTable         string
		targetIndexToMember ast.NodeID = ast.InvalidNode
		indexToMemberStr    string
		targetMemberToIndex ast.NodeID = ast.InvalidNode
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

		// 4. Split Multiple Assignment
		if (node.Kind == ast.KindLocalAssign || node.Kind == ast.KindAssign) && targetMultiAssign == ast.InvalidNode {
			if int(node.Left) < len(doc.Tree.Nodes) {
				lhsList := doc.Tree.Nodes[node.Left]
				if lhsList.Count > 1 {
					canSplit := true

					if node.Right != ast.InvalidNode && int(node.Right) < len(doc.Tree.Nodes) {
						rhsList := doc.Tree.Nodes[node.Right]
						if rhsList.Count > lhsList.Count {
							canSplit = false
						} else if lhsList.Count > rhsList.Count && rhsList.Count > 0 && rhsList.Extra+uint32(rhsList.Count-1) < uint32(len(doc.Tree.ExtraList)) {
							lastRhsID := doc.Tree.ExtraList[rhsList.Extra+uint32(rhsList.Count-1)]
							if int(lastRhsID) < len(doc.Tree.Nodes) {
								lastRhsNode := doc.Tree.Nodes[lastRhsID]
								if lastRhsNode.Kind == ast.KindCallExpr || lastRhsNode.Kind == ast.KindMethodCall || lastRhsNode.Kind == ast.KindVararg {
									canSplit = false
								}
							}
						}
					}

					if canSplit {
						targetMultiAssign = curr
					}
				}
			}
		}

		// 5. Swap If/Else Branches
		if node.Kind == ast.KindIf && targetSwapIfElse == ast.InvalidNode {
			var hasElseIf, hasElse bool

			for i := uint16(0); i < node.Count; i++ {
				if node.Extra+uint32(i) < uint32(len(doc.Tree.ExtraList)) {
					childID := doc.Tree.ExtraList[node.Extra+uint32(i)]
					if int(childID) < len(doc.Tree.Nodes) {
						child := doc.Tree.Nodes[childID]
						if child.Kind == ast.KindElseIf {
							hasElseIf = true
						} else if child.Kind == ast.KindElse {
							hasElse = true
						}
					}
				}
			}

			if hasElse && !hasElseIf {
				targetSwapIfElse = curr
			}
		}

		// 6. Remove Redundant Parentheses
		if node.Kind == ast.KindParenExpr && targetParen == ast.InvalidNode {
			targetParen = curr
		}

		// 7. Convert for i=1, #t to ipairs
		if node.Kind == ast.KindForNum && targetForNum == ast.InvalidNode {
			if node.Count >= 2 && node.Extra+1 < uint32(len(doc.Tree.ExtraList)) {
				initID := doc.Tree.ExtraList[node.Extra]
				limitID := doc.Tree.ExtraList[node.Extra+1]

				initVal, ok := doc.evalNode(initID, 0)
				if ok && initVal.kind == ast.KindNumber && initVal.num == 1 {
					if int(limitID) < len(doc.Tree.Nodes) {
						limitNode := doc.Tree.Nodes[limitID]
						if limitNode.Kind == ast.KindUnaryExpr {
							limitSrc := doc.Source[limitNode.Start:limitNode.End]
							if bytes.HasPrefix(limitSrc, []byte("#")) {
								targetForNum = curr

								forNumTable = string(bytes.TrimSpace(limitSrc[1:]))
							}
						}
					}
				}
			}
		}

		// 8. Convert Index to Member (t["prop"] -> t.prop)
		if node.Kind == ast.KindIndexExpr && targetIndexToMember == ast.InvalidNode {
			if int(node.Right) < len(doc.Tree.Nodes) {
				res, ok := doc.evalNode(node.Right, 0)
				if ok && res.kind == ast.KindString {
					if s.isValidIdentifier(res.str) {
						targetIndexToMember = curr

						indexToMemberStr = res.str
					}
				}
			}
		}

		// 9. Convert Member to Index (t.prop -> t["prop"])
		if node.Kind == ast.KindMemberExpr && targetMemberToIndex == ast.InvalidNode {
			targetMemberToIndex = curr
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

	// 6. Split Multiple Assignment
	if targetMultiAssign != ast.InvalidNode {
		actions = append(actions, CodeAction{
			Title:       "Split into multiple assignments",
			Kind:        "refactor.rewrite",
			IsPreferred: false,
			Data: map[string]any{
				"type":   "splitMultiAssign",
				"uri":    uri,
				"nodeId": float64(targetMultiAssign),
			},
		})
	}

	// 7. Swap If/Else Branches
	if targetSwapIfElse != ast.InvalidNode {
		actions = append(actions, CodeAction{
			Title:       "Swap 'if' and 'else' branches",
			Kind:        "refactor.rewrite",
			IsPreferred: false,
			Data: map[string]any{
				"type":   "swapIfElse",
				"uri":    uri,
				"nodeId": float64(targetSwapIfElse),
			},
		})
	}

	// 8. Remove Redundant Parentheses
	if targetParen != ast.InvalidNode {
		actions = append(actions, CodeAction{
			Title:       "Remove redundant parentheses",
			Kind:        "refactor.rewrite",
			IsPreferred: false,
			Data: map[string]any{
				"type":   "removeParen",
				"uri":    uri,
				"nodeId": float64(targetParen),
			},
		})
	}

	// 9. Convert for i=1, #t to ipairs
	if targetForNum != ast.InvalidNode {
		actions = append(actions, CodeAction{
			Title:       "Convert to 'ipairs'",
			Kind:        "refactor.rewrite",
			IsPreferred: false,
			Data: map[string]any{
				"type":   "forNumToIpairs",
				"uri":    uri,
				"nodeId": float64(targetForNum),
				"table":  forNumTable,
			},
		})
	}

	// 10. Convert Index to Member
	if targetIndexToMember != ast.InvalidNode {
		actions = append(actions, CodeAction{
			Title:       "Convert to dot notation",
			Kind:        "refactor.rewrite",
			IsPreferred: false,
			Data: map[string]any{
				"type":   "indexToMember",
				"uri":    uri,
				"nodeId": float64(targetIndexToMember),
				"prop":   indexToMemberStr,
			},
		})
	}

	// 11. Convert Member to Index
	if targetMemberToIndex != ast.InvalidNode {
		actions = append(actions, CodeAction{
			Title:       "Convert to bracket notation",
			Kind:        "refactor.rewrite",
			IsPreferred: false,
			Data: map[string]any{
				"type":   "memberToIndex",
				"uri":    uri,
				"nodeId": float64(targetMemberToIndex),
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
}

func (s *Server) handleCodeActionResolve(req Request) {
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
			} else if actionType == "splitMultiAssign" {
				assignNode := doc.Tree.Nodes[nodeID]

				if int(assignNode.Left) < len(doc.Tree.Nodes) {
					lhsList := doc.Tree.Nodes[assignNode.Left]

					var rhsList ast.Node

					if assignNode.Right != ast.InvalidNode && int(assignNode.Right) < len(doc.Tree.Nodes) {
						rhsList = doc.Tree.Nodes[assignNode.Right]
					}

					assignLine, _ := doc.Tree.Position(assignNode.Start)

					var lineStart uint32

					if int(assignLine) < len(doc.Tree.LineOffsets) {
						lineStart = doc.Tree.LineOffsets[assignLine]
					}

					var indentBytes []byte

					for i := lineStart; i < assignNode.Start && i < uint32(len(doc.Source)); i++ {
						if doc.Source[i] == ' ' || doc.Source[i] == '\t' {
							indentBytes = append(indentBytes, doc.Source[i])
						} else {
							break
						}
					}

					indent := string(indentBytes)

					var newText strings.Builder

					isLocal := assignNode.Kind == ast.KindLocalAssign

					for i := uint16(0); i < lhsList.Count; i++ {
						if i > 0 {
							newText.WriteString("\n")
							newText.WriteString(indent)
						}

						if isLocal {
							newText.WriteString("local ")
						}

						if lhsList.Extra+uint32(i) < uint32(len(doc.Tree.ExtraList)) {
							lhsID := doc.Tree.ExtraList[lhsList.Extra+uint32(i)]
							if int(lhsID) < len(doc.Tree.Nodes) {
								lhsNode := doc.Tree.Nodes[lhsID]

								if lhsNode.Start <= lhsNode.End && lhsNode.End <= uint32(len(doc.Source)) {
									lhsText := ast.String(doc.Source[lhsNode.Start:lhsNode.End])
									newText.WriteString(lhsText)

									if isLocal {
										attr := ast.Attr(lhsNode.Extra)

										switch attr {
										case ast.AttrConst:
											newText.WriteString(" <const>")
										case ast.AttrClose:
											newText.WriteString(" <close>")
										}
									}
								}
							}
						}

						newText.WriteString(" = ")

						if assignNode.Right != ast.InvalidNode && i < rhsList.Count && rhsList.Extra+uint32(i) < uint32(len(doc.Tree.ExtraList)) {
							rhsID := doc.Tree.ExtraList[rhsList.Extra+uint32(i)]
							if int(rhsID) < len(doc.Tree.Nodes) {
								rhsNode := doc.Tree.Nodes[rhsID]

								if rhsNode.Start <= rhsNode.End && rhsNode.End <= uint32(len(doc.Source)) {
									rhsText := ast.String(doc.Source[rhsNode.Start:rhsNode.End])

									newText.WriteString(rhsText)
								} else {
									newText.WriteString("nil")
								}
							} else {
								newText.WriteString("nil")
							}
						} else {
							newText.WriteString("nil")
						}
					}

					action.Edit = &WorkspaceEdit{
						Changes: map[string][]TextEdit{
							uri: {{
								Range:   getNodeRange(doc.Tree, nodeID),
								NewText: newText.String(),
							}},
						},
					}
				}
			} else if actionType == "swapIfElse" {
				ifNode := doc.Tree.Nodes[nodeID]

				var elseBlockID ast.NodeID

				for i := uint16(0); i < ifNode.Count; i++ {
					if ifNode.Extra+uint32(i) < uint32(len(doc.Tree.ExtraList)) {
						childID := doc.Tree.ExtraList[ifNode.Extra+uint32(i)]
						if int(childID) < len(doc.Tree.Nodes) && doc.Tree.Nodes[childID].Kind == ast.KindElse {
							elseBlockID = childID

							break
						}
					}
				}

				if elseBlockID != ast.InvalidNode {
					elseNode := doc.Tree.Nodes[elseBlockID]
					newCond := s.invertCondition(doc, ifNode.Left)

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

					elseBody := s.flattenBlock(doc, elseNode.Left, innerIndent, 999)
					if elseBody != "" {
						newText.WriteString(innerIndent)
						newText.WriteString(elseBody)
						newText.WriteString("\n")
					}

					newText.WriteString(indent)
					newText.WriteString("else\n")

					thenBody := s.flattenBlock(doc, ifNode.Right, innerIndent, 999)
					if thenBody != "" {
						newText.WriteString(innerIndent)
						newText.WriteString(thenBody)
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
			} else if actionType == "removeParen" {
				parenNode := doc.Tree.Nodes[nodeID]

				if parenNode.Left != ast.InvalidNode && int(parenNode.Left) < len(doc.Tree.Nodes) {
					innerNode := doc.Tree.Nodes[parenNode.Left]

					if innerNode.Start <= innerNode.End && innerNode.End <= uint32(len(doc.Source)) {
						innerSrc := ast.String(doc.Source[innerNode.Start:innerNode.End])

						action.Edit = &WorkspaceEdit{
							Changes: map[string][]TextEdit{
								uri: {{
									Range:   getNodeRange(doc.Tree, nodeID),
									NewText: innerSrc,
								}},
							},
						}
					}
				}
			} else if actionType == "forNumToIpairs" {
				forNode := doc.Tree.Nodes[nodeID]

				if int(forNode.Left) < len(doc.Tree.Nodes) {
					identNode := doc.Tree.Nodes[forNode.Left]

					if identNode.Start <= identNode.End && identNode.End <= uint32(len(doc.Source)) {
						identName := ast.String(doc.Source[identNode.Start:identNode.End])
						tableName, _ := data["table"].(string)

						ifLine, _ := doc.Tree.Position(forNode.Start)

						var lineStart uint32

						if int(ifLine) < len(doc.Tree.LineOffsets) {
							lineStart = doc.Tree.LineOffsets[ifLine]
						}

						var indentBytes []byte

						for i := lineStart; i < forNode.Start && i < uint32(len(doc.Source)); i++ {
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

						newText.WriteString(fmt.Sprintf("for %s, v in ipairs(%s) do\n", identName, tableName))

						body := s.flattenBlock(doc, forNode.Right, innerIndent, 999)
						if body != "" {
							newText.WriteString(innerIndent)
							newText.WriteString(body)
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
			} else if actionType == "indexToMember" {
				indexNode := doc.Tree.Nodes[nodeID]
				propStr, _ := data["prop"].(string)

				if int(indexNode.Left) < len(doc.Tree.Nodes) {
					recNode := doc.Tree.Nodes[indexNode.Left]
					if recNode.Start <= recNode.End && recNode.End <= uint32(len(doc.Source)) {
						recStr := ast.String(doc.Source[recNode.Start:recNode.End])

						action.Edit = &WorkspaceEdit{
							Changes: map[string][]TextEdit{
								uri: {{
									Range:   getNodeRange(doc.Tree, nodeID),
									NewText: recStr + "." + propStr,
								}},
							},
						}
					}
				}
			} else if actionType == "memberToIndex" {
				memberNode := doc.Tree.Nodes[nodeID]

				if int(memberNode.Left) < len(doc.Tree.Nodes) && int(memberNode.Right) < len(doc.Tree.Nodes) {
					recNode := doc.Tree.Nodes[memberNode.Left]
					propNode := doc.Tree.Nodes[memberNode.Right]

					if recNode.Start <= recNode.End && recNode.End <= uint32(len(doc.Source)) &&
						propNode.Start <= propNode.End && propNode.End <= uint32(len(doc.Source)) {

						recStr := ast.String(doc.Source[recNode.Start:recNode.End])
						propStr := ast.String(doc.Source[propNode.Start:propNode.End])

						action.Edit = &WorkspaceEdit{
							Changes: map[string][]TextEdit{
								uri: {{
									Range:   getNodeRange(doc.Tree, nodeID),
									NewText: fmt.Sprintf("%s[\"%s\"]", recStr, propStr),
								}},
							},
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
}

func (s *Server) handleExecuteCommand(req Request) {
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
}

func (s *Server) handlePrepareRename(req Request) {
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
}

func (s *Server) handleRename(req Request) {
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
			if !s.canSeeSymbol(dUri, ctx.TargetURI) {
				continue
			}

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

func (s *Server) handleLinkedEditingRange(req Request) {
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
}

func (s *Server) getSafeFixesForDocument(doc *Document, actualReads []int) []SafeFix {
	var fixes []SafeFix

	if doc.IsMeta {
		return fixes
	}

	if actualReads == nil {
		if cap(s.actualReadsBuf) < len(doc.Tree.Nodes) {
			s.actualReadsBuf = make([]int, len(doc.Tree.Nodes))
		} else {
			s.actualReadsBuf = s.actualReadsBuf[:len(doc.Tree.Nodes)]

			clear(s.actualReadsBuf)
		}

		actualReads = s.actualReadsBuf

		for refID, defID := range doc.Resolver.References {
			if defID != ast.InvalidNode && ast.NodeID(refID) != defID {
				if s.isActualRead(doc, ast.NodeID(refID), defID) {
					actualReads[defID]++
				}
			}
		}
	}

	if cap(s.unusedDefsBuf) < len(doc.Tree.Nodes) {
		s.unusedDefsBuf = make([]bool, len(doc.Tree.Nodes))
	} else {
		s.unusedDefsBuf = s.unusedDefsBuf[:len(doc.Tree.Nodes)]

		clear(s.unusedDefsBuf)
	}

	unusedDefs := s.unusedDefsBuf

	if s.deadStoresBuf == nil {
		s.deadStoresBuf = make(map[ast.NodeID]*DeadStoreInfo)
	} else {
		clear(s.deadStoresBuf)
	}

	deadStores := s.deadStoresBuf

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
		case ast.KindDo:
			if node.Left != ast.InvalidNode && doc.Tree.Nodes[node.Left].Count == 0 {
				fixes = append(fixes, SafeFix{
					Coverage: []ast.NodeID{nodeID},
					Edits: []TextEdit{{
						Range:   s.getStatementRemovalRange(doc, nodeID),
						NewText: "",
					}},
					Title: "Remove empty 'do' block",
				})
			}
		case ast.KindLocalAssign:
			s.processListForFixes(doc, node.Left, node.Right, unusedDefs, deadStores, &fixes, true)

			// Check for redundant values
			lhsList := doc.Tree.Nodes[node.Left]

			if node.Right != ast.InvalidNode {
				rhsList := doc.Tree.Nodes[node.Right]

				if lhsList.Count != rhsList.Count && rhsList.Count > 0 {
					lastRhsID := doc.Tree.ExtraList[rhsList.Extra+uint32(rhsList.Count-1)]
					lastRhsNode := doc.Tree.Nodes[lastRhsID]

					isDynamic := lastRhsNode.Kind == ast.KindCallExpr || lastRhsNode.Kind == ast.KindMethodCall || lastRhsNode.Kind == ast.KindVararg

					if !isDynamic || rhsList.Count > lhsList.Count {
						if rhsList.Count > lhsList.Count {
							firstRedundantID := doc.Tree.ExtraList[rhsList.Extra+uint32(lhsList.Count)]
							prevRhsID := doc.Tree.ExtraList[rhsList.Extra+uint32(lhsList.Count-1)]

							startOff := s.findCommaBefore(doc.Source, doc.Tree.Nodes[firstRedundantID].Start, doc.Tree.Nodes[prevRhsID].End)

							fixes = append(fixes, SafeFix{
								Coverage: []ast.NodeID{nodeID},
								Edits: []TextEdit{{
									Range:   getRange(doc.Tree, startOff, lastRhsNode.End),
									NewText: "",
								}},
								Title: "Remove redundant values",
							})
						}
					}
				}
			}
		case ast.KindAssign:
			// Check for self-assignment
			lhsList := doc.Tree.Nodes[node.Left]
			if node.Right != ast.InvalidNode {
				rhsList := doc.Tree.Nodes[node.Right]
				if lhsList.Count == 1 && rhsList.Count == 1 {
					lID := doc.Tree.ExtraList[lhsList.Extra]
					rID := doc.Tree.ExtraList[rhsList.Extra]

					lSource := doc.Source[doc.Tree.Nodes[lID].Start:doc.Tree.Nodes[lID].End]
					rSource := doc.Source[doc.Tree.Nodes[rID].Start:doc.Tree.Nodes[rID].End]

					if bytes.Equal(lSource, rSource) {
						fixes = append(fixes, SafeFix{
							Coverage: []ast.NodeID{nodeID},
							Edits: []TextEdit{{
								Range:   s.getStatementRemovalRange(doc, nodeID),
								NewText: "",
							}},
							Title: "Remove self-assignment",
						})
					}
				}

				// Check for redundant values
				if lhsList.Count != rhsList.Count && rhsList.Count > 0 {
					lastRhsID := doc.Tree.ExtraList[rhsList.Extra+uint32(rhsList.Count-1)]
					lastRhsNode := doc.Tree.Nodes[lastRhsID]

					isDynamic := lastRhsNode.Kind == ast.KindCallExpr || lastRhsNode.Kind == ast.KindMethodCall || lastRhsNode.Kind == ast.KindVararg

					if !isDynamic || rhsList.Count > lhsList.Count {
						if rhsList.Count > lhsList.Count {
							firstRedundantID := doc.Tree.ExtraList[rhsList.Extra+uint32(lhsList.Count)]
							prevRhsID := doc.Tree.ExtraList[rhsList.Extra+uint32(lhsList.Count-1)]

							startOff := s.findCommaBefore(doc.Source, doc.Tree.Nodes[firstRedundantID].Start, doc.Tree.Nodes[prevRhsID].End)

							fixes = append(fixes, SafeFix{
								Coverage: []ast.NodeID{nodeID},
								Edits: []TextEdit{{
									Range:   getRange(doc.Tree, startOff, lastRhsNode.End),
									NewText: "",
								}},
								Title: "Remove redundant values",
							})
						}
					}
				}
			}
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
			} else {
				pID := node.Parent
				if pID != ast.InvalidNode {
					pNode := doc.Tree.Nodes[pID]
					if pNode.Kind == ast.KindBlock && pNode.Count > 0 && doc.Tree.ExtraList[pNode.Extra+uint32(pNode.Count-1)] == nodeID {
						gpID := pNode.Parent
						if gpID != ast.InvalidNode {
							gpNode := doc.Tree.Nodes[gpID]
							if gpNode.Kind == ast.KindFunctionExpr || gpNode.Kind == ast.KindFile {
								fixes = append(fixes, SafeFix{
									Coverage: []ast.NodeID{nodeID},
									Edits: []TextEdit{{
										Range:   s.getStatementRemovalRange(doc, nodeID),
										NewText: "",
									}},
									Title: "Remove redundant return",
								})
							}
						}
					}
				}
			}
		case ast.KindCallExpr, ast.KindMethodCall:
			var funcIdentID ast.NodeID

			if node.Kind == ast.KindMethodCall {
				funcIdentID = node.Right
			} else {
				funcIdentID = node.Left
				if int(funcIdentID) < len(doc.Tree.Nodes) && doc.Tree.Nodes[funcIdentID].Kind == ast.KindMemberExpr {
					funcIdentID = doc.Tree.Nodes[funcIdentID].Right
				}
			}

			if funcIdentID != ast.InvalidNode && int(funcIdentID) < len(doc.Tree.Nodes) {
				ctx := s.resolveSymbolNode(doc.URI, doc, funcIdentID)
				if ctx != nil {
					var defs []GlobalSymbol

					if len(ctx.GlobalDefs) > 0 {
						defs = ctx.GlobalDefs
					} else if ctx.TargetDefID != ast.InvalidNode {
						defs = []GlobalSymbol{{URI: ctx.TargetURI, NodeID: ctx.TargetDefID}}
					}

					var (
						matchedAny           bool
						maxExpectedArgs      int
						maxExpectedArgsFound bool
					)

					for _, def := range defs {
						tDoc := s.Documents[def.URI]
						if tDoc == nil {
							continue
						}

						valID := tDoc.getAssignedValue(def.NodeID)
						if valID != ast.InvalidNode && int(valID) < len(tDoc.Tree.Nodes) && tDoc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
							funcNode := tDoc.Tree.Nodes[valID]

							var hasVararg bool

							if funcNode.Count > 0 {
								lastParamID := tDoc.Tree.ExtraList[funcNode.Extra+uint32(funcNode.Count-1)]
								if tDoc.Tree.Nodes[lastParamID].Kind == ast.KindVararg {
									hasVararg = true
								}
							}

							if hasVararg {
								matchedAny = true

								break
							}

							hasImplicitSelfCall := node.Kind == ast.KindMethodCall

							var hasImplicitSelfDef bool

							pDefID := tDoc.Tree.Nodes[def.NodeID].Parent
							if pDefID != ast.InvalidNode && int(pDefID) < len(tDoc.Tree.Nodes) && tDoc.Tree.Nodes[pDefID].Kind == ast.KindMethodName {
								hasImplicitSelfDef = true
							}

							var paramOffset int

							if hasImplicitSelfCall && !hasImplicitSelfDef {
								paramOffset = 1
							} else if !hasImplicitSelfCall && hasImplicitSelfDef {
								paramOffset = -1
							}

							expectedArgs := int(funcNode.Count) - paramOffset
							if expectedArgs < 0 {
								expectedArgs = 0
							}

							if int(node.Count) <= expectedArgs {
								matchedAny = true

								break
							}

							if !maxExpectedArgsFound || expectedArgs > maxExpectedArgs {
								maxExpectedArgs = expectedArgs
								maxExpectedArgsFound = true
							}
						}
					}

					if !matchedAny && maxExpectedArgsFound {
						expectedArgs := maxExpectedArgs
						if int(node.Count) > expectedArgs {
							firstRedundantID := doc.Tree.ExtraList[node.Extra+uint32(expectedArgs)]
							lastArgID := doc.Tree.ExtraList[node.Extra+uint32(node.Count-1)]

							var limit uint32

							if expectedArgs > 0 {
								limit = doc.Tree.Nodes[doc.Tree.ExtraList[node.Extra+uint32(expectedArgs-1)]].End
							} else {
								limit = doc.Tree.Nodes[node.Left].End
							}

							startOff := s.findCommaBefore(doc.Source, doc.Tree.Nodes[firstRedundantID].Start, limit)

							fixes = append(fixes, SafeFix{
								Coverage: []ast.NodeID{nodeID},
								Edits: []TextEdit{{
									Range:   getRange(doc.Tree, startOff, doc.Tree.Nodes[lastArgID].End),
									NewText: "",
								}},
								Title: "Remove redundant parameters",
							})
						}
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

	safeName := s.generateSafeName(doc, id, name, true)

	var edits []TextEdit

	edits = append(edits, TextEdit{
		Range:   getNodeRange(doc.Tree, id),
		NewText: safeName,
	})

	for i, refDefID := range doc.Resolver.References {
		if refDefID == id && ast.NodeID(i) != id {
			edits = append(edits, TextEdit{
				Range:   getNodeRange(doc.Tree, ast.NodeID(i)),
				NewText: safeName,
			})
		}
	}

	return SafeFix{
		Coverage: []ast.NodeID{id},
		Edits:    edits,
		Title:    fmt.Sprintf("Rename to '%s'", safeName),
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

func (s *Server) isNameSafe(doc *Document, defID ast.NodeID, newNameBytes []byte) bool {
	newName := string(newNameBytes)
	if !s.isValidIdentifier(newName) {
		return false
	}

	if s.isKnownGlobal(newNameBytes) {
		return false
	}

	hash := ast.HashBytes(newNameBytes)
	if syms, exists := s.GlobalIndex[GlobalKey{ReceiverHash: 0, PropHash: hash}]; exists && len(syms) > 0 {
		return false
	}

	node := doc.Tree.Nodes[defID]
	isSafe := true

	doc.GetLocalsAt(node.Start, func(name []byte, id ast.NodeID) bool {
		if bytes.Equal(name, newNameBytes) && id != defID {
			isSafe = false

			return false
		}

		return true
	})

	if !isSafe {
		return false
	}

	for i, refDefID := range doc.Resolver.References {
		if refDefID == defID && ast.NodeID(i) != defID {
			refNode := doc.Tree.Nodes[i]

			doc.GetLocalsAt(refNode.Start, func(name []byte, id ast.NodeID) bool {
				if bytes.Equal(name, newNameBytes) && id != defID {
					isSafe = false

					return false
				}

				return true
			})

			if !isSafe {
				return false
			}
		}
	}

	return true
}

func (s *Server) generateSafeName(doc *Document, defID ast.NodeID, baseName string, isIgnore bool) string {
	buf := make([]byte, 0, len(baseName)+16)

	check := func() string {
		if s.isNameSafe(doc, defID, buf) {
			return string(buf)
		}

		return ""
	}

	if isIgnore {
		buf = append(buf[:0], '_')
		buf = append(buf, baseName...)

		if res := check(); res != "" {
			return res
		}

		buf = append(buf[:0], '_')
		buf = append(buf, baseName...)
		buf = append(buf, "_ignored"...)

		if res := check(); res != "" {
			return res
		}

		for i := 2; i < 100; i++ {
			buf = append(buf[:0], '_')
			buf = append(buf, baseName...)
			buf = append(buf, '_')
			buf = strconv.AppendInt(buf, int64(i), 10)

			if res := check(); res != "" {
				return res
			}
		}

		return "_" + baseName
	}

	var (
		prefix   string
		coreName string
	)

	if len(baseName) >= 2 && baseName[0] >= 'a' && baseName[0] <= 'z' && baseName[1] >= 'A' && baseName[1] <= 'Z' {
		prefix = baseName[:1]
		coreName = baseName[1:]
	} else if len(baseName) >= 2 && baseName[0] == '_' {
		prefix = "_"
		coreName = baseName[1:]
	} else {
		coreName = baseName
	}

	var (
		hasUnderscore bool
		hasLower      bool
		hasUpper      bool
		firstUpper    bool
	)

	if len(coreName) > 0 {
		firstUpper = coreName[0] >= 'A' && coreName[0] <= 'Z'
	}

	for i := 0; i < len(coreName); i++ {
		c := coreName[i]

		if c == '_' {
			hasUnderscore = true
		} else if c >= 'a' && c <= 'z' {
			hasLower = true
		} else if c >= 'A' && c <= 'Z' {
			hasUpper = true
		}
	}

	isUpper := hasUpper && !hasLower
	isSnake := hasUnderscore && !hasUpper
	isCamel := !hasUnderscore && hasLower && hasUpper && !firstUpper
	isPascal := !hasUnderscore && hasLower && hasUpper && firstUpper

	buildCandidate := func(modLower, modPascal, modUpper string) string {
		buf = buf[:0]
		buf = append(buf, prefix...)

		if isUpper {
			buf = append(buf, modUpper...)

			if len(coreName) > 0 && coreName[0] != '_' {
				buf = append(buf, '_')
			}

			buf = append(buf, coreName...)
		} else if isSnake || (hasUnderscore && !hasUpper) {
			buf = append(buf, modLower...)

			if len(coreName) > 0 && coreName[0] != '_' {
				buf = append(buf, '_')
			}

			buf = append(buf, coreName...)
		} else if isPascal || (prefix != "" && prefix != "_") {
			buf = append(buf, modPascal...)
			buf = append(buf, coreName...)
		} else if isCamel {
			buf = append(buf, modLower...)

			if len(coreName) > 0 {
				buf = append(buf, coreName[0]-32)
				buf = append(buf, coreName[1:]...)
			}
		} else {
			buf = append(buf, modLower...)
			if len(coreName) > 0 {
				c := coreName[0]

				if c >= 'a' && c <= 'z' {
					buf = append(buf, c-32)
					buf = append(buf, coreName[1:]...)
				} else {
					buf = append(buf, '_')
					buf = append(buf, coreName...)
				}
			}
		}

		return check()
	}

	pID := doc.Tree.Nodes[defID].Parent
	if pID != ast.InvalidNode && int(pID) < len(doc.Tree.Nodes) {
		if doc.Tree.Nodes[pID].Kind == ast.KindFunctionExpr {
			if prefix != "p" {
				if res := buildCandidate("p", "P", "P"); res != "" {
					return res
				}
			}
		}
	}

	if res := buildCandidate("new", "New", "NEW"); res != "" {
		return res
	}

	if res := buildCandidate("local", "Local", "LOCAL"); res != "" {
		return res
	}

	buf = append(buf[:0], baseName...)

	if isUpper {
		buf = append(buf, "_VAL"...)
	} else if isSnake {
		buf = append(buf, "_val"...)
	} else {
		buf = append(buf, "Val"...)
	}
	if res := check(); res != "" {
		return res
	}

	buf = append(buf[:0], baseName...)

	if isUpper {
		buf = append(buf, "_COPY"...)
	} else if isSnake {
		buf = append(buf, "_copy"...)
	} else {
		buf = append(buf, "Copy"...)
	}

	if res := check(); res != "" {
		return res
	}

	for i := 2; i < 100; i++ {
		buf = append(buf[:0], baseName...)
		buf = append(buf, '_')
		buf = strconv.AppendInt(buf, int64(i), 10)

		if res := check(); res != "" {
			return res
		}
	}

	return baseName + "_new"
}

func (s *Server) isValidIdentifier(name string) bool {
	if len(name) == 0 {
		return false
	}

	if !((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z') || name[0] == '_') {
		return false
	}

	for i := 1; i < len(name); i++ {
		c := name[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}

	return !slices.Contains(luaKeywords, name)
}
