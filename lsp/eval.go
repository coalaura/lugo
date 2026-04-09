package lsp

import (
	"bytes"
	"math"
	"strconv"
	"strings"

	"github.com/coalaura/lugo/ast"
	"github.com/coalaura/lugo/token"
)

type evalResult struct {
	kind    ast.NodeKind
	num     float64
	str     string
	boolVal bool
}

// FindEvaluableParent climbs the AST from the cursor to find the highest
// expression that can be statically evaluated.
func (doc *Document) FindEvaluableParent(offset uint32) (uint32, uint32, string, bool) {
	curr := doc.Tree.NodeAt(offset)

	var (
		highestEvalNode ast.NodeID = ast.InvalidNode
		highestVal      string
	)

	for curr != ast.InvalidNode {
		node := doc.Tree.Nodes[curr]

		// Stop climbing if we hit a statement boundary
		switch node.Kind {
		case ast.KindFile, ast.KindBlock, ast.KindReturn, ast.KindAssign, ast.KindLocalAssign, ast.KindExprList, ast.KindNameList, ast.KindFunctionStmt, ast.KindLocalFunction, ast.KindIf, ast.KindElseIf, ast.KindWhile, ast.KindRepeat, ast.KindForIn, ast.KindForNum, ast.KindCallExpr, ast.KindMethodCall:
			curr = ast.InvalidNode

			continue
		}

		if !(node.Kind == ast.KindNumber || node.Kind == ast.KindString || node.Kind == ast.KindTrue || node.Kind == ast.KindFalse || node.Kind == ast.KindNil || node.Kind == ast.KindUnaryExpr || node.Kind == ast.KindBinaryExpr || node.Kind == ast.KindParenExpr || node.Kind == ast.KindIdent) {
			break
		}

		val, ok := doc.EvaluateExpression(curr)
		if ok {
			highestEvalNode = curr
			highestVal = val
		}

		curr = node.Parent
	}

	if highestEvalNode != ast.InvalidNode {
		node := doc.Tree.Nodes[highestEvalNode]
		return node.Start, node.End, highestVal, true
	}

	// Fallback: If left-associativity broke the evaluation (e.g. Call() * 1.25 * 1000)
	// we extract the contiguous segment of constants from the associative chain.
	return doc.findPartialEval(offset)
}

func isAssociative(op token.Kind) bool {
	switch op {
	case token.Plus, token.Asterisk, token.BitAnd, token.BitOr, token.BitXor, token.And, token.Or, token.Concat:
		return true
	}
	return false
}

func (doc *Document) findPartialEval(offset uint32) (uint32, uint32, string, bool) {
	curr := doc.Tree.NodeAt(offset)
	if curr == ast.InvalidNode {
		return 0, 0, "", false
	}

	var (
		chainRoot ast.NodeID = ast.InvalidNode
		chainOp   token.Kind
	)

	// 1. Climb up to find the root of the current associative operator chain
	temp := curr
	for temp != ast.InvalidNode {
		node := doc.Tree.Nodes[temp]
		if node.Kind == ast.KindBinaryExpr {
			op := token.Kind(node.Extra)
			if isAssociative(op) {
				if chainRoot == ast.InvalidNode || chainOp == op {
					chainRoot = temp
					chainOp = op
				} else {
					break
				}
			} else {
				break
			}
		} else if node.Kind == ast.KindParenExpr || node.Kind == ast.KindReturn || node.Kind == ast.KindAssign || node.Kind == ast.KindLocalAssign {
			if chainRoot != ast.InvalidNode {
				break
			}
		}
		temp = node.Parent
	}

	if chainRoot == ast.InvalidNode {
		return 0, 0, "", false
	}

	// 2. Flatten the tree into an array of operands (left-to-right)
	var (
		operands []ast.NodeID
		flatten  func(id ast.NodeID)
	)

	flatten = func(id ast.NodeID) {
		node := doc.Tree.Nodes[id]
		if node.Kind == ast.KindBinaryExpr && token.Kind(node.Extra) == chainOp {
			flatten(node.Left)
			flatten(node.Right)
		} else {
			operands = append(operands, id)
		}
	}

	flatten(chainRoot)

	// 3. Find the contiguous segment of evaluable constants that the cursor is inside
	bestStart, bestEnd := -1, -1
	currentStart := -1

	for i, opID := range operands {
		_, ok := doc.evalNode(opID, 0)
		if ok {
			if currentStart == -1 {
				currentStart = i
			}

			startOffset := doc.Tree.Nodes[operands[currentStart]].Start
			endOffset := doc.Tree.Nodes[opID].End

			if offset >= startOffset && offset <= endOffset {
				bestStart = currentStart
				bestEnd = i
			}
		} else {
			currentStart = -1
		}
	}

	// 4. If we found at least 2 constants together, evaluate them
	if bestStart != -1 && bestEnd != -1 && bestEnd > bestStart {
		res, ok := doc.evalNode(operands[bestStart], 0)
		if !ok {
			return 0, 0, "", false
		}

		for i := bestStart + 1; i <= bestEnd; i++ {
			nextRes, ok := doc.evalNode(operands[i], 0)
			if !ok {
				return 0, 0, "", false
			}

			res, ok = applyOp(res, nextRes, chainOp)
			if !ok {
				return 0, 0, "", false
			}
		}

		startOffset := doc.Tree.Nodes[operands[bestStart]].Start
		endOffset := doc.Tree.Nodes[operands[bestEnd]].End

		str, ok := formatEvalResult(res)
		if ok {
			return startOffset, endOffset, str, true
		}
	}

	return 0, 0, "", false
}

