package lexer_test

import (
	"testing"

	"github.com/coalaura/lugo/lexer"
	"github.com/coalaura/lugo/token"
)

func TestLexer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []token.Kind
	}{
		{
			name:     "Keywords and Identifiers",
			input:    "local function foo() return true end",
			expected: []token.Kind{token.Local, token.Function, token.Ident, token.LParen, token.RParen, token.Return, token.True, token.End, token.EOF},
		},
		{
			name:     "Numbers (Lua 5.4)",
			input:    "42 3.14 0xFF 0x1p-3 1e10",
			expected: []token.Kind{token.Number, token.Number, token.Number, token.Number, token.Number, token.EOF},
		},
		{
			name:     "Strings and Comments",
			input:    `"hello" 'world' [[multiline]] -- single` + "\n" + `--[[ multi ]]`,
			expected: []token.Kind{token.String, token.String, token.String, token.Comment, token.Comment, token.EOF},
		},
		{
			name:  "Operators (Bitwise & Math)",
			input: "+ - * / // % ^ # & | ~ << >> .. == ~= <= >=",
			expected: []token.Kind{
				token.Plus, token.Minus, token.Asterisk, token.Slash, token.FloorSlash,
				token.Modulo, token.Caret, token.Hash, token.BitAnd, token.BitOr,
				token.BitXor, token.ShiftLeft, token.ShiftRight, token.Concat,
				token.Eq, token.NotEq, token.LessEq, token.GreaterEq, token.EOF,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New([]byte(tt.input))

			for i, expKind := range tt.expected {
				tok := l.Next()

				if tok.Kind != expKind {
					t.Fatalf("token %d: expected %v, got %v (text: %q)", i, expKind, tok.Kind, tok.Text([]byte(tt.input)))
				}
			}
		})
	}
}
