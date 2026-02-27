package lexer

import (
	"github.com/coalaura/lugo/token"
)

type Lexer struct {
	source []byte
	cursor uint32
	read   uint32
	ch     byte
}

func New(source []byte) *Lexer {
	l := &Lexer{
		source: source,
	}

	l.advance()

	return l
}

func (l *Lexer) advance() {
	if l.read >= uint32(len(l.source)) {
		l.ch = 0

		l.cursor = l.read

		return
	}

	l.ch = l.source[l.read]

	l.cursor = l.read

	l.read++
}

func (l *Lexer) peek() byte {
	if l.read >= uint32(len(l.source)) {
		return 0
	}

	return l.source[l.read]
}

func (l *Lexer) Next() token.Token {
	l.skipWhitespace()

	start := l.cursor

	if l.ch == 0 {
		return token.Token{Kind: token.EOF, Start: start, End: start}
	}

	ch := l.ch

	l.advance()

	switch ch {
	case '+':
		return token.Token{Kind: token.Plus, Start: start, End: l.cursor}
	case '*':
		return token.Token{Kind: token.Asterisk, Start: start, End: l.cursor}
	case '%':
		return token.Token{Kind: token.Modulo, Start: start, End: l.cursor}
	case '^':
		return token.Token{Kind: token.Caret, Start: start, End: l.cursor}
	case '&':
		return token.Token{Kind: token.BitAnd, Start: start, End: l.cursor}
	case '|':
		return token.Token{Kind: token.BitOr, Start: start, End: l.cursor}
	case '#':
		return token.Token{Kind: token.Hash, Start: start, End: l.cursor}
	case '(':
		return token.Token{Kind: token.LParen, Start: start, End: l.cursor}
	case ')':
		return token.Token{Kind: token.RParen, Start: start, End: l.cursor}
	case '{':
		return token.Token{Kind: token.LBrace, Start: start, End: l.cursor}
	case '}':
		return token.Token{Kind: token.RBrace, Start: start, End: l.cursor}
	case ']':
		return token.Token{Kind: token.RBrack, Start: start, End: l.cursor}
	case ';':
		return token.Token{Kind: token.Semicolon, Start: start, End: l.cursor}
	case ',':
		return token.Token{Kind: token.Comma, Start: start, End: l.cursor}
	case '-':
		if l.ch == '-' {
			return l.readComment(start)
		}

		return token.Token{Kind: token.Minus, Start: start, End: l.cursor}
	case '/':
		if l.ch == '/' {
			l.advance()

			return token.Token{Kind: token.FloorSlash, Start: start, End: l.cursor}
		}

		return token.Token{Kind: token.Slash, Start: start, End: l.cursor}
	case '=':
		if l.ch == '=' {
			l.advance()

			return token.Token{Kind: token.Eq, Start: start, End: l.cursor}
		}

		return token.Token{Kind: token.Assign, Start: start, End: l.cursor}
	case '~':
		if l.ch == '=' {
			l.advance()

			return token.Token{Kind: token.NotEq, Start: start, End: l.cursor}
		}

		return token.Token{Kind: token.BitXor, Start: start, End: l.cursor}
	case '<':
		if l.ch == '=' {
			l.advance()

			return token.Token{Kind: token.LessEq, Start: start, End: l.cursor}
		} else if l.ch == '<' {
			l.advance()

			return token.Token{Kind: token.ShiftLeft, Start: start, End: l.cursor}
		}

		return token.Token{Kind: token.Less, Start: start, End: l.cursor}
	case '>':
		if l.ch == '=' {
			l.advance()

			return token.Token{Kind: token.GreaterEq, Start: start, End: l.cursor}
		} else if l.ch == '>' {
			l.advance()

			return token.Token{Kind: token.ShiftRight, Start: start, End: l.cursor}
		}

		return token.Token{Kind: token.Greater, Start: start, End: l.cursor}
	case ':':
		if l.ch == ':' {
			l.advance()

			return token.Token{Kind: token.DoubleColon, Start: start, End: l.cursor}
		}

		return token.Token{Kind: token.Colon, Start: start, End: l.cursor}
	case '.':
		if l.ch == '.' {
			l.advance()

			if l.ch == '.' {
				l.advance()

				return token.Token{Kind: token.Vararg, Start: start, End: l.cursor}
			}

			return token.Token{Kind: token.Concat, Start: start, End: l.cursor}
		}

		if l.ch >= '0' && l.ch <= '9' {
			return l.readNumber(start)
		}

		return token.Token{Kind: token.Dot, Start: start, End: l.cursor}
	case '\'', '"':
		return l.readString(start, ch)
	case '[':
		if l.ch == '[' || l.ch == '=' {
			return l.readLongString(start)
		}

		return token.Token{Kind: token.LBrack, Start: start, End: l.cursor}
	}

	if isLetter(ch) {
		return l.readIdent(start)
	}

	if isDigit(ch) {
		return l.readNumber(start)
	}

	return token.Token{Kind: token.Illegal, Start: start, End: l.cursor}
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.advance()
	}
}

