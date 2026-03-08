package ast_test

import (
	"testing"

	"github.com/coalaura/lugo/ast"
	"github.com/coalaura/lugo/parser"
)

func TestTree_PositionAndOffset(t *testing.T) {
	// 0123 4567 89
	// abc\n def\n g
	input := []byte("abc\ndef\ng")
	tree := ast.NewTree(input)

	tests := []struct {
		offset      uint32
		expectedLn  uint32
		expectedCol uint32
	}{
		{0, 0, 0}, // 'a'
		{2, 0, 2}, // 'c'
		{3, 0, 3}, // '\n'
		{4, 1, 0}, // 'd'
		{7, 1, 3}, // '\n'
		{8, 2, 0}, // 'g'
	}

	for _, tt := range tests {
		line, col := tree.Position(tt.offset)
		if line != tt.expectedLn || col != tt.expectedCol {
			t.Errorf("Position(%d): expected %d:%d, got %d:%d", tt.offset, tt.expectedLn, tt.expectedCol, line, col)
		}

		offset := tree.Offset(tt.expectedLn, tt.expectedCol)
		if offset != tt.offset {
			t.Errorf("Offset(%d, %d): expected %d, got %d", tt.expectedLn, tt.expectedCol, tt.offset, offset)
		}
	}

	// Out of bounds check
	outLine, _ := tree.Position(999)
	if outLine != 2 {
		t.Errorf("Out of bounds Position should clamp to last line, got %d", outLine)
	}

	outOffset := tree.Offset(99, 99)
	if outOffset != uint32(len(input)) {
		t.Errorf("Out of bounds Offset should clamp to EOF (%d), got %d", len(input), outOffset)
	}
}

func TestTree_NodeAt(t *testing.T) {
	input := []byte("local a = 1\nprint(a)")
	tree := ast.NewTree(input)
	p := parser.New(input, tree, 0)
	p.Parse()

	// 'a' is at offset 6
	nodeID := tree.NodeAt(6)
	if nodeID == ast.InvalidNode {
		t.Fatalf("Expected valid node at offset 6, got InvalidNode")
	}

	node := tree.Nodes[nodeID]
	if node.Kind != ast.KindIdent || string(tree.Source[node.Start:node.End]) != "a" {
		t.Errorf("Expected identifier 'a', got %v", node.Kind)
	}
}

func BenchmarkNodeAt(b *testing.B) {
	src := []byte(`
		local function fib(n)
			if n < 2 then return n end
			return fib(n-1) + fib(n-2)
		end
	`)

	tree := ast.NewTree(src)

	p := parser.New(src, tree, 0)
	p.Parse()

	b.ReportAllocs()

	for b.Loop() {
		// Search for 'n' in 'n < 2' (offset ~35)
		tree.NodeAt(35)
	}
}
