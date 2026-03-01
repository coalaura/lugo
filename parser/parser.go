package parser

import (
	"slices"

	"github.com/coalaura/lugo/ast"
	"github.com/coalaura/lugo/lexer"
	"github.com/coalaura/lugo/token"
)

const (
	Lowest     int = iota
	Or             // or
	And            // and
	Comparison     // < > <= >= ~= ==
	BitOr          // |
	BitXor         // ~
	BitAnd         // &
	BitShift       // << >>
	Concat         // ..
	Term           // + -
	Factor         // * / // %
	Unary          // not # - ~
	Power          // ^
	CallIndex      // () [] .
)

var precedences = [256]int{}

func init() {
	precedences[token.Or] = Or
	precedences[token.And] = And
	precedences[token.Less] = Comparison
	precedences[token.LessEq] = Comparison
	precedences[token.Greater] = Comparison
	precedences[token.GreaterEq] = Comparison
	precedences[token.Eq] = Comparison
	precedences[token.NotEq] = Comparison
	precedences[token.BitOr] = BitOr
	precedences[token.BitXor] = BitXor
	precedences[token.BitAnd] = BitAnd
	precedences[token.ShiftLeft] = BitShift
	precedences[token.ShiftRight] = BitShift
	precedences[token.Concat] = Concat
	precedences[token.Plus] = Term
	precedences[token.Minus] = Term
	precedences[token.Asterisk] = Factor
	precedences[token.Slash] = Factor
	precedences[token.FloorSlash] = Factor
	precedences[token.Modulo] = Factor
	precedences[token.Caret] = Power
	precedences[token.LParen] = CallIndex
	precedences[token.LBrack] = CallIndex
	precedences[token.Dot] = CallIndex
	precedences[token.String] = CallIndex
	precedences[token.LBrace] = CallIndex
	precedences[token.Colon] = CallIndex
}

type ParseError struct {
	Message    string
	Start, End uint32
}

type Parser struct {
	lex       *lexer.Lexer
	tree      *ast.Tree
	Errors    []ParseError
	prev      token.Token
	curr      token.Token
	peek      token.Token
	loopDepth int
	listStack []ast.NodeID
}

func New(source []byte, tree *ast.Tree) *Parser {
	p := &Parser{
		lex:       lexer.New(source),
		tree:      tree,
		listStack: make([]ast.NodeID, 0, 256),
	}

	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) Reset(source []byte, tree *ast.Tree) {
	if p.lex == nil {
		p.lex = lexer.New(source)
	} else {
		p.lex.Reset(source)
	}

	p.tree = tree
	p.Errors = p.Errors[:0]
	p.listStack = p.listStack[:0]
	p.loopDepth = 0

	p.nextToken()
	p.nextToken()
}

func (p *Parser) GetTree() *ast.Tree {
	return p.tree
}

func (p *Parser) Parse() ast.NodeID {
	block := p.parseBlock(token.EOF)

	p.tree.Root = p.tree.AddNode(ast.Node{
		Kind:  ast.KindFile,
		Start: 0,
		End:   uint32(len(p.tree.Source)),
		Left:  block,
	})

	return p.tree.Root
}

func (p *Parser) nextToken() {
	p.prev = p.curr
	p.curr = p.peek
	p.peek = p.lex.Next()

	for p.peek.Kind == token.Comment {
		p.tree.Comments = append(p.tree.Comments, p.peek)

		p.peek = p.lex.Next()
	}
}

func (p *Parser) isAt(kinds ...token.Kind) bool {
	return slices.Contains(kinds, p.curr.Kind)
}

func (p *Parser) error(msg string) {
	p.Errors = append(p.Errors, ParseError{Message: msg, Start: p.curr.Start, End: p.curr.End})
}

func (p *Parser) errorAt(start, end uint32, msg string) {
	p.Errors = append(p.Errors, ParseError{Message: msg, Start: start, End: end})
}

