package lsp

import (
	"bytes"
	"strings"

	"github.com/coalaura/lugo/ast"
)

func setCfg[T comparable](dst *T, value T, flag *bool) {
	if *dst == value {
		return
	}

	*dst = value

	if flag != nil {
		*flag = true
	}
}

func mapsEqualStringBool(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}

	for k, av := range a {
		if bv, ok := b[k]; !ok || bv != av {
			return false
		}
	}

	return true
}

// Fast, zero-allocation Levenshtein distance for strings up to 63 bytes.
func levenshteinFast(str1, str2 string, maxDist int) int {
	if len(str1) > len(str2) {
		str1, str2 = str2, str1
	}

	len1 := len(str1)
	len2 := len(str2)

	if len1 == 0 {
		return len2
	}

	if len2-len1 > maxDist || len2 > 63 {
		return maxDist + 1
	}

	var (
		row0 [64]int
		row1 [64]int
	)

	prevRow, currRow := &row0, &row1

	for i := 0; i <= len1; i++ {
		prevRow[i] = i
	}

	for i := range len2 {
		currRow[0] = i + 1

		minDistForRow := currRow[0]

		for j := range len1 {
			cost := 1

			if str2[i] == str1[j] {
				cost = 0
			}

			delCost := currRow[j] + 1
			insCost := prevRow[j+1] + 1
			subCost := prevRow[j] + cost

			minCost := min(subCost, min(insCost, delCost))

			currRow[j+1] = minCost

			if minCost < minDistForRow {
				minDistForRow = minCost
			}
		}

		if minDistForRow > maxDist {
			return maxDist + 1
		}

		prevRow, currRow = currRow, prevRow
	}

	return prevRow[len1]
}

func containsFold(text, queryLower []byte) bool {
	if len(queryLower) == 0 {
		return true
	}

	if len(text) < len(queryLower) {
		return false
	}

	for i := 0; i <= len(text)-len(queryLower); i++ {
		match := true

		for j := range queryLower {
			charText := text[i+j]

			if charText >= 'A' && charText <= 'Z' {
				charText += 32 // fast to-lower
			}

			charQuery := queryLower[j]

			if charText != charQuery {
				if (charText == '.' && charQuery == ':') || (charText == ':' && charQuery == '.') {
					continue
				}

				match = false

				break
			}
		}

		if match {
			return true
		}
	}

	return false
}

func hasPrefixFold(text, prefix []byte) bool {
	if len(text) < len(prefix) {
		return false
	}

	for i, charPrefix := range prefix {
		charText := text[i]

		if charText >= 'A' && charText <= 'Z' {
			charText += 32 // fast to-lower
		}

		if charText != charPrefix {
			return false
		}
	}

	return true
}

func trimTrailingWhitespace(text string) string {
	var out strings.Builder

	lines := strings.Split(text, "\n")

	for i, line := range lines {
		out.WriteString(strings.TrimRight(line, " \t\r"))

		if i < len(lines)-1 {
			out.WriteString("\n")
		}
	}

	return out.String()
}

func reindentNodeText(doc *Document, id ast.NodeID, targetIndent string) string {
	if id == ast.InvalidNode || int(id) >= len(doc.Tree.Nodes) {
		return ""
	}

	node := doc.Tree.Nodes[id]
	if node.Start >= node.End || node.End > uint32(len(doc.Source)) {
		return ""
	}

	text := string(doc.Source[node.Start:node.End])

	var baseIndent string

	for i := int(node.Start) - 1; i >= 0; i-- {
		c := doc.Source[i]
		if c == '\n' {
			baseIndent = string(doc.Source[i+1 : node.Start])

			break
		} else if c != ' ' && c != '\t' {
			baseIndent = ""

			break
		}
	}

	if len(baseIndent) == 0 && node.Start > 0 {
		for i := 0; i < int(node.Start); i++ {
			c := doc.Source[i]

			if c == ' ' || c == '\t' {
				baseIndent += string(c)
			} else {
				baseIndent = ""
			}
		}
	}

	var out strings.Builder

	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			out.WriteString("\n")
			out.WriteString(targetIndent)

			if strings.HasPrefix(line, baseIndent) {
				out.WriteString(line[len(baseIndent):])
			} else {
				out.WriteString(strings.TrimLeft(line, " \t"))
			}
		} else {
			out.WriteString(line)
		}
	}

	return out.String()
}

