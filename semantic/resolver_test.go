package semantic_test

import (
	"testing"

	"github.com/coalaura/lugo/ast"
	"github.com/coalaura/lugo/parser"
	"github.com/coalaura/lugo/semantic"
)

// Helper to find all identifiers matching a specific string in the AST
func findIdents(tree *ast.Tree, name string) []ast.NodeID {
	var ids []ast.NodeID
	for i := 1; i < len(tree.Nodes); i++ {
		if tree.Nodes[i].Kind == ast.KindIdent {
			text := string(tree.Source[tree.Nodes[i].Start:tree.Nodes[i].End])
			if text == name {
				ids = append(ids, ast.NodeID(i))
			}
		}
	}
	return ids
}

func TestResolver_LocalScope(t *testing.T) {
	input := []byte(`
		local target = 1
		print(target)
	`)

	tree := ast.NewTree(input)
	p := parser.New(input, tree, 0)
	root := p.Parse()

	res := semantic.New(tree)
	res.Resolve(root)

	targets := findIdents(tree, "target")
	if len(targets) != 2 {
		t.Fatalf("Expected 2 'target' identifiers, found %d", len(targets))
	}

	defID := targets[0]
	refID := targets[1]

	if res.References[refID] != defID {
		t.Errorf("Reference did not resolve to correct local definition")
	}

	if res.UsageCount[defID] != 1 {
		t.Errorf("Expected usage count of 1, got %d", res.UsageCount[defID])
	}
}

func TestResolver_Shadowing(t *testing.T) {
	input := []byte(`
		local a = 1
		do
			local a = 2
			print(a)
		end
	`)

	tree := ast.NewTree(input)
	p := parser.New(input, tree, 0)
	root := p.Parse()

	res := semantic.New(tree)
	res.Resolve(root)

	aNodes := findIdents(tree, "a")
	if len(aNodes) != 3 {
		t.Fatalf("Expected 3 'a' identifiers, found %d", len(aNodes))
	}

	outerDef := aNodes[0]
	innerDef := aNodes[1]
	innerRef := aNodes[2]

	// The reference should point to the INNER definition, not the outer one
	if res.References[innerRef] != innerDef {
		t.Errorf("Shadowed variable resolved to outer scope instead of inner scope")
	}

	// Verify the resolver explicitly logged the shadow event for Diagnostics
	if len(res.ShadowedOuter) != 1 {
		t.Fatalf("Expected 1 shadow event, got %d", len(res.ShadowedOuter))
	}

	shadowEvent := res.ShadowedOuter[0]
	if shadowEvent.Shadowing != innerDef || shadowEvent.Shadowed != outerDef {
		t.Errorf("Shadow event recorded incorrect nodes")
	}
}

func TestResolver_Globals(t *testing.T) {
	input := []byte(`
		MyGlobal = 10
		print(MyGlobal)
	`)

	tree := ast.NewTree(input)
	p := parser.New(input, tree, 0)
	root := p.Parse()

	res := semantic.New(tree)
	res.Resolve(root)

	if len(res.GlobalDefs) != 1 {
		t.Errorf("Expected 1 global definition, got %d", len(res.GlobalDefs))
	}

	if len(res.GlobalRefs) != 1 { // 'print' is also a global ref, but let's check total including MyGlobal
		// Actually print + MyGlobal = 2 global refs
		if len(res.GlobalRefs) != 2 {
			t.Errorf("Expected 2 global references (print, MyGlobal), got %d", len(res.GlobalRefs))
		}
	}
}
