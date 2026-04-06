package lsp

import (
	"bytes"
	"strings"

	"github.com/coalaura/lugo/ast"
	"github.com/coalaura/lugo/token"
)

type BasicType uint16

const (
	TypeUnknown  BasicType = 0
	TypeNil      BasicType = 1 << 0
	TypeBoolean  BasicType = 1 << 1
	TypeNumber   BasicType = 1 << 2
	TypeString   BasicType = 1 << 3
	TypeFunction BasicType = 1 << 4
	TypeTable    BasicType = 1 << 5
	TypeUserdata BasicType = 1 << 6
	TypeThread   BasicType = 1 << 7
	TypeAny      BasicType = 1 << 8
)

// TypeSet efficiently represents union types as bitmasks and custom names.
type TypeSet struct {
	CustomName string
	DeclURI    string
	DeclNode   ast.NodeID
	MetaURI    string
	MetaNode   ast.NodeID
	Basics     BasicType
}

func ParseTypeString(tStr string) TypeSet {
	var typeSet TypeSet

	for part := range strings.SplitSeq(tStr, "|") {
		part = strings.TrimSpace(part)

		if strings.HasSuffix(part, "?") {
			part = part[:len(part)-1]

			typeSet.Basics |= TypeNil
		}

		switch part {
		case "number", "integer", "float":
			typeSet.Basics |= TypeNumber
		case "string":
			typeSet.Basics |= TypeString
		case "boolean", "bool":
			typeSet.Basics |= TypeBoolean
		case "table":
			typeSet.Basics |= TypeTable
		case "function", "fun":
			typeSet.Basics |= TypeFunction
		case "nil":
			typeSet.Basics |= TypeNil
		case "any":
			typeSet.Basics |= TypeAny
		case "userdata":
			typeSet.Basics |= TypeUserdata
		case "thread":
			typeSet.Basics |= TypeThread
		default:
			if strings.HasPrefix(part, "fun(") {
				typeSet.Basics |= TypeFunction
			} else if strings.HasPrefix(part, "{") {
				typeSet.Basics |= TypeTable
			} else if part != "" {
				typeSet.CustomName = part
			}
		}
	}

	return typeSet
}

func (typeSet TypeSet) Format() string {
	if typeSet.Basics&TypeAny != 0 {
		return "any"
	}

	var parts []string

	if typeSet.Basics&TypeNumber != 0 {
		parts = append(parts, "number")
	}

	if typeSet.Basics&TypeString != 0 {
		parts = append(parts, "string")
	}

	if typeSet.Basics&TypeBoolean != 0 {
		parts = append(parts, "boolean")
	}

	if typeSet.Basics&TypeTable != 0 {
		parts = append(parts, "table")
	}

	if typeSet.Basics&TypeFunction != 0 {
		parts = append(parts, "function")
	}

	if typeSet.Basics&TypeUserdata != 0 {
		parts = append(parts, "userdata")
	}

	if typeSet.Basics&TypeThread != 0 {
		parts = append(parts, "thread")
	}

	if typeSet.CustomName != "" {
		parts = append(parts, typeSet.CustomName)
	}

	if typeSet.Basics&TypeNil != 0 {
		parts = append(parts, "nil")
	}

	if len(parts) == 0 {
		return "any"
	}

	return strings.Join(parts, " | ")
}