func (p *Parser) validateLHS(listNodeID ast.NodeID) {
	if listNodeID == ast.InvalidNode {
		return
	}

	listNode := p.tree.Nodes[listNodeID]

	for i := uint16(0); i < listNode.Count; i++ {
		exprID := p.tree.ExtraList[listNode.Extra+uint32(i)]
		kind := p.tree.Nodes[exprID].Kind

		if kind != ast.KindIdent && kind != ast.KindIndexExpr && kind != ast.KindMemberExpr {
			node := p.tree.Nodes[exprID]

			p.errorAt(node.Start, node.End, "syntax error: cannot assign to expression")
		}
	}
}

func (p *Parser) sync() {
	p.nextToken()

	for p.curr.Kind != token.EOF {
		// Semicolons are explicit statement boundaries
		if p.curr.Kind == token.Semicolon {
			p.nextToken()

			return
		}

		// Keywords that start a new statement or end a block
		switch p.curr.Kind {
		case token.Local, token.Function, token.If, token.For, token.While, token.Repeat, token.Return, token.Do, token.Break, token.End, token.Goto, token.DoubleColon:
			return
		}

		// identifiers almost always start an assignment or function call.
		// Stopping here prevents cascading errors from wiping out valid lines below.
		if p.curr.Kind == token.Ident {
			return
		}

		p.nextToken()
	}
}

func (p *Parser) parseBlock(stopTokens ...token.Kind) ast.NodeID {
	start := p.curr.Start

	stackStart := len(p.listStack)

	for p.curr.Kind != token.EOF && !p.isAt(stopTokens...) {
		if p.curr.Kind == token.Semicolon {
			p.nextToken()

			continue
		}

		stmt := p.parseStatement()

		if stmt != ast.InvalidNode {
			p.listStack = append(p.listStack, stmt)

			if p.tree.Nodes[stmt].Kind == ast.KindReturn {
				if p.curr.Kind != token.EOF && !p.isAt(stopTokens...) {
					p.error("syntax error: statements are not allowed after a 'return'")
				}
			}
		} else {
			p.sync()
		}
	}

	extraStart, count := p.flushListStack(stackStart)

	return p.tree.AddNode(ast.Node{
		Kind:  ast.KindBlock,
		Start: start, End: p.curr.End,
		Extra: extraStart, Count: uint16(count),
	})
}

func (p *Parser) parseStatement() ast.NodeID {
	switch p.curr.Kind {
	case token.Local:
		return p.parseLocal()
	case token.If:
		return p.parseIf()
	case token.Return:
		return p.parseReturn()
	case token.While:
		return p.parseWhile()
	case token.Repeat:
		return p.parseRepeat()
	case token.For:
		return p.parseFor()
	case token.Do:
		return p.parseDo()
	case token.Break:
		return p.parseBreak()
	case token.Goto:
		return p.parseGoto()
	case token.DoubleColon:
		return p.parseLabel()
	case token.Function:
		return p.parseFunctionStmt()
	default:
		return p.parseAssignmentOrCall()
	}
}

func (p *Parser) parseAssignmentOrCall() ast.NodeID {
	start := p.curr.Start

	lhsList := p.parseExprList()

	if lhsList == ast.InvalidNode {
		return ast.InvalidNode
	}

	if p.curr.Kind == token.Assign {
		p.nextToken() // consume '='

		p.validateLHS(lhsList)

		rhsList := p.parseExprList()

		return p.tree.AddNode(ast.Node{
			Kind:  ast.KindAssign,
			Start: start, End: p.prev.End,
			Left: lhsList, Right: rhsList,
		})
	}

	lhsNode := p.tree.Nodes[lhsList]

	if lhsNode.Count == 1 {
		firstExpr := p.tree.ExtraList[lhsNode.Extra]
		exprKind := p.tree.Nodes[firstExpr].Kind

		if exprKind == ast.KindCallExpr || exprKind == ast.KindMethodCall {
			return firstExpr
		}
	}

	p.errorAt(p.tree.Nodes[lhsList].Start, p.tree.Nodes[lhsList].End, "syntax error: unexpected symbol (expected assignment or function call)")

	return ast.InvalidNode
}

