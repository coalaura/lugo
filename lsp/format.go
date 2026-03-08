package lsp

import (
	"bytes"

	"github.com/coalaura/lugo/lexer"
	"github.com/coalaura/lugo/token"
)

type Formatter struct {
	IndentSize int
	UseTabs    bool
}

func NewFormatter(indentSize int, useTabs bool) *Formatter {
	return &Formatter{
		IndentSize: indentSize,
		UseTabs:    useTabs,
	}
}

const (
	ScopeBlock = iota
	ScopeParen
	ScopeBrace
	ScopeBrack
)

type Scope struct {
	Kind        int
	BaseIndent  int
	InnerIndent int
	IsComplex   bool
	LineIdx     int
}

// Format iterates over the source's token stream and elegantly fixes whitespace.
// It keeps existing vertical line breaks (up to 2), correctly processes minified statements,
// and strictly enforces Lua indentation rules.
func (f *Formatter) Format(source []byte) []byte {
	lex := lexer.New(source)

	tokens := make([]token.Token, 0, len(source)/4)

	for {
		tok := lex.Next()
		if tok.Kind == token.EOF {
			break
		}

		tokens = append(tokens, tok)
	}

	var out bytes.Buffer

	out.Grow(len(source) + len(source)/10)

	var (
		stack       []Scope
		isLineStart = true
		prevTok     token.Token
		prevPrev    token.Kind
		lineIdx     int
	)

	for i, tok := range tokens {
		var (
			gap []byte
			nl  int
		)

		if prevTok.Kind != 0 {
			gap = source[prevTok.End:tok.Start]
			nl = bytes.Count(gap, []byte{'\n'})
		}

		var forceNl bool

		if nl == 0 && prevTok.Kind != 0 {
			forceNl = f.needsNewline(prevTok.Kind, tok.Kind, stack)
		}

		if nl > 0 || forceNl {
			if nl > 2 {
				nl = 2
			}

			if forceNl && nl == 0 {
				nl = 1
			}

			out.Write(bytes.Repeat([]byte{'\n'}, nl))

			isLineStart = true
			lineIdx += nl
		}

		if isLineStart {
			lineIndent := f.calculateLineIndent(stack, tokens, i, source)
			if lineIndent > 0 {
				if f.UseTabs {
					out.Write(bytes.Repeat([]byte{'\t'}, lineIndent))
				} else {
					out.Write(bytes.Repeat([]byte{' '}, lineIndent*f.IndentSize))
				}
			}

			isLineStart = false
		} else if prevTok.Kind != 0 {
			if f.needsSpace(prevTok.Kind, tok.Kind, prevPrev) {
				out.WriteByte(' ')
			} else if (prevTok.Kind == token.LBrace || tok.Kind == token.RBrace) && bytes.ContainsRune(gap, ' ') {
				out.WriteByte(' ')
			}
		}

		out.Write(source[tok.Start:tok.End])

		popScope := func(kind int) {
			for j := len(stack) - 1; j >= 0; j-- {
				if stack[j].Kind == kind {
					stack = stack[:j]

					return
				}
			}
		}

		switch tok.Kind {
		case token.End, token.Until, token.ElseIf, token.Else:
			popScope(ScopeBlock)
		case token.RBrace:
			popScope(ScopeBrace)
		case token.RParen:
			popScope(ScopeParen)
		case token.RBrack:
			popScope(ScopeBrack)
		}

		pushScope := func(kind int, isComplex bool) {
			var base int

			if len(stack) > 0 {
				base = stack[len(stack)-1].InnerIndent
			}

			if len(stack) > 0 && stack[len(stack)-1].LineIdx == lineIdx {
				base = stack[len(stack)-1].BaseIndent
			}

			stack = append(stack, Scope{
				Kind:        kind,
				BaseIndent:  base,
				InnerIndent: base + 1,
				IsComplex:   isComplex,
				LineIdx:     lineIdx,
			})
		}

		switch tok.Kind {
		case token.Do, token.Then, token.Repeat, token.Function, token.Else:
			pushScope(ScopeBlock, false)
		case token.LBrace:
			pushScope(ScopeBrace, f.isComplexTable(tokens, i))
		case token.LParen:
			pushScope(ScopeParen, false)
		case token.LBrack:
			pushScope(ScopeBrack, false)
		}

		prevPrev = prevTok.Kind
		prevTok = tok
	}

	res := out.Bytes()
	res = bytes.TrimRight(res, " \t\r\n")

	if len(res) > 0 {
		res = append(res, '\n')
	}

	return res
}