// InferType infers the type of a given AST node lazily and caches it.
func (doc *Document) InferType(id ast.NodeID) TypeSet {
	if id == ast.InvalidNode {
		return TypeSet{}
	}

	if t, ok := doc.TypeCache[id]; ok {
		if t.Basics != TypeUnknown || t.CustomName != "" || t.DeclNode != ast.InvalidNode {
			return t
		}
	}

	if doc.Inferring[id] {
		return TypeSet{} // Cycle detected
	}

	doc.Inferring[id] = true

	defer func() {
		doc.Inferring[id] = false
	}()

	var typeSet TypeSet

	node := doc.Tree.Nodes[id]

	switch node.Kind {
	case ast.KindNumber:
		typeSet.Basics = TypeNumber
	case ast.KindString:
		typeSet.Basics = TypeString
	case ast.KindTrue, ast.KindFalse:
		typeSet.Basics = TypeBoolean
	case ast.KindNil:
		typeSet.Basics = TypeNil
	case ast.KindFunctionExpr, ast.KindLocalFunction, ast.KindFunctionStmt:
		typeSet.Basics = TypeFunction
		typeSet.DeclNode = id
		typeSet.DeclURI = doc.URI
	case ast.KindTableExpr:
		typeSet.Basics = TypeTable
		typeSet.DeclNode = id
		typeSet.DeclURI = doc.URI
	case ast.KindBinaryExpr:
		op := node.Extra

		switch token.Kind(op) {
		case token.Plus, token.Minus, token.Asterisk, token.Slash, token.FloorSlash, token.Modulo, token.Caret, token.BitAnd, token.BitOr, token.BitXor, token.ShiftLeft, token.ShiftRight:
			leftType := doc.InferType(node.Left)
			rightType := doc.InferType(node.Right)

			if leftType.CustomName != "" {
				typeSet.CustomName = leftType.CustomName
				typeSet.Basics = leftType.Basics
			} else if rightType.CustomName != "" {
				typeSet.CustomName = rightType.CustomName
				typeSet.Basics = rightType.Basics
			} else {
				if leftType.Basics == TypeUnknown || rightType.Basics == TypeUnknown || leftType.Basics&TypeAny != 0 || rightType.Basics&TypeAny != 0 {
					typeSet.Basics = TypeAny
				} else {
					typeSet.Basics = TypeNumber
				}
			}
		case token.Concat:
			typeSet.Basics = TypeString
		case token.Eq, token.NotEq, token.Less, token.LessEq, token.Greater, token.GreaterEq:
			typeSet.Basics = TypeBoolean
		case token.And, token.Or:
			leftType := doc.InferType(node.Left)
			rightType := doc.InferType(node.Right)

			typeSet.Basics = leftType.Basics | rightType.Basics

			if leftType.Basics == TypeUnknown && leftType.CustomName == "" {
				typeSet.Basics |= TypeAny
			}

			if rightType.Basics == TypeUnknown && rightType.CustomName == "" {
				typeSet.Basics |= TypeAny
			}

			if typeSet.CustomName == "" {
				typeSet.CustomName = leftType.CustomName
			}

			if typeSet.CustomName == "" {
				typeSet.CustomName = rightType.CustomName
			}
		}
	case ast.KindUnaryExpr:
		src := doc.Source[node.Start:node.End]

		if bytes.HasPrefix(src, []byte("not")) {
			typeSet.Basics = TypeBoolean
		} else if len(src) > 0 && src[0] == '#' {
			typeSet.Basics = TypeNumber
		} else {
			typeSet.Basics = TypeNumber
		}
	case ast.KindParenExpr:
		typeSet = doc.InferType(node.Left)
	case ast.KindIdent:
		typeSet = doc.inferIdent(id)
	case ast.KindMemberExpr:
		typeSet = doc.inferMemberExpr(node)
	case ast.KindCallExpr, ast.KindMethodCall:
		typeSet = doc.inferCallExpr(node)
	}

	doc.TypeCache[id] = typeSet

	return typeSet
}