func (p *Parser) parseLocal() ast.NodeID {
	start := p.curr.Start

	p.nextToken() // consume 'local'

	if p.curr.Kind == token.Function {
		p.nextToken() // consume 'function'

		var name ast.NodeID

		if p.curr.Kind == token.Ident {
			name = p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.End})

			p.nextToken()
		} else {
			p.error("expected function name")

			name = p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.Start})
		}

		funcBody := p.parseFunctionBody(start)

		return p.tree.AddNode(ast.Node{
			Kind:  ast.KindLocalFunction,
			Start: start, End: p.tree.Nodes[funcBody].End,
			Left: name, Right: funcBody,
		})
	}

	stackStart := len(p.listStack)

	for {
		if p.curr.Kind != token.Ident {
			p.error("expected identifier")
			break
		}

		ident := p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.End})

		p.nextToken()

		var attr ast.Attr = ast.AttrNone

		if p.curr.Kind == token.Less && p.peek.Kind == token.Ident {
			p.nextToken() // consume '<'

			attrText := p.curr.Text(p.tree.Source)

			switch string(attrText) {
			case "const":
				attr = ast.AttrConst
			case "close":
				attr = ast.AttrClose
			}

			p.nextToken() // consume 'const'/'close'

			if p.curr.Kind == token.Greater {
				p.nextToken()
			}
		}

		p.tree.Nodes[ident].Extra = uint32(attr)
		p.listStack = append(p.listStack, ident)

		if p.curr.Kind == token.Comma {
			p.nextToken()
		} else {
			break
		}
	}

	extraStart, count := p.flushListStack(stackStart)

	var (
		lhsStart uint32
		lhsEnd   uint32
	)

	if count > 0 {
		firstID := p.tree.ExtraList[extraStart]
		lastID := p.tree.ExtraList[extraStart+uint32(count-1)]

		lhsStart = p.tree.Nodes[firstID].Start
		lhsEnd = p.tree.Nodes[lastID].End
	} else {
		lhsStart = p.curr.Start
		lhsEnd = p.curr.Start
	}

	lhsList := p.tree.AddNode(ast.Node{
		Kind:  ast.KindNameList,
		Start: lhsStart,
		End:   lhsEnd,
		Extra: extraStart,
		Count: uint16(count),
	})

	var rhsList ast.NodeID = ast.InvalidNode

	if p.curr.Kind == token.Assign {
		p.nextToken() // consume '='

		rhsList = p.parseExprList()
	}

	return p.tree.AddNode(ast.Node{
		Kind:  ast.KindLocalAssign,
		Start: start, End: p.curr.End,
		Left: lhsList, Right: rhsList,
	})
}

func (p *Parser) parseIf() ast.NodeID {
	start := p.curr.Start

	p.nextToken()

	condition := p.parseExpression(Lowest)

	if p.curr.Kind == token.Then {
		p.nextToken()
	} else {
		p.error("expected 'then'")
	}

	thenBlock := p.parseBlock(token.ElseIf, token.Else, token.End)
	stackStart := len(p.listStack)

	for p.curr.Kind == token.ElseIf {
		elseifStart := p.curr.Start

		p.nextToken()

		cond := p.parseExpression(Lowest)

		if p.curr.Kind == token.Then {
			p.nextToken()
		} else {
			p.error("expected 'then'")
		}

		blk := p.parseBlock(token.ElseIf, token.Else, token.End)

		elseifNode := p.tree.AddNode(ast.Node{
			Kind: ast.KindElseIf, Start: elseifStart, End: p.curr.End,
			Left: cond, Right: blk,
		})

		p.listStack = append(p.listStack, elseifNode)
	}

	var elseBlock ast.NodeID = ast.InvalidNode

	if p.curr.Kind == token.Else {
		elseStart := p.curr.Start

		p.nextToken()

		blk := p.parseBlock(token.End)

		elseBlock = p.tree.AddNode(ast.Node{
			Kind: ast.KindElse, Start: elseStart, End: p.curr.End,
			Left: blk,
		})
	}

	if p.curr.Kind == token.End {
		p.nextToken()
	} else {
		p.error("expected 'end'")
	}

	if elseBlock != ast.InvalidNode {
		p.listStack = append(p.listStack, elseBlock)
	}

	extraStart, count := p.flushListStack(stackStart)

	return p.tree.AddNode(ast.Node{
		Kind:  ast.KindIf,
		Start: start, End: p.prev.End,
		Left: condition, Right: thenBlock,
		Extra: extraStart, Count: uint16(count),
	})
}

