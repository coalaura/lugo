package semantic

import (
	"bytes"

	"github.com/coalaura/lugo/ast"
)

type FieldDef struct {
	ReceiverName []byte
	ReceiverHash uint64
	PropHash     uint64
	ReceiverDef  ast.NodeID
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

type FieldKey struct {
	RecDef   ast.NodeID
	RecHash  uint64
	PropHash uint64
}

type Reassignment struct {
	NameHash uint64
	DefID    ast.NodeID
	ValID    ast.NodeID
}

// Resolver walks the AST and links variable references to their local definitions.
type Resolver struct {
	Tree *ast.Tree

	References []ast.NodeID
	GlobalRefs []ast.NodeID
	GlobalDefs []ast.NodeID
	FieldDefs  []FieldDef
	fieldMap   map[FieldKey]ast.NodeID

	PendingFields []FieldRef

	scopeStack  []ast.NodeID
	scopeStarts []int

	DuplicateLocals []ast.NodeID
	LocalDefs       []ast.NodeID
	ShadowedOuter   []ShadowPair
	Reassignments   []Reassignment

	nameArena []byte
}

func New(tree *ast.Tree) *Resolver {
	return &Resolver{
		Tree:            tree,
		References:      make([]ast.NodeID, len(tree.Nodes)),
		LocalDefs:       make([]ast.NodeID, 0, 512),
		ShadowedOuter:   make([]ShadowPair, 0, 64),
		PendingFields:   make([]FieldRef, 0, 128),
		FieldDefs:       make([]FieldDef, 0, 512),
		fieldMap:        make(map[FieldKey]ast.NodeID, 512),
		GlobalDefs:      make([]ast.NodeID, 0, 256),
		GlobalRefs:      make([]ast.NodeID, 0, 512),
		scopeStack:      make([]ast.NodeID, 0, 256),
		scopeStarts:     make([]int, 0, 64),
		DuplicateLocals: make([]ast.NodeID, 0, 16),
		Reassignments:   make([]Reassignment, 0, 128),
		nameArena:       make([]byte, 0, 2048),
	}
}

func (r *Resolver) Reset() {
	nodeCount := len(r.Tree.Nodes)

	if cap(r.References) >= nodeCount {
		r.References = r.References[:nodeCount]

		clear(r.References)
	} else {
		r.References = make([]ast.NodeID, nodeCount)
	}

	r.GlobalDefs = r.GlobalDefs[:0]
	r.GlobalRefs = r.GlobalRefs[:0]
	r.FieldDefs = r.FieldDefs[:0]

	if r.fieldMap == nil {
		r.fieldMap = make(map[FieldKey]ast.NodeID, 512)
	} else {
		clear(r.fieldMap)
	}

	r.PendingFields = r.PendingFields[:0]

	if r.scopeStack == nil {
		r.scopeStack = make([]ast.NodeID, 0, 256)
	} else {
		r.scopeStack = r.scopeStack[:0]
	}

	if r.scopeStarts == nil {
		r.scopeStarts = make([]int, 0, 64)
	} else {
		r.scopeStarts = r.scopeStarts[:0]
	}

	r.DuplicateLocals = r.DuplicateLocals[:0]
	r.LocalDefs = r.LocalDefs[:0]
	r.ShadowedOuter = r.ShadowedOuter[:0]
	r.Reassignments = r.Reassignments[:0]

	if r.nameArena == nil {
		r.nameArena = make([]byte, 0, 2048)
	} else {
		r.nameArena = r.nameArena[:0]
	}
}

func (r *Resolver) Cleanup() {
	r.fieldMap = nil
	r.scopeStack = nil
	r.scopeStarts = nil

	// intentionally not resetting nameArena, FieldDef.ReceiverName slices point to it
}

func (r *Resolver) Resolve(root ast.NodeID) {
	r.visit(root)

	for _, pref := range r.PendingFields {
		fk := FieldKey{
			RecDef:   pref.ReceiverDef,
			RecHash:  pref.ReceiverHash,
			PropHash: pref.PropHash,
		}

		if defID, ok := r.fieldMap[fk]; ok {
			r.References[pref.PropNodeID] = defID
		}
	}
}

func (r *Resolver) pushScope() int {
	startScope := len(r.scopeStack)

	r.scopeStarts = append(r.scopeStarts, startScope)

	return startScope
}

func (r *Resolver) popScope(startScope int) {
	if len(r.scopeStarts) > 0 {
		r.scopeStarts = r.scopeStarts[:len(r.scopeStarts)-1]
	}

	if startScope >= 0 && startScope <= len(r.scopeStack) {
		r.scopeStack = r.scopeStack[:startScope]
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
	if len(name) > 0 && name[0] != '_' && !(len(name) > 2 && name[0] == '.' && name[1] == '.' && name[2] == '.') {
		var scopeStart int

		if len(r.scopeStarts) > 0 {
			scopeStart = r.scopeStarts[len(r.scopeStarts)-1]
		}

		nameLen := uint32(len(name))
		for i := len(r.scopeStack) - 1; i >= 0; i-- {
			defID := r.scopeStack[i]
			defNode := r.Tree.Nodes[defID]

			if defNode.End-defNode.Start == nameLen {
				if bytes.Equal(r.Tree.Source[defNode.Start:defNode.End], name) {
					if i >= scopeStart {
						r.DuplicateLocals = append(r.DuplicateLocals, identID)
					} else {
						r.ShadowedOuter = append(r.ShadowedOuter, ShadowPair{
							Shadowing: identID,
							Shadowed:  defID,
						})
					}

					break
				}
			}
		}
	}

	r.scopeStack = append(r.scopeStack, identID)
}

func (r *Resolver) defineField(memberNodeID ast.NodeID) {
	node := r.Tree.Nodes[memberNodeID]
	if node.Right == ast.InvalidNode || r.Tree.Nodes[node.Right].Kind != ast.KindIdent || r.Tree.Nodes[node.Right].Start == r.Tree.Nodes[node.Right].End {
		return
	}

	recDef, recHash, recName := r.getReceiverContextArena(node.Left)
	if len(recName) == 0 {
		return
	}

	propHash := ast.HashBytes(r.source(node.Right))

	fk := FieldKey{
		RecDef:   recDef,
		RecHash:  recHash,
		PropHash: propHash,
	}

	if existingID, ok := r.fieldMap[fk]; ok {
		r.References[node.Right] = existingID

		return
	}

	r.FieldDefs = append(r.FieldDefs, FieldDef{
		ReceiverDef:  recDef,
		ReceiverHash: recHash,
		ReceiverName: recName,
		PropHash:     propHash,
		NodeID:       node.Right,
	})

	r.fieldMap[fk] = node.Right
	r.References[node.Right] = node.Right
}

func (r *Resolver) resolveReference(identID ast.NodeID, isDef bool) {
	if identID == ast.InvalidNode {
		return
	}

	targetNode := r.Tree.Nodes[identID]
	if targetNode.Start >= targetNode.End || targetNode.End > uint32(len(r.Tree.Source)) {
		return
	}

	targetSrc := r.Tree.Source[targetNode.Start:targetNode.End]
	targetLen := targetNode.End - targetNode.Start

	for i := len(r.scopeStack) - 1; i >= 0; i-- {
		defID := r.scopeStack[i]
		defNode := r.Tree.Nodes[defID]

		if defNode.End-defNode.Start == targetLen {
			if bytes.Equal(targetSrc, r.Tree.Source[defNode.Start:defNode.End]) {
				r.References[identID] = defID

				return
			}
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

func (r *Resolver) GetReceiverContext(recID ast.NodeID) (ast.NodeID, uint64, []byte) {
	if recID == ast.InvalidNode {
		return ast.InvalidNode, 0, nil
	}

	curr := recID

	var rootDef ast.NodeID = ast.InvalidNode

	for curr != ast.InvalidNode {
		node := r.Tree.Nodes[curr]

		if node.Kind == ast.KindIdent {
			rootDef = r.References[curr]

			break
		} else if node.Kind == ast.KindMemberExpr {
			curr = node.Left
		} else {
			return ast.InvalidNode, 0, nil
		}
	}

	recBytes := r.buildMemberName(recID, nil)

	return rootDef, ast.HashBytes(recBytes), recBytes
}

func (r *Resolver) getReceiverContextArena(recID ast.NodeID) (ast.NodeID, uint64, []byte) {
	if recID == ast.InvalidNode {
		return ast.InvalidNode, 0, nil
	}

	curr := recID

	var rootDef ast.NodeID = ast.InvalidNode

	for curr != ast.InvalidNode {
		node := r.Tree.Nodes[curr]

		if node.Kind == ast.KindIdent {
			rootDef = r.References[curr]
			break
		} else if node.Kind == ast.KindMemberExpr {
			curr = node.Left
		} else {
			return ast.InvalidNode, 0, nil
		}
	}

	startIdx := len(r.nameArena)

	r.nameArena = r.buildMemberName(recID, r.nameArena)

	recBytes := r.nameArena[startIdx:]

	return rootDef, ast.HashBytes(recBytes), recBytes
}

func (r *Resolver) buildMemberName(id ast.NodeID, buf []byte) []byte {
	if id == ast.InvalidNode {
		return buf
	}

	node := r.Tree.Nodes[id]

	switch node.Kind {
	case ast.KindIdent:
		buf = append(buf, r.source(id)...)
	case ast.KindMemberExpr:
		buf = r.buildMemberName(node.Left, buf)

		buf = append(buf, '.')

		buf = r.buildMemberName(node.Right, buf)
	}

	return buf
}

func (r *Resolver) getTableReceiver(id ast.NodeID) (ast.NodeID, []byte) {
	parentID := r.Tree.Nodes[id].Parent
	if parentID == ast.InvalidNode {
		return ast.InvalidNode, nil
	}

	parentNode := r.Tree.Nodes[parentID]

	if parentNode.Kind == ast.KindExprList {
		grandParentID := parentNode.Parent
		if grandParentID == ast.InvalidNode {
			return ast.InvalidNode, nil
		}

		grandParentNode := r.Tree.Nodes[grandParentID]
		if (grandParentNode.Kind != ast.KindAssign && grandParentNode.Kind != ast.KindLocalAssign) || grandParentNode.Right != parentID {
			return ast.InvalidNode, nil
		}

		idx := r.Tree.IndexOfExtra(parentID, id)
		if idx == -1 {
			return ast.InvalidNode, nil
		}

		lhsNode := r.Tree.Nodes[grandParentNode.Left]
		if uint16(idx) >= lhsNode.Count {
			return ast.InvalidNode, nil
		}

		leftID := r.Tree.ExtraList[lhsNode.Extra+uint32(idx)]

		if grandParentNode.Kind == ast.KindLocalAssign {
			return leftID, r.source(leftID)
		} else if r.Tree.Nodes[leftID].Kind == ast.KindIdent {
			return r.References[leftID], r.source(leftID)
		} else if r.Tree.Nodes[leftID].Kind == ast.KindMemberExpr {
			defID, _, recBytes := r.getReceiverContextArena(leftID)
			return defID, recBytes
		}

		return ast.InvalidNode, nil
	}

	if parentNode.Kind == ast.KindRecordField {
		grandParentID := parentNode.Parent
		if grandParentID != ast.InvalidNode && r.Tree.Nodes[grandParentID].Kind == ast.KindTableExpr {
			parentDef, parentRec := r.getTableReceiver(grandParentID)
			if len(parentRec) > 0 {
				startIdx := len(r.nameArena)

				r.nameArena = append(r.nameArena, parentRec...)
				r.nameArena = append(r.nameArena, '.')
				r.nameArena = append(r.nameArena, r.source(parentNode.Left)...)

				return parentDef, r.nameArena[startIdx:]
			}
		}
	}

	return ast.InvalidNode, nil
}

func (r *Resolver) source(id ast.NodeID) []byte {
	if uint(id) < uint(len(r.Tree.Nodes)) {
		node := r.Tree.Nodes[id]

		if node.Start <= node.End && uint(node.End) <= uint(len(r.Tree.Source)) {
			return r.Tree.Source[node.Start:node.End]
		}
	}

	return nil
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
		startScope := r.pushScope()

		for i := uint16(0); i < node.Count; i++ {
			r.visit(r.Tree.ExtraList[node.Extra+uint32(i)])
		}

		r.popScope(startScope)
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

		startScope := r.pushScope()

		r.declare(node.Left)
		r.visit(node.Right)

		r.popScope(startScope)
	case ast.KindForIn:
		r.visit(ast.NodeID(node.Extra))

		startScope := r.pushScope()
		nameList := r.Tree.Nodes[node.Left]

		for i := uint16(0); i < nameList.Count; i++ {
			r.declare(r.Tree.ExtraList[nameList.Extra+uint32(i)])
		}

		r.visit(node.Right)

		r.popScope(startScope)
	case ast.KindIdent, ast.KindVararg:
		r.resolveReference(id, false)
	case ast.KindAssign:
		listNode := r.Tree.Nodes[node.Left]
		for i := uint16(0); i < listNode.Count; i++ {
			exprID := r.Tree.ExtraList[listNode.Extra+uint32(i)]
			exprNode := r.Tree.Nodes[exprID]

			switch exprNode.Kind {
			case ast.KindIdent:
				r.resolveReference(exprID, true)

				defID := r.References[exprID]
				rhsList := node.Right

				var valID ast.NodeID = ast.InvalidNode

				if rhsList != ast.InvalidNode {
					rhsNode := r.Tree.Nodes[rhsList]
					if i < uint16(rhsNode.Count) {
						valID = r.Tree.ExtraList[rhsNode.Extra+uint32(i)]
					}
				}

				var nameHash uint64

				if defID == ast.InvalidNode {
					nameHash = ast.HashBytes(r.source(exprID))
				}

				r.Reassignments = append(r.Reassignments, Reassignment{
					DefID:    defID,
					NameHash: nameHash,
					ValID:    valID,
				})
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

		if node.Right != ast.InvalidNode && r.Tree.Nodes[node.Right].Kind == ast.KindIdent && r.Tree.Nodes[node.Right].Start < r.Tree.Nodes[node.Right].End {
			recDef, recHash, recName := r.getReceiverContextArena(node.Left)

			if len(recName) > 0 {
				propHash := ast.HashBytes(r.source(node.Right))

				r.PendingFields = append(r.PendingFields, FieldRef{
					PropNodeID:   node.Right,
					ReceiverDef:  recDef,
					ReceiverHash: recHash,
					ReceiverName: recName,
					PropHash:     propHash,
				})
			}
		}

		if node.Kind == ast.KindMethodCall {
			r.visitArgs(node.Extra, node.Count)
		}
	case ast.KindMethodName:
		r.visit(node.Left)
	case ast.KindCallExpr:
		r.visit(node.Left)
		r.visitArgs(node.Extra, node.Count)
	case ast.KindTableExpr:
		recDef, recBytes := r.getTableReceiver(id)

		var recHash uint64

		if len(recBytes) > 0 {
			recHash = ast.HashBytes(recBytes)
		}

		for i := uint16(0); i < node.Count; i++ {
			fieldID := r.Tree.ExtraList[node.Extra+uint32(i)]
			fieldNode := r.Tree.Nodes[fieldID]

			switch fieldNode.Kind {
			case ast.KindRecordField:
				if len(recBytes) > 0 && r.Tree.Nodes[fieldNode.Left].Kind == ast.KindIdent && r.Tree.Nodes[fieldNode.Left].Start < r.Tree.Nodes[fieldNode.Left].End {
					propHash := ast.HashBytes(r.source(fieldNode.Left))

					r.FieldDefs = append(r.FieldDefs, FieldDef{
						ReceiverDef:  recDef,
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
		startScope := r.pushScope()

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

		r.popScope(startScope)
	case ast.KindRepeat:
		startScope := r.pushScope()

		// Condition is evaluated inside the block's scope
		blockNode := r.Tree.Nodes[node.Left]

		for i := uint16(0); i < blockNode.Count; i++ {
			r.visit(r.Tree.ExtraList[blockNode.Extra+uint32(i)])
		}

		r.visit(node.Right)

		r.popScope(startScope)
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