func (f *Formatter) calculateLineIndent(stack []Scope, tokens []token.Token, startIndex int, source []byte) int {
	tempStack := make([]Scope, len(stack))
	copy(tempStack, stack)

	lineIndent := -1

	for i := startIndex; i < len(tokens); i++ {
		tok := tokens[i]

		if i > startIndex {
			prev := tokens[i-1]
			gap := source[prev.End:tok.Start]

			if bytes.ContainsRune(gap, '\n') || f.needsNewline(prev.Kind, tok.Kind, tempStack) {
				break
			}
		}

		if tok.Kind == token.Comment {
			continue
		}

		poppedBase := -1

		pop := func(kind int) bool {
			for j := len(tempStack) - 1; j >= 0; j-- {
				if tempStack[j].Kind == kind {
					poppedBase = tempStack[j].BaseIndent
					tempStack = tempStack[:j]

					return true
				}
			}

			return false
		}

		var isCloser bool

		switch tok.Kind {
		case token.End, token.Until, token.ElseIf, token.Else:
			isCloser = pop(ScopeBlock)
		case token.RBrace:
			isCloser = pop(ScopeBrace)
		case token.RParen:
			isCloser = pop(ScopeParen)
		case token.RBrack:
			isCloser = pop(ScopeBrack)
		}

		if isCloser {
			if poppedBase != -1 {
				lineIndent = poppedBase
			}
		} else {
			break
		}
	}

	if lineIndent != -1 {
		return lineIndent
	}

	if len(tempStack) > 0 {
		return tempStack[len(tempStack)-1].InnerIndent
	}

	return 0
}

func (f *Formatter) isComplexTable(tokens []token.Token, startIndex int) bool {
	depth := 1

	for i := startIndex + 1; i < len(tokens); i++ {
		tok := tokens[i]

		switch tok.Kind {
		case token.LBrace:
			depth++
		case token.RBrace:
			depth--

			if depth == 0 {
				return false
			}
		case token.Function, token.If, token.For, token.While:
			if depth == 1 {
				return true
			}
		}
	}

	return false
}

func (f *Formatter) isWord(k token.Kind) bool {
	if k >= token.And && k <= token.While {
		return true
	}

	if k == token.Ident || k == token.Number || k == token.String {
		return true
	}

	return false
}

func (f *Formatter) isKeyword(k token.Kind) bool {
	return k >= token.And && k <= token.While
}

func (f *Formatter) isOperator(k token.Kind) bool {
	return k >= token.Plus && k <= token.Assign
}

func (f *Formatter) isExprEnd(k token.Kind) bool {
	switch k {
	case token.Ident, token.Number, token.String, token.RParen, token.RBrack, token.RBrace, token.True, token.False, token.Nil, token.Vararg, token.End:
		return true
	}

	return false
}

func (f *Formatter) needsNewline(left, right token.Kind, stack []Scope) bool {
	if left == token.Illegal || left == token.EOF || right == token.Illegal || right == token.EOF {
		return false
	}

	if len(stack) > 0 {
		top := stack[len(stack)-1]

		if top.Kind == ScopeBrace && top.IsComplex {
			if left == token.LBrace || left == token.Comma {
				return true
			}

			if right == token.RBrace {
				return true
			}
		}
	}

	switch right {
	case token.Local, token.If, token.While, token.For, token.Repeat, token.Break, token.Return, token.Goto, token.DoubleColon:
		return true
	case token.End, token.ElseIf, token.Else, token.Until:
		return true
	}

	switch left {
	case token.Do, token.Then, token.Else, token.Repeat:
		return true
	case token.Semicolon:
		if len(stack) == 0 || stack[len(stack)-1].Kind != ScopeBrace {
			return true
		}
	}

	if f.isExprEnd(left) && (right == token.Ident || right == token.Function) {
		return true
	}

	return false
}

func (f *Formatter) needsSpace(left, right token.Kind, leftOfLeft token.Kind) bool {
	if left == token.Illegal || left == token.EOF || right == token.Illegal || right == token.EOF {
		return false
	}

	if left == token.Comment || right == token.Comment {
		return true
	}

	if right == token.Comma || right == token.Semicolon || right == token.Colon {
		return false
	}

	if left == token.Dot || right == token.Dot || left == token.DoubleColon || right == token.DoubleColon || left == token.Colon {
		return false
	}

	if left == token.LParen || right == token.RParen || left == token.LBrack || right == token.RBrack {
		return false
	}

	if right == token.LParen || right == token.LBrace || right == token.String {
		if left == token.Ident || left == token.RParen || left == token.RBrack || left == token.RBrace || left == token.String || left == token.End {
			return false // print(), print{}, print""
		}
	}

	if left == token.LBrace || right == token.RBrace {
		return false
	}

	if f.isWord(left) && f.isWord(right) {
		return true
	}

	if f.isKeyword(left) {
		if left == token.Function && (right == token.LParen || right == token.Dot || right == token.Colon) {
			return false
		}

		return true
	}

	if f.isKeyword(right) {
		return true
	}

	if left == token.Comma || left == token.Semicolon {
		return true
	}

	if left == token.Hash {
		return false
	}

	if left == token.Minus || left == token.BitXor {
		if !f.isExprEnd(leftOfLeft) {
			return false
		}
	}

	if f.isOperator(left) || f.isOperator(right) {
		return true
	}

	return false
}