func (p *Parser) parseReturn() ast.NodeID {
	start := p.curr.Start

	p.nextToken()

	var exprList ast.NodeID = ast.InvalidNode

	if p.curr.Kind != token.End && p.curr.Kind != token.ElseIf && p.curr.Kind != token.Else && p.curr.Kind != token.EOF && p.curr.Kind != token.Semicolon {
		exprList = p.parseExprList()
	}

	if p.curr.Kind == token.Semicolon {
		p.nextToken()
	}

	return p.tree.AddNode(ast.Node{
		Kind:  ast.KindReturn,
		Start: start, End: p.curr.End,
		Left: exprList,
	})
}

func (p *Parser) parseWhile() ast.NodeID {
	start := p.curr.Start

	p.nextToken()

	condition := p.parseExpression(Lowest)

	if p.curr.Kind == token.Do {
		p.nextToken()
	} else {
		p.error("expected 'do'")
	}

	p.loopDepth++
	block := p.parseBlock(token.End)
	p.loopDepth--

	end := p.curr.End

	if p.curr.Kind == token.End {
		p.nextToken()
	} else {
		p.error("expected 'end'")
	}

	return p.tree.AddNode(ast.Node{Kind: ast.KindWhile, Start: start, End: end, Left: condition, Right: block})
}

func (p *Parser) parseRepeat() ast.NodeID {
	start := p.curr.Start

	p.nextToken()

	p.loopDepth++
	block := p.parseBlock(token.Until)
	p.loopDepth--

	if p.curr.Kind == token.Until {
		p.nextToken()
	} else {
		p.error("expected 'until'")
	}

	condition := p.parseExpression(Lowest)

	return p.tree.AddNode(ast.Node{Kind: ast.KindRepeat, Start: start, End: p.curr.End, Left: block, Right: condition})
}

func (p *Parser) parseFor() ast.NodeID {
	start := p.curr.Start

	p.nextToken()

	if p.curr.Kind != token.Ident {
		p.error("expected identifier after 'for'")

		return ast.InvalidNode
	}

	firstIdent := p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.End})

	p.nextToken()

	if p.curr.Kind == token.Assign {
		p.nextToken() // consume '='

		stackStart := len(p.listStack)

		initExpr := p.parseExpression(Lowest)

		p.listStack = append(p.listStack, initExpr)

		if p.curr.Kind == token.Comma {
			p.nextToken()
		} else {
			p.error("expected ','")
		}

		limitExpr := p.parseExpression(Lowest)

		p.listStack = append(p.listStack, limitExpr)

		if p.curr.Kind == token.Comma {
			p.nextToken()

			stepExpr := p.parseExpression(Lowest)

			p.listStack = append(p.listStack, stepExpr)
		}

		if p.curr.Kind == token.Do {
			p.nextToken()
		} else {
			p.error("expected 'do'")
		}

		p.loopDepth++
		block := p.parseBlock(token.End)
		p.loopDepth--

		end := p.curr.End

		if p.curr.Kind == token.End {
			p.nextToken()
		} else {
			p.error("expected 'end'")
		}

		extraStart, count := p.flushListStack(stackStart)

		return p.tree.AddNode(ast.Node{
			Kind: ast.KindForNum, Start: start, End: end,
			Left: firstIdent, Right: block, Extra: extraStart, Count: uint16(count),
		})
	}

	stackStart := len(p.listStack)
	p.listStack = append(p.listStack, firstIdent)

	for p.curr.Kind == token.Comma {
		p.nextToken()

		if p.curr.Kind == token.Ident {
			ident := p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.End})

			p.listStack = append(p.listStack, ident)

			p.nextToken()
		} else {
			p.error("expected identifier")

			break
		}
	}

	count := len(p.listStack) - stackStart
	extraStartNames := uint32(len(p.tree.ExtraList))

	p.tree.ExtraList = append(p.tree.ExtraList, p.listStack[stackStart:]...)
	p.listStack = p.listStack[:stackStart]

	nameList := p.tree.AddNode(ast.Node{
		Kind: ast.KindNameList, Extra: extraStartNames, Count: uint16(count),
		Start: p.tree.Nodes[firstIdent].Start, End: p.curr.End,
	})

	if p.curr.Kind == token.In {
		p.nextToken()
	} else {
		p.error("expected 'in'")
	}

	exprList := p.parseExprList()

	if p.curr.Kind == token.Do {
		p.nextToken()
	} else {
		p.error("expected 'do'")
	}

	p.loopDepth++
	block := p.parseBlock(token.End)
	p.loopDepth--

	end := p.curr.End

	if p.curr.Kind == token.End {
		p.nextToken()
	} else {
		p.error("expected 'end'")
	}

	return p.tree.AddNode(ast.Node{
		Kind: ast.KindForIn, Start: start, End: end,
		Left: nameList, Right: block, Extra: uint32(exprList),
	})
}