// EvaluateExpression attempts to evaluate an AST node into a literal string.
func (doc *Document) EvaluateExpression(id ast.NodeID) (string, bool) {
	val, ok := doc.evalNode(id, 0)
	if !ok {
		return "", false
	}

	// Don't format if it's just a single literal (no value added for the user)
	node := doc.Tree.Nodes[id]
	if node.Kind == ast.KindNumber || node.Kind == ast.KindString || node.Kind == ast.KindTrue || node.Kind == ast.KindFalse || node.Kind == ast.KindNil {
		return "", false
	}

	return formatEvalResult(val)
}

func formatEvalResult(val evalResult) (string, bool) {
	switch val.kind {
	case ast.KindNumber:
		if val.num == math.Trunc(val.num) && val.num >= math.MinInt64 && val.num <= math.MaxInt64 {
			return strconv.FormatInt(int64(val.num), 10), true
		}

		return strconv.FormatFloat(val.num, 'g', 14, 64), true
	case ast.KindString:
		return strconv.Quote(val.str), true
	case ast.KindTrue:
		return "true", true
	case ast.KindFalse:
		return "false", true
	case ast.KindNil:
		return "nil", true
	}

	return "", false
}

func (doc *Document) evalNode(id ast.NodeID, depth int) (evalResult, bool) {
	if id == ast.InvalidNode || depth > 20 { // depth guard prevents cyclical constants
		return evalResult{}, false
	}

	node := doc.Tree.Nodes[id]

	switch node.Kind {
	case ast.KindNumber:
		raw := ast.String(doc.Source[node.Start:node.End])

		floatVal, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			// Fallback for hex/octal ints
			if i, err2 := strconv.ParseInt(raw, 0, 64); err2 == nil {
				floatVal = float64(i)
			} else {
				return evalResult{}, false
			}
		}

		return evalResult{kind: ast.KindNumber, num: floatVal}, true
	case ast.KindString:
		raw := ast.String(doc.Source[node.Start:node.End])
		if len(raw) >= 2 && (raw[0] == '\'' || raw[0] == '"') {
			unq, err := strconv.Unquote(raw)
			if err == nil {
				return evalResult{kind: ast.KindString, str: unq}, true
			}

			return evalResult{kind: ast.KindString, str: raw[1 : len(raw)-1]}, true
		} else if strings.HasPrefix(raw, "[") {
			idx := strings.IndexByte(raw[1:], '[')
			if idx != -1 {
				start := 2 + idx
				if start < len(raw) && raw[start] == '\n' {
					start++
				}

				end := len(raw) - (2 + idx)
				if start <= end {
					return evalResult{kind: ast.KindString, str: raw[start:end]}, true
				}
			}
		}

		return evalResult{}, false
	case ast.KindTrue:
		return evalResult{kind: ast.KindTrue, boolVal: true}, true
	case ast.KindFalse:
		return evalResult{kind: ast.KindFalse, boolVal: false}, true
	case ast.KindNil:
		return evalResult{kind: ast.KindNil}, true
	case ast.KindParenExpr:
		return doc.evalNode(node.Left, depth+1)
	case ast.KindIdent:
		defID := doc.Resolver.References[id]
		if defID != ast.InvalidNode {
			valID := doc.getAssignedValue(defID)
			if valID != ast.InvalidNode && valID != id {
				return doc.evalNode(valID, depth+1)
			}
		}

		return evalResult{}, false
	case ast.KindUnaryExpr:
		right, ok := doc.evalNode(node.Right, depth+1)
		if !ok {
			return evalResult{}, false
		}

		src := doc.Source[node.Start:node.End]
		if bytes.HasPrefix(src, []byte("-")) && right.kind == ast.KindNumber {
			return evalResult{kind: ast.KindNumber, num: -right.num}, true
		} else if bytes.HasPrefix(src, []byte("~")) && right.kind == ast.KindNumber {
			return evalResult{kind: ast.KindNumber, num: float64(^int64(right.num))}, true
		} else if bytes.HasPrefix(src, []byte("not")) {
			isTruthy := true

			if right.kind == ast.KindFalse || right.kind == ast.KindNil {
				isTruthy = false
			}

			if isTruthy {
				return evalResult{kind: ast.KindFalse, boolVal: false}, true
			}

			return evalResult{kind: ast.KindTrue, boolVal: true}, true
		} else if bytes.HasPrefix(src, []byte("#")) && right.kind == ast.KindString {
			return evalResult{kind: ast.KindNumber, num: float64(len(right.str))}, true
		}

		return evalResult{}, false
	case ast.KindBinaryExpr:
		left, okL := doc.evalNode(node.Left, depth+1)
		right, okR := doc.evalNode(node.Right, depth+1)

		if !okL || !okR {
			return evalResult{}, false
		}

		return applyOp(left, right, token.Kind(node.Extra))
	}

	return evalResult{}, false
}

