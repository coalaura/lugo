package semantic

import (
	"bytes"

	"github.com/coalaura/lugo/ast"
)

type FieldDef struct {
	ReceiverDef  ast.NodeID // Valid if local table
	ReceiverHash uint64     // Valid if global table
	ReceiverName []byte
	PropHash     uint64
	NodeID       ast.NodeID
}

type FieldRef struct {
	PropNodeID   ast.NodeID
	ReceiverDef  ast.NodeID
	ReceiverHash uint64
	ReceiverName []byte
	PropHash     uint64
}

type ShadowPair struct {
	Shadowing ast.NodeID
	Shadowed  ast.NodeID
}

// Resolver walks the AST and links variable references to their local definitions.
type Resolver struct {
	Tree *ast.Tree

	References []ast.NodeID
	GlobalRefs []ast.NodeID
	GlobalDefs []ast.NodeID
	FieldDefs  []FieldDef

	PendingFields []FieldRef

	scopeStack []ast.NodeID

	UsageCount    []uint16
	LocalDefs     []ast.NodeID
	ShadowedOuter []ShadowPair
}

func New(tree *ast.Tree) *Resolver {
	return &Resolver{
		Tree:          tree,
		References:    make([]ast.NodeID, len(tree.Nodes)),
		UsageCount:    make([]uint16, len(tree.Nodes)),
		LocalDefs:     make([]ast.NodeID, 0, 128),
		ShadowedOuter: make([]ShadowPair, 0, 16),
		PendingFields: make([]FieldRef, 0, 128),
		scopeStack:    make([]ast.NodeID, 0, 64),
	}
}

func (r *Resolver) Resolve(root ast.NodeID) {
	r.visit(root)

	for _, pref := range r.PendingFields {
		for _, fd := range r.FieldDefs {
			if fd.PropHash == pref.PropHash && fd.ReceiverDef == pref.ReceiverDef && fd.ReceiverHash == pref.ReceiverHash {
				r.References[pref.PropNodeID] = fd.NodeID

				break
			}
		}
	}
}

func (r *Resolver) declare(identID ast.NodeID) {
	if identID == ast.InvalidNode {
		return
	}

	r.References[identID] = identID

	r.LocalDefs = append(r.LocalDefs, identID)

	name := r.source(identID)

	// ignore "_" prefix
	if len(name) > 0 && name[0] != '_' {
		for i := len(r.scopeStack) - 1; i >= 0; i-- {
			if bytes.Equal(r.source(r.scopeStack[i]), name) {
				r.ShadowedOuter = append(r.ShadowedOuter, ShadowPair{
					Shadowing: identID,
					Shadowed:  r.scopeStack[i],
				})

				break
			}
		}
	}

	r.scopeStack = append(r.scopeStack, identID)
}

func (r *Resolver) defineField(memberNodeID ast.NodeID) {
	node := r.Tree.Nodes[memberNodeID]
	if node.Right == ast.InvalidNode || r.Tree.Nodes[node.Right].Kind != ast.KindIdent {
		return
	}

	recDef, recHash, recName := r.getReceiverContext(node.Left)
	propHash := ast.HashBytes(r.source(node.Right))

	for _, fd := range r.FieldDefs {
		if fd.PropHash == propHash && fd.ReceiverDef == recDef && fd.ReceiverHash == recHash {
			r.References[node.Right] = fd.NodeID

			return
		}
	}

	r.FieldDefs = append(r.FieldDefs, FieldDef{
		ReceiverDef:  recDef,
		ReceiverHash: recHash,
		ReceiverName: recName,
		PropHash:     propHash,
		NodeID:       node.Right,
	})

	r.References[node.Right] = node.Right
}

func (r *Resolver) resolveReference(identID ast.NodeID, isDef bool) {
	if identID == ast.InvalidNode {
		return
	}

	targetSrc := r.source(identID)

	for i := len(r.scopeStack) - 1; i >= 0; i-- {
		defID := r.scopeStack[i]

		if bytes.Equal(targetSrc, r.source(defID)) {
			r.References[identID] = defID

			r.UsageCount[defID]++

			return
		}
	}

	if bytes.Equal(targetSrc, []byte("self")) {
		return
	}

	if isDef {
		r.GlobalDefs = append(r.GlobalDefs, identID)
	} else {
		r.GlobalRefs = append(r.GlobalRefs, identID)
	}
}

