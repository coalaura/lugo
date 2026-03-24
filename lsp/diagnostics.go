package lsp

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/coalaura/lugo/ast"
)

type DepInfo struct {
	IsDep bool
	Msg   string
}

func (s *Server) publishWorkspaceDiagnostics() {
	start := time.Now()

	var diagCount int

	for uri := range s.Documents {
		if s.isWorkspaceURI(uri) || s.OpenFiles[uri] {
			s.publishDiagnostics(uri)

			diagCount++
		}
	}

	took := time.Since(start)

	s.Log.Printf("Published diagnostics for %d files in %s\n", diagCount, took)
}

func (s *Server) publishDiagnostics(uri string) {
	if s.IsIndexing {
		return
	}

	if strings.HasPrefix(uri, "std://") {
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

	if doc.IsMeta {
		WriteMessage(s.Writer, OutgoingNotification{
			RPC:    "2.0",
			Method: "textDocument/publishDiagnostics",
			Params: PublishDiagnosticsParams{
				URI:         uri,
				Diagnostics: s.diagBuf,
			},
		})

		return
	}

	// 2. Undefined Globals
	if s.DiagUndefinedGlobals {
		suggestCache := make(map[string]string)

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
				identStr := ast.String(identBytes)

				suggestion, ok := suggestCache[identStr]
				if !ok {
					suggestion = s.suggestGlobal(identStr)
					suggestCache[identStr] = suggestion
				}

				msg := fmt.Sprintf("Undefined global '%s'.", identStr)

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

			var isDefinedAtRoot bool

			if syms, ok := s.GlobalIndex[key]; ok {
				for _, sym := range syms {
					if symDoc, docOk := s.Documents[sym.URI]; docOk {
						if isRootLevel(symDoc.Tree, sym.NodeID) {
							isDefinedAtRoot = true

							break
						}
					}
				}
			}

			if isDefinedAtRoot {
				continue
			}

			s.diagBuf = append(s.diagBuf, Diagnostic{
				Range:    getNodeRange(doc.Tree, defID),
				Severity: SeverityWarning,
				Code:     "implicit-global",
				Message:  fmt.Sprintf("Implicit global creation '%s'. Did you forget the 'local' keyword?", ast.String(identBytes)),
			})
		}
	}

	// 4. Shadowing & Unused Variables
	if s.DiagShadowing || s.DiagShadowingLoopVar || s.DiagUnusedLocal || s.DiagUnusedFunction || s.DiagUnusedParameter || s.DiagUnusedLoopVar {
		actualReads := make([]int, len(doc.Tree.Nodes))

		for refID, defID := range doc.Resolver.References {
			if defID != ast.InvalidNode && ast.NodeID(refID) != defID {
				if s.isActualRead(doc, ast.NodeID(refID), defID) {
					actualReads[defID]++
				}
			}
		}

		fixes := s.getSafeFixesForDocument(doc, actualReads)

		fixMap := make([]string, len(doc.Tree.Nodes))

		for _, f := range fixes {
			for _, id := range f.Coverage {
				fixMap[id] = f.Title
			}
		}

		for _, defID := range doc.Resolver.LocalDefs {
			node := doc.Tree.Nodes[defID]
			if node.Start == node.End {
				continue
			}

			nameBytes := doc.Source[node.Start:node.End]

			if len(nameBytes) > 0 && nameBytes[0] == '_' {
				continue
			}

			r := getNodeRange(doc.Tree, defID)

			if actualReads[defID] == 0 {
				if ast.Attr(doc.Tree.Nodes[defID].Extra) == ast.AttrClose {
					continue
				}

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
					msg = fmt.Sprintf("Unused %s '%s'.", category, ast.String(nameBytes))

					if fixTitle := fixMap[defID]; fixTitle != "" {
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

			var isLoopVar bool

			pID := doc.Tree.Nodes[defID].Parent
			if pID != ast.InvalidNode {
				pNode := doc.Tree.Nodes[pID]
				if pNode.Kind == ast.KindForNum && pNode.Left == defID {
					isLoopVar = true
				} else if pNode.Kind == ast.KindNameList {
					gpID := pNode.Parent
					if gpID != ast.InvalidNode && doc.Tree.Nodes[gpID].Kind == ast.KindForIn {
						isLoopVar = true
					}
				}
			}

			shouldReportShadow := s.DiagShadowing
			if isLoopVar {
				shouldReportShadow = s.DiagShadowingLoopVar
			}

			if shouldReportShadow {
				varType := "Local"
				if isLoopVar {
					varType = "Loop"
				}

				if s.isKnownGlobal(nameBytes) {
					s.diagBuf = append(s.diagBuf, Diagnostic{
						Range:    r,
						Severity: SeverityWarning,
						Code:     "shadow-global",
						Message:  fmt.Sprintf("%s variable '%s' shadows a known global.", varType, ast.String(nameBytes)),
					})
				} else {
					hash := ast.HashBytes(nameBytes)

					if syms, exists := s.GlobalIndex[GlobalKey{ReceiverHash: 0, PropHash: hash}]; exists && len(syms) > 0 {
						sym := syms[0]

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
								Message: fmt.Sprintf("Global '%s' defined here%s", ast.String(nameBytes), fromFile),
							})
						}

						s.diagBuf = append(s.diagBuf, Diagnostic{
							Range:              r,
							Severity:           SeverityWarning,
							Code:               "shadow-global",
							Message:            fmt.Sprintf("%s variable '%s' shadows a global definition.", varType, ast.String(nameBytes)),
							RelatedInformation: related,
						})
					}
				}
			}
		}
	}

	// 5. Shadowing Outer Locals
	if s.DiagShadowing || s.DiagShadowingLoopVar {
		for _, pair := range doc.Resolver.ShadowedOuter {
			var isLoopVar bool

			pID := doc.Tree.Nodes[pair.Shadowing].Parent
			if pID != ast.InvalidNode {
				pNode := doc.Tree.Nodes[pID]
				if pNode.Kind == ast.KindForNum && pNode.Left == pair.Shadowing {
					isLoopVar = true
				} else if pNode.Kind == ast.KindNameList {
					gpID := pNode.Parent
					if gpID != ast.InvalidNode && doc.Tree.Nodes[gpID].Kind == ast.KindForIn {
						isLoopVar = true
					}
				}
			}

			if !s.DiagShadowing && !isLoopVar {
				continue
			}

			if !s.DiagShadowingLoopVar && isLoopVar {
				continue
			}

			node := doc.Tree.Nodes[pair.Shadowing]
			nameBytes := doc.Source[node.Start:node.End]

			var related []DiagnosticRelatedInformation

			related = append(related, DiagnosticRelatedInformation{
				Location: Location{
					URI:   uri,
					Range: getNodeRange(doc.Tree, pair.Shadowed),
				},
				Message: fmt.Sprintf("Outer local '%s' defined here", ast.String(nameBytes)),
			})

			varType := "Local"
			if isLoopVar {
				varType = "Loop"
			}

			s.diagBuf = append(s.diagBuf, Diagnostic{
				Range:              getNodeRange(doc.Tree, pair.Shadowing),
				Severity:           SeverityWarning,
				Code:               "shadow-outer",
				Message:            fmt.Sprintf("%s variable '%s' shadows a variable from an outer scope.", varType, ast.String(nameBytes)),
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
		depCache := make(map[ast.NodeID]DepInfo)

		checkDep := func(d *Document, id ast.NodeID) DepInfo {
			if info, ok := depCache[id]; ok && d == doc {
				return info
			}
			isDep, msg := d.HasDeprecatedTag(id)
			info := DepInfo{isDep, msg}
			if d == doc {
				depCache[id] = info
			}
			return info
		}

		// Check all resolved local references and locally resolved fields
		for i, defID := range doc.Resolver.References {
			if defID != ast.InvalidNode && defID != ast.NodeID(i) {
				if doc.Tree.Nodes[i].Kind != ast.KindIdent {
					continue
				}

				info := checkDep(doc, defID)
				if info.IsDep {
					identBytes := doc.Source[doc.Tree.Nodes[i].Start:doc.Tree.Nodes[i].End]
					diagMsg := fmt.Sprintf("Use of deprecated symbol '%s'", ast.String(identBytes))

					if info.Msg != "" {
						diagMsg += ": " + info.Msg
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

		// Check unresolved global references
		for _, refID := range doc.Resolver.GlobalRefs {
			identBytes := doc.Source[doc.Tree.Nodes[refID].Start:doc.Tree.Nodes[refID].End]
			hash := ast.HashBytes(identBytes)

			if syms, ok := s.getGlobalSymbols(0, hash); ok && len(syms) > 0 && syms[0].NodeID != ast.InvalidNode {
				sym := syms[0]
				if symDoc, docOk := s.Documents[sym.URI]; docOk {
					info := checkDep(symDoc, sym.NodeID)
					if info.IsDep {
						diagMsg := fmt.Sprintf("Use of deprecated symbol '%s'", ast.String(identBytes))

						if info.Msg != "" {
							diagMsg += ": " + info.Msg
						} else {
							diagMsg += "."
						}

						s.diagBuf = append(s.diagBuf, Diagnostic{
							Range:    getNodeRange(doc.Tree, refID),
							Severity: SeverityHint,
							Code:     "deprecated",
							Tags:     []DiagnosticTag{Deprecated},
							Message:  diagMsg,
						})
					}
				}
			}
		}

		// Check unresolved global field accesses
		for _, pf := range doc.Resolver.PendingFields {
			if doc.Resolver.References[pf.PropNodeID] == ast.InvalidNode && pf.ReceiverHash != 0 {
				if syms, ok := s.getGlobalSymbols(pf.ReceiverHash, pf.PropHash); ok && len(syms) > 0 && syms[0].NodeID != ast.InvalidNode {
					sym := syms[0]
					if symDoc, docOk := s.Documents[sym.URI]; docOk {
						info := checkDep(symDoc, sym.NodeID)
						if info.IsDep {
							identBytes := doc.Source[doc.Tree.Nodes[pf.PropNodeID].Start:doc.Tree.Nodes[pf.PropNodeID].End]
							diagMsg := fmt.Sprintf("Use of deprecated symbol '%s'", ast.String(identBytes))

							if info.Msg != "" {
								diagMsg += ": " + info.Msg
							} else {
								diagMsg += "."
							}

							s.diagBuf = append(s.diagBuf, Diagnostic{
								Range:    getNodeRange(doc.Tree, pf.PropNodeID),
								Severity: SeverityHint,
								Code:     "deprecated",
								Tags:     []DiagnosticTag{Deprecated},
								Message:  diagMsg,
							})
						}
					}
				}
			}
		}
	}

	// 8. Syntax & Correctness Checks
	for i := 1; i < len(doc.Tree.Nodes); i++ {
		nodeID := ast.NodeID(i)
		node := doc.Tree.Nodes[nodeID]

		// Loop Variable Mutation
		if s.DiagLoopVarMutation && (node.Kind == ast.KindAssign) {
			lhsList := doc.Tree.Nodes[node.Left]

			for j := uint16(0); j < lhsList.Count; j++ {
				lhsID := doc.Tree.ExtraList[lhsList.Extra+uint32(j)]
				if doc.Tree.Nodes[lhsID].Kind == ast.KindIdent {
					defID := doc.Resolver.References[lhsID]
					if defID != ast.InvalidNode {
						pID := doc.Tree.Nodes[defID].Parent
						if pID != ast.InvalidNode {
							var isLoopVar bool

							pNode := doc.Tree.Nodes[pID]
							if pNode.Kind == ast.KindForNum && pNode.Left == defID {
								isLoopVar = true
							} else if pNode.Kind == ast.KindNameList {
								gpID := pNode.Parent
								if gpID != ast.InvalidNode && doc.Tree.Nodes[gpID].Kind == ast.KindForIn {
									isLoopVar = true
								}
							}

							if isLoopVar {
								s.diagBuf = append(s.diagBuf, Diagnostic{
									Range:    getNodeRange(doc.Tree, lhsID),
									Severity: SeverityWarning,
									Code:     "loop-var-mutation",
									Message:  "Mutation of a loop variable. This can lead to unexpected behavior.",
								})
							}
						}
					}
				}
			}
		}

		// Empty block
		if s.DiagEmptyBlock && node.Kind == ast.KindBlock && node.Count == 0 {
			if node.Parent != ast.InvalidNode && doc.Tree.Nodes[node.Parent].Kind != ast.KindFile {
				var data any

				pNode := doc.Tree.Nodes[node.Parent]
				if pNode.Kind == ast.KindDo {
					data = float64(node.Parent)
				}

				s.diagBuf = append(s.diagBuf, Diagnostic{
					Range:    getNodeRange(doc.Tree, nodeID),
					Severity: SeverityHint,
					Code:     "empty-block",
					Tags:     []DiagnosticTag{Unnecessary},
					Message:  "This block is empty.",
					Data:     data,
				})
			}
		}

		// Incorrect Vararg Usage
		if s.DiagIncorrectVararg && node.Kind == ast.KindVararg {
			pID := node.Parent
			if pID != ast.InvalidNode && doc.Tree.Nodes[pID].Kind != ast.KindFunctionExpr {
				var (
					isVarargFunc bool
					foundFunc    bool
				)

				curr := pID

				for curr != ast.InvalidNode {
					n := doc.Tree.Nodes[curr]
					if n.Kind == ast.KindFunctionExpr {
						foundFunc = true

						if n.Count > 0 {
							lastParamID := doc.Tree.ExtraList[n.Extra+uint32(n.Count-1)]
							if doc.Tree.Nodes[lastParamID].Kind == ast.KindVararg {
								isVarargFunc = true
							}
						}

						break
					} else if n.Kind == ast.KindFile {
						foundFunc = true
						isVarargFunc = true

						break
					}

					curr = n.Parent
				}

				if foundFunc && !isVarargFunc {
					s.diagBuf = append(s.diagBuf, Diagnostic{
						Range:    getNodeRange(doc.Tree, nodeID),
						Severity: SeverityError,
						Code:     "incorrect-vararg",
						Message:  "Cannot use '...' outside a vararg function.",
					})
				}
			}
		}

		// Duplicate table fields
		if s.DiagDuplicateField && node.Kind == ast.KindTableExpr {
			seenKeys := make(map[uint64]ast.NodeID)

			for j := uint16(0); j < node.Count; j++ {
				fieldID := doc.Tree.ExtraList[node.Extra+uint32(j)]
				fieldNode := doc.Tree.Nodes[fieldID]

				if fieldNode.Kind == ast.KindRecordField {
					keyNode := doc.Tree.Nodes[fieldNode.Left]
					if keyNode.Kind == ast.KindIdent {
						keyBytes := doc.Source[keyNode.Start:keyNode.End]
						hash := ast.HashBytes(keyBytes)

						if prevID, exists := seenKeys[hash]; exists {
							s.diagBuf = append(s.diagBuf, Diagnostic{
								Range:    getNodeRange(doc.Tree, fieldNode.Left),
								Severity: SeverityWarning,
								Code:     "duplicate-field",
								Message:  fmt.Sprintf("Duplicate field '%s' in table.", ast.String(keyBytes)),
								RelatedInformation: []DiagnosticRelatedInformation{
									{
										Location: Location{URI: uri, Range: getNodeRange(doc.Tree, prevID)},
										Message:  "Previously defined here",
									},
								},
							})
						} else {
							seenKeys[hash] = fieldNode.Left
						}
					}
				}
			}
		}

		// Unbalanced assignments & Redundant values
		if (s.DiagUnbalancedAssignment || s.DiagRedundantValue) && (node.Kind == ast.KindLocalAssign || node.Kind == ast.KindAssign) {
			lhsList := doc.Tree.Nodes[node.Left]
			if node.Right != ast.InvalidNode {
				rhsList := doc.Tree.Nodes[node.Right]

				if lhsList.Count != rhsList.Count && rhsList.Count > 0 {
					lastRhsID := doc.Tree.ExtraList[rhsList.Extra+uint32(rhsList.Count-1)]
					lastRhsNode := doc.Tree.Nodes[lastRhsID]

					isDynamic := lastRhsNode.Kind == ast.KindCallExpr || lastRhsNode.Kind == ast.KindMethodCall || lastRhsNode.Kind == ast.KindVararg

					if !isDynamic || rhsList.Count > lhsList.Count {
						if rhsList.Count > lhsList.Count && s.DiagRedundantValue {
							firstRedundantID := doc.Tree.ExtraList[rhsList.Extra+uint32(lhsList.Count)]
							prevRhsID := doc.Tree.ExtraList[rhsList.Extra+uint32(lhsList.Count-1)]

							startOff := s.findCommaBefore(doc.Source, doc.Tree.Nodes[firstRedundantID].Start, doc.Tree.Nodes[prevRhsID].End)

							s.diagBuf = append(s.diagBuf, Diagnostic{
								Range:    getRange(doc.Tree, startOff, lastRhsNode.End),
								Severity: SeverityWarning,
								Code:     "redundant-value",
								Tags:     []DiagnosticTag{Unnecessary},
								Message:  "Assigning more values than variables; excess values will be discarded.",
								Data:     float64(nodeID),
							})
						} else if lhsList.Count > rhsList.Count && s.DiagUnbalancedAssignment && !isDynamic {
							firstUnbalancedID := doc.Tree.ExtraList[lhsList.Extra+uint32(rhsList.Count)]
							lastLhsID := doc.Tree.ExtraList[lhsList.Extra+uint32(lhsList.Count-1)]

							s.diagBuf = append(s.diagBuf, Diagnostic{
								Range:    getRange(doc.Tree, doc.Tree.Nodes[firstUnbalancedID].Start, doc.Tree.Nodes[lastLhsID].End),
								Severity: SeverityWarning,
								Code:     "unbalanced-assignment",
								Message:  "Assigning fewer values than variables; these variables will be initialized to nil.",
							})
						}
					}
				}
			}
		}

		// Unreachable Elseif/Else
		if s.DiagUnreachableElse && node.Kind == ast.KindIf {
			var foundTruthy bool

			if s.isStaticallyConstant(doc, node.Left) {
				res, ok := doc.evalNode(node.Left, 0)
				if ok {
					isTruthy := res.kind != ast.KindFalse && res.kind != ast.KindNil
					if isTruthy {
						foundTruthy = true
					} else {
						s.diagBuf = append(s.diagBuf, Diagnostic{
							Range:    getNodeRange(doc.Tree, node.Right),
							Severity: SeverityWarning,
							Code:     "unreachable-branch",
							Tags:     []DiagnosticTag{Unnecessary},
							Message:  "This branch is unreachable because the condition is always falsy.",
						})
					}
				}
			}

			for j := uint16(0); j < node.Count; j++ {
				if node.Extra+uint32(j) >= uint32(len(doc.Tree.ExtraList)) {
					continue
				}

				childID := doc.Tree.ExtraList[node.Extra+uint32(j)]
				if int(childID) >= len(doc.Tree.Nodes) {
					continue
				}

				childNode := doc.Tree.Nodes[childID]

				if foundTruthy {
					s.diagBuf = append(s.diagBuf, Diagnostic{
						Range:    getNodeRange(doc.Tree, childID),
						Severity: SeverityWarning,
						Code:     "unreachable-branch",
						Tags:     []DiagnosticTag{Unnecessary},
						Message:  "This branch is unreachable because a previous condition is always truthy.",
					})

					continue
				}

				if childNode.Kind == ast.KindElseIf {
					if s.isStaticallyConstant(doc, childNode.Left) {
						res, ok := doc.evalNode(childNode.Left, 0)
						if ok {
							isTruthy := res.kind != ast.KindFalse && res.kind != ast.KindNil
							if isTruthy {
								foundTruthy = true
							} else {
								s.diagBuf = append(s.diagBuf, Diagnostic{
									Range:    getNodeRange(doc.Tree, childNode.Right),
									Severity: SeverityWarning,
									Code:     "unreachable-branch",
									Tags:     []DiagnosticTag{Unnecessary},
									Message:  "This branch is unreachable because the condition is always falsy.",
								})
							}
						}
					}
				}
			}
		}

		// Self assignment
		if s.DiagSelfAssignment && node.Kind == ast.KindAssign {
			lhsList := doc.Tree.Nodes[node.Left]
			if node.Right != ast.InvalidNode {
				rhsList := doc.Tree.Nodes[node.Right]

				maxCheck := min(rhsList.Count, lhsList.Count)

				for j := range maxCheck {
					lID := doc.Tree.ExtraList[lhsList.Extra+uint32(j)]
					rID := doc.Tree.ExtraList[rhsList.Extra+uint32(j)]

					lSource := doc.Source[doc.Tree.Nodes[lID].Start:doc.Tree.Nodes[lID].End]
					rSource := doc.Source[doc.Tree.Nodes[rID].Start:doc.Tree.Nodes[rID].End]

					if bytes.Equal(lSource, rSource) {
						s.diagBuf = append(s.diagBuf, Diagnostic{
							Range:    getRange(doc.Tree, doc.Tree.Nodes[lID].Start, doc.Tree.Nodes[rID].End),
							Severity: SeverityWarning,
							Code:     "self-assignment",
							Tags:     []DiagnosticTag{Unnecessary},
							Message:  fmt.Sprintf("Assigning variable '%s' to itself.", ast.String(lSource)),
							Data:     float64(nodeID),
						})
					}
				}
			}
		}

		// Type check: call non-function
		if s.DiagTypeCheck && (node.Kind == ast.KindCallExpr || node.Kind == ast.KindMethodCall) {
			var funcID ast.NodeID

			if node.Kind == ast.KindMethodCall {
				funcID = node.Left
			} else {
				funcID = node.Left
			}

			switch node.Kind {
			case ast.KindCallExpr:
				t := doc.InferType(funcID)
				if t.Basics != TypeUnknown && t.CustomName == "" && t.Basics&(TypeFunction|TypeAny|TypeTable|TypeUserdata) == 0 {
					s.diagBuf = append(s.diagBuf, Diagnostic{
						Range:    getNodeRange(doc.Tree, funcID),
						Severity: SeverityWarning,
						Code:     "call-non-function",
						Message:  fmt.Sprintf("Attempt to call a non-function value (inferred type: %s).", t.Format()),
					})
				}
			case ast.KindMethodCall:
				t := doc.InferType(funcID)
				if t.Basics != TypeUnknown && t.CustomName == "" && t.Basics&(TypeTable|TypeAny|TypeString|TypeUserdata) == 0 {
					s.diagBuf = append(s.diagBuf, Diagnostic{
						Range:    getNodeRange(doc.Tree, funcID),
						Severity: SeverityWarning,
						Code:     "index-non-table",
						Message:  fmt.Sprintf("Attempt to index a non-table value (inferred type: %s).", t.Format()),
					})
				}
			}
		}

		// Type check: index non-table
		if s.DiagTypeCheck && (node.Kind == ast.KindMemberExpr || node.Kind == ast.KindIndexExpr) {
			t := doc.InferType(node.Left)
			if t.Basics != TypeUnknown && t.CustomName == "" && t.Basics&(TypeTable|TypeAny|TypeString|TypeUserdata) == 0 {
				s.diagBuf = append(s.diagBuf, Diagnostic{
					Range:    getNodeRange(doc.Tree, node.Left),
					Severity: SeverityWarning,
					Code:     "index-non-table",
					Message:  fmt.Sprintf("Attempt to index a non-table value (inferred type: %s).", t.Format()),
				})
			}
		}

		// Redundant Return
		if s.DiagRedundantReturn && node.Kind == ast.KindReturn {
			if node.Left == ast.InvalidNode || doc.Tree.Nodes[node.Left].Count == 0 {
				pID := node.Parent
				if pID != ast.InvalidNode {
					pNode := doc.Tree.Nodes[pID]
					if pNode.Kind == ast.KindBlock && pNode.Count > 0 && doc.Tree.ExtraList[pNode.Extra+uint32(pNode.Count-1)] == nodeID {
						gpID := pNode.Parent
						if gpID != ast.InvalidNode {
							gpNode := doc.Tree.Nodes[gpID]
							if gpNode.Kind == ast.KindFunctionExpr || gpNode.Kind == ast.KindFile {
								s.diagBuf = append(s.diagBuf, Diagnostic{
									Range:    getNodeRange(doc.Tree, nodeID),
									Severity: SeverityWarning,
									Code:     "redundant-return",
									Tags:     []DiagnosticTag{Unnecessary},
									Message:  "Redundant return statement. The function will exit here anyway.",
									Data:     float64(nodeID),
								})
							}
						}
					}
				}
			}
		}

		// Redundant Parameter
		if s.DiagRedundantParameter && (node.Kind == ast.KindCallExpr || node.Kind == ast.KindMethodCall) {
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
				ctx := s.resolveSymbolNode(uri, doc, funcIdentID)
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

							paramOffset := 0
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

							s.diagBuf = append(s.diagBuf, Diagnostic{
								Range:    getRange(doc.Tree, startOff, doc.Tree.Nodes[lastArgID].End),
								Severity: SeverityWarning,
								Code:     "redundant-parameter",
								Tags:     []DiagnosticTag{Unnecessary},
								Message:  fmt.Sprintf("Function expects %d argument(s), but got %d. Excess arguments will be ignored.", expectedArgs, node.Count),
								Data:     float64(nodeID),
							})
						}
					}
				}
			}
		}

		// Format string validation
		if s.DiagFormatString && (node.Kind == ast.KindCallExpr || node.Kind == ast.KindMethodCall) {
			var (
				isFormatCall     bool
				formatStringNode ast.NodeID
				numArgsProvided  int
			)

			switch node.Kind {
			case ast.KindCallExpr:
				if int(node.Left) < len(doc.Tree.Nodes) {
					leftNode := doc.Tree.Nodes[node.Left]
					if leftNode.Kind == ast.KindMemberExpr {
						recID := leftNode.Left
						propID := leftNode.Right
						if int(recID) < len(doc.Tree.Nodes) && int(propID) < len(doc.Tree.Nodes) {
							recNode := doc.Tree.Nodes[recID]
							propNode := doc.Tree.Nodes[propID]

							if recNode.Start <= recNode.End && recNode.End <= uint32(len(doc.Source)) &&
								propNode.Start <= propNode.End && propNode.End <= uint32(len(doc.Source)) {

								recBytes := doc.Source[recNode.Start:recNode.End]
								propBytes := doc.Source[propNode.Start:propNode.End]

								if bytes.Equal(recBytes, []byte("string")) && bytes.Equal(propBytes, []byte("format")) {
									isFormatCall = true

									if node.Count > 0 {
										formatStringNode = doc.Tree.ExtraList[node.Extra]
										numArgsProvided = int(node.Count) - 1
									}
								}
							}
						}
					}
				}
			case ast.KindMethodCall:
				if int(node.Left) < len(doc.Tree.Nodes) && int(node.Right) < len(doc.Tree.Nodes) {
					recNode := doc.Tree.Nodes[node.Left]
					propNode := doc.Tree.Nodes[node.Right]

					if propNode.Start <= propNode.End && propNode.End <= uint32(len(doc.Source)) {
						propBytes := doc.Source[propNode.Start:propNode.End]

						if recNode.Kind == ast.KindString && bytes.Equal(propBytes, []byte("format")) {
							isFormatCall = true

							formatStringNode = node.Left
							numArgsProvided = int(node.Count)
						}
					}
				}
			}

			if isFormatCall && formatStringNode != ast.InvalidNode && int(formatStringNode) < len(doc.Tree.Nodes) {
				fmtNode := doc.Tree.Nodes[formatStringNode]
				if fmtNode.Kind == ast.KindString && fmtNode.Start <= fmtNode.End && fmtNode.End <= uint32(len(doc.Source)) {
					var hasDynamicArgs bool

					for j := uint16(0); j < node.Count; j++ {
						if node.Extra+uint32(j) >= uint32(len(doc.Tree.ExtraList)) {
							continue
						}

						argID := doc.Tree.ExtraList[node.Extra+uint32(j)]
						if int(argID) < len(doc.Tree.Nodes) {
							argKind := doc.Tree.Nodes[argID].Kind
							if argKind == ast.KindVararg || argKind == ast.KindCallExpr || argKind == ast.KindMethodCall {
								hasDynamicArgs = true

								break
							}
						}
					}

					if !hasDynamicArgs {
						fmtBytes := doc.Source[fmtNode.Start:fmtNode.End]

						var (
							expectedArgs int
							inSpecifier  bool
						)

						for j := range fmtBytes {
							c := fmtBytes[j]

							if !inSpecifier {
								if c == '%' {
									inSpecifier = true
								}
							} else {
								if c == '%' {
									inSpecifier = false // %% escaped
								} else if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
									expectedArgs++

									inSpecifier = false
								}
							}
						}

						if expectedArgs != numArgsProvided {
							msg := fmt.Sprintf("Format string expects %d argument(s), but got %d.", expectedArgs, numArgsProvided)

							s.diagBuf = append(s.diagBuf, Diagnostic{
								Range:    getNodeRange(doc.Tree, nodeID),
								Severity: SeverityWarning,
								Code:     "format-string",
								Message:  msg,
							})
						}
					}
				}
			}
		}
	}

	// 9. Duplicate Locals
	if s.DiagDuplicateLocal {
		for _, defID := range doc.Resolver.DuplicateLocals {
			node := doc.Tree.Nodes[defID]
			nameBytes := doc.Source[node.Start:node.End]

			s.diagBuf = append(s.diagBuf, Diagnostic{
				Range:    getNodeRange(doc.Tree, defID),
				Severity: SeverityWarning,
				Code:     "duplicate-local",
				Message:  fmt.Sprintf("Local variable '%s' is already defined in the current scope.", ast.String(nameBytes)),
			})
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

func (s *Server) isActualRead(doc *Document, refID ast.NodeID, defID ast.NodeID) bool {
	curr := refID

	for curr != ast.InvalidNode && int(curr) < len(doc.Tree.Nodes) {
		parentID := doc.Tree.Nodes[curr].Parent
		if parentID == ast.InvalidNode || int(parentID) >= len(doc.Tree.Nodes) {
			return true
		}

		pNode := doc.Tree.Nodes[parentID]

		switch pNode.Kind {
		case ast.KindMemberExpr, ast.KindMethodName, ast.KindIndexExpr:
			if pNode.Left == curr {
				if isLHSOfAssignment(doc, parentID) {
					return true
				}

				curr = parentID

				continue
			}

			return true
		case ast.KindLocalAssign, ast.KindAssign:
			if pNode.Left == curr {
				return false
			}
			if pNode.Right == curr {
				if int(pNode.Left) < len(doc.Tree.Nodes) {
					lhsList := doc.Tree.Nodes[pNode.Left]
					if lhsList.Count == 1 && lhsList.Extra < uint32(len(doc.Tree.ExtraList)) {
						lhsExprID := doc.Tree.ExtraList[lhsList.Extra]
						if s.getRootDef(doc, lhsExprID) == defID {
							return false
						}
					}
				}
			}

			return true
		case ast.KindExprList, ast.KindNameList:
			if pNode.Kind == ast.KindExprList {
				gpID := pNode.Parent
				if gpID != ast.InvalidNode && int(gpID) < len(doc.Tree.Nodes) {
					gpNode := doc.Tree.Nodes[gpID]
					if (gpNode.Kind == ast.KindAssign || gpNode.Kind == ast.KindLocalAssign) && gpNode.Left == parentID {
						return false
					}
				}
			}

			curr = parentID

			continue
		case ast.KindParenExpr, ast.KindBinaryExpr, ast.KindUnaryExpr:
			curr = parentID

			continue
		default:
			return true
		}
	}

	return true
}

func (s *Server) isSideEffectFree(doc *Document, id ast.NodeID) bool {
	if id == ast.InvalidNode || int(id) >= len(doc.Tree.Nodes) {
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
			if node.Extra+uint32(i) >= uint32(len(doc.Tree.ExtraList)) || !s.isSideEffectFree(doc, doc.Tree.ExtraList[node.Extra+uint32(i)]) {
				return false
			}
		}

		return true
	case ast.KindTableExpr:
		for i := uint16(0); i < node.Count; i++ {
			if node.Extra+uint32(i) >= uint32(len(doc.Tree.ExtraList)) {
				return false
			}

			fID := doc.Tree.ExtraList[node.Extra+uint32(i)]
			if fID == ast.InvalidNode || int(fID) >= len(doc.Tree.Nodes) {
				return false
			}

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
		var nameBytes []byte

		if node.Kind == ast.KindMethodCall {
			if node.Right != ast.InvalidNode && int(node.Right) < len(doc.Tree.Nodes) {
				nameNode := doc.Tree.Nodes[node.Right]
				if nameNode.Start <= nameNode.End && nameNode.End <= uint32(len(doc.Source)) {
					nameBytes = doc.Source[nameNode.Start:nameNode.End]
				}
			}
		} else {
			if node.Left != ast.InvalidNode && int(node.Left) < len(doc.Tree.Nodes) {
				leftNode := doc.Tree.Nodes[node.Left]

				if leftNode.Kind == ast.KindIdent {
					if leftNode.Start <= leftNode.End && leftNode.End <= uint32(len(doc.Source)) {
						nameBytes = doc.Source[leftNode.Start:leftNode.End]
					}
				} else if leftNode.Kind == ast.KindMemberExpr && leftNode.Right != ast.InvalidNode && int(leftNode.Right) < len(doc.Tree.Nodes) {
					rightNode := doc.Tree.Nodes[leftNode.Right]
					if rightNode.Start <= rightNode.End && rightNode.End <= uint32(len(doc.Source)) {
						nameBytes = doc.Source[rightNode.Start:rightNode.End]
					}
				}
			}
		}

		if len(nameBytes) > 0 {
			if hasPrefixFold(nameBytes, []byte("get")) ||
				hasPrefixFold(nameBytes, []byte("is")) ||
				hasPrefixFold(nameBytes, []byte("has")) ||
				hasPrefixFold(nameBytes, []byte("can")) ||
				hasPrefixFold(nameBytes, []byte("unpack")) ||
				hasPrefixFold(nameBytes, []byte("math.")) ||
				hasPrefixFold(nameBytes, []byte("type")) ||
				hasPrefixFold(nameBytes, []byte("tostring")) ||
				hasPrefixFold(nameBytes, []byte("tonumber")) ||
				hasPrefixFold(nameBytes, []byte("pairs")) ||
				hasPrefixFold(nameBytes, []byte("ipairs")) {

				// Check args
				for i := uint16(0); i < node.Count; i++ {
					if node.Extra+uint32(i) >= uint32(len(doc.Tree.ExtraList)) || !s.isSideEffectFree(doc, doc.Tree.ExtraList[node.Extra+uint32(i)]) {
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

func (s *Server) isStaticallyConstant(doc *Document, id ast.NodeID) bool {
	if id == ast.InvalidNode || int(id) >= len(doc.Tree.Nodes) {
		return false
	}

	node := doc.Tree.Nodes[id]

	switch node.Kind {
	case ast.KindNumber, ast.KindString, ast.KindTrue, ast.KindFalse, ast.KindNil:
		return true
	case ast.KindUnaryExpr:
		return s.isStaticallyConstant(doc, node.Right)
	case ast.KindBinaryExpr:
		return s.isStaticallyConstant(doc, node.Left) && s.isStaticallyConstant(doc, node.Right)
	case ast.KindParenExpr:
		return s.isStaticallyConstant(doc, node.Left)
	default:
		return false
	}
}

func (s *Server) getRootDef(doc *Document, exprID ast.NodeID) ast.NodeID {
	curr := exprID

	for curr != ast.InvalidNode && int(curr) < len(doc.Tree.Nodes) {
		var breakOut bool

		node := doc.Tree.Nodes[curr]

		switch node.Kind {
		case ast.KindIdent:
			return doc.Resolver.References[curr]
		case ast.KindMemberExpr, ast.KindIndexExpr:
			curr = node.Left
		default:
			breakOut = true
		}

		if breakOut {
			break
		}
	}

	return ast.InvalidNode
}
