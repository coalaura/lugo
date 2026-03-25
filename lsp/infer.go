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
	Basics     BasicType
}

func ParseTypeString(tStr string) TypeSet {
	var t TypeSet

	for p := range strings.SplitSeq(tStr, "|") {
		p = strings.TrimSpace(p)

		if strings.HasSuffix(p, "?") {
			p = p[:len(p)-1]

			t.Basics |= TypeNil
		}

		switch p {
		case "number", "integer", "float":
			t.Basics |= TypeNumber
		case "string":
			t.Basics |= TypeString
		case "boolean", "bool":
			t.Basics |= TypeBoolean
		case "table":
			t.Basics |= TypeTable
		case "function", "fun":
			t.Basics |= TypeFunction
		case "nil":
			t.Basics |= TypeNil
		case "any":
			t.Basics |= TypeAny
		case "userdata":
			t.Basics |= TypeUserdata
		case "thread":
			t.Basics |= TypeThread
		default:
			if strings.HasPrefix(p, "fun(") {
				t.Basics |= TypeFunction
			} else if strings.HasPrefix(p, "{") {
				t.Basics |= TypeTable
			} else if p != "" {
				t.CustomName = p
			}
		}
	}

	return t
}

func (t TypeSet) Format() string {
	if t.Basics&TypeAny != 0 {
		return "any"
	}

	var parts []string

	if t.Basics&TypeNumber != 0 {
		parts = append(parts, "number")
	}

	if t.Basics&TypeString != 0 {
		parts = append(parts, "string")
	}

	if t.Basics&TypeBoolean != 0 {
		parts = append(parts, "boolean")
	}

	if t.Basics&TypeTable != 0 {
		parts = append(parts, "table")
	}

	if t.Basics&TypeFunction != 0 {
		parts = append(parts, "function")
	}

	if t.Basics&TypeUserdata != 0 {
		parts = append(parts, "userdata")
	}

	if t.Basics&TypeThread != 0 {
		parts = append(parts, "thread")
	}

	if t.CustomName != "" {
		parts = append(parts, t.CustomName)
	}

	if t.Basics&TypeNil != 0 {
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

	var t TypeSet

	node := doc.Tree.Nodes[id]

	switch node.Kind {
	case ast.KindNumber:
		t.Basics = TypeNumber
	case ast.KindString:
		t.Basics = TypeString
	case ast.KindTrue, ast.KindFalse:
		t.Basics = TypeBoolean
	case ast.KindNil:
		t.Basics = TypeNil
	case ast.KindFunctionExpr, ast.KindLocalFunction, ast.KindFunctionStmt:
		t.Basics = TypeFunction
		t.DeclNode = id
		t.DeclURI = doc.URI
	case ast.KindTableExpr:
		t.Basics = TypeTable
		t.DeclNode = id
		t.DeclURI = doc.URI
	case ast.KindBinaryExpr:
		op := node.Extra

		switch token.Kind(op) {
		case token.Plus, token.Minus, token.Asterisk, token.Slash, token.FloorSlash, token.Modulo, token.Caret, token.BitAnd, token.BitOr, token.BitXor, token.ShiftLeft, token.ShiftRight:
			tLeft := doc.InferType(node.Left)
			tRight := doc.InferType(node.Right)

			if tLeft.CustomName != "" {
				t.CustomName = tLeft.CustomName
				t.Basics = tLeft.Basics
			} else if tRight.CustomName != "" {
				t.CustomName = tRight.CustomName
				t.Basics = tRight.Basics
			} else {
				if tLeft.Basics == TypeUnknown || tRight.Basics == TypeUnknown || tLeft.Basics&TypeAny != 0 || tRight.Basics&TypeAny != 0 {
					t.Basics = TypeAny
				} else {
					t.Basics = TypeNumber
				}
			}
		case token.Concat:
			t.Basics = TypeString
		case token.Eq, token.NotEq, token.Less, token.LessEq, token.Greater, token.GreaterEq:
			t.Basics = TypeBoolean
		case token.And, token.Or:
			tLeft := doc.InferType(node.Left)
			tRight := doc.InferType(node.Right)

			t.Basics = tLeft.Basics | tRight.Basics

			if tLeft.Basics == TypeUnknown && tLeft.CustomName == "" {
				t.Basics |= TypeAny
			}

			if tRight.Basics == TypeUnknown && tRight.CustomName == "" {
				t.Basics |= TypeAny
			}

			if t.CustomName == "" {
				t.CustomName = tLeft.CustomName
			}

			if t.CustomName == "" {
				t.CustomName = tRight.CustomName
			}
		}
	case ast.KindUnaryExpr:
		src := doc.Source[node.Start:node.End]

		if bytes.HasPrefix(src, []byte("not")) {
			t.Basics = TypeBoolean
		} else if len(src) > 0 && src[0] == '#' {
			t.Basics = TypeNumber
		} else {
			t.Basics = TypeNumber
		}
	case ast.KindParenExpr:
		t = doc.InferType(node.Left)
	case ast.KindIdent:
		t = doc.inferIdent(id)
	case ast.KindMemberExpr:
		t = doc.inferMemberExpr(node)
	case ast.KindCallExpr, ast.KindMethodCall:
		t = doc.inferCallExpr(node)
	}

	doc.TypeCache[id] = t

	return t
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

	luadoc := parseLuaDoc(targetDoc.getCommentsAbove(targetDef))

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
			pID := targetDoc.Tree.Nodes[targetDef].Parent
			if pID != ast.InvalidNode {
				pNode := targetDoc.Tree.Nodes[pID]

				switch pNode.Kind {
				case ast.KindFunctionExpr:
					t = targetDoc.inferFunctionParameter(targetDef, pID)
				case ast.KindNameList:
					t = targetDoc.inferLoopVariable(targetDef, pID, pNode)
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
	gpID := doc.Tree.Nodes[funcExprID].Parent
	if gpID == ast.InvalidNode {
		return TypeSet{}
	}

	gpNode := doc.Tree.Nodes[gpID]

	var funcDefID ast.NodeID = ast.InvalidNode

	switch gpNode.Kind {
	case ast.KindLocalFunction, ast.KindFunctionStmt:
		funcDefID = gpNode.Left
	case ast.KindAssign, ast.KindLocalAssign, ast.KindRecordField:
		funcDefID = gpID
	}

	if funcDefID == ast.InvalidNode {
		return TypeSet{}
	}

	funcDoc := parseLuaDoc(doc.getCommentsAbove(funcDefID))
	paramName := string(doc.Source[doc.Tree.Nodes[defID].Start:doc.Tree.Nodes[defID].End])

	for _, p := range funcDoc.Params {
		if p.Name == paramName {
			return ParseTypeString(p.Type)
		}
	}

	return TypeSet{}
}

func (doc *Document) inferLoopVariable(defID, nameListID ast.NodeID, nameList ast.Node) TypeSet {
	gpID := doc.Tree.Nodes[nameListID].Parent
	if gpID == ast.InvalidNode {
		return TypeSet{}
	}

	gpNode := doc.Tree.Nodes[gpID]
	if gpNode.Kind != ast.KindForIn || gpNode.Extra == 0 {
		return TypeSet{}
	}

	idx := -1

	for i := uint16(0); i < nameList.Count; i++ {
		if doc.Tree.ExtraList[nameList.Extra+uint32(i)] == defID {
			idx = int(i)

			break
		}
	}

	if idx == -1 {
		return TypeSet{}
	}

	exprList := doc.Tree.Nodes[gpNode.Extra]
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

	// 1. Check initial table literal
	if targetDoc != nil {
		recNode := targetDoc.Tree.Nodes[leftType.DeclNode]
		if recNode.Kind == ast.KindTableExpr {
			for i := uint16(0); i < recNode.Count; i++ {
				fieldID := targetDoc.Tree.ExtraList[recNode.Extra+uint32(i)]
				field := targetDoc.Tree.Nodes[fieldID]

				if field.Kind == ast.KindRecordField {
					key := targetDoc.Tree.Nodes[field.Left]

					if key.Kind == ast.KindIdent {
						keyName := targetDoc.Source[key.Start:key.End]
						if bytes.Equal(keyName, fieldName) {
							t = targetDoc.InferType(field.Right)

							break
						}
					}
				}
			}
		}
	}

	// 2. Check subsequent assignments in the local document
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
					}
				}
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
			}
		}

		ctx := doc.Server.resolveSymbolNode(doc.URI, doc, funcIdentID)
		if ctx != nil && ctx.TargetDoc != nil && ctx.TargetDefID != ast.InvalidNode {
			luadoc := parseLuaDoc(ctx.TargetDoc.getCommentsAbove(ctx.TargetDefID))

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
