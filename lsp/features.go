package lsp

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/coalaura/lugo/ast"
)

type SemanticToken struct {
	Start     uint32
	End       uint32
	TokenType uint32
	Modifiers uint32
}

func (s *Server) handleHover(req Request) {
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
		r = new(getNodeRange(doc.Tree, ctx.IdentNodeID))

		if ctx.TargetURI != "" && ctx.TargetURI != uri {
			fromFile = filepath.Base(ctx.TargetDoc.Path)
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

				if !ctx.IsGlobal && !ctx.IsProp && ctx.TargetDefID == ctx.IdentNodeID {
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
				docBuilder.WriteString("**Generics**\n\n")

				for _, g := range luadoc.Generics {
					docBuilder.WriteString("* **`" + g.Name + "`**")

					if g.Parent != "" {
						docBuilder.WriteString(" : *`" + g.Parent + "`*")
					}

					docBuilder.WriteString("\n")
				}

				docBuilder.WriteString("\n")
			}

			if len(luadoc.Params) > 0 {
				docBuilder.WriteString("**Parameters**\n\n")

				for _, p := range luadoc.Params {
					docBuilder.WriteString("* **`" + p.Name + "`**")

					if p.Type != "" {
						docBuilder.WriteString(" *`" + p.Type + "`*")
					}

					if p.Desc != "" {
						// Replace newlines with indented newlines for list alignment
						desc := strings.ReplaceAll(p.Desc, "\n", "\n  ")
						docBuilder.WriteString(" - " + desc)
					}

					docBuilder.WriteString("\n")
				}

				docBuilder.WriteString("\n")
			}

			if len(luadoc.Returns) > 0 {
				docBuilder.WriteString("**Returns**\n\n")

				for _, ret := range luadoc.Returns {
					docBuilder.WriteString("* ")

					if ret.Type != "" {
						docBuilder.WriteString("*`" + ret.Type + "`*")
					}

					if ret.Desc != "" {
						if ret.Type != "" {
							docBuilder.WriteString(" - ")
						}

						desc := strings.ReplaceAll(ret.Desc, "\n", "\n  ")
						docBuilder.WriteString(desc)
					}

					docBuilder.WriteString("\n")
				}

				docBuilder.WriteString("\n")
			}

			if len(luadoc.Fields) > 0 && matchedField == nil {
				docBuilder.WriteString("**Fields**\n\n")

				for _, f := range luadoc.Fields {
					docBuilder.WriteString("* **`" + f.Name + "`**")

					if f.Type != "" {
						docBuilder.WriteString(" *`" + f.Type + "`*")
					}

					if f.Desc != "" {
						desc := strings.ReplaceAll(f.Desc, "\n", "\n  ")
						docBuilder.WriteString(" - " + desc)
					}

					docBuilder.WriteString("\n")
				}

				docBuilder.WriteString("\n")
			}

			if len(luadoc.Overloads) > 0 {
				docBuilder.WriteString("**Overloads**\n\n")

				for _, o := range luadoc.Overloads {
					docBuilder.WriteString("```lua\n" + o + "\n```\n")
				}

				docBuilder.WriteString("\n")
			}

			if len(luadoc.See) > 0 {
				docBuilder.WriteString("**See also**\n\n")

				for _, see := range luadoc.See {
					docBuilder.WriteString("* `" + see + "`\n")
				}

				docBuilder.WriteString("\n")
			}

			docString := strings.TrimSpace(docBuilder.String())

			hoverText = "```lua\n" + code + "\n```"

			if docString != "" {
				hoverText += "\n---\n\n" + docString
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

			r = new(getRange(doc.Tree, startOff, endOff))
		}
	}

	if hoverText == "" {
		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

		return
	}

	WriteMessage(s.Writer, Response{
		RPC: "2.0",
		ID:  req.ID,
		Result: Hover{
			Contents: MarkupContent{Kind: "markdown", Value: hoverText},
			Range:    r,
		},
	})
}

