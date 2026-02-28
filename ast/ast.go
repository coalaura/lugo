package ast

import (
	"bytes"

	"github.com/coalaura/lugo/token"
)

// NodeKind defines what type of syntax this node represents.
type NodeKind uint8

type Attr uint32

const (
	KindInvalid NodeKind = iota
	KindFile
	KindBlock
	KindLocalAssign // local a = 1
	KindAssign      // a = 1
	KindIdent
	KindNumber
	KindString
	KindBinaryExpr // a + b
	KindUnaryExpr  // -a
	KindParenExpr

	KindNil
	KindTrue
	KindFalse
	KindVararg
	KindFunctionExpr
	KindTableExpr

	KindIndexExpr  // a[b]
	KindMemberExpr // a.b
	KindCallExpr   // a(b)

	KindRecordField // a = 1 inside a table
	KindIndexField  // [a] = 1 inside a table
	KindMethodCall  // a:b()
	KindMethodName  // a:b in function definition <-- NEW

	KindExprList
	KindNameList

	KindBreak
	KindReturn
	KindLabel
	KindGoto
	KindDo
	KindWhile
	KindRepeat
	KindIf
	KindElseIf
	KindElse
	KindForNum
	KindForIn
	KindLocalFunction
	KindFunctionStmt
)

const (
	AttrNone Attr = iota
	AttrConst
	AttrClose
)

// NodeID is an index into the Tree's Nodes slice.
type NodeID uint32

// InvalidNode is used to represent an empty or missing child.
const InvalidNode NodeID = 0

// Node is a packed, uniform structure for all AST nodes.
type Node struct {
	Start, End uint32
	Parent     NodeID
	Left       NodeID
	Right      NodeID
	Extra      uint32   // Index into Tree.ExtraList
	Count      uint16   // Number of items in ExtraList
	Kind       NodeKind // 1 byte
}

// Tree holds the source code and all AST data in flat arrays.
// Reusing this for multiple files results in zero allocs.
type Tree struct {
	Source []byte
	Root   NodeID

	Nodes     []Node
	Comments  []token.Token // Store comment boundaries continuously
	ExtraList []NodeID      // A flattened list of child nodes for N-ary structures

	// LineOffsets stores the byte offset of the start of each line
	LineOffsets []uint32
}

func NewTree(source []byte) *Tree {
	lines := make([]uint32, 1, 128)
	lines[0] = 0

	var offset int

	for {
		idx := bytes.IndexByte(source[offset:], '\n')
		if idx == -1 {
			break
		}

		offset += idx + 1

		lines = append(lines, uint32(offset))
	}

	t := &Tree{
		Source:      source,
		Nodes:       make([]Node, 1, 1024), // reserve index 0
		ExtraList:   make([]NodeID, 0, 1024),
		LineOffsets: lines,
	}

	t.Nodes[0] = Node{Kind: KindInvalid, Start: 0xFFFFFFFF, End: 0xFFFFFFFF}

	return t
}

// Position converts a byte offset to a 0-indexed Line and Column
func (t *Tree) Position(offset uint32) (line, col uint32) {
	var (
		low  int
		high = len(t.LineOffsets)
	)

	for low < high {
		mid := int(uint(low+high) >> 1)

		if t.LineOffsets[mid] <= offset {
			low = mid + 1
		} else {
			high = mid
		}
	}

	lineIdx := uint32(low - 1)

	return lineIdx, offset - t.LineOffsets[lineIdx]
}

// Offset converts a 0-indexed Line and Column into a byte offset
func (t *Tree) Offset(line, col uint32) uint32 {
	if int(line) >= len(t.LineOffsets) {
		return uint32(len(t.Source))
	}

	offset := t.LineOffsets[line] + col

	if offset > uint32(len(t.Source)) {
		return uint32(len(t.Source))
	}

	return offset
}

// NodeAt finds the narrowest AST node containing the given byte offset in O(depth) time.
func (t *Tree) NodeAt(offset uint32) NodeID {
	curr := t.Root
	if curr == InvalidNode || offset < t.Nodes[curr].Start || offset > t.Nodes[curr].End {
		return InvalidNode
	}

	for {
		node := t.Nodes[curr]

		var next NodeID = InvalidNode

		check := func(childID NodeID) {
			if childID != InvalidNode && next == InvalidNode {
				c := t.Nodes[childID]
				if offset >= c.Start && offset <= c.End {
					next = childID
				}
			}
		}

		check(node.Left)
		check(node.Right)

		for i := uint16(0); i < node.Count; i++ {
			check(t.ExtraList[node.Extra+uint32(i)])
		}

		if next != InvalidNode {
			curr = next
		} else {
			return curr
		}
	}
}

// AddNode pushes a node to the flat array and returns its ID.
func (t *Tree) AddNode(n Node) NodeID {
	id := NodeID(len(t.Nodes))

	n.Parent = InvalidNode

	t.Nodes = append(t.Nodes, n)

	if n.Left != InvalidNode {
		t.Nodes[n.Left].Parent = id
	}

	if n.Right != InvalidNode {
		t.Nodes[n.Right].Parent = id
	}

	for i := uint16(0); i < n.Count; i++ {
		child := t.ExtraList[n.Extra+uint32(i)]

		if child != InvalidNode {
			t.Nodes[child].Parent = id
		}
	}

	return id
}

func (t *Tree) Reset(source []byte) {
	t.Source = source
	t.Nodes = t.Nodes[:1] // Keep InvalidNode at index 0
	t.ExtraList = t.ExtraList[:0]
	t.Comments = t.Comments[:0]

	t.LineOffsets = t.LineOffsets[:1] // Keep 0 at index 0

	var offset int

	for {
		idx := bytes.IndexByte(source[offset:], '\n')
		if idx == -1 {
			break
		}

		offset += idx + 1

		t.LineOffsets = append(t.LineOffsets, uint32(offset))
	}
}

// HashBytes implements the FNV-1a 64-bit hash algorithm for zero-alloc map keys
func HashBytes(b []byte) uint64 {
	var hash uint64 = 14695981039346656037

	for _, c := range b {
		hash ^= uint64(c)
		hash *= 1099511628211
	}

	return hash
}