func (r *Resolver) getReceiverContext(recID ast.NodeID) (ast.NodeID, uint64, []byte) {
	if r.Tree.Nodes[recID].Kind == ast.KindIdent {
		def := r.References[recID]

		if def != ast.InvalidNode {
			return def, 0, r.source(recID)
		}
	}

	recBytes := r.source(recID)

	return ast.InvalidNode, ast.HashBytes(recBytes), recBytes
}

func (r *Resolver) getTableReceiver(id ast.NodeID) []byte {
	parentID := r.Tree.Nodes[id].Parent
	if parentID == ast.InvalidNode {
		return nil
	}

	pNode := r.Tree.Nodes[parentID]

	if pNode.Kind == ast.KindExprList {
		gpID := pNode.Parent
		if gpID == ast.InvalidNode {
			return nil
		}

		gpNode := r.Tree.Nodes[gpID]

		if (gpNode.Kind == ast.KindAssign || gpNode.Kind == ast.KindLocalAssign) && gpNode.Right == parentID {
			idx := -1

			for i := uint16(0); i < pNode.Count; i++ {
				if r.Tree.ExtraList[pNode.Extra+uint32(i)] == id {
					idx = int(i)

					break
				}
			}

			if idx != -1 {
				lhsNode := r.Tree.Nodes[gpNode.Left]
				if uint16(idx) < lhsNode.Count {
					return r.source(r.Tree.ExtraList[lhsNode.Extra+uint32(idx)])
				}
			}
		}

		return nil
	}

	if pNode.Kind == ast.KindRecordField {
		keyNode := r.Tree.Nodes[pNode.Left]

		grandParentID := pNode.Parent
		if grandParentID != ast.InvalidNode && r.Tree.Nodes[grandParentID].Kind == ast.KindTableExpr {
			parentRec := r.getTableReceiver(grandParentID)
			if len(parentRec) > 0 {
				res := make([]byte, 0, len(parentRec)+1+int(keyNode.End-keyNode.Start))

				res = append(res, parentRec...)
				res = append(res, '.')
				res = append(res, r.source(pNode.Left)...)

				return res
			}
		}
	}

	return nil
}

func (r *Resolver) source(id ast.NodeID) []byte {
	node := r.Tree.Nodes[id]

	return r.Tree.Source[node.Start:node.End]
}