func (doc *Document) inferIdent(id ast.NodeID) TypeSet {
	var (
		targetDoc *Document  = doc
		targetDef ast.NodeID = doc.Resolver.References[id]
	)

	localDefID := targetDef
	identName := doc.Source[doc.Tree.Nodes[id].Start:doc.Tree.Nodes[id].End]
	identHash := ast.HashBytes(identName)

	if doc.Server != nil {
		ctx := doc.Server.resolveSymbolNode(doc.URI, doc, id)
		if ctx != nil && ctx.TargetDoc != nil && ctx.TargetDefID != ast.InvalidNode {
			targetDoc = ctx.TargetDoc
			targetDef = ctx.TargetDefID
		}
	}

	if targetDef == ast.InvalidNode {
		return TypeSet{}
	}

	enableAlerts := doc.Server != nil && doc.Server.FeatureFormatAlerts
	luadoc := parseLuaDoc(targetDoc.getCommentsAbove(targetDef), enableAlerts)

	var t TypeSet

	if luadoc.Type != nil {
		t = ParseTypeString(luadoc.Type.Type)
	} else if luadoc.Class != nil {
		t = TypeSet{CustomName: luadoc.Class.Name}
	} else {
		valID := targetDoc.getAssignedValue(targetDef)
		if valID != ast.InvalidNode {
			t = targetDoc.InferType(valID)
		} else if targetDoc.Tree.Nodes[targetDef].Kind == ast.KindIdent {
			parentID := targetDoc.Tree.Nodes[targetDef].Parent
			if parentID != ast.InvalidNode {
				parentNode := targetDoc.Tree.Nodes[parentID]

				switch parentNode.Kind {
				case ast.KindFunctionExpr:
					t = targetDoc.inferFunctionParameter(targetDef, parentID)
				case ast.KindNameList:
					t = targetDoc.inferLoopVariable(targetDef, parentID)
				}
			}
		}
	}

	checkReassignments := func(d *Document) {
		for _, reassignment := range d.Resolver.Reassignments {
			var match bool

			if reassignment.DefID != ast.InvalidNode {
				if d == targetDoc && reassignment.DefID == targetDef {
					match = true
				} else if d == doc && reassignment.DefID == localDefID {
					match = true
				}
			} else {
				if reassignment.NameHash == identHash {
					match = true
				}
			}

			if match {
				rt := d.InferType(reassignment.ValID)

				if rt.Basics == TypeUnknown && rt.CustomName == "" {
					t.Basics |= TypeAny
				} else {
					t.Basics |= rt.Basics

					if t.CustomName == "" {
						t.CustomName = rt.CustomName
					}

					if t.DeclNode == ast.InvalidNode && rt.DeclNode != ast.InvalidNode {
						t.DeclNode = rt.DeclNode
						t.DeclURI = rt.DeclURI
					}

					if t.MetaNode == ast.InvalidNode && rt.MetaNode != ast.InvalidNode {
						t.MetaNode = rt.MetaNode
						t.MetaURI = rt.MetaURI
					}
				}
			}
		}
	}

	checkReassignments(doc)

	if targetDoc != doc {
		checkReassignments(targetDoc)
	}

	return t
}

func (doc *Document) inferFunctionParameter(defID, funcExprID ast.NodeID) TypeSet {
	grandParentID := doc.Tree.Nodes[funcExprID].Parent
	if grandParentID == ast.InvalidNode {
		return TypeSet{}
	}

	grandParentNode := doc.Tree.Nodes[grandParentID]

	var funcDefID ast.NodeID = ast.InvalidNode

	switch grandParentNode.Kind {
	case ast.KindLocalFunction, ast.KindFunctionStmt:
		funcDefID = grandParentNode.Left
	case ast.KindAssign, ast.KindLocalAssign, ast.KindRecordField:
		funcDefID = grandParentID
	}

	if funcDefID == ast.InvalidNode {
		return TypeSet{}
	}

	enableAlerts := doc.Server != nil && doc.Server.FeatureFormatAlerts
	funcDoc := parseLuaDoc(doc.getCommentsAbove(funcDefID), enableAlerts)
	paramName := string(doc.Source[doc.Tree.Nodes[defID].Start:doc.Tree.Nodes[defID].End])

	for _, p := range funcDoc.Params {
		if p.Name == paramName {
			return ParseTypeString(p.Type)
		}
	}

	return TypeSet{}
}