func (p *Parser) parseFunctionStmt() ast.NodeID {
	start := p.curr.Start

	p.nextToken()

	var nameNode ast.NodeID

	if p.curr.Kind == token.Ident {
		nameNode = p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.End})

		p.nextToken()
	} else {
		p.error("expected function name")

		nameNode = p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.Start})
	}

	for p.curr.Kind == token.Dot {
		p.nextToken()

		if p.curr.Kind != token.Ident {
			p.error("expected identifier")

			break
		}

		right := p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.End})

		p.nextToken()

		nameNode = p.tree.AddNode(ast.Node{
			Kind: ast.KindMemberExpr, Start: p.tree.Nodes[nameNode].Start, End: p.prev.End,
			Left: nameNode, Right: right,
		})
	}

	if p.curr.Kind == token.Colon {
		p.nextToken()

		if p.curr.Kind != token.Ident {
			p.error("expected identifier")
		} else {
			right := p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.End})
			p.nextToken()

			nameNode = p.tree.AddNode(ast.Node{
				Kind: ast.KindMethodName, Start: p.tree.Nodes[nameNode].Start, End: p.prev.End,
				Left: nameNode, Right: right,
			})
		}
	}

	funcBody := p.parseFunctionBody(start)

	return p.tree.AddNode(ast.Node{
		Kind: ast.KindFunctionStmt, Start: start, End: p.tree.Nodes[funcBody].End,
		Left: nameNode, Right: funcBody,
	})
}

func (p *Parser) parseDo() ast.NodeID {
	start := p.curr.Start

	p.nextToken()

	block := p.parseBlock(token.End)

	end := p.curr.End

	if p.curr.Kind == token.End {
		p.nextToken()
	} else {
		p.error("expected 'end'")
	}

	return p.tree.AddNode(ast.Node{Kind: ast.KindDo, Start: start, End: end, Left: block})
}

func (p *Parser) parseBreak() ast.NodeID {
	start, end := p.curr.Start, p.curr.End

	if p.loopDepth == 0 {
		p.error("syntax error: <break> not inside a loop")
	}

	p.nextToken()

	return p.tree.AddNode(ast.Node{Kind: ast.KindBreak, Start: start, End: end})
}

func (p *Parser) parseGoto() ast.NodeID {
	start := p.curr.Start

	p.nextToken()

	var label ast.NodeID = ast.InvalidNode

	if p.curr.Kind == token.Ident {
		label = p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.End})

		p.nextToken()
	} else {
		p.error("expected label name after 'goto'")
	}

	return p.tree.AddNode(ast.Node{Kind: ast.KindGoto, Start: start, End: p.curr.End, Left: label})
}