func matchGlob(pattern, path string) bool {
	pLen, tLen := len(pattern), len(path)

	var (
		p int
		t int
	)

	for p < pLen {
		if pattern[p] == '*' {
			isDouble := p+1 < pLen && pattern[p+1] == '*'
			if isDouble {
				p += 2

				if p == pLen {
					return true
				}

				for i := t; i <= tLen; i++ {
					if matchGlob(pattern[p:], path[i:]) {
						return true
					}
				}

				return false
			}

			p++

			if p == pLen {
				return strings.IndexByte(path[t:], '/') == -1 && strings.IndexByte(path[t:], '\\') == -1
			}

			for i := t; i <= tLen; i++ {
				if i > t && (path[i-1] == '/' || path[i-1] == '\\') {
					break
				}

				if matchGlob(pattern[p:], path[i:]) {
					return true
				}
			}

			return false
		} else if pattern[p] == '?' {
			if t == tLen {
				return false
			}

			p++
			t++
		} else {
			if t == tLen {
				return false
			}

			charPattern := pattern[p]
			charPath := path[t]

			if charPattern >= 'A' && charPattern <= 'Z' {
				charPattern += 32
			}

			if charPath >= 'A' && charPath <= 'Z' {
				charPath += 32
			}

			if charPattern == '\\' {
				charPattern = '/'
			}

			if charPath == '\\' {
				charPath = '/'
			}

			if charPattern != charPath {
				return false
			}

			p++
			t++
		}
	}

	return t == tLen
}

func getImplicitSelfOffset(callNode ast.Node, targetDoc *Document, defID ast.NodeID) int {
	if targetDoc == nil || defID == ast.InvalidNode || int(defID) >= len(targetDoc.Tree.Nodes) {
		return 0
	}

	hasImplicitSelfCall := callNode.Kind == ast.KindMethodCall

	var hasImplicitSelfDef bool

	pDefID := targetDoc.Tree.Nodes[defID].Parent
	if pDefID != ast.InvalidNode && int(pDefID) < len(targetDoc.Tree.Nodes) {
		if targetDoc.Tree.Nodes[pDefID].Kind == ast.KindMethodName {
			hasImplicitSelfDef = true
		}
	}

	if hasImplicitSelfCall && !hasImplicitSelfDef {
		return 1
	} else if !hasImplicitSelfCall && hasImplicitSelfDef {
		return -1
	}

	return 0
}

func getRange(tree *ast.Tree, start, end uint32) Range {
	sLine, sCol := tree.Position(start)
	eLine, eCol := tree.Position(end)

	return Range{
		Start: Position{Line: sLine, Character: sCol},
		End:   Position{Line: eLine, Character: eCol},
	}
}

func getNodeRange(tree *ast.Tree, nodeID ast.NodeID) Range {
	if nodeID == ast.InvalidNode || int(nodeID) >= len(tree.Nodes) {
		return Range{}
	}

	node := tree.Nodes[nodeID]

	return getRange(tree, node.Start, node.End)
}

func getASTDepth(tree *ast.Tree, id ast.NodeID) int {
	var depth int

	curr := tree.Nodes[id].Parent

	for curr != ast.InvalidNode {
		depth++

		curr = tree.Nodes[curr].Parent
	}

	return depth
}

func isRootLevel(tree *ast.Tree, id ast.NodeID) bool {
	curr := tree.Nodes[id].Parent

	for curr != ast.InvalidNode {
		n := tree.Nodes[curr]

		if n.Kind == ast.KindBlock {
			parentID := n.Parent
			if parentID == tree.Root {
				return true
			}

			pNode := tree.Nodes[parentID]
			if pNode.Kind == ast.KindIf || pNode.Kind == ast.KindElseIf || pNode.Kind == ast.KindElse || pNode.Kind == ast.KindDo {
				curr = parentID

				continue
			}

			return false
		}

		curr = n.Parent
	}

	return false
}

func isLoopVariable(tree *ast.Tree, defID ast.NodeID) bool {
	if defID == ast.InvalidNode || int(defID) >= len(tree.Nodes) {
		return false
	}

	parentID := tree.Nodes[defID].Parent
	if parentID == ast.InvalidNode || int(parentID) >= len(tree.Nodes) {
		return false
	}

	parentNode := tree.Nodes[parentID]
	if parentNode.Kind == ast.KindForNum && parentNode.Left == defID {
		return true
	} else if parentNode.Kind == ast.KindNameList {
		grandParentID := parentNode.Parent
		if grandParentID != ast.InvalidNode && int(grandParentID) < len(tree.Nodes) {
			if tree.Nodes[grandParentID].Kind == ast.KindForIn {
				return true
			}
		}
	}

	return false
}