func (doc *Document) inferLoopVariable(defID, nameListID ast.NodeID) TypeSet {
	grandParentID := doc.Tree.Nodes[nameListID].Parent
	if grandParentID == ast.InvalidNode {
		return TypeSet{}
	}

	grandParentNode := doc.Tree.Nodes[grandParentID]
	if grandParentNode.Kind != ast.KindForIn || grandParentNode.Extra == 0 {
		return TypeSet{}
	}

	idx := doc.Tree.IndexOfExtra(nameListID, defID)

	if idx == -1 {
		return TypeSet{}
	}

	exprList := doc.Tree.Nodes[grandParentNode.Extra]
	if exprList.Count == 0 {
		return TypeSet{}
	}

	firstExprID := doc.Tree.ExtraList[exprList.Extra]
	firstExpr := doc.Tree.Nodes[firstExprID]

	if firstExpr.Kind != ast.KindCallExpr || firstExpr.Count == 0 {
		return TypeSet{}
	}

	funcID := firstExpr.Left
	if doc.Tree.Nodes[funcID].Kind != ast.KindIdent {
		return TypeSet{}
	}

	if doc.Resolver.References[funcID] != ast.InvalidNode {
		return TypeSet{}
	}

	funcName := doc.Source[doc.Tree.Nodes[funcID].Start:doc.Tree.Nodes[funcID].End]

	if bytes.Equal(funcName, []byte("ipairs")) {
		switch idx {
		case 0:
			return TypeSet{Basics: TypeNumber}
		case 1:
			if firstExpr.Count > 0 {
				argID := doc.Tree.ExtraList[firstExpr.Extra]

				return doc.extractArrayElementType(doc.InferType(argID))
			}
		}
	} else if bytes.Equal(funcName, []byte("pairs")) {
		switch idx {
		case 0:
			return TypeSet{Basics: TypeAny}
		case 1:
			if firstExpr.Count > 0 {
				argID := doc.Tree.ExtraList[firstExpr.Extra]

				return doc.extractArrayElementType(doc.InferType(argID))
			}
		}
	}

	return TypeSet{}
}

func (doc *Document) inferMemberExpr(node ast.Node) TypeSet {
	leftType := doc.InferType(node.Left)

	var (
		t         TypeSet
		targetDoc *Document
	)

	if leftType.DeclNode != ast.InvalidNode && leftType.DeclURI != "" {
		targetDoc = doc
		if leftType.DeclURI != doc.URI {
			if doc.Server != nil {
				targetDoc = doc.Server.Documents[leftType.DeclURI]
			} else {
				targetDoc = nil
			}
		}
	}

	rightNode := doc.Tree.Nodes[node.Right]
	if rightNode.Kind != ast.KindIdent {
		return TypeSet{}
	}

	fieldName := doc.Source[rightNode.Start:rightNode.End]
	propHash := ast.HashBytes(fieldName)

	checkTableFields := func(tDoc *Document, tableID ast.NodeID) {
		if tableID == ast.InvalidNode || int(tableID) >= len(tDoc.Tree.Nodes) || (t.Basics != TypeUnknown || t.CustomName != "") {
			return
		}

		tableNode := tDoc.Tree.Nodes[tableID]
		if tableNode.Kind == ast.KindTableExpr {
			for i := uint16(0); i < tableNode.Count; i++ {
				fieldID := tDoc.Tree.ExtraList[tableNode.Extra+uint32(i)]
				field := tDoc.Tree.Nodes[fieldID]

				if field.Kind == ast.KindRecordField {
					key := tDoc.Tree.Nodes[field.Left]

					if key.Kind == ast.KindIdent {
						keyName := tDoc.Source[key.Start:key.End]
						if bytes.Equal(keyName, fieldName) {
							t = tDoc.InferType(field.Right)
							return
						}
					}
				}
			}
		}

		recDef := tDoc.getDefForValue(tableID)
		if recDef != ast.InvalidNode {
			for _, fd := range tDoc.Resolver.FieldDefs {
				if fd.ReceiverDef == recDef && fd.PropHash == propHash {
					valID := tDoc.getAssignedValue(fd.NodeID)
					if valID != ast.InvalidNode {
						rt := tDoc.InferType(valID)
						if rt.Basics == TypeUnknown && rt.CustomName == "" {
							t.Basics |= TypeAny
						} else {
							t.Basics |= rt.Basics
							if t.CustomName == "" {
								t.CustomName = rt.CustomName
							}

							if t.DeclNode == ast.InvalidNode && rt.DeclNode != ast.InvalidNode {
								t.DeclNode = rt.DeclNode
								t.DeclURI = rt.DeclURI
							}

							if t.MetaNode == ast.InvalidNode && rt.MetaNode != ast.InvalidNode {
								t.MetaNode = rt.MetaNode
								t.MetaURI = rt.MetaURI
							}
						}
					}
					return
				}
			}
		}
	}

	// 1. Check initial table literal and its subsequent assignments
	if targetDoc != nil {
		checkTableFields(targetDoc, leftType.DeclNode)
	}

	// 2. Check subsequent assignments on the local variable itself
	if t.Basics == TypeUnknown && t.CustomName == "" {
		recDef, recHash, _ := doc.Resolver.GetReceiverContext(node.Left)

		for _, fd := range doc.Resolver.FieldDefs {
			if (recDef != ast.InvalidNode && fd.ReceiverDef == recDef) || (recDef == ast.InvalidNode && recHash != 0 && fd.ReceiverHash == recHash) {
				if fd.PropHash == propHash {
					valID := doc.getAssignedValue(fd.NodeID)
					if valID != ast.InvalidNode {
						rt := doc.InferType(valID)
						if rt.Basics == TypeUnknown && rt.CustomName == "" {
							t.Basics |= TypeAny
						} else {
							t.Basics |= rt.Basics
							if t.CustomName == "" {
								t.CustomName = rt.CustomName
							}

							if t.DeclNode == ast.InvalidNode && rt.DeclNode != ast.InvalidNode {
								t.DeclNode = rt.DeclNode
								t.DeclURI = rt.DeclURI
							}

							if t.MetaNode == ast.InvalidNode && rt.MetaNode != ast.InvalidNode {
								t.MetaNode = rt.MetaNode
								t.MetaURI = rt.MetaURI
							}
						}
					}
				}
			}
		}
	}

	// 3. Check Metatable __index
	if t.Basics == TypeUnknown && t.CustomName == "" {
		if leftType.MetaNode != ast.InvalidNode {
			metaDoc := doc
			if leftType.MetaURI != "" && leftType.MetaURI != doc.URI && doc.Server != nil {
				metaDoc = doc.Server.Documents[leftType.MetaURI]
			}

			if metaDoc != nil {
				indexDoc, indexTableID := metaDoc.getIndexTable(leftType.MetaNode)
				if indexDoc != nil && indexTableID != ast.InvalidNode {
					checkTableFields(indexDoc, indexTableID)
				}
			}
		}

		if t.Basics == TypeUnknown && t.CustomName == "" && leftType.CustomName != "" && doc.Server != nil {
			currClassName := leftType.CustomName
			for i := 0; i < 10; i++ {
				if currClassName == "" {
					break
				}

				classHash := ast.HashBytes([]byte(currClassName))
				if syms, ok := doc.Server.getGlobalSymbols(doc, classHash, propHash); ok && len(syms) > 0 {
					sym := syms[0]
					if gDoc, ok := doc.Server.Documents[sym.URI]; ok {
						valID := gDoc.getAssignedValue(sym.NodeID)
						if valID != ast.InvalidNode {
							t = gDoc.InferType(valID)
						} else {
							t.Basics = TypeAny
						}

						break
					}
				}

				classSyms, ok := doc.Server.GlobalIndex[GlobalKey{ReceiverHash: 0, PropHash: classHash}]
				if !ok || len(classSyms) == 0 {
					break
				}

				currClassName = classSyms[0].Parent
			}
		}
	}

	return t
}