func (r *Resolver) visit(id ast.NodeID) {
	if id == ast.InvalidNode {
		return
	}

	node := r.Tree.Nodes[id]

	switch node.Kind {
	case ast.KindFile, ast.KindDo, ast.KindWhile, ast.KindElseIf, ast.KindElse:
		r.visit(node.Left)
		r.visit(node.Right)
	case ast.KindBlock:
		startScope := len(r.scopeStack)

		for i := uint16(0); i < node.Count; i++ {
			r.visit(r.Tree.ExtraList[node.Extra+uint32(i)])
		}

		r.scopeStack = r.scopeStack[:startScope]
	case ast.KindLocalAssign:
		r.visit(node.Right) // RHS evaluated before LHS is added to scope

		nameList := r.Tree.Nodes[node.Left]

		for i := uint16(0); i < nameList.Count; i++ {
			r.declare(r.Tree.ExtraList[nameList.Extra+uint32(i)])
		}
	case ast.KindLocalFunction:
		r.declare(node.Left) // Local functions are in scope for their own body
		r.visit(node.Right)
	case ast.KindForNum:
		for i := uint16(0); i < node.Count; i++ {
			r.visit(r.Tree.ExtraList[node.Extra+uint32(i)])
		}

		startScope := len(r.scopeStack)

		r.declare(node.Left)
		r.visit(node.Right)

		r.scopeStack = r.scopeStack[:startScope]
	case ast.KindForIn:
		r.visit(ast.NodeID(node.Extra))

		startScope := len(r.scopeStack)
		nameList := r.Tree.Nodes[node.Left]

		for i := uint16(0); i < nameList.Count; i++ {
			r.declare(r.Tree.ExtraList[nameList.Extra+uint32(i)])
		}

		r.visit(node.Right)
		r.scopeStack = r.scopeStack[:startScope]
	case ast.KindIdent:
		r.resolveReference(id, false)
	case ast.KindAssign:
		listNode := r.Tree.Nodes[node.Left]
		for i := uint16(0); i < listNode.Count; i++ {
			exprID := r.Tree.ExtraList[listNode.Extra+uint32(i)]
			exprNode := r.Tree.Nodes[exprID]

			switch exprNode.Kind {
			case ast.KindIdent:
				r.resolveReference(exprID, true)
			case ast.KindMemberExpr, ast.KindIndexExpr:
				r.visit(exprNode.Left)

				if exprNode.Kind == ast.KindMemberExpr {
					r.defineField(exprID)
				} else {
					r.visit(exprNode.Right)
				}
			default:
				r.visit(exprID)
			}
		}
		r.visit(node.Right)
	case ast.KindBinaryExpr, ast.KindUnaryExpr, ast.KindParenExpr, ast.KindIndexExpr, ast.KindReturn:
		r.visit(node.Left)
		r.visit(node.Right)
	case ast.KindMemberExpr, ast.KindMethodCall:
		r.visit(node.Left)

		if node.Right != ast.InvalidNode && r.Tree.Nodes[node.Right].Kind == ast.KindIdent {
			recDef, recHash, recName := r.getReceiverContext(node.Left)
			propHash := ast.HashBytes(r.source(node.Right))

			r.PendingFields = append(r.PendingFields, FieldRef{
				PropNodeID:   node.Right,
				ReceiverDef:  recDef,
				ReceiverHash: recHash,
				ReceiverName: recName,
				PropHash:     propHash,
			})
		}

		if node.Kind == ast.KindMethodCall {
			r.visitArgs(node.Extra, node.Count)
		}
	case ast.KindMethodName:
		r.visit(node.Left)
		r.visitArgs(node.Extra, node.Count)
	case ast.KindCallExpr:
		r.visit(node.Left)
		r.visitArgs(node.Extra, node.Count)
	case ast.KindTableExpr:
		recBytes := r.getTableReceiver(id)

		var recHash uint64

		if len(recBytes) > 0 {
			recHash = ast.HashBytes(recBytes)
		}

		for i := uint16(0); i < node.Count; i++ {
			fieldID := r.Tree.ExtraList[node.Extra+uint32(i)]
			fieldNode := r.Tree.Nodes[fieldID]

			switch fieldNode.Kind {
			case ast.KindRecordField:
				if len(recBytes) > 0 && r.Tree.Nodes[fieldNode.Left].Kind == ast.KindIdent {
					propHash := ast.HashBytes(r.source(fieldNode.Left))

					r.FieldDefs = append(r.FieldDefs, FieldDef{
						ReceiverDef:  ast.InvalidNode,
						ReceiverHash: recHash,
						ReceiverName: recBytes,
						PropHash:     propHash,
						NodeID:       fieldNode.Left,
					})

					r.References[fieldNode.Left] = fieldNode.Left
				}

				r.visit(fieldNode.Right)
			case ast.KindIndexField:
				r.visit(fieldNode.Left)
				r.visit(fieldNode.Right)
			default:
				r.visit(fieldID)
			}
		}
	case ast.KindFunctionExpr, ast.KindFunctionStmt:
		startScope := len(r.scopeStack)

		if node.Kind == ast.KindFunctionExpr {
			for i := uint16(0); i < node.Count; i++ {
				r.declare(r.Tree.ExtraList[node.Extra+uint32(i)])
			}
		} else {
			leftNode := r.Tree.Nodes[node.Left]

			switch leftNode.Kind {
			case ast.KindIdent:
				r.resolveReference(node.Left, true)
			case ast.KindMethodName, ast.KindMemberExpr:
				r.visit(leftNode.Left)
				r.defineField(node.Left)
			default:
				r.visit(node.Left)
			}
		}

		r.visit(node.Right)

		r.scopeStack = r.scopeStack[:startScope]
	case ast.KindRepeat:
		startScope := len(r.scopeStack)

		// Condition is evaluated inside the block's scope
		blockNode := r.Tree.Nodes[node.Left]

		for i := uint16(0); i < blockNode.Count; i++ {
			r.visit(r.Tree.ExtraList[blockNode.Extra+uint32(i)])
		}

		r.visit(node.Right)

		r.scopeStack = r.scopeStack[:startScope]
	case ast.KindExprList:
		r.visitArgs(node.Extra, node.Count)
	case ast.KindIf:
		r.visit(node.Left)
		r.visit(node.Right)

		for i := uint16(0); i < node.Count; i++ {
			r.visit(r.Tree.ExtraList[node.Extra+uint32(i)])
		}
	}
}

func (r *Resolver) visitArgs(extraStart uint32, count uint16) {
	for i := range count {
		r.visit(r.Tree.ExtraList[extraStart+uint32(i)])
	}
}
