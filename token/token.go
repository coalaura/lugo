package token

// Kind represents the type of a token.
type Kind uint8

const (
	Illegal Kind = iota
	EOF
	Comment

	// Identifiers and basic literals
	Ident
	Number
	String

	// Keywords (Lua 5.4)
	And
	Break
	Do
	Else
	ElseIf
	End
	False
	For
	Function
	Goto
	If
	In
	Local
	Nil
	Not
	Or
	Repeat
	Return
	Then
	True
	Until
	While

	// Operators
	Plus
	Minus
	Asterisk
	Slash
	FloorSlash // + - * / //
	Modulo
	Caret
	Hash // % ^ #
	BitAnd
	BitOr
	BitXor // & | ~
	ShiftLeft
	ShiftRight // << >>
	Concat     // ..

	// Relational & Assignment
	Eq
	NotEq
	Less
	LessEq
	Greater
	GreaterEq // == ~= < <= > >=
	Assign    // =

	// Delimiters
	LParen
	RParen // ( )
	LBrace
	RBrace // { }
	LBrack
	RBrack      // [ ]
	DoubleColon // ::
	Semicolon
	Colon // ; :
	Comma
	Dot    // , .
	Vararg // ...
)

// Token represents a lexical token.
type Token struct {
	Kind       Kind
	Start, End uint32 // Byte offsets in the source []byte
}

// TokenSet uses bitmasks for zero-allocation, O(1) token lookups.
// Supports up to 128 token kinds (currently Lua only needs ~60).
type TokenSet [2]uint64

func NewTokenSet(kinds ...Kind) TokenSet {
	var s TokenSet

	for _, k := range kinds {
		s[k/64] |= 1 << (k % 64)
	}

	return s
}

func (s TokenSet) Contains(k Kind) bool {
	return (s[k/64] & (1 << (k % 64))) != 0
}

// Text extracts the token's string value from the source without allocating,
// unless explicitly cast to a string by the caller.
func (t Token) Text(source []byte) []byte {
	return source[t.Start:t.End]
}