func (doc *Document) inferCallExpr(node ast.Node) TypeSet {
	funcIdentID := node.Left

	if node.Kind == ast.KindMethodCall {
		funcIdentID = node.Right
	} else if doc.Tree.Nodes[funcIdentID].Kind == ast.KindMemberExpr {
		funcIdentID = doc.Tree.Nodes[funcIdentID].Right
	}

	if funcIdentID == ast.InvalidNode {
		return TypeSet{}
	}

	if doc.Server != nil {
		if doc.Tree.Nodes[funcIdentID].Kind == ast.KindIdent {
			funcName := doc.Source[doc.Tree.Nodes[funcIdentID].Start:doc.Tree.Nodes[funcIdentID].End]
			if bytes.Equal(funcName, []byte("require")) && node.Count > 0 && node.Extra < uint32(len(doc.Tree.ExtraList)) {
				argID := doc.Tree.ExtraList[node.Extra]

				res, ok := doc.evalNode(argID, 0)
				if ok && res.kind == ast.KindString {
					targetDoc := doc.Server.resolveModule(doc.URI, res.str)
					if targetDoc != nil && targetDoc.ExportedNode != ast.InvalidNode {
						return targetDoc.InferType(targetDoc.ExportedNode)
					}
				}
			} else if bytes.Equal(funcName, []byte("setmetatable")) && node.Count >= 2 && node.Extra+1 < uint32(len(doc.Tree.ExtraList)) {
				arg1ID := doc.Tree.ExtraList[node.Extra]
				arg2ID := doc.Tree.ExtraList[node.Extra+1]

				t := doc.InferType(arg1ID)

				metaNodeID := arg2ID
				metaURI := doc.URI

				if doc.Tree.Nodes[arg2ID].Kind == ast.KindIdent {
					defID := doc.Resolver.References[arg2ID]
					if defID != ast.InvalidNode {
						valID := doc.getAssignedValue(defID)
						if valID != ast.InvalidNode {
							metaNodeID = valID
						}
					} else {
						identHash := ast.HashBytes(doc.Source[doc.Tree.Nodes[arg2ID].Start:doc.Tree.Nodes[arg2ID].End])
						if syms, ok := doc.Server.getGlobalSymbols(doc, 0, identHash); ok && len(syms) > 0 {
							sym := syms[0]
							if gDoc, ok := doc.Server.Documents[sym.URI]; ok {
								valID := gDoc.getAssignedValue(sym.NodeID)
								if valID != ast.InvalidNode {
									metaNodeID = valID
									metaURI = sym.URI
								}
							}
						}
					}
				}

				t.MetaNode = metaNodeID
				t.MetaURI = metaURI

				return t
			}
		}

		ctx := doc.Server.resolveSymbolNode(doc.URI, doc, funcIdentID)
		if ctx != nil && ctx.TargetDefID != ast.InvalidNode && ctx.TargetDoc != nil {
			enableAlerts := doc.Server != nil && doc.Server.FeatureFormatAlerts

			luadoc := parseLuaDoc(ctx.TargetDoc.getCommentsAbove(ctx.TargetDefID), enableAlerts)

			if len(luadoc.Returns) > 0 {
				return ParseTypeString(luadoc.Returns[0].Type)
			}

			if luadoc.Class != nil {
				return TypeSet{CustomName: luadoc.Class.Name}
			}

			valID := ctx.TargetDoc.getAssignedValue(ctx.TargetDefID)
			if valID != ast.InvalidNode && ctx.TargetDoc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
				return ctx.TargetDoc.inferFunctionReturnType(valID)
			}
		}
	}

	return TypeSet{}
}