func (l *Lexer) readIdent(start uint32) token.Token {
	for isLetter(l.ch) || isDigit(l.ch) {
		l.advance()
	}

	// The compiler explicitly optimizes switch string(bytes) to do zero heap allocations.
	// This is significantly faster than a map hash lookup.
	kind := token.Ident
	switch string(l.source[start:l.cursor]) {
	case "and":
		kind = token.And
	case "break":
		kind = token.Break
	case "do":
		kind = token.Do
	case "else":
		kind = token.Else
	case "elseif":
		kind = token.ElseIf
	case "end":
		kind = token.End
	case "false":
		kind = token.False
	case "for":
		kind = token.For
	case "function":
		kind = token.Function
	case "goto":
		kind = token.Goto
	case "if":
		kind = token.If
	case "in":
		kind = token.In
	case "local":
		kind = token.Local
	case "nil":
		kind = token.Nil
	case "not":
		kind = token.Not
	case "or":
		kind = token.Or
	case "repeat":
		kind = token.Repeat
	case "return":
		kind = token.Return
	case "then":
		kind = token.Then
	case "true":
		kind = token.True
	case "until":
		kind = token.Until
	case "while":
		kind = token.While
	}

	return token.Token{Kind: kind, Start: start, End: l.cursor}
}

func (l *Lexer) readNumber(start uint32) token.Token {
	isHex := false

	if l.source[start] == '0' && (l.ch == 'x' || l.ch == 'X') {
		isHex = true

		l.advance() // consume 'x'
	}

	for l.ch != 0 {
		if isHex && (isDigit(l.ch) || isHexLetter(l.ch) || l.ch == '.') {
			l.advance()
		} else if !isHex && (isDigit(l.ch) || l.ch == '.') {
			l.advance()
		} else if isHex && (l.ch == 'p' || l.ch == 'P') {
			l.advance()

			if l.ch == '+' || l.ch == '-' {
				l.advance()
			}
		} else if !isHex && (l.ch == 'e' || l.ch == 'E') {
			l.advance()

			if l.ch == '+' || l.ch == '-' {
				l.advance()
			}
		} else {
			break
		}
	}

	return token.Token{Kind: token.Number, Start: start, End: l.cursor}
}

func (l *Lexer) readString(start uint32, quote byte) token.Token {
	for l.ch != 0 {
		if l.ch == '\\' {
			l.advance() // Skip escape char

			if l.ch != 0 {
				l.advance() // Skip the actual escaped character
			}
		} else if l.ch == quote {
			l.advance()

			break
		} else {
			l.advance()
		}
	}

	return token.Token{Kind: token.String, Start: start, End: l.cursor}
}

func (l *Lexer) readLongString(start uint32) token.Token {
	eqCount := 0

	for l.ch == '=' {
		eqCount++

		l.advance()
	}

	if l.ch != '[' {
		return token.Token{Kind: token.Illegal, Start: start, End: l.cursor}
	}

	l.advance()

	for l.ch != 0 {
		if l.ch == ']' {
			l.advance()

			var closingEq int

			for l.ch == '=' {
				closingEq++

				l.advance()
			}

			if l.ch == ']' && closingEq == eqCount {
				l.advance()

				break
			}
		} else {
			l.advance()
		}
	}

	return token.Token{Kind: token.String, Start: start, End: l.cursor}
}

func (l *Lexer) readComment(start uint32) token.Token {
	l.advance()

	if l.ch == '[' {
		peekChar := l.peek()

		if peekChar == '[' || peekChar == '=' {
			l.advance()

			tok := l.readLongString(l.cursor - 1)

			tok.Kind = token.Comment
			tok.Start = start

			return tok
		}
	}

	for l.ch != '\n' && l.ch != 0 {
		l.advance()
	}

	return token.Token{Kind: token.Comment, Start: start, End: l.cursor}
}

func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}