func (s *Server) handleCompletion(req Request) {
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

	addCompletion := func(label string, kind CompletionItemKind, detail string, isDep bool, sortText string, insertText string, insertFormat InsertTextFormat) {
		if label == "" || seen[label] {
			return
		}

		seen[label] = true

		var tags []CompletionItemTag

		if isDep {
			tags = append(tags, CompletionItemTagDeprecated)
		}

		items = append(items, CompletionItem{
			Label:            label,
			Kind:             kind,
			Detail:           detail,
			SortText:         sortText,
			Tags:             tags,
			InsertText:       insertText,
			InsertTextFormat: insertFormat,
		})
	}

	buildFuncSnippet := func(label string, dDoc *Document, valID ast.NodeID, isMethod bool) (string, InsertTextFormat) {
		if !s.SuggestFunctionParams || valID == ast.InvalidNode || dDoc == nil {
			return label, PlainTextTextFormat
		}

		node := dDoc.Tree.Nodes[valID]
		if node.Kind != ast.KindFunctionExpr {
			return label, PlainTextTextFormat
		}

		var params []string

		snippetIdx := 1

		for i := uint16(0); i < node.Count; i++ {
			if node.Extra+uint32(i) >= uint32(len(dDoc.Tree.ExtraList)) {
				continue
			}

			pID := dDoc.Tree.ExtraList[node.Extra+uint32(i)]
			if int(pID) >= len(dDoc.Tree.Nodes) {
				continue
			}

			pNode := dDoc.Tree.Nodes[pID]
			if pNode.Start > pNode.End || pNode.End > uint32(len(dDoc.Source)) {
				continue
			}

			pName := ast.String(dDoc.Source[pNode.Start:pNode.End])

			if isMethod && i == 0 && pName == "self" {
				continue
			}

			if pName == "..." {
				params = append(params, fmt.Sprintf("${%d:...}", snippetIdx))
			} else {
				params = append(params, fmt.Sprintf("${%d:%s}", snippetIdx, pName))
			}

			snippetIdx++
		}

		return fmt.Sprintf("%s(%s)", label, strings.Join(params, ", ")), SnippetTextFormat
	}

	var (
		recName  []byte
		isMember bool
		isColon  bool
		endId    int
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
		isColon = doc.Source[i] == ':'

		i--

		for i >= 0 {
			c := doc.Source[i]

			if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				i--
			} else {
				break
			}
		}

		endId = i + 1

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

		var rootName []byte

		for i, c := range recName {
			if c == '.' || c == ':' {
				rootName = recName[:i]

				break
			}
		}

		if rootName == nil {
			rootName = recName
		}

		var recDef ast.NodeID = ast.InvalidNode

		for name, defID := range doc.LocalsAt(offset) {
			if bytes.Equal(name, rootName) {
				recDef = defID

				break
			}
		}

		var modHash uint64

		if recDef != ast.InvalidNode {
			valID := doc.getAssignedValue(recDef)

			modName := s.getRequireModName(doc, valID)
			if modName != "" {
				targetDoc := s.resolveModule(uri, modName)
				if targetDoc != nil {
					modHash = ast.HashBytesConcat([]byte("module:"), nil, []byte(targetDoc.URI))
				}
			}
		}

		for _, fd := range doc.Resolver.FieldDefs {
			if fd.ReceiverHash == recHash && (recDef == ast.InvalidNode || fd.ReceiverDef == recDef) {
				node := doc.Tree.Nodes[fd.NodeID]

				kind := FieldCompletion
				label := ast.String(doc.Source[node.Start:node.End])
				insertText := label
				insertFormat := PlainTextTextFormat

				valID := doc.getAssignedValue(fd.NodeID)
				if valID != ast.InvalidNode && doc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
					kind = FunctionCompletion

					insertText, insertFormat = buildFuncSnippet(label, doc, valID, isColon)
				}

				isDep, _ := doc.HasDeprecatedTag(fd.NodeID)

				addCompletion(label, kind, "field", isDep, "1", insertText, insertFormat)
			}
		}

		validRecs := make(map[uint64]bool)

		currRec := recHash

		for i := 0; i < 10 && currRec != 0; i++ {
			validRecs[currRec] = true

			currRec = s.getGlobalAlias(currRec)
		}

		if modHash != 0 {
			validRecs[modHash] = true
		}

		recNodeID := doc.Tree.NodeAt(uint32(endId - 1))
		if recNodeID != ast.InvalidNode {
			pID := doc.Tree.Nodes[recNodeID].Parent
			if pID != ast.InvalidNode {
				pNode := doc.Tree.Nodes[pID]
				if (pNode.Kind == ast.KindMemberExpr || pNode.Kind == ast.KindMethodCall) && pNode.Right == recNodeID {
					recNodeID = pID
				}
			}
		}

		var recType TypeSet

		if recNodeID != ast.InvalidNode {
			recType = doc.InferType(recNodeID)
		}

		if recType.CustomName != "" {
			currClassName := recType.CustomName

			for i := 0; i < 10; i++ {
				if currClassName == "" {
					break
				}

				classHash := ast.HashBytes([]byte(currClassName))

				validRecs[classHash] = true

				classSyms, ok := s.GlobalIndex[GlobalKey{ReceiverHash: 0, PropHash: classHash}]
				if !ok || len(classSyms) == 0 {
					break
				}

				currClassName = classSyms[0].Parent
			}
		}

		if recType.MetaNode != ast.InvalidNode {
			metaDoc := doc
			if recType.MetaURI != "" && recType.MetaURI != doc.URI {
				metaDoc = s.Documents[recType.MetaURI]
			}

			if metaDoc != nil {
				indexDoc, indexTableID := metaDoc.getIndexTable(recType.MetaNode)
				if indexDoc != nil && indexTableID != ast.InvalidNode {
					tableNode := indexDoc.Tree.Nodes[indexTableID]

					for i := uint16(0); i < tableNode.Count; i++ {
						if tableNode.Extra+uint32(i) >= uint32(len(indexDoc.Tree.ExtraList)) {
							continue
						}

						fieldID := indexDoc.Tree.ExtraList[tableNode.Extra+uint32(i)]
						if int(fieldID) >= len(indexDoc.Tree.Nodes) {
							continue
						}

						field := indexDoc.Tree.Nodes[fieldID]
						if field.Kind == ast.KindRecordField {
							key := indexDoc.Tree.Nodes[field.Left]
							if key.Kind == ast.KindIdent {
								label := ast.String(indexDoc.Source[key.Start:key.End])

								kind := FieldCompletion
								insertText := label
								insertFormat := PlainTextTextFormat

								valID := field.Right
								if valID != ast.InvalidNode && int(valID) < len(indexDoc.Tree.Nodes) && indexDoc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
									kind = FunctionCompletion
									insertText, insertFormat = buildFuncSnippet(label, indexDoc, valID, isColon)
								}

								isDep, _ := indexDoc.HasDeprecatedTag(field.Left)

								addCompletion(label, kind, "metatable field", isDep, "1", insertText, insertFormat)
							}
						}
					}
				}
			}
		}

		for key, syms := range s.GlobalIndex {
			if validRecs[key.ReceiverHash] && key.PropHash != 0 && len(syms) > 0 {
				var visibleSym *GlobalSymbol

				for _, sym := range syms {
					if tgtDoc, ok := s.Documents[sym.URI]; ok && s.canSeeSymbol(doc, tgtDoc) {
						visibleSym = &sym

						break
					}
				}

				if visibleSym == nil {
					continue
				}

				sym := *visibleSym

				if symDoc, ok := s.Documents[sym.URI]; ok {
					node := symDoc.Tree.Nodes[sym.NodeID]

					kind := FieldCompletion
					label := ast.String(symDoc.Source[node.Start:node.End])
					insertText := label
					insertFormat := PlainTextTextFormat

					valID := symDoc.getAssignedValue(sym.NodeID)
					if valID != ast.InvalidNode && symDoc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
						kind = FunctionCompletion

						insertText, insertFormat = buildFuncSnippet(label, symDoc, valID, isColon)
					}

					isDep, _ := symDoc.HasDeprecatedTag(sym.NodeID)

					sortGroup := "2"
					if sym.URI == uri {
						sortGroup = "1"
					}

					addCompletion(label, kind, "field", isDep, sortGroup, insertText, insertFormat)
				}
			}
		}
	} else {
		for name, defID := range doc.LocalsAt(offset) {
			isDep, _ := doc.HasDeprecatedTag(defID)

			kind := VariableCompletion
			label := ast.String(name)
			insertText := label
			insertFormat := PlainTextTextFormat

			valID := doc.getAssignedValue(defID)
			if valID != ast.InvalidNode && doc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
				kind = FunctionCompletion

				insertText, insertFormat = buildFuncSnippet(label, doc, valID, false)
			}

			addCompletion(label, kind, "local", isDep, "0", insertText, insertFormat)
		}

		for key, syms := range s.GlobalIndex {
			if key.ReceiverHash == 0 && key.PropHash != 0 && len(syms) > 0 {
				var visibleSym *GlobalSymbol

				for _, sym := range syms {
					if tgtDoc, ok := s.Documents[sym.URI]; ok && s.canSeeSymbol(doc, tgtDoc) {
						visibleSym = &sym

						break
					}
				}

				if visibleSym == nil {
					continue
				}

				sym := *visibleSym

				if symDoc, ok := s.Documents[sym.URI]; ok {
					node := symDoc.Tree.Nodes[sym.NodeID]

					if node.Kind == ast.KindIdent || node.Kind == ast.KindMethodName {
						kind := VariableCompletion
						label := ast.String(symDoc.Source[node.Start:node.End])
						insertText := label
						insertFormat := PlainTextTextFormat

						valID := symDoc.getAssignedValue(sym.NodeID)

						if valID != ast.InvalidNode && symDoc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
							kind = FunctionCompletion

							insertText, insertFormat = buildFuncSnippet(label, symDoc, valID, false)
						}

						isDep, _ := symDoc.HasDeprecatedTag(sym.NodeID)

						sortGroup := "2"
						if sym.URI == uri {
							sortGroup = "1"
						}

						addCompletion(label, kind, "global", isDep, sortGroup, insertText, insertFormat)
					}
				}
			}
		}

		for _, kw := range luaKeywords {
			addCompletion(kw, KeywordCompletion, "keyword", false, "3", kw, PlainTextTextFormat)
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
}