func (p *Parser) parseLabel() ast.NodeID {
	start := p.curr.Start

	p.nextToken()

	var name ast.NodeID = ast.InvalidNode

	if p.curr.Kind == token.Ident {
		name = p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.End})

		p.nextToken()
	} else {
		p.error("expected label name")
	}

	if p.curr.Kind == token.DoubleColon {
		p.nextToken()
	} else {
		p.error("expected '::'")
	}

	return p.tree.AddNode(ast.Node{Kind: ast.KindLabel, Start: start, End: p.curr.End, Left: name})
}

func (p *Parser) parseExprList() ast.NodeID {
	start := p.curr.Start

	stackStart := len(p.listStack)

	for {
		expr := p.parseExpression(Lowest)
		if expr != ast.InvalidNode {
			p.listStack = append(p.listStack, expr)
		}

		if p.curr.Kind == token.Comma {
			p.nextToken()
		} else {
			break
		}
	}

	count := len(p.listStack) - stackStart
	if count == 0 {
		return ast.InvalidNode
	}

	extraStart := uint32(len(p.tree.ExtraList))
	p.tree.ExtraList = append(p.tree.ExtraList, p.listStack[stackStart:]...)

	p.listStack = p.listStack[:stackStart]

	return p.tree.AddNode(ast.Node{
		Kind:  ast.KindExprList,
		Start: start, End: p.curr.End,
		Extra: extraStart, Count: uint16(count),
	})
}

func (p *Parser) parseExpression(precedence int) ast.NodeID {
	leftID := p.parsePrefix()

	if leftID == ast.InvalidNode {
		return ast.InvalidNode
	}

	for precedence < precedences[p.curr.Kind] {
		leftID = p.parseInfix(leftID)
	}

	return leftID
}

func (p *Parser) parsePrefix() ast.NodeID {
	var id ast.NodeID

	switch p.curr.Kind {
	case token.Ident:
		id = p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.End})

		p.nextToken()
	case token.Number:
		id = p.tree.AddNode(ast.Node{Kind: ast.KindNumber, Start: p.curr.Start, End: p.curr.End})

		p.nextToken()
	case token.String:
		id = p.tree.AddNode(ast.Node{Kind: ast.KindString, Start: p.curr.Start, End: p.curr.End})

		p.nextToken()
	case token.Nil:
		id = p.tree.AddNode(ast.Node{Kind: ast.KindNil, Start: p.curr.Start, End: p.curr.End})

		p.nextToken()
	case token.True:
		id = p.tree.AddNode(ast.Node{Kind: ast.KindTrue, Start: p.curr.Start, End: p.curr.End})

		p.nextToken()
	case token.False:
		id = p.tree.AddNode(ast.Node{Kind: ast.KindFalse, Start: p.curr.Start, End: p.curr.End})

		p.nextToken()
	case token.Vararg:
		id = p.tree.AddNode(ast.Node{Kind: ast.KindVararg, Start: p.curr.Start, End: p.curr.End})

		p.nextToken()
	case token.LBrace:
		return p.parseTableConstructor()
	case token.Function:
		start := p.curr.Start

		p.nextToken()

		return p.parseFunctionBody(start)
	case token.Minus, token.Not, token.Hash, token.BitXor:
		start := p.curr.Start

		p.nextToken()

		right := p.parseExpression(Unary)

		return p.tree.AddNode(ast.Node{
			Kind: ast.KindUnaryExpr, Start: start, End: p.prev.End, Right: right,
		})
	case token.LParen:
		start := p.curr.Start

		p.nextToken()

		id = p.parseExpression(Lowest)

		end := p.curr.End

		if p.curr.Kind == token.RParen {
			p.nextToken()
		} else {
			p.error("expected closing ')'")
		}

		return p.tree.AddNode(ast.Node{
			Kind:  ast.KindParenExpr,
			Start: start, End: end,
			Left: id,
		})
	default:
		p.error("expected expression")

		return ast.InvalidNode
	}

	return id
}

