package lsp

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/coalaura/lugo/ast"
	"github.com/coalaura/lugo/lexer"
	"github.com/coalaura/lugo/token"
)

const DefaultMaxLineLength = 120

type Formatter struct {
	IndentSize    int
	MaxLineLength int
	UseTabs       bool
	Opinionated   bool
}

const (
	ScopeBlock = iota
	ScopeParen
	ScopeBrace
	ScopeBrack
)

type Scope struct {
	Kind       int
	Open       int
	BaseIndent int
	Indent     int
	Broken     bool
}

type StmtKind int

const (
	StmtUnknown StmtKind = iota
	StmtLocal
	StmtAssign
	StmtGlobalAssign
	StmtCall
	StmtControl
	StmtFunction
	StmtReturn
)

type FormatGroup struct {
	Open      int32
	Close     int32
	Width     int32
	Kind      uint8
	Broke     bool
	Multiline bool
}

func NewFormatter(indentSize int, useTabs bool, opinionated bool) *Formatter {
	if indentSize <= 0 {
		indentSize = 4
	}

	return &Formatter{
		IndentSize:    indentSize,
		MaxLineLength: DefaultMaxLineLength,
		UseTabs:       useTabs,
		Opinionated:   opinionated,
	}
}

func (s *Server) formatDocument(uri string, options FormattingOptions, formatRange *Range) []TextEdit {
	doc, ok := s.Documents[uri]
	if !ok {
		return nil
	}

	start := time.Now()

	formatter := NewFormatter(options.TabSize, !options.InsertSpaces, s.FormatOpinionated)

	formatter.MaxLineLength = s.FormatMaxLineLength

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

// Format rewrites the whitespace between tokens. Every layout decision is made
// from the token stream and the precomputed group table, never from the gaps in
// the source, so running Format on its own output produces no edits.
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

	if len(tokens) == 0 {
		return nil
	}

	groups, groupAt := f.analyze(tokens, source)

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

	stack := make([]Scope, 1, 64) // index 0 is implicit file level block

	var (
		prevTok           token.Token
		prevIdx           = -1
		prevNonCommentTok token.Token
		prevNonCommentIdx = -1
		prevPrev          token.Kind
		lineIdx           int
		stmtStartLine     int
		lastStmtKind      StmtKind
		lineIndent        int
		col               int
	)

	for i, tok := range tokens {
		var (
			gap   []byte
			srcNl int
		)

		if prevIdx >= 0 {
			gap = source[prevTok.End:tok.Start]
			srcNl = bytes.Count(gap, []byte{'\n'})
		}

		top := stack[len(stack)-1]
		isStmtLevel := top.Kind == ScopeBlock

		if f.Opinionated && isStmtLevel && tok.Kind == token.Semicolon && f.isRedundantSemicolon(tokens, i, source) {
			continue
		}

		// how many line breaks precede this token
		var nl int

		switch {
		case prevIdx < 0:
		case isStmtLevel:
			// statements keep user's line structure
			nl = min(srcNl, 2)

			if nl == 0 && f.needsNewline(prevNonCommentTok.Kind, tok.Kind) {
				nl = 1
			}
		case top.Broken:
			switch {
			case isScopeCloser(top.Kind, tok.Kind):
				nl = 1
			case prevIdx == top.Open:
				nl = 1
			case prevTok.Kind == token.Comma || prevTok.Kind == token.Semicolon:
				nl = 1
			case isLineComment(source, prevTok):
				nl = 1
			case tok.Kind == token.Comment && srcNl > 0:
				nl = 1
			}
		case isLineComment(source, prevTok):
			nl = 1
		}

		// opinionated blank lines between unrelated statements
		if f.Opinionated && isStmtLevel && (nl > 0 || prevIdx < 0) {
			var (
				isFirstOfGroup bool
				targetStmtIdx  = -1
			)

			if tok.Kind != token.Comment {
				if prevTok.Kind != token.Comment || nl > 1 {
					isFirstOfGroup = true
					targetStmtIdx = i
				}
			} else if prevTok.Kind != token.Comment || nl > 1 {
				contiguous := true

				for j := i + 1; j < len(tokens); j++ {
					if bytes.Count(source[tokens[j-1].End:tokens[j].Start], []byte{'\n'}) > 1 {
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

			if isFirstOfGroup && targetStmtIdx != -1 {
				var prevStmtEnd token.Kind

				if prevNonCommentIdx != -1 {
					prevStmtEnd = tokens[prevNonCommentIdx].Kind
				}

				if f.isStatementStart(prevStmtEnd) {
					currStmtKind := f.getStmtKind(doc, tokens, targetStmtIdx)

					if currStmtKind != StmtUnknown {
						wantsBlank := f.wantsBlankLine(lastStmtKind, currStmtKind)

						// anything that ended up spanning lines gets breathing room.
						// lineIdx/stmtStartLine are output lines, not source lines,
						// which keeps this stable across repeated formats.
						if prevStmtEnd == token.End || prevStmtEnd == token.Until || lineIdx > stmtStartLine {
							wantsBlank = true
						}

						if wantsBlank {
							isJustAfterBlockOpener := prevStmtEnd == token.Do || prevStmtEnd == token.Then || prevStmtEnd == token.Repeat || prevStmtEnd == token.Else || prevStmtEnd == token.ElseIf || prevStmtEnd == token.LBrace

							if prevStmtEnd == token.RParen && prevNonCommentIdx != -1 && f.isFunctionSignatureEnd(tokens, prevNonCommentIdx) {
								isJustAfterBlockOpener = true
							}

							next := tokens[targetStmtIdx].Kind
							isJustBeforeBlockCloser := next == token.End || next == token.Until || next == token.ElseIf || next == token.Else

							if !isJustAfterBlockOpener && !isJustBeforeBlockCloser && nl < 2 {
								nl = 2
							}
						}

						lastStmtKind = currStmtKind
					}
				}

				stmtStartLine = lineIdx + nl
			}
		}

		lineIdx += nl

		isLineStart := nl > 0 || prevIdx < 0

		// close scopes ending on this token *before* indenting, so closers
		// align with the line their opener started on without any lookahead
		closeIndent := -1

		switch tok.Kind {
		case token.End, token.Until, token.ElseIf, token.Else:
			if !f.isKeywordAsIdentifier(tokens, i) {
				stack, closeIndent = popScope(stack, ScopeBlock)
			}
		case token.RBrace:
			stack, closeIndent = popScope(stack, ScopeBrace)
		case token.RParen:
			stack, closeIndent = popScope(stack, ScopeParen)
		case token.RBrack:
			stack, closeIndent = popScope(stack, ScopeBrack)
		}

		gapBuilder.Reset()

		for range nl {
			gapBuilder.WriteByte('\n')
		}

		if isLineStart {
			if closeIndent >= 0 {
				lineIndent = closeIndent
			} else {
				lineIndent = f.lineIndent(stack, tokens, i, prevNonCommentIdx)
			}

			col = lineIndent * f.IndentSize

			if f.UseTabs {
				for range lineIndent {
					gapBuilder.WriteByte('\t')
				}
			} else {
				for range col {
					gapBuilder.WriteByte(' ')
				}
			}
		} else if f.needsSpace(prevTok.Kind, tok.Kind, prevPrev) {
			gapBuilder.WriteByte(' ')

			col++
		} else if (prevTok.Kind == token.LBrace || tok.Kind == token.RBrace) && bytes.IndexByte(gap, ' ') != -1 {
			gapBuilder.WriteByte(' ')

			col++
		}

		startCol := col

		col = advanceColumn(col, source, tok)

		newGap := gapBuilder.Bytes()

		var gapStart uint32

		if prevIdx >= 0 {
			gapStart = prevTok.End
		}

		gapEnd := tok.Start
		origGap := source[gapStart:gapEnd]

		if !bytes.Equal(origGap, newGap) {
			inRange := true

			if formatRange != nil && (gapEnd < rangeStart || gapStart > rangeEnd) {
				inRange = false
			}

			if inRange {
				edits = append(edits, TextEdit{
					Range:   getRange(doc.Tree, gapStart, gapEnd),
					NewText: string(newGap),
				})
			}
		}

		// open scopes started by this token
		switch tok.Kind {
		case token.Do, token.Then, token.Repeat, token.Function, token.Else:
			if !f.isKeywordAsIdentifier(tokens, i) {
				stack = append(stack, Scope{
					Kind: ScopeBlock, Open: i,
					BaseIndent: lineIndent, Indent: lineIndent + 1,
				})
			}
		case token.LParen, token.LBrace, token.LBrack:
			kind := ScopeParen

			switch tok.Kind {
			case token.LBrace:
				kind = ScopeBrace
			case token.LBrack:
				kind = ScopeBrack
			}

			stack = append(stack, Scope{
				Kind: kind, Open: i,
				BaseIndent: lineIndent, Indent: lineIndent + 1,
				Broken: f.shouldBreak(groups, groupAt, i, kind, startCol),
			})
		}

		if tok.Kind != token.Comment {
			prevPrev = prevNonCommentTok.Kind
			prevNonCommentTok = tok
			prevNonCommentIdx = i
		}

		prevTok = tok
		prevIdx = i
	}

	gapStart := prevTok.End
	gapEnd := uint32(len(source))
	origGap := source[gapStart:gapEnd]
	newGap := []byte("\n")

	if !bytes.Equal(origGap, newGap) {
		inRange := true

		if formatRange != nil && (gapEnd < rangeStart || gapStart > rangeEnd) {
			inRange = false
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

func (f *Formatter) analyze(tokens []token.Token, source []byte) ([]FormatGroup, []int32) {
	var (
		groups  = make([]FormatGroup, 0, len(tokens)/8+8)
		groupAt = make([]int32, len(tokens))
		cum     = make([]int32, len(tokens)+1)
		spaces  = make([]uint8, len(tokens))
		stack   = make([]int32, 0, 32)
	)

	var (
		prevTok  token.Token
		prevNC   token.Kind
		prevPrev token.Kind
	)

	// mark(true): the enclosing group can never be one line
	// mark(false): the author put a line break at the enclosing group's level
	mark := func(multiline bool) {
		if len(stack) == 0 {
			return
		}

		group := &groups[stack[len(stack)-1]]

		if multiline {
			group.Multiline = true
		} else {
			group.Broke = true
		}
	}

	for i := range tokens {
		tok := tokens[i]

		groupAt[i] = -1

		width := int32(tok.End - tok.Start)

		if tok.Kind == token.String || tok.Kind == token.Comment {
			text := source[tok.Start:tok.End]

			if idx := bytes.LastIndexByte(text, '\n'); idx != -1 {
				width = int32(len(text) - idx - 1)

				mark(true)
			} else if isLineComment(source, tok) {
				mark(true)
			}
		}

		var space uint8

		if i > 0 && f.needsSpace(prevTok.Kind, tok.Kind, prevPrev) {
			space = 1
		}

		spaces[i] = space
		cum[i+1] = cum[i] + int32(space) + width

		if i > 0 && bytes.IndexByte(source[prevTok.End:tok.Start], '\n') != -1 {
			mark(false)
		}

		switch tok.Kind {
		case token.Do, token.Then, token.Repeat, token.Function, token.Else:
			if !f.isKeywordAsIdentifier(tokens, i) {
				mark(true)
			}
		case token.LParen, token.LBrace, token.LBrack:
			kind := ScopeParen

			switch tok.Kind {
			case token.LBrace:
				kind = ScopeBrace
			case token.LBrack:
				kind = ScopeBrack
			}

			groupAt[i] = int32(len(groups))

			groups = append(groups, FormatGroup{Open: int32(i), Close: -1, Kind: uint8(kind)})
			stack = append(stack, groupAt[i])
		case token.RParen, token.RBrace, token.RBrack:
			kind := ScopeParen

			switch tok.Kind {
			case token.RBrace:
				kind = ScopeBrace
			case token.RBrack:
				kind = ScopeBrack
			}

			for j := len(stack) - 1; j >= 0; j-- {
				g := &groups[stack[j]]

				if int(g.Kind) != kind {
					continue
				}

				g.Close = int32(i)
				g.Width = cum[i+1] - cum[g.Open] - int32(spaces[g.Open])

				if g.Kind == ScopeBrace && g.Broke {
					g.Multiline = true
				}

				if g.Multiline && j > 0 {
					groups[stack[j-1]].Multiline = true
				}

				stack = stack[:j]

				break
			}
		}

		prevTok = tok

		if tok.Kind != token.Comment {
			prevPrev = prevNC
			prevNC = tok.Kind
		}
	}

	return groups, groupAt
}

func (f *Formatter) shouldBreak(groups []FormatGroup, groupAt []int32, index, kind, col int) bool {
	gi := groupAt[index]
	if gi < 0 {
		return false
	}

	g := groups[gi]
	if g.Close < 0 {
		return false // unbalanced source, leave it alone
	}

	if g.Multiline {
		return kind == ScopeBrace
	}

	return f.MaxLineLength > 0 && col+int(g.Width) > f.MaxLineLength
}

func (f *Formatter) lineIndent(stack []Scope, tokens []token.Token, index, prevNonComment int) int {
	top := stack[len(stack)-1]
	indent := top.Indent

	if top.Kind != ScopeBlock || prevNonComment < 0 {
		return indent
	}

	prevK := tokens[prevNonComment].Kind
	currK := tokens[index].Kind

	var isContinuation bool

	if (prevK >= token.Plus && prevK <= token.Assign) || prevK == token.And || prevK == token.Or || prevK == token.Not || prevK == token.Concat {
		isContinuation = true
	} else if prevK == token.Return || prevK == token.Local || prevK == token.Comma {
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

	return indent
}

func (f *Formatter) isRedundantSemicolon(tokens []token.Token, index int, source []byte) bool {
	for j := index + 1; j < len(tokens); j++ {
		if tokens[j].Kind == token.Comment {
			continue
		}

		// a leading '(' or '[' would glue onto the previous statement
		if tokens[j].Kind == token.LParen || tokens[j].Kind == token.LBrack {
			return false
		}

		return bytes.IndexByte(source[tokens[index].End:tokens[j].Start], '\n') != -1 || f.needsNewline(tokens[index].Kind, tokens[j].Kind)
	}

	return true
}

func (f *Formatter) getStmtKind(doc *Document, tokens []token.Token, startIndex int) StmtKind {
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
					// check if the root identifier is a global
					if doc != nil && tokens[startIndex].Kind == token.Ident {
						nodeID := doc.Tree.NodeAt(tokens[startIndex].Start)

						for nodeID != ast.InvalidNode {
							node := doc.Tree.Nodes[nodeID]
							if node.Start == tokens[startIndex].Start && node.End == tokens[startIndex].End && node.Kind == ast.KindIdent {
								if doc.Resolver.References[nodeID] == ast.InvalidNode {
									return StmtGlobalAssign
								}

								break
							}

							nodeID = node.Parent
						}
					}

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

func (f *Formatter) wantsBlankLine(prev, curr StmtKind) bool {
	if prev == StmtUnknown || curr == StmtUnknown {
		return false
	}

	// Group locals and regular assignments together
	if (prev == StmtLocal || prev == StmtAssign) && (curr == StmtLocal || curr == StmtAssign) {
		return false
	}

	// Group global assignments together
	if prev == StmtGlobalAssign && curr == StmtGlobalAssign {
		return false
	}

	// Group consecutive function calls together
	if prev == StmtCall && curr == StmtCall {
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
	return (k >= token.And && k <= token.While) || k == token.Ident || k == token.Number || k == token.String
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

func (f *Formatter) needsNewline(left, right token.Kind) bool {
	if left == token.Illegal || left == token.EOF || right == token.Illegal || right == token.EOF {
		return false
	}

	switch right {
	case token.Local, token.If, token.While, token.For, token.Repeat, token.Break, token.Return, token.Goto, token.DoubleColon:
		return true
	case token.End, token.ElseIf, token.Else, token.Until:
		return true
	}

	switch left {
	case token.Do, token.Then, token.Else, token.Repeat, token.Semicolon:
		return true
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

func popScope(stack []Scope, kind int) ([]Scope, int) {
	for i := len(stack) - 1; i > 0; i-- {
		if stack[i].Kind == kind {
			return stack[:i], stack[i].BaseIndent
		}
	}

	return stack, -1
}

func isScopeCloser(kind int, k token.Kind) bool {
	switch kind {
	case ScopeParen:
		return k == token.RParen
	case ScopeBrace:
		return k == token.RBrace
	case ScopeBrack:
		return k == token.RBrack
	}

	return false
}

func isLineComment(source []byte, tok token.Token) bool {
	if tok.Kind != token.Comment || int(tok.End) > len(source) {
		return false
	}

	text := source[tok.Start:tok.End]

	return !bytes.HasPrefix(text, []byte("--[[")) && !bytes.HasPrefix(text, []byte("--[="))
}

func advanceColumn(col int, source []byte, tok token.Token) int {
	if tok.Kind == token.String || tok.Kind == token.Comment {
		text := source[tok.Start:tok.End]

		if idx := bytes.LastIndexByte(text, '\n'); idx != -1 {
			return len(text) - idx - 1
		}
	}

	return col + int(tok.End-tok.Start)
}
