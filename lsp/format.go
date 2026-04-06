package lsp

import (
	"bytes"
	"encoding/json"
	"time"

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

func (s *Server) formatDocument(uri string, options FormattingOptions, formatRange *Range) []TextEdit {
	doc, ok := s.Documents[uri]
	if !ok {
		return nil
	}

	start := time.Now()

	formatter := NewFormatter(options.TabSize, !options.InsertSpaces, s.FormatOpinionated)

	edits := formatter.Format(doc, formatRange)

	s.Log.Printf("Formatted document in %s (%d edits generated)\n", time.Since(start), len(edits))

	return edits
}

func (s *Server) handleFormatting(req Request) {
	if !s.FeatureFormatting {
		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

		return
	}

	var params DocumentFormattingParams

	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		return
	}

	changes := s.formatDocument(s.normalizeURI(params.TextDocument.URI), params.Options, nil)

	WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: changes})
}

func (s *Server) handleRangeFormatting(req Request) {
	if !s.FeatureFormatting {
		WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: nil})

		return
	}

	var params DocumentRangeFormattingParams

	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		return
	}

	changes := s.formatDocument(s.normalizeURI(params.TextDocument.URI), params.Options, &params.Range)

	WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: changes})
}

// Format iterates over the source's token stream and elegantly fixes whitespace.
// It enforces blank lines between unrelated statements, strips trailing semicolons safely,
// expands minified code, and strictly enforces Lua indentation rules.
func (f *Formatter) Format(doc *Document, formatRange *Range) []TextEdit {
	source := doc.Source
	lex := lexer.New(source)

	tokens := make([]token.Token, 0, len(source)/4)

	for {
		tok := lex.Next()
		if tok.Kind == token.EOF {
			break
		}

		tokens = append(tokens, tok)
	}

	var (
		rangeStart uint32
		rangeEnd   uint32
	)

	if formatRange != nil {
		rangeStart = doc.Tree.Offset(formatRange.Start.Line, formatRange.Start.Character)
		rangeEnd = doc.Tree.Offset(formatRange.End.Line, formatRange.End.Character)
	}

	var (
		edits      []TextEdit
		gapBuilder bytes.Buffer
	)

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

		if f.Opinionated && isStmtLevel {
			if nl > 0 || forceNl || i == 0 {
				var (
					isFirstOfGroup bool
					targetStmtIdx  = -1
				)

				if tok.Kind != token.Comment {
					if prevTok.Kind != token.Comment || bytes.Count(gap, []byte{'\n'}) > 1 {
						isFirstOfGroup = true
						targetStmtIdx = i
					}
				} else {
					if prevTok.Kind != token.Comment || bytes.Count(gap, []byte{'\n'}) > 1 {
						contiguous := true

						for j := i + 1; j < len(tokens); j++ {
							gapAfter := source[tokens[j-1].End:tokens[j].Start]
							if bytes.Count(gapAfter, []byte{'\n'}) > 1 {
								contiguous = false

								break
							}

							if tokens[j].Kind != token.Comment {
								targetStmtIdx = j

								break
							}
						}

						if contiguous && targetStmtIdx != -1 {
							isFirstOfGroup = true
						}
					}
				}

				if isFirstOfGroup && targetStmtIdx != -1 {
					var prevStmtEnd token.Kind

					if prevNonCommentIdx != -1 {
						prevStmtEnd = tokens[prevNonCommentIdx].Kind
					}

					if f.isStatementStart(prevStmtEnd) {
						currStmtKind := f.getStmtKind(tokens, targetStmtIdx)

						if currStmtKind != StmtUnknown {
							if f.wantsBlankLine(lastStmtKind, currStmtKind) {
								isJustAfterBlockOpener := prevStmtEnd == token.Do || prevStmtEnd == token.Then || prevStmtEnd == token.Repeat || prevStmtEnd == token.Else || prevStmtEnd == token.ElseIf || prevStmtEnd == token.LBrace

								if prevStmtEnd == token.RParen && prevNonCommentIdx != -1 {
									if f.isFunctionSignatureEnd(tokens, prevNonCommentIdx) {
										isJustAfterBlockOpener = true
									}
								}

								isJustBeforeBlockCloser := tokens[targetStmtIdx].Kind == token.End || tokens[targetStmtIdx].Kind == token.Until || tokens[targetStmtIdx].Kind == token.ElseIf || tokens[targetStmtIdx].Kind == token.Else

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
			}
		}

		gapBuilder.Reset()

		if nl > 0 || forceNl {
			if nl > 2 {
				nl = 2
			}

			if forceNl && nl == 0 {
				nl = 1
			}

			gapBuilder.Write(bytes.Repeat([]byte{'\n'}, nl))

			isLineStart = true
			lineIdx += nl
		}

		if isLineStart {
			currentLineIndent = f.calculateLineIndent(stack, tokens, i, source)
			if currentLineIndent > 0 {
				if f.UseTabs {
					gapBuilder.Write(bytes.Repeat([]byte{'\t'}, currentLineIndent))
				} else {
					gapBuilder.Write(bytes.Repeat([]byte{' '}, currentLineIndent*f.IndentSize))
				}
			}

			isLineStart = false
		} else if prevTok.Kind != 0 {
			if f.needsSpace(prevTok.Kind, tok.Kind, prevPrev) {
				gapBuilder.WriteByte(' ')
			} else if (prevTok.Kind == token.LBrace || tok.Kind == token.RBrace) && bytes.ContainsRune(gap, ' ') {
				gapBuilder.WriteByte(' ')
			}
		}

		newGap := gapBuilder.Bytes()

		var gapStart uint32

		if prevTok.Kind != 0 {
			gapStart = prevTok.End
		}

		gapEnd := tok.Start
		origGap := source[gapStart:gapEnd]

		if !bytes.Equal(origGap, newGap) {
			inRange := true

			if formatRange != nil {
				if gapEnd < rangeStart || gapStart > rangeEnd {
					inRange = false
				}
			}

			if inRange {
				edits = append(edits, TextEdit{
					Range:   getRange(doc.Tree, gapStart, gapEnd),
					NewText: string(newGap),
				})
			}
		}

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
			if f.isKeywordAsIdentifier(tokens, i) {
				break
			}

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
			if f.isKeywordAsIdentifier(tokens, i) {
				break
			}

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

	var gapStart uint32

	if len(tokens) > 0 {
		gapStart = prevTok.End
	}

	gapEnd := uint32(len(source))
	origGap := source[gapStart:gapEnd]

	var newGap []byte

	if len(tokens) > 0 {
		newGap = []byte("\n")
	}

	if !bytes.Equal(origGap, newGap) {
		inRange := true

		if formatRange != nil {
			if gapEnd < rangeStart || gapStart > rangeEnd {
				inRange = false
			}
		}

		if inRange {
			edits = append(edits, TextEdit{
				Range:   getRange(doc.Tree, gapStart, gapEnd),
				NewText: string(newGap),
			})
		}
	}

	return edits
}

func (f *Formatter) isKeywordAsIdentifier(tokens []token.Token, i int) bool {
	tok := tokens[i]
	if !f.isKeyword(tok.Kind) {
		return false
	}

	if i > 0 {
		prev := tokens[i-1].Kind
		if prev == token.Dot || prev == token.Colon {
			return true
		}
	}

	if i+1 < len(tokens) {
		next := tokens[i+1].Kind
		if next == token.Assign {
			return true
		}
	}

	return false
}

func (f *Formatter) calculateLineIndent(stack []Scope, tokens []token.Token, startIndex int, source []byte) int {
	var indent int

	if len(stack) > 0 {
		indent = stack[len(stack)-1].InnerIndent
	}

	currentDepth := len(stack)
	minDepth := currentDepth

	for i := startIndex; i < len(tokens); i++ {
		tok := tokens[i]

		if i > startIndex {
			prev := tokens[i-1]
			gap := source[prev.End:tok.Start]

			if bytes.ContainsRune(gap, '\n') || f.needsNewline(prev.Kind, tok.Kind, stack) {
				break
			}
		}

		if tok.Kind == token.Comment {
			continue
		}

		switch tok.Kind {
		case token.End, token.Until, token.ElseIf, token.Else, token.RBrace, token.RParen, token.RBrack:
			if f.isKeywordAsIdentifier(tokens, i) {
				break
			}
			currentDepth--

			if currentDepth < minDepth {
				minDepth = currentDepth
			}
		}

		switch tok.Kind {
		case token.Do, token.Then, token.Repeat, token.Function, token.Else, token.LBrace, token.LParen, token.LBrack:
			if f.isKeywordAsIdentifier(tokens, i) {
				break
			}
			currentDepth++
		}
	}

	if minDepth < len(stack) {
		if minDepth < 0 {
			minDepth = 0 // Safeguard against malformed code with too many closers
		}

		return stack[minDepth].BaseIndent
	}

	isBlock := len(stack) == 0 || stack[len(stack)-1].Kind == ScopeBlock

	if startIndex > 0 {
		prevNonCommentIdx := startIndex - 1

		for prevNonCommentIdx >= 0 && tokens[prevNonCommentIdx].Kind == token.Comment {
			prevNonCommentIdx--
		}

		if prevNonCommentIdx >= 0 {
			prevK := tokens[prevNonCommentIdx].Kind
			currK := tokens[startIndex].Kind

			var isContinuation bool

			if (prevK >= token.Plus && prevK <= token.Assign) || prevK == token.And || prevK == token.Or || prevK == token.Not || prevK == token.Concat {
				isContinuation = true
			} else if prevK == token.Return || prevK == token.Local {
				isContinuation = true
			} else if prevK == token.Comma && isBlock {
				isContinuation = true
			}

			if !isContinuation {
				if (currK >= token.Plus && currK <= token.GreaterEq) || currK == token.And || currK == token.Or || currK == token.Concat {
					if currK != token.Minus && currK != token.Hash && currK != token.Not && currK != token.BitXor {
						isContinuation = true
					}
				} else if currK == token.Dot || currK == token.Colon {
					isContinuation = true
				}
			}

			if isContinuation {
				indent++
			}
		}
	}

	return indent
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
			tokenKind := tokens[j].Kind

			if tokenKind == token.LParen || tokenKind == token.LBrace || tokenKind == token.LBrack {
				depth++
			} else if tokenKind == token.RParen || tokenKind == token.RBrace || tokenKind == token.RBrack {
				depth--
			} else if depth == 0 {
				if tokenKind == token.Assign || tokenKind == token.Comma {
					return StmtAssign
				}

				switch tokenKind {
				case token.Do, token.Then, token.Else, token.ElseIf, token.End, token.Until, token.Semicolon, token.If, token.For, token.While, token.Repeat, token.Break, token.Goto, token.DoubleColon, token.Local, token.Return, token.Function:
					break Loop
				}

				if j > startIndex {
					prev := tokens[j-1].Kind

					if f.isExprEnd(prev) && tokenKind == token.Ident {
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