func (p *Parser) parseInfix(left ast.NodeID) ast.NodeID {
	opKind := p.curr.Kind
	precedence := precedences[opKind]

	if opKind == token.Caret || opKind == token.Concat {
		precedence--
	}

	start := p.tree.Nodes[left].Start

	switch opKind {
	case token.Plus, token.Minus, token.Asterisk, token.Slash, token.FloorSlash, token.Modulo, token.Caret, token.Concat, token.Eq, token.NotEq, token.Less, token.LessEq, token.Greater, token.GreaterEq, token.And, token.Or, token.BitAnd, token.BitOr, token.BitXor, token.ShiftLeft, token.ShiftRight:
		p.nextToken()

		right := p.parseExpression(precedence)

		return p.tree.AddNode(ast.Node{
			Kind: ast.KindBinaryExpr, Start: start, End: p.prev.End,
			Left: left, Right: right, Extra: uint32(opKind),
		})
	case token.Dot:
		p.nextToken()

		if p.curr.Kind != token.Ident {
			p.error("expected identifier after '.'")

			return left
		}

		right := p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.End})

		p.nextToken()

		return p.tree.AddNode(ast.Node{
			Kind: ast.KindMemberExpr, Start: start, End: p.prev.End,
			Left: left, Right: right,
		})
	case token.LBrack:
		p.nextToken()

		right := p.parseExpression(Lowest)

		if p.curr.Kind != token.RBrack {
			p.error("expected ']'")
		}

		end := p.curr.End

		if p.curr.Kind == token.RBrack {
			p.nextToken()
		}

		return p.tree.AddNode(ast.Node{
			Kind: ast.KindIndexExpr, Start: start, End: end,
			Left: left, Right: right,
		})
	case token.LParen, token.String, token.LBrace:

		return p.parseCallArgs(left, opKind)
	case token.Colon:
		p.nextToken()

		if p.curr.Kind != token.Ident {
			p.error("expected method name")

			return left
		}

		methodName := p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.End})

		p.nextToken()

		if p.curr.Kind != token.LParen && p.curr.Kind != token.String && p.curr.Kind != token.LBrace {
			p.error("expected function arguments")

			return left
		}

		callNode := p.parseCallArgs(methodName, p.curr.Kind)

		p.tree.Nodes[callNode].Kind = ast.KindMethodCall
		p.tree.Nodes[callNode].Left = left
		p.tree.Nodes[callNode].Right = methodName
		p.tree.Nodes[callNode].Start = start

		if left != ast.InvalidNode {
			p.tree.Nodes[left].Parent = callNode
		}

		return callNode
	default:
		return left
	}
}

func (p *Parser) parseTableConstructor() ast.NodeID {
	start := p.curr.Start

	p.nextToken()

	stackStart := len(p.listStack)

	for p.curr.Kind != token.RBrace && p.curr.Kind != token.EOF {
		var field ast.NodeID

		if p.curr.Kind == token.LBrack {
			fieldStart := p.curr.Start

			p.nextToken()

			key := p.parseExpression(Lowest)

			if p.curr.Kind != token.RBrack {
				p.error("expected ']'")
			} else {
				p.nextToken()
			}

			if p.curr.Kind != token.Assign {
				p.error("expected '='")
			} else {
				p.nextToken()
			}

			val := p.parseExpression(Lowest)

			field = p.tree.AddNode(ast.Node{
				Kind: ast.KindIndexField, Left: key, Right: val,
				Start: fieldStart, End: p.prev.End,
			})
		} else if p.curr.Kind == token.Ident && p.peek.Kind == token.Assign {
			fieldStart := p.curr.Start

			key := p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.End})

			p.nextToken()
			p.nextToken() // consume '='

			val := p.parseExpression(Lowest)

			field = p.tree.AddNode(ast.Node{
				Kind: ast.KindRecordField, Left: key, Right: val,
				Start: fieldStart, End: p.prev.End,
			})
		} else {
			field = p.parseExpression(Lowest)
		}

		p.listStack = append(p.listStack, field)

		if p.curr.Kind == token.Comma || p.curr.Kind == token.Semicolon {
			p.nextToken()
		} else {
			break
		}
	}

	end := p.curr.End

	if p.curr.Kind == token.RBrace {
		p.nextToken()
	} else {
		p.error("expected '}'")
	}

	extraStart, count := p.flushListStack(stackStart)

	return p.tree.AddNode(ast.Node{
		Kind: ast.KindTableExpr, Start: start, End: end, Extra: extraStart, Count: uint16(count),
	})
}