func (doc *Document) findFieldInTable(tableID ast.NodeID, fieldName string) ast.NodeID {
	if tableID == ast.InvalidNode || int(tableID) >= len(doc.Tree.Nodes) {
		return ast.InvalidNode
	}

	node := doc.Tree.Nodes[tableID]
	if node.Kind != ast.KindTableExpr {
		return ast.InvalidNode
	}

	for i := uint16(0); i < node.Count; i++ {
		if node.Extra+uint32(i) >= uint32(len(doc.Tree.ExtraList)) {
			continue
		}

		fieldID := doc.Tree.ExtraList[node.Extra+uint32(i)]
		if int(fieldID) >= len(doc.Tree.Nodes) {
			continue
		}

		field := doc.Tree.Nodes[fieldID]
		if field.Kind == ast.KindRecordField {
			key := doc.Tree.Nodes[field.Left]
			if key.Kind == ast.KindIdent {
				name := doc.Source[key.Start:key.End]
				if string(name) == fieldName {
					return field.Right
				}
			}
		}
	}

	return ast.InvalidNode
}

func (doc *Document) getDefForValue(valID ast.NodeID) ast.NodeID {
	if valID == ast.InvalidNode {
		return ast.InvalidNode
	}

	parentID := doc.Tree.Nodes[valID].Parent
	if parentID == ast.InvalidNode {
		return ast.InvalidNode
	}

	parentNode := doc.Tree.Nodes[parentID]

	switch parentNode.Kind {
	case ast.KindExprList:
		grandParentID := parentNode.Parent
		if grandParentID != ast.InvalidNode {
			grandParentNode := doc.Tree.Nodes[grandParentID]
			if grandParentNode.Kind == ast.KindLocalAssign || grandParentNode.Kind == ast.KindAssign {
				idx := doc.Tree.IndexOfExtra(parentID, valID)
				if idx != -1 {
					lhsNode := doc.Tree.Nodes[grandParentNode.Left]
					if uint16(idx) < lhsNode.Count {
						lhsID := doc.Tree.ExtraList[lhsNode.Extra+uint32(idx)]

						switch doc.Tree.Nodes[lhsID].Kind {
						case ast.KindIdent:
							if grandParentNode.Kind == ast.KindLocalAssign {
								return lhsID
							}
							return doc.Resolver.References[lhsID]
						case ast.KindMemberExpr:
							return doc.Tree.Nodes[lhsID].Right
						case ast.KindIndexExpr:
							return lhsID
						}
					}
				}
			}
		}
	case ast.KindLocalFunction, ast.KindFunctionStmt:
		if parentNode.Right == valID {
			leftNode := doc.Tree.Nodes[parentNode.Left]

			switch leftNode.Kind {
			case ast.KindIdent:
				if parentNode.Kind == ast.KindLocalFunction {
					return parentNode.Left
				}

				return doc.Resolver.References[parentNode.Left]
			case ast.KindMethodName, ast.KindMemberExpr:
				return leftNode.Right
			}
		}
	case ast.KindRecordField, ast.KindIndexField:
		if parentNode.Right == valID {
			if doc.Tree.Nodes[parentNode.Left].Kind == ast.KindIdent {
				return parentNode.Left
			}
		}
	}

	return ast.InvalidNode
}

