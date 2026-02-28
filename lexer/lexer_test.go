package lexer_test

import (
	"testing"

	"github.com/coalaura/lugo/lexer"
	"github.com/coalaura/lugo/token"
)

type lexTest struct {
	kind token.Kind
	text string
}

func TestLexer_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []lexTest
	}{
		{
			name:  "Keywords and Identifiers",
			input: "local function foo_bar() return true end",
			expected: []lexTest{
				{token.Local, "local"}, {token.Function, "function"},
				{token.Ident, "foo_bar"}, {token.LParen, "("}, {token.RParen, ")"},
				{token.Return, "return"}, {token.True, "true"}, {token.End, "end"},
				{token.EOF, ""},
			},
		},
		{
			name:  "Numbers (Lua 5.4 Standard & Hex Floats)",
			input: "42 3.14 .5 5. 0xFF 0x1p-3 1e10",
			expected: []lexTest{
				{token.Number, "42"}, {token.Number, "3.14"}, {token.Number, ".5"},
				{token.Number, "5."}, {token.Number, "0xFF"}, {token.Number, "0x1p-3"},
				{token.Number, "1e10"}, {token.EOF, ""},
			},
		},
		{
			name:  "Strings and Escapes",
			input: `"hello" 'world' "esc\"ape" '\x41'`,
			expected: []lexTest{
				{token.String, `"hello"`}, {token.String, `'world'`},
				{token.String, `"esc\"ape"`}, {token.String, `'\x41'`},
				{token.EOF, ""},
			},
		},
		{
			name:  "Long Strings",
			input: `[[basic]] [=[level 1]=] [==[level 2]==]`,
			expected: []lexTest{
				{token.String, `[[basic]]`}, {token.String, `[=[level 1]=]`},
				{token.String, `[==[level 2]==]`}, {token.EOF, ""},
			},
		},
		{
			name:  "Comments",
			input: "-- single\n--[[ multi ]]\n--[=[ multi 2 ]=]\nlocal",
			expected: []lexTest{
				{token.Comment, "-- single"}, {token.Comment, "--[[ multi ]]"},
				{token.Comment, "--[=[ multi 2 ]=]"}, {token.Local, "local"},
				{token.EOF, ""},
			},
		},
		{
			name:  "Operators (Math & Bitwise)",
			input: "+ - * / // % ^ # & | ~ << >> .. == ~= <= >=",
			expected: []lexTest{
				{token.Plus, "+"}, {token.Minus, "-"}, {token.Asterisk, "*"},
				{token.Slash, "/"}, {token.FloorSlash, "//"}, {token.Modulo, "%"},
				{token.Caret, "^"}, {token.Hash, "#"}, {token.BitAnd, "&"},
				{token.BitOr, "|"}, {token.BitXor, "~"}, {token.ShiftLeft, "<<"},
				{token.ShiftRight, ">>"}, {token.Concat, ".."}, {token.Eq, "=="},
				{token.NotEq, "~="}, {token.LessEq, "<="}, {token.GreaterEq, ">="},
				{token.EOF, ""},
			},
		},
		{
			name:  "Varargs and Dots",
			input: ". .. ...",
			expected: []lexTest{
				{token.Dot, "."}, {token.Concat, ".."}, {token.Vararg, "..."},
				{token.EOF, ""},
			},
		},
		{
			name:  "Labels and Colons",
			input: ": ::",
			expected: []lexTest{
				{token.Colon, ":"}, {token.DoubleColon, "::"},
				{token.EOF, ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := []byte(tt.input)
			l := lexer.New(src)

			for i, exp := range tt.expected {
				tok := l.Next()
				text := string(src[tok.Start:tok.End])

				if tok.Kind != exp.kind {
					t.Fatalf("Token %d: expected kind %v, got %v (text: %q)", i, exp.kind, tok.Kind, text)
				}

				if text != exp.text {
					t.Fatalf("Token %d: expected text %q, got %q", i, exp.text, text)
				}
			}
		})
	}
}