func (p *Parser) parseCallArgs(left ast.NodeID, callToken token.Kind) ast.NodeID {
	start := p.tree.Nodes[left].Start
	stackStart := len(p.listStack)

	var end uint32

	switch callToken {
	case token.LParen:
		p.nextToken()

		for p.curr.Kind != token.RParen && p.curr.Kind != token.EOF {
			arg := p.parseExpression(Lowest)

			if arg != ast.InvalidNode {
				p.listStack = append(p.listStack, arg)
			}

			if p.curr.Kind == token.Comma {
				p.nextToken()
			} else {
				break
			}
		}

		end = p.curr.End

		if p.curr.Kind == token.RParen {
			p.nextToken()
		} else {
			p.error("expected ')'")
		}
	case token.String:
		arg := p.tree.AddNode(ast.Node{Kind: ast.KindString, Start: p.curr.Start, End: p.curr.End})

		p.listStack = append(p.listStack, arg)

		end = p.curr.End

		p.nextToken()
	case token.LBrace:
		arg := p.parseTableConstructor()

		p.listStack = append(p.listStack, arg)

		end = p.tree.Nodes[arg].End
	}

	extraStart, count := p.flushListStack(stackStart)

	return p.tree.AddNode(ast.Node{
		Kind: ast.KindCallExpr, Start: start, End: end, Left: left, Extra: extraStart, Count: uint16(count),
	})
}

func (p *Parser) parseFunctionBody(start uint32) ast.NodeID {
	if p.curr.Kind != token.LParen {
		p.error("expected '(' for function parameters")
	} else {
		p.nextToken() // consume '('
	}

	stackStart := len(p.listStack)

	for p.curr.Kind != token.RParen && p.curr.Kind != token.EOF {
		if p.curr.Kind == token.Ident || p.curr.Kind == token.Vararg {
			param := p.tree.AddNode(ast.Node{Kind: ast.KindIdent, Start: p.curr.Start, End: p.curr.End})

			isVararg := p.curr.Kind == token.Vararg

			if isVararg {
				p.tree.Nodes[param].Kind = ast.KindVararg
			}

			p.listStack = append(p.listStack, param)

			p.nextToken()

			if isVararg && p.curr.Kind == token.Comma {
				p.error("syntax error: vararg '...' must be the last parameter")
			}

			if p.curr.Kind == token.Comma {
				p.nextToken()
			} else {
				break
			}
		} else {
			p.error("expected identifier or '...' in parameters")

			break
		}
	}

	if p.curr.Kind == token.RParen {
		p.nextToken()
	} else {
		p.error("expected ')'")
	}

	block := p.parseBlock(token.End)

	end := p.curr.End

	if p.curr.Kind == token.End {
		p.nextToken()
	} else {
		p.error("expected 'end'")
	}

	extraStart, count := p.flushListStack(stackStart)

	return p.tree.AddNode(ast.Node{
		Kind:  ast.KindFunctionExpr,
		Start: start, End: end,
		Extra: extraStart, Count: uint16(count),
		Right: block,
	})
}

func (p *Parser) flushListStack(stackStart int) (extraStart uint32, count uint16) {
	count = uint16(len(p.listStack) - stackStart)
	if count == 0 {
		return 0, 0
	}

	extraStart = uint32(len(p.tree.ExtraList))
	p.tree.ExtraList = append(p.tree.ExtraList, p.listStack[stackStart:]...)
	p.listStack = p.listStack[:stackStart]

	return extraStart, count
}