func (doc *Document) getIndexTable(metaNodeID ast.NodeID) (*Document, ast.NodeID) {
	indexValID := doc.findFieldInTable(metaNodeID, "__index")

	if indexValID == ast.InvalidNode {
		recDef := doc.getDefForValue(metaNodeID)
		if recDef != ast.InvalidNode {
			propHash := ast.HashBytes([]byte("__index"))
			for _, fd := range doc.Resolver.FieldDefs {
				if fd.ReceiverDef == recDef && fd.PropHash == propHash {
					indexValID = doc.getAssignedValue(fd.NodeID)
					if indexValID == ast.InvalidNode {
						indexValID = fd.NodeID
					}

					break
				}
			}
		}
	}

	if indexValID != ast.InvalidNode {
		if doc.Tree.Nodes[indexValID].Kind == ast.KindTableExpr {
			return doc, indexValID
		}

		if doc.Tree.Nodes[indexValID].Kind == ast.KindIdent {
			defID := doc.Resolver.References[indexValID]
			if defID != ast.InvalidNode {
				valID := doc.getAssignedValue(defID)
				if valID != ast.InvalidNode && doc.Tree.Nodes[valID].Kind == ast.KindTableExpr {
					return doc, valID
				}
			} else if doc.Server != nil {
				identHash := ast.HashBytes(doc.Source[doc.Tree.Nodes[indexValID].Start:doc.Tree.Nodes[indexValID].End])
				if syms, ok := doc.Server.getGlobalSymbols(doc, 0, identHash); ok && len(syms) > 0 {
					sym := syms[0]
					if gDoc, ok := doc.Server.Documents[sym.URI]; ok {
						valID := gDoc.getAssignedValue(sym.NodeID)
						if valID != ast.InvalidNode && gDoc.Tree.Nodes[valID].Kind == ast.KindTableExpr {
							return gDoc, valID
						}
					}
				}
			}
		}
	}

	return nil, ast.InvalidNode
}

func (doc *Document) extractArrayElementType(t TypeSet) TypeSet {
	if t.DeclNode != ast.InvalidNode && t.DeclURI != "" {
		targetDoc := doc
		if t.DeclURI != doc.URI {
			if doc.Server != nil {
				targetDoc = doc.Server.Documents[t.DeclURI]
			} else {
				targetDoc = nil
			}
		}

		if targetDoc != nil {
			node := targetDoc.Tree.Nodes[t.DeclNode]
			if node.Kind == ast.KindTableExpr {
				for i := uint16(0); i < node.Count; i++ {
					childID := targetDoc.Tree.ExtraList[node.Extra+uint32(i)]

					child := targetDoc.Tree.Nodes[childID]
					if child.Kind != ast.KindRecordField && child.Kind != ast.KindIndexField {
						return targetDoc.InferType(childID)
					}
				}
			}
		}
	}

	return TypeSet{Basics: TypeUnknown}
}

