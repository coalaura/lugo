package parser_test

import (
	"testing"

	"github.com/coalaura/lugo/parser"
)

func TestParser_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "Basic Math and Local",
			input: `
				local a = 10 + 20 * 3
				local b = (a + 5) / 2
			`,
		},
		{
			name: "Function and Loops",
			input: `
				local function calculate(max)
					local sum = 0
					for i = 1, max do
						if i % 2 == 0 then
							sum = sum + i
						elseif i == 5 then
							break
						else
							sum = sum - 1
						end
					end
					return sum
				end
			`,
		},
		{
			name: "Table Constructor",
			input: `
				local config = {
					debug = true,
					["timeout"] = 5000,
					retries = 3,
					10, 20, 30
				}
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New([]byte(tt.input), nil)

			p.Parse()

			if len(p.Errors) > 0 {
				for _, err := range p.Errors {
					t.Errorf("Unexpected error: %s", err.Message)
				}
			}

			tree := p.GetTree()

			if len(tree.Nodes) == 0 {
				t.Fatalf("Expected AST nodes, got 0")
			}
		})
	}
}

// Tests that the parser doesn't completely crash or halt on bad syntax
func TestParser_ErrorRecovery(t *testing.T) {
	input := `
		local a = 
		local b = 10
	`

	p := parser.New([]byte(input), nil)

	p.Parse()

	if len(p.Errors) == 0 {
		t.Fatalf("Expected syntax error, got none")
	}

	// It should have recovered and still parsed 'local b = 10'
	tree := p.GetTree()

	if len(tree.Nodes) < 5 { // Magic number: Needs enough nodes for the File, Block, and Assignment
		t.Errorf("Expected parser to recover and build AST, but got %d nodes", len(tree.Nodes))
	}
}