func (s *Server) handleSignatureHelp(req Request) {
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
	if ctx == nil {
		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

		return
	}

	var defs []GlobalSymbol

	if len(ctx.GlobalDefs) > 0 {
		defs = ctx.GlobalDefs
	} else if ctx.TargetDefID != ast.InvalidNode {
		defs = []GlobalSymbol{{URI: ctx.TargetURI, NodeID: ctx.TargetDefID}}
	}

	if len(defs) == 0 {
		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

		return
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

	var (
		signatures     []SignatureInformation
		bestSigIndex   int
		bestMatchScore int = -1
	)

	for _, def := range defs {
		tDoc := s.Documents[def.URI]
		if tDoc == nil {
			continue
		}

		valID := tDoc.getAssignedValue(def.NodeID)
		if valID == ast.InvalidNode || int(valID) >= len(tDoc.Tree.Nodes) || tDoc.Tree.Nodes[valID].Kind != ast.KindFunctionExpr {
			continue
		}

		luadoc := parseLuaDoc(tDoc.getCommentsAbove(def.NodeID))
		funcNode := tDoc.Tree.Nodes[valID]

		var (
			paramsInfo []ParameterInformation
			labels     []string
		)

		paramDocs := make(map[string]LuaDocParam)

		for _, p := range luadoc.Params {
			paramDocs[p.Name] = p
		}

		paramOffset := getImplicitSelfOffset(callNode, tDoc, def.NodeID)

		for i := uint16(0); i < funcNode.Count; i++ {
			if funcNode.Extra+uint32(i) >= uint32(len(tDoc.Tree.ExtraList)) {
				continue
			}

			pID := tDoc.Tree.ExtraList[funcNode.Extra+uint32(i)]
			if pID == ast.InvalidNode || int(pID) >= len(tDoc.Tree.Nodes) {
				continue
			}

			pNode := tDoc.Tree.Nodes[pID]
			if pNode.Start > pNode.End || pNode.End > uint32(len(tDoc.Source)) {
				continue
			}

			pName := ast.String(tDoc.Source[pNode.Start:pNode.End])

			if i == 0 && paramOffset == 1 && pName == "self" {
				continue
			}

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

		var funcDoc *MarkupContent

		if luadoc.Description != "" {
			funcDoc = &MarkupContent{Kind: "markdown", Value: luadoc.Description}
		}

		signatures = append(signatures, SignatureInformation{
			Label:         ctx.DisplayName + "(" + strings.Join(labels, ", ") + ")",
			Documentation: funcDoc,
			Parameters:    paramsInfo,
		})

		expectedArgs := int(funcNode.Count) - paramOffset
		if expectedArgs < 0 {
			expectedArgs = 0
		}

		var score int

		if expectedArgs == int(callNode.Count) {
			score = 2
		} else if expectedArgs > int(callNode.Count) {
			score = 1
		}

		if score > bestMatchScore {
			bestMatchScore = score
			bestSigIndex = len(signatures) - 1
		}
	}

	if len(signatures) == 0 {
		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

		return
	}

	WriteMessage(s.Writer, Response{
		RPC: "2.0",
		ID:  req.ID,
		Result: SignatureHelp{
			Signatures:      signatures,
			ActiveSignature: bestSigIndex,
			ActiveParameter: activeParam,
		},
	})
}

func (s *Server) handleInlayHint(req Request) {
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

		paramOffset := getImplicitSelfOffset(node, ctx.TargetDoc, ctx.TargetDefID)

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
}

func (s *Server) handleDocumentHighlight(req Request) {
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
}

func (s *Server) handleSemanticTokensFull(req Request) {
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

		var (
			tokenType uint32 = 0xFFFFFFFF
			modifiers uint32 = 0
		)

		switch node.Kind {
		case ast.KindNumber:
			tokenType = 6
		case ast.KindString:
			tokenType = 7
		case ast.KindTrue, ast.KindFalse, ast.KindNil:
			tokenType = 8
		case ast.KindIdent:
			tokenType = 0

			identBytes := doc.Source[node.Start:node.End]

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

					if syms, ok := s.getGlobalSymbols(doc, recHash, hash); ok && len(syms) > 0 {
						sym := syms[0]
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
						parentID := targetDoc.Tree.Nodes[targetDef].Parent
						if parentID != ast.InvalidNode {
							parentNode := targetDoc.Tree.Nodes[parentID]
							if parentNode.Kind == ast.KindFunctionStmt || parentNode.Kind == ast.KindLocalFunction {
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
		}

		if tokenType == 0xFFFFFFFF {
			continue
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
}

func (s *Server) handleSelectionRange(req Request) {
	var params SelectionRangeParams

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

	var ranges []SelectionRange

	for _, pos := range params.Positions {
		offset := doc.Tree.Offset(pos.Line, pos.Character)
		currID := doc.Tree.NodeAt(offset)

		var (
			first *SelectionRange
			curr  *SelectionRange
		)

		for currID != ast.InvalidNode {
			node := doc.Tree.Nodes[currID]

			sr := &SelectionRange{
				Range: getRange(doc.Tree, node.Start, node.End),
			}

			if first == nil {
				first = sr
				curr = sr
			} else {
				// Skip duplicate ranges to make the selection expansion feel natural
				if curr.Range != sr.Range {
					curr.Parent = sr
					curr = sr
				}
			}

			currID = node.Parent
		}

		if first != nil {
			ranges = append(ranges, *first)
		} else {
			ranges = append(ranges, SelectionRange{
				Range: Range{Start: pos, End: pos},
			})
		}
	}

	if ranges == nil {
		ranges = []SelectionRange{}
	}

	WriteMessage(s.Writer, Response{
		RPC:    "2.0",
		ID:     req.ID,
		Result: ranges,
	})
}

func (s *Server) handleFoldingRange(req Request) {
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
}

func (s *Server) handleCodeLens(req Request) {
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
}

func (s *Server) handleCodeLensResolve(req Request) {
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
}

func (s *Server) handlePrepareCallHierarchy(req Request) {
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
}

func (s *Server) handleCallHierarchyIncomingCalls(req Request) {
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
}

func (s *Server) handleCallHierarchyOutgoingCalls(req Request) {
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
		detail = filepath.Base(doc.Path)
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