func (doc *Document) inferFunctionReturnType(funcExprID ast.NodeID) TypeSet {
	var t TypeSet

	var walk func(id ast.NodeID)

	walk = func(id ast.NodeID) {
		if id == ast.InvalidNode {
			return
		}

		node := doc.Tree.Nodes[id]

		if node.Kind == ast.KindFunctionExpr && id != funcExprID {
			return
		}

		if node.Kind == ast.KindReturn {
			if node.Left != ast.InvalidNode {
				exprList := doc.Tree.Nodes[node.Left]
				if exprList.Count > 0 {
					firstExpr := doc.Tree.ExtraList[exprList.Extra]

					rt := doc.InferType(firstExpr)

					t.Basics |= rt.Basics

					if rt.Basics == TypeUnknown && rt.CustomName == "" {
						t.Basics |= TypeAny
					}

					if t.CustomName == "" {
						t.CustomName = rt.CustomName
					}

					if t.DeclNode == ast.InvalidNode && rt.DeclNode != ast.InvalidNode {
						t.DeclNode = rt.DeclNode
						t.DeclURI = rt.DeclURI
					}

					if t.MetaNode == ast.InvalidNode && rt.MetaNode != ast.InvalidNode {
						t.MetaNode = rt.MetaNode
						t.MetaURI = rt.MetaURI
					}
				}
			} else {
				t.Basics |= TypeNil
			}
		}

		walk(node.Left)
		walk(node.Right)

		for i := uint16(0); i < node.Count; i++ {
			walk(doc.Tree.ExtraList[node.Extra+uint32(i)])
		}
	}

	walk(funcExprID)

	return t
}

// ContextualType checks for control flow narrowing directly above the node.
func (doc *Document) ContextualType(id ast.NodeID, offset uint32, base TypeSet) TypeSet {
	if id == ast.InvalidNode || doc.Tree.Nodes[id].Kind != ast.KindIdent {
		return base
	}

	identName := doc.Source[doc.Tree.Nodes[id].Start:doc.Tree.Nodes[id].End]

	curr := id

	for curr != ast.InvalidNode {
		node := doc.Tree.Nodes[curr]

		if node.Kind == ast.KindIf || node.Kind == ast.KindElseIf {
			if node.Right != ast.InvalidNode {
				block := doc.Tree.Nodes[node.Right]

				// Narrow type if we are statically inside a successful type check block
				if offset >= block.Start && offset <= block.End {
					narrowed := doc.checkTypeCondition(node.Left, identName)

					if narrowed.Basics != TypeUnknown || narrowed.CustomName != "" {
						return narrowed
					}
				}
			}
		}

		curr = node.Parent
	}

	return base
}

func (doc *Document) checkTypeCondition(condID ast.NodeID, targetName []byte) TypeSet {
	if condID == ast.InvalidNode {
		return TypeSet{}
	}

	cond := doc.Tree.Nodes[condID]

	// Look for: type(x) == "..."
	if cond.Kind == ast.KindBinaryExpr && cond.Extra == uint32(token.Eq) {
		left := doc.Tree.Nodes[cond.Left]
		right := doc.Tree.Nodes[cond.Right]

		if left.Kind == ast.KindCallExpr {
			fnID := left.Left

			if doc.Tree.Nodes[fnID].Kind == ast.KindIdent {
				fnName := doc.Source[doc.Tree.Nodes[fnID].Start:doc.Tree.Nodes[fnID].End]

				if bytes.Equal(fnName, []byte("type")) {
					if left.Count > 0 {
						argID := doc.Tree.ExtraList[left.Extra]

						if doc.Tree.Nodes[argID].Kind == ast.KindIdent {
							argName := doc.Source[doc.Tree.Nodes[argID].Start:doc.Tree.Nodes[argID].End]

							if bytes.Equal(argName, targetName) {
								if right.Kind == ast.KindString {
									s := doc.Source[right.Start:right.End]

									if len(s) >= 2 {
										s = s[1 : len(s)-1]
									}

									return ParseTypeString(string(s))
								}
							}
						}
					}
				}
			}
		}
	}

	return TypeSet{}
}