func applyOp(left, right evalResult, op token.Kind) (evalResult, bool) {
	if op == token.Concat {
		var (
			str1 string
			str2 string
		)

		switch left.kind {
		case ast.KindString:
			str1 = left.str
		case ast.KindNumber:
			if left.num == math.Trunc(left.num) && left.num >= math.MinInt64 && left.num <= math.MaxInt64 {
				str1 = strconv.FormatInt(int64(left.num), 10)
			} else {
				str1 = strconv.FormatFloat(left.num, 'g', 14, 64)
			}
		default:
			return evalResult{}, false
		}

		switch right.kind {
		case ast.KindString:
			str2 = right.str
		case ast.KindNumber:
			if right.num == math.Trunc(right.num) && right.num >= math.MinInt64 && right.num <= math.MaxInt64 {
				str2 = strconv.FormatInt(int64(right.num), 10)
			} else {
				str2 = strconv.FormatFloat(right.num, 'g', 14, 64)
			}
		default:
			return evalResult{}, false
		}

		return evalResult{kind: ast.KindString, str: str1 + str2}, true
	}

	if left.kind == ast.KindNumber && right.kind == ast.KindNumber {
		numKind := ast.KindNumber

		switch op {
		case token.Plus:
			return evalResult{kind: numKind, num: left.num + right.num}, true
		case token.Minus:
			return evalResult{kind: numKind, num: left.num - right.num}, true
		case token.Asterisk:
			return evalResult{kind: numKind, num: left.num * right.num}, true
		case token.Slash:
			if right.num == 0 {
				return evalResult{}, false
			}

			return evalResult{kind: numKind, num: left.num / right.num}, true
		case token.FloorSlash:
			if right.num == 0 {
				return evalResult{}, false
			}

			return evalResult{kind: numKind, num: math.Floor(left.num / right.num)}, true
		case token.Modulo:
			if right.num == 0 {
				return evalResult{}, false
			}

			mod := left.num - math.Floor(left.num/right.num)*right.num // True Lua modulo

			return evalResult{kind: numKind, num: mod}, true
		case token.Caret:
			return evalResult{kind: numKind, num: math.Pow(left.num, right.num)}, true
		case token.BitAnd:
			return evalResult{kind: numKind, num: float64(int64(left.num) & int64(right.num))}, true
		case token.BitOr:
			return evalResult{kind: numKind, num: float64(int64(left.num) | int64(right.num))}, true
		case token.BitXor:
			return evalResult{kind: numKind, num: float64(int64(left.num) ^ int64(right.num))}, true
		case token.ShiftLeft:
			shift := int64(right.num)
			if shift < 0 {
				return evalResult{}, false
			}

			return evalResult{kind: numKind, num: float64(int64(left.num) << shift)}, true
		case token.ShiftRight:
			shift := int64(right.num)
			if shift < 0 {
				return evalResult{}, false
			}

			return evalResult{kind: numKind, num: float64(int64(left.num) >> shift)}, true
		case token.Eq:
			return evalResult{kind: ast.KindTrue, boolVal: left.num == right.num}, true
		case token.NotEq:
			return evalResult{kind: ast.KindTrue, boolVal: left.num != right.num}, true
		case token.Less:
			return evalResult{kind: ast.KindTrue, boolVal: left.num < right.num}, true
		case token.LessEq:
			return evalResult{kind: ast.KindTrue, boolVal: left.num <= right.num}, true
		case token.Greater:
			return evalResult{kind: ast.KindTrue, boolVal: left.num > right.num}, true
		case token.GreaterEq:
			return evalResult{kind: ast.KindTrue, boolVal: left.num >= right.num}, true
		}
	}

	if left.kind == ast.KindString && right.kind == ast.KindString {
		switch op {
		case token.Eq:
			return evalResult{kind: ast.KindTrue, boolVal: left.str == right.str}, true
		case token.NotEq:
			return evalResult{kind: ast.KindTrue, boolVal: left.str != right.str}, true
		}
	}

	if op == token.And || op == token.Or {
		lTruthy := left.kind != ast.KindFalse && left.kind != ast.KindNil
		if op == token.And {
			if lTruthy {
				return right, true
			}

			return left, true
		}

		if lTruthy {
			return left, true
		}

		return right, true
	}

	return evalResult{}, false
}
