package parser_test

import (
	"testing"

	"github.com/coalaura/lugo/ast"
	"github.com/coalaura/lugo/parser"
)

func TestParser_ValidSyntax(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "Assignments and Attributes",
			input: `
				local a = 1
				local b, c = 2, 3
				local d <const> = 4
				local e <close> = 5
				x, y = y, x
			`,
		},
		{
			name: "Functions and Methods",
			input: `
				function globalFunc(a, b, ...) return a + b end
				local function localFunc() end
				function object:method(param) self.prop = param end
				function object.sub:method() end
				local anon = function() return true end
			`,
		},
		{
			name: "Control Flow Structures",
			input: `
				if a == 1 then
					do_a()
				elseif b == 2 then
					do_b()
				else
					do_c()
				end

				while true do break end

				repeat
					local x = 1
				until x == 1

				for i = 1, 10, 2 do end
				for k, v in pairs(t) do end
			`,
		},
		{
			name: "Labels and Goto",
			input: `
				::start::
				local a = 1
				if a < 10 then
					a = a + 1
					goto start
				end
			`,
		},
		{
			name: "Table Constructors",
			input: `
				local t1 = {}
				local t2 = { 1, 2, 3 }
				local t3 = { a = 1, ["b"] = 2 }
				local t4 = {
					nested = { x = 10 },
					[1 + 2] = function() end,
				}
			`,
		},
		{
			name: "Call Variations",
			input: `
				func()
				func "string"
				func {}
				obj:method()
				obj:method "string"
				obj:method {}
			`,
		},
		{
			name: "Complex Expressions",
			input: `
				local x = -a + b * c ^ d == e and f or g
				local y = not #t == 0
				local z = (a + b) * c
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := []byte(tt.input)

			tree := ast.NewTree(src)

			p := parser.New(src, tree, 0)

			rootID := p.Parse()

			if len(p.Errors) > 0 {
				for _, err := range p.Errors {
					t.Errorf("Unexpected syntax error: %s", err.Message)
				}
			}

			if rootID == ast.InvalidNode || len(tree.Nodes) <= 1 {
				t.Fatalf("Expected AST nodes to be generated, got empty tree")
			}
		})
	}
}

func TestParser_ErrorRecovery(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedErrors int
		verifyIdent    string
	}{
		{
			name: "Missing RHS Assignment",
			input: `
				local broken =
				local recovered_a = 10
			`,
			expectedErrors: 1,
			verifyIdent:    "recovered_a",
		},
		{
			name: "Invalid Left Hand Side",
			input: `
				10 + 20 = 5
				local recovered_b = 20
			`,
			expectedErrors: 1,
			verifyIdent:    "recovered_b",
		},
		{
			name: "Missing End in Function",
			input: `
				local function broken_func()
					local x = 1

				local recovered_c = 30
			`,
			expectedErrors: 1,
			verifyIdent:    "recovered_c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := []byte(tt.input)

			tree := ast.NewTree(src)

			p := parser.New(src, tree, 0)

			p.Parse()

			if len(p.Errors) != tt.expectedErrors {
				t.Fatalf("Expected %d errors, got %d. Errors: %v", tt.expectedErrors, len(p.Errors), p.Errors)
			}

			found := false

			for _, node := range tree.Nodes {
				if node.Kind == ast.KindIdent {
					name := string(src[node.Start:node.End])
					if name == tt.verifyIdent {
						found = true

						break
					}
				}
			}

			if !found {
				t.Errorf("Parser failed to recover! Could not find the subsequent identifier %q in the AST", tt.verifyIdent)
			}
		})
	}
}

func BenchmarkParser(b *testing.B) {
	src := []byte(`
		local function fib(n)
			if n < 2 then return n end
			return fib(n-1) + fib(n-2)
		end
		local result = fib(10)
	`)

	tree := ast.NewTree(src)

	p := parser.New(src, tree, 0)

	b.ReportAllocs()

	for b.Loop() {
		tree.Reset(src)
		p.Reset(src, tree)
		p.Parse()
	}
}
