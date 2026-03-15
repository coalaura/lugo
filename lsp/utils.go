package lsp

import (
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
func levenshteinFast(s, t string, maxDist int) int {
	if len(s) > len(t) {
		s, t = t, s
	}

	ls := len(s)
	lt := len(t)

	if ls == 0 {
		return lt
	}

	if lt-ls > maxDist || lt > 63 {
		return maxDist + 1
	}

	var (
		v0 [64]int
		v1 [64]int
	)

	p0, p1 := &v0, &v1

	for i := 0; i <= ls; i++ {
		p0[i] = i
	}

	for i := range lt {
		p1[0] = i + 1

		minDistForRow := p1[0]

		for j := range ls {
			cost := 1

			if t[i] == s[j] {
				cost = 0
			}

			a := p1[j] + 1
			b := p0[j+1] + 1
			c := p0[j] + cost

			m := min(c, min(b, a))

			p1[j+1] = m

			if m < minDistForRow {
				minDistForRow = m
			}
		}

		if minDistForRow > maxDist {
			return maxDist + 1
		}

		p0, p1 = p1, p0
	}

	return p0[ls]
}

func containsFold(b, queryLower []byte) bool {
	if len(queryLower) == 0 {
		return true
	}

	if len(b) < len(queryLower) {
		return false
	}

	for i := 0; i <= len(b)-len(queryLower); i++ {
		match := true

		for j := range queryLower {
			cb := b[i+j]

			if cb >= 'A' && cb <= 'Z' {
				cb += 32 // fast to-lower
			}

			qb := queryLower[j]

			if cb != qb {
				if (cb == '.' && qb == ':') || (cb == ':' && qb == '.') {
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

func hasPrefixFold(b, prefix []byte) bool {
	if len(b) < len(prefix) {
		return false
	}

	for i, pb := range prefix {
		cb := b[i]

		if cb >= 'A' && cb <= 'Z' {
			cb += 32 // fast to-lower
		}

		if cb != pb {
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

func isWriteAccess(tree *ast.Tree, nodeID ast.NodeID) bool {
	if nodeID == ast.InvalidNode || int(nodeID) >= len(tree.Nodes) {
		return false
	}

	pID := tree.Nodes[nodeID].Parent
	if pID == ast.InvalidNode || int(pID) >= len(tree.Nodes) {
		return false
	}

	pNode := tree.Nodes[pID]

	switch pNode.Kind {
	case ast.KindNameList:
		gpID := pNode.Parent
		if gpID != ast.InvalidNode && int(gpID) < len(tree.Nodes) {
			gpNode := tree.Nodes[gpID]

			return gpNode.Kind == ast.KindLocalAssign || gpNode.Kind == ast.KindForIn
		}
	case ast.KindExprList:
		gpID := pNode.Parent
		if gpID != ast.InvalidNode && int(gpID) < len(tree.Nodes) {
			gpNode := tree.Nodes[gpID]

			return gpNode.Kind == ast.KindAssign && gpNode.Left == pID
		}
	case ast.KindForNum, ast.KindLocalFunction, ast.KindFunctionStmt, ast.KindRecordField:
		return pNode.Left == nodeID
	case ast.KindMethodName:
		return pNode.Right == nodeID
	case ast.KindMemberExpr:
		if pNode.Right == nodeID {
			gpID := pNode.Parent
			if gpID != ast.InvalidNode && int(gpID) < len(tree.Nodes) {
				gpNode := tree.Nodes[gpID]
				if gpNode.Kind == ast.KindExprList {
					ggpID := gpNode.Parent
					if ggpID != ast.InvalidNode && int(ggpID) < len(tree.Nodes) {
						ggpNode := tree.Nodes[ggpID]

						return ggpNode.Kind == ast.KindAssign && ggpNode.Left == gpID
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

	pID := doc.Tree.Nodes[nodeID].Parent
	if pID == ast.InvalidNode || int(pID) >= len(doc.Tree.Nodes) {
		return false
	}

	pNode := doc.Tree.Nodes[pID]
	if pNode.Kind == ast.KindExprList {
		gpID := pNode.Parent
		if gpID != ast.InvalidNode && int(gpID) < len(doc.Tree.Nodes) {
			gpNode := doc.Tree.Nodes[gpID]
			if (gpNode.Kind == ast.KindAssign || gpNode.Kind == ast.KindLocalAssign) && gpNode.Left == pID {
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

		var hasElse bool

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
			case ast.KindElse:
				hasElse = true

				if !isTerminal(tree, childNode.Left) {
					return false
				}
			}
		}

		return hasElse
	}

	return false
}
