package lsp

import (
	"bytes"

	"github.com/coalaura/lugo/lexer"
	"github.com/coalaura/lugo/token"
)

type Formatter struct {
	IndentSize  int
	UseTabs     bool
	Opinionated bool
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

type StmtKind int

const (
	StmtUnknown StmtKind = iota
	StmtLocal
	StmtAssign
	StmtCall
	StmtControl
	StmtFunction
	StmtReturn
)

func NewFormatter(indentSize int, useTabs bool, opinionated bool) *Formatter {
	return &Formatter{
		IndentSize:  indentSize,
		UseTabs:     useTabs,
		Opinionated: opinionated,
	}
}

// Format iterates over the source's token stream and elegantly fixes whitespace.
// It enforces blank lines between unrelated statements, strips trailing semicolons safely,
// expands minified code, and strictly enforces Lua indentation rules.
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
		stack             []Scope
		isLineStart       = true
		prevTok           token.Token
		prevNonCommentTok token.Token
		prevNonCommentIdx int = -1
		prevPrev          token.Kind
		lineIdx           int
		lastStmtKind      StmtKind
		currentLineIndent int
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
			forceNl = f.needsNewline(prevNonCommentTok.Kind, tok.Kind, stack)
		}

		var skipSemicolon bool

		if f.Opinionated && tok.Kind == token.Semicolon {
			var nextNonComment *token.Token

			for j := i + 1; j < len(tokens); j++ {
				if tokens[j].Kind != token.Comment {
					nextNonComment = &tokens[j]

					break
				}
			}

			if nextNonComment == nil {
				skipSemicolon = true
			} else {
				gapAfter := source[tok.End:nextNonComment.Start]

				if bytes.ContainsRune(gapAfter, '\n') || f.needsNewline(tok.Kind, nextNonComment.Kind, stack) {
					if nextNonComment.Kind != token.LParen && nextNonComment.Kind != token.LBrack {
						skipSemicolon = true
					}
				}
			}
		}

		if skipSemicolon {
			continue
		}

		isStmtLevel := len(stack) == 0 || stack[len(stack)-1].Kind == ScopeBlock

		if f.Opinionated && isStmtLevel && tok.Kind != token.Comment {
			if (nl > 0 || forceNl || i == 0) && f.isStatementStart(prevNonCommentTok.Kind) {
				currStmtKind := f.getStmtKind(tokens, i)

				if currStmtKind != StmtUnknown {
					if f.wantsBlankLine(lastStmtKind, currStmtKind) {
						isJustAfterBlockOpener := prevNonCommentTok.Kind == token.Do || prevNonCommentTok.Kind == token.Then || prevNonCommentTok.Kind == token.Repeat || prevNonCommentTok.Kind == token.Else || prevNonCommentTok.Kind == token.ElseIf

						if prevNonCommentTok.Kind == token.RParen && prevNonCommentIdx != -1 {
							if f.isFunctionSignatureEnd(tokens, prevNonCommentIdx) {
								isJustAfterBlockOpener = true
							}
						}

						isJustBeforeBlockCloser := tok.Kind == token.End || tok.Kind == token.Until || tok.Kind == token.ElseIf || tok.Kind == token.Else

						if !isJustAfterBlockOpener && !isJustBeforeBlockCloser {
							if nl < 2 {
								nl = 2
								forceNl = true
							}
						}
					}

					lastStmtKind = currStmtKind
				}
			}
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
			currentLineIndent = f.calculateLineIndent(stack, tokens, i, source)
			if currentLineIndent > 0 {
				if f.UseTabs {
					out.Write(bytes.Repeat([]byte{'\t'}, currentLineIndent))
				} else {
					out.Write(bytes.Repeat([]byte{' '}, currentLineIndent*f.IndentSize))
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
			stack = append(stack, Scope{
				Kind:        kind,
				BaseIndent:  currentLineIndent,
				InnerIndent: currentLineIndent + 1,
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

		if tok.Kind != token.Comment {
			prevPrev = prevNonCommentTok.Kind
			prevNonCommentTok = tok
			prevNonCommentIdx = i
		}

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

func (f *Formatter) getStmtKind(tokens []token.Token, startIndex int) StmtKind {
	tok := tokens[startIndex]

	switch tok.Kind {
	case token.Local:
		for j := startIndex + 1; j < len(tokens); j++ {
			if tokens[j].Kind == token.Comment {
				continue
			}

			if tokens[j].Kind == token.Function {
				return StmtFunction
			}

			break
		}
		return StmtLocal
	case token.If, token.For, token.While, token.Repeat, token.Break, token.Goto, token.DoubleColon, token.Do:
		return StmtControl
	case token.Function:
		return StmtFunction
	case token.Return:
		return StmtReturn
	case token.Ident, token.LParen:
		depth := 0
	Loop:
		for j := startIndex; j < len(tokens); j++ {
			t := tokens[j].Kind

			if t == token.LParen || t == token.LBrace || t == token.LBrack {
				depth++
			} else if t == token.RParen || t == token.RBrace || t == token.RBrack {
				depth--
			} else if depth == 0 {
				if t == token.Assign || t == token.Comma {
					return StmtAssign
				}

				switch t {
				case token.Do, token.Then, token.Else, token.ElseIf, token.End, token.Until, token.Semicolon, token.If, token.For, token.While, token.Repeat, token.Break, token.Goto, token.DoubleColon, token.Local, token.Return, token.Function:
					break Loop
				}

				if j > startIndex {
					prev := tokens[j-1].Kind

					if f.isExprEnd(prev) && t == token.Ident {
						break Loop
					}
				}
			}
		}

		return StmtCall
	}

	return StmtUnknown
}

func (f *Formatter) wantsBlankLine(a, b StmtKind) bool {
	if a == StmtUnknown || b == StmtUnknown {
		return false
	}

	// Group locals and assignments together
	if (a == StmtLocal || a == StmtAssign) && (b == StmtLocal || b == StmtAssign) {
		return false
	}

	// Group consecutive function calls together
	if a == StmtCall && b == StmtCall {
		return false
	}

	// Enforce blank lines between completely unrelated statement blocks
	return true
}

func (f *Formatter) isStatementStart(prev token.Kind) bool {
	if prev == 0 || prev == token.Semicolon {
		return true
	}

	switch prev {
	case token.Do, token.Then, token.Else, token.Repeat:
		return true
	}

	return f.isExprEnd(prev)
}

// isFunctionSignatureEnd identifies if an RParen is closing a function parameters list
func (f *Formatter) isFunctionSignatureEnd(tokens []token.Token, rParenIdx int) bool {
	var depth int

	for i := rParenIdx; i >= 0; i-- {
		k := tokens[i].Kind

		if k == token.RParen {
			depth++
		} else if k == token.LParen {
			depth--
			if depth == 0 {
				for j := i - 1; j >= 0; j-- {
					k2 := tokens[j].Kind
					if k2 == token.Comment {
						continue
					}

					if k2 == token.Function {
						return true
					}

					if k2 == token.Ident || k2 == token.Dot || k2 == token.Colon {
						continue
					}

					return false
				}
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