func isWriteAccess(tree *ast.Tree, nodeID ast.NodeID) bool {
	if nodeID == ast.InvalidNode || int(nodeID) >= len(tree.Nodes) {
		return false
	}

	parentID := tree.Nodes[nodeID].Parent
	if parentID == ast.InvalidNode || int(parentID) >= len(tree.Nodes) {
		return false
	}

	parentNode := tree.Nodes[parentID]

	switch parentNode.Kind {
	case ast.KindNameList:
		grandParentID := parentNode.Parent
		if grandParentID != ast.InvalidNode && int(grandParentID) < len(tree.Nodes) {
			grandParentNode := tree.Nodes[grandParentID]

			return grandParentNode.Kind == ast.KindLocalAssign || grandParentNode.Kind == ast.KindForIn
		}
	case ast.KindExprList:
		grandParentID := parentNode.Parent
		if grandParentID != ast.InvalidNode && int(grandParentID) < len(tree.Nodes) {
			grandParentNode := tree.Nodes[grandParentID]

			return grandParentNode.Kind == ast.KindAssign && grandParentNode.Left == parentID
		}
	case ast.KindForNum, ast.KindLocalFunction, ast.KindFunctionStmt, ast.KindRecordField:
		return parentNode.Left == nodeID
	case ast.KindMethodName:
		return parentNode.Right == nodeID
	case ast.KindMemberExpr:
		if parentNode.Right == nodeID {
			grandParentID := parentNode.Parent
			if grandParentID != ast.InvalidNode && int(grandParentID) < len(tree.Nodes) {
				grandParentNode := tree.Nodes[grandParentID]
				if grandParentNode.Kind == ast.KindExprList {
					greatGrandParentID := grandParentNode.Parent
					if greatGrandParentID != ast.InvalidNode && int(greatGrandParentID) < len(tree.Nodes) {
						greatGrandParentNode := tree.Nodes[greatGrandParentID]

						return greatGrandParentNode.Kind == ast.KindAssign && greatGrandParentNode.Left == grandParentID
					}
				}
			}
		}
	}

	return false
}

func isLHSOfAssignment(doc *Document, nodeID ast.NodeID) bool {
	if nodeID == ast.InvalidNode || int(nodeID) >= len(doc.Tree.Nodes) {
		return false
	}

	parentID := doc.Tree.Nodes[nodeID].Parent
	if parentID == ast.InvalidNode || int(parentID) >= len(doc.Tree.Nodes) {
		return false
	}

	parentNode := doc.Tree.Nodes[parentID]
	if parentNode.Kind == ast.KindExprList {
		grandParentID := parentNode.Parent
		if grandParentID != ast.InvalidNode && int(grandParentID) < len(doc.Tree.Nodes) {
			grandParentNode := doc.Tree.Nodes[grandParentID]
			if (grandParentNode.Kind == ast.KindAssign || grandParentNode.Kind == ast.KindLocalAssign) && grandParentNode.Left == parentID {
				return true
			}
		}
	}

	return false
}

func isTerminal(tree *ast.Tree, id ast.NodeID) bool {
	if id == ast.InvalidNode || int(id) >= len(tree.Nodes) {
		return false
	}

	node := tree.Nodes[id]

	switch node.Kind {
	case ast.KindReturn, ast.KindBreak, ast.KindGoto:
		return true
	case ast.KindDo:
		return isTerminal(tree, node.Left)
	case ast.KindBlock:
		for i := uint16(0); i < node.Count; i++ {
			if node.Extra+uint32(i) < uint32(len(tree.ExtraList)) {
				if isTerminal(tree, tree.ExtraList[node.Extra+uint32(i)]) {
					return true
				}
			}
		}

		return false
	case ast.KindIf:
		if !isTerminal(tree, node.Right) {
			return false
		}

		var (
			hasElse      bool
			isExhaustive bool
		)

		for i := uint16(0); i < node.Count; i++ {
			if node.Extra+uint32(i) >= uint32(len(tree.ExtraList)) {
				continue
			}

			childID := tree.ExtraList[node.Extra+uint32(i)]
			if childID == ast.InvalidNode || int(childID) >= len(tree.Nodes) {
				continue
			}

			childNode := tree.Nodes[childID]

			switch childNode.Kind {
			case ast.KindElseIf:
				if !isTerminal(tree, childNode.Right) {
					return false
				}

				if !isExhaustive && isOppositeCondition(tree, node.Left, childNode.Left) {
					isExhaustive = true
				}
			case ast.KindElse:
				hasElse = true

				if !isTerminal(tree, childNode.Left) {
					return false
				}
			}
		}

		return hasElse || isExhaustive
	}

	return false
}

func isOppositeCondition(tree *ast.Tree, a, b ast.NodeID) bool {
	if a == ast.InvalidNode || b == ast.InvalidNode {
		return false
	}

	checkNot := func(n, other ast.NodeID) bool {
		node := tree.Nodes[n]
		if node.Kind == ast.KindUnaryExpr {
			src := tree.Source[node.Start:node.End]
			if bytes.HasPrefix(src, []byte("not")) {
				right := tree.Nodes[node.Right]

				rightSrc := bytes.TrimSpace(tree.Source[right.Start:right.End])
				otherSrc := bytes.TrimSpace(tree.Source[tree.Nodes[other].Start:tree.Nodes[other].End])

				// Strip optional parentheses for comparison
				if bytes.HasPrefix(rightSrc, []byte("(")) && bytes.HasSuffix(rightSrc, []byte(")")) {
					rightSrc = bytes.TrimSpace(rightSrc[1 : len(rightSrc)-1])
				}

				if bytes.HasPrefix(otherSrc, []byte("(")) && bytes.HasSuffix(otherSrc, []byte(")")) {
					otherSrc = bytes.TrimSpace(otherSrc[1 : len(otherSrc)-1])
				}

				return bytes.Equal(rightSrc, otherSrc)
			}
		}

		return false
	}

	return checkNot(a, b) || checkNot(b, a)
}
