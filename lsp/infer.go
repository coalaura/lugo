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
	Basics     BasicType
	CustomName string
	DeclNode   ast.NodeID
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

	if int(id) < len(doc.TypeCache) {
		if doc.TypeCache[id].Basics != TypeUnknown || doc.TypeCache[id].CustomName != "" || doc.TypeCache[id].DeclNode != ast.InvalidNode {
			return doc.TypeCache[id]
		}

		if doc.Inferring[id] {
			return TypeSet{} // Cycle detected
		}

		doc.Inferring[id] = true

		defer func() {
			doc.Inferring[id] = false
		}()
	}

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
	case ast.KindTableExpr:
		t.Basics = TypeTable
		t.DeclNode = id
	case ast.KindBinaryExpr:
		op := node.Extra

		switch token.Kind(op) {
		case token.Plus, token.Minus, token.Asterisk, token.Slash, token.FloorSlash, token.Modulo, token.Caret, token.BitAnd, token.BitOr, token.BitXor, token.ShiftLeft, token.ShiftRight:
			t.Basics = TypeNumber
		case token.Concat:
			t.Basics = TypeString
		case token.Eq, token.NotEq, token.Less, token.LessEq, token.Greater, token.GreaterEq:
			t.Basics = TypeBoolean
		case token.And, token.Or:
			tLeft := doc.InferType(node.Left)
			tRight := doc.InferType(node.Right)

			t.Basics = tLeft.Basics | tRight.Basics

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
		defID := doc.Resolver.References[id]
		if defID != ast.InvalidNode {
			t = doc.inferIdent(defID)
		}
	case ast.KindMemberExpr:
		t = doc.inferMemberExpr(node)
	case ast.KindCallExpr, ast.KindMethodCall:
		t = doc.inferCallExpr(node)
	}

	if int(id) < len(doc.TypeCache) {
		doc.TypeCache[id] = t
	}

	return t
}

func (doc *Document) inferIdent(defID ast.NodeID) TypeSet {
	// 1. Check LuaDoc first
	luadoc := parseLuaDoc(doc.getCommentsAbove(defID))
	if luadoc.Type != nil {
		return ParseTypeString(luadoc.Type.Type)
	}

	if luadoc.Class != nil {
		return TypeSet{CustomName: luadoc.Class.Name}
	}

	// 2. Check assignment value
	valID := doc.getAssignedValue(defID)
	if valID != ast.InvalidNode {
		return doc.InferType(valID)
	}

	// 3. Fallback: Could be a parameter or loop variable
	if doc.Tree.Nodes[defID].Kind != ast.KindIdent {
		return TypeSet{}
	}

	pID := doc.Tree.Nodes[defID].Parent
	if pID == ast.InvalidNode {
		return TypeSet{}
	}

	pNode := doc.Tree.Nodes[pID]

	switch pNode.Kind {
	case ast.KindFunctionExpr:
		return doc.inferFunctionParameter(defID, pID)
	case ast.KindNameList:
		return doc.inferLoopVariable(defID, pID, pNode)
	}

	return TypeSet{}
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
			argID := doc.Tree.ExtraList[firstExpr.Extra]
			return doc.extractArrayElementType(doc.InferType(argID))
		}
	} else if bytes.Equal(funcName, []byte("pairs")) {
		switch idx {
		case 0:
			return TypeSet{Basics: TypeAny}
		case 1:
			argID := doc.Tree.ExtraList[firstExpr.Extra]
			return doc.extractArrayElementType(doc.InferType(argID))
		}
	}

	return TypeSet{}
}

func (doc *Document) inferMemberExpr(node ast.Node) TypeSet {
	leftType := doc.InferType(node.Left)

	if leftType.DeclNode == ast.InvalidNode {
		return TypeSet{}
	}

	recNode := doc.Tree.Nodes[leftType.DeclNode]
	if recNode.Kind != ast.KindTableExpr {
		return TypeSet{}
	}

	rightNode := doc.Tree.Nodes[node.Right]
	if rightNode.Kind != ast.KindIdent {
		return TypeSet{}
	}

	fieldName := doc.Source[rightNode.Start:rightNode.End]

	for i := uint16(0); i < recNode.Count; i++ {
		fieldID := doc.Tree.ExtraList[recNode.Extra+uint32(i)]
		field := doc.Tree.Nodes[fieldID]

		if field.Kind == ast.KindRecordField {
			key := doc.Tree.Nodes[field.Left]

			if key.Kind == ast.KindIdent {
				keyName := doc.Source[key.Start:key.End]
				if bytes.Equal(keyName, fieldName) {
					return doc.InferType(field.Right)
				}
			}
		}
	}

	return TypeSet{}
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

	defID := doc.Resolver.References[funcIdentID]
	if defID == ast.InvalidNode {
		return TypeSet{}
	}

	luadoc := parseLuaDoc(doc.getCommentsAbove(defID))
	if len(luadoc.Returns) > 0 {
		return ParseTypeString(luadoc.Returns[0].Type)
	}

	valID := doc.getAssignedValue(defID)
	if valID != ast.InvalidNode && doc.Tree.Nodes[valID].Kind == ast.KindFunctionExpr {
		return doc.inferFunctionReturnType(valID)
	}

	return TypeSet{}
}

func (doc *Document) extractArrayElementType(t TypeSet) TypeSet {
	if t.DeclNode != ast.InvalidNode {
		node := doc.Tree.Nodes[t.DeclNode]
		if node.Kind == ast.KindTableExpr {
			for i := uint16(0); i < node.Count; i++ {
				childID := doc.Tree.ExtraList[node.Extra+uint32(i)]

				child := doc.Tree.Nodes[childID]
				if child.Kind != ast.KindRecordField && child.Kind != ast.KindIndexField {
					return doc.InferType(childID)
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
