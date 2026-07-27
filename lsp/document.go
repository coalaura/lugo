package lsp

import (
	"bytes"
	"iter"
	"strings"
	"time"

	"github.com/coalaura/lugo/ast"
	"github.com/coalaura/lugo/parser"
	"github.com/coalaura/lugo/semantic"
	"github.com/coalaura/lugo/token"
	"github.com/coalaura/lugo/utils"
)

type DiagPragmas struct {
	FileDisabled map[string]bool
	LineDisabled map[uint32]map[string]bool
}

type Document struct {
	TypeCache          []TypeSet
	Inferring          []bool
	LuaDocCache        []*LuaDoc
	ActualReads        []uint16
	MutatedLocals      []bool
	ExportedGlobalDefs []ExportedSymbol
	Source             []byte
	Errors             []parser.ParseError
	FiveMLuaExports    []FiveMLuaExport
	ModTime            time.Time
	Version            uint64
	TypeCacheVersion   uint64
	URI                string
	Path               string
	LowerPath          string
	Dir                string
	ModuleName         string
	FiveMRoot          string
	DiagPragmas        DiagPragmas
	Server             *Server
	Tree               *ast.Tree
	Resolver           *semantic.Resolver
	ExportedNode       ast.NodeID
	FiveMEnv           FileEnv
	SafeFixCache       *SafeFixCacheEntry
	IsMeta             bool
	IsLibrary          bool
	IsWorkspace        bool
	IsFiveMManifest    bool
	FiveMResolved      bool
	EnvResolved        bool
}

func (doc *Document) parseDiagnosticPragmas() {
	doc.DiagPragmas.FileDisabled = make(map[string]bool)
	doc.DiagPragmas.LineDisabled = make(map[uint32]map[string]bool)

	for _, c := range doc.Tree.Comments {
		src := doc.Source[c.Start:c.End]

		_, after, ok := bytes.Cut(src, []byte("---@diagnostic"))
		if !ok {
			continue
		}

		rest := after
		rest = bytes.TrimLeft(rest, " \t")

		var action string

		if bytes.HasPrefix(rest, []byte("disable-line")) {
			action = "disable-line"
			rest = rest[12:]
		} else if bytes.HasPrefix(rest, []byte("disable-next-line")) {
			action = "disable-next-line"
			rest = rest[17:]
		} else if bytes.HasPrefix(rest, []byte("disable-file")) {
			action = "disable-file"
			rest = rest[12:]
		} else {
			continue
		}

		rest = bytes.TrimLeft(rest, " \t")
		before, _, ok := bytes.Cut(rest, []byte{' '})

		var rulesBytes []byte

		if ok {
			rulesBytes = before
		} else {
			rulesBytes = rest
		}

		rulesStr := string(bytes.TrimSpace(rulesBytes))
		rules := strings.SplitSeq(rulesStr, ",")

		line, _ := doc.Tree.Position(c.Start)

		if action == "disable-file" {
			for rule := range rules {
				doc.DiagPragmas.FileDisabled[strings.TrimSpace(rule)] = true
			}
		} else {
			targetLine := line
			if action == "disable-next-line" {
				targetLine = line + 1
			}

			if doc.DiagPragmas.LineDisabled[targetLine] == nil {
				doc.DiagPragmas.LineDisabled[targetLine] = make(map[string]bool)
			}

			for rule := range rules {
				doc.DiagPragmas.LineDisabled[targetLine][strings.TrimSpace(rule)] = true
			}
		}
	}
}

type FiveMLuaExport struct {
	Name   string
	NodeID ast.NodeID
}

func (doc *Document) getAssignedValue(id ast.NodeID) ast.NodeID {
	if id == ast.InvalidNode {
		return ast.InvalidNode
	}

	if uint(id) < uint(len(doc.Tree.Nodes)) {
		kind := doc.Tree.Nodes[id].Kind
		if kind == ast.KindFunctionExpr || kind == ast.KindTableExpr || kind == ast.KindString || kind == ast.KindNumber || kind == ast.KindTrue || kind == ast.KindFalse || kind == ast.KindNil {
			return id
		}
	}

	curr := id

	for {
		parentID := doc.Tree.Nodes[curr].Parent
		if parentID == ast.InvalidNode {
			return ast.InvalidNode
		}

		parent := doc.Tree.Nodes[parentID]

		switch parent.Kind {
		case ast.KindLocalFunction:
			if parent.Left == curr {
				return parent.Right // Return the FunctionExpr body
			}

			return ast.InvalidNode
		case ast.KindFunctionStmt:
			return parent.Right
		case ast.KindRecordField, ast.KindIndexField:
			if parent.Left == curr {
				return parent.Right
			}

			return ast.InvalidNode
		case ast.KindNameList:
			grandParentID := doc.Tree.Nodes[parentID].Parent
			if grandParentID == ast.InvalidNode || uint(grandParentID) >= uint(len(doc.Tree.Nodes)) {
				return ast.InvalidNode
			}

			grandParentNode := doc.Tree.Nodes[grandParentID]
			if grandParentNode.Kind != ast.KindLocalAssign || grandParentNode.Right == ast.InvalidNode || uint(grandParentNode.Right) >= uint(len(doc.Tree.Nodes)) {
				return ast.InvalidNode
			}

			idx := doc.Tree.IndexOfExtra(parentID, curr)
			if idx == -1 {
				return ast.InvalidNode
			}

			rhsNode := doc.Tree.Nodes[grandParentNode.Right]
			if uint16(idx) >= rhsNode.Count {
				return ast.InvalidNode
			}

			return doc.Tree.ExtraList[rhsNode.Extra+uint32(idx)]
		case ast.KindExprList:
			grandParentID := doc.Tree.Nodes[parentID].Parent
			if grandParentID == ast.InvalidNode || uint(grandParentID) >= uint(len(doc.Tree.Nodes)) {
				return ast.InvalidNode
			}

			grandParentNode := doc.Tree.Nodes[grandParentID]
			if grandParentNode.Kind != ast.KindAssign || grandParentNode.Left != parentID || grandParentNode.Right == ast.InvalidNode || uint(grandParentNode.Right) >= uint(len(doc.Tree.Nodes)) {
				return ast.InvalidNode
			}

			idx := doc.Tree.IndexOfExtra(parentID, curr)
			if idx == -1 {
				return ast.InvalidNode
			}

			rhsNode := doc.Tree.Nodes[grandParentNode.Right]
			if uint16(idx) >= rhsNode.Count {
				return ast.InvalidNode
			}

			return doc.Tree.ExtraList[rhsNode.Extra+uint32(idx)]
		case ast.KindMemberExpr, ast.KindMethodName:
			curr = parentID
		default:
			return ast.InvalidNode
		}
	}
}

func (doc *Document) getFunctionParams(funcExprID ast.NodeID, luadoc *LuaDoc) string {
	node := doc.Tree.Nodes[funcExprID]
	if node.Kind != ast.KindFunctionExpr {
		return ""
	}

	if luadoc != nil && node.Count == 0 && len(luadoc.Params) > 0 {
		var params []string

		for _, p := range luadoc.Params {
			if p.Type != "" {
				params = append(params, p.Name+": "+p.Type)
			} else {
				params = append(params, p.Name)
			}
		}

		return strings.Join(params, ", ")
	}

	paramTypes := make(map[string]string)

	if luadoc != nil {
		for _, p := range luadoc.Params {
			paramTypes[p.Name] = p.Type
		}
	}

	var params []string

	for i := uint16(0); i < node.Count; i++ {
		pID := doc.Tree.ExtraList[node.Extra+uint32(i)]
		pNode := doc.Tree.Nodes[pID]

		name := utils.String(doc.Source[pNode.Start:pNode.End])

		if typ, ok := paramTypes[name]; ok && typ != "" {
			params = append(params, name+": "+typ)
		} else {
			params = append(params, name)
		}
	}

	return strings.Join(params, ", ")
}

func (doc *Document) findCommentIndex(offset uint32) int {
	var (
		low  int
		high = len(doc.Tree.Comments)
	)

	for low < high {
		mid := int(uint(low+high) >> 1)

		if doc.Tree.Comments[mid].End <= offset {
			low = mid + 1
		} else {
			high = mid
		}
	}

	return low - 1
}

// IterateCommentsAbove finds the contiguous block of comments directly above an AST node
// and yields each comment in reverse order (bottom-up).
func (doc *Document) IterateCommentsAbove(id ast.NodeID) iter.Seq[token.Token] {
	return func(yield func(token.Token) bool) {
		if id == ast.InvalidNode {
			return
		}

		stmtID := id

		for {
			parentID := doc.Tree.Nodes[stmtID].Parent
			if parentID == ast.InvalidNode {
				break
			}

			pKind := doc.Tree.Nodes[parentID].Kind
			if pKind == ast.KindBlock || pKind == ast.KindFile || pKind == ast.KindTableExpr {
				break
			}

			stmtID = parentID
		}

		stmtStart := doc.Tree.Nodes[stmtID].Start
		idx := doc.findCommentIndex(stmtStart)

		lastValidOffset := stmtStart

		for i := idx; i >= 0; i-- {
			c := doc.Tree.Comments[i]

			gap := doc.Source[c.End:lastValidOffset]

			if bytes.Count(gap, []byte{'\n'}) <= 1 {
				if !yield(c) {
					return
				}

				lastValidOffset = c.Start
			} else {
				break
			}
		}
	}
}

func (doc *Document) getCommentsAbove(id ast.NodeID) []byte {
	var validComments []token.Token

	for c := range doc.IterateCommentsAbove(id) {
		validComments = append(validComments, c)
	}

	if len(validComments) == 0 {
		return nil
	}

	doc.Server.sharedCommentBuf = doc.Server.sharedCommentBuf[:0]

	b := doc.Server.sharedCommentBuf

	for i := len(validComments) - 1; i >= 0; i-- {
		c := validComments[i]
		rawC := doc.Source[c.Start:c.End]

		b = cleanLuaCommentBytes(b, rawC)

		if i > 0 && len(b) > 0 && b[len(b)-1] != '\n' {
			b = append(b, '\n')
		}
	}

	doc.Server.sharedCommentBuf = b

	return bytes.TrimSpace(b)
}

// GetLuaDoc parses and caches the LuaDoc for a given node.
func (doc *Document) GetLuaDoc(id ast.NodeID) *LuaDoc {
	if id == ast.InvalidNode || int(id) >= len(doc.Tree.Nodes) {
		return nil
	}

	if int(id) >= len(doc.LuaDocCache) {
		if int(id) < cap(doc.LuaDocCache) {
			doc.LuaDocCache = doc.LuaDocCache[:len(doc.Tree.Nodes)]
		} else {
			newCache := make([]*LuaDoc, len(doc.Tree.Nodes))

			copy(newCache, doc.LuaDocCache)

			doc.LuaDocCache = newCache
		}
	}

	if ld := doc.LuaDocCache[id]; ld != nil {
		return ld
	}

	enableAlerts := doc.Server != nil && doc.Server.FeatureFormatAlerts

	comments := doc.getCommentsAbove(id)

	var ld LuaDoc

	if len(comments) > 0 {
		ld = parseLuaDoc(comments, enableAlerts)
	}

	doc.LuaDocCache[id] = &ld

	return &ld
}

// LocalsAt walks up the AST from the given offset and yields every local variable in scope.
func (doc *Document) LocalsAt(offset uint32) iter.Seq2[[]byte, ast.NodeID] {
	return func(yield func([]byte, ast.NodeID) bool) {
		nodeID := doc.Tree.NodeAt(offset)
		if nodeID == ast.InvalidNode {
			return
		}

		curr := nodeID

		for curr != ast.InvalidNode {
			node := doc.Tree.Nodes[curr]

			switch node.Kind {
			case ast.KindBlock, ast.KindFile:
				// Binary search for the active statement
				low, high := 0, int(node.Count)

				for low < high {
					mid := int(uint(low+high) >> 1)
					stmtID := doc.Tree.ExtraList[node.Extra+uint32(mid)]

					if doc.Tree.Nodes[stmtID].Start >= offset {
						high = mid
					} else {
						low = mid + 1
					}
				}

				lastStmtIdx := low - 1

				for i := lastStmtIdx; i >= 0; i-- {
					stmtID := doc.Tree.ExtraList[node.Extra+uint32(i)]
					stmtNode := doc.Tree.Nodes[stmtID]

					switch stmtNode.Kind {
					case ast.KindLocalAssign:
						if offset >= stmtNode.End {
							nameList := doc.Tree.Nodes[stmtNode.Left]

							// Iterate backwards to support `local a, a = 1, 2`
							for j := int(nameList.Count) - 1; j >= 0; j-- {
								identID := doc.Tree.ExtraList[nameList.Extra+uint32(j)]
								identNode := doc.Tree.Nodes[identID]

								if identNode.Start <= identNode.End && identNode.End <= uint32(len(doc.Source)) {
									if !yield(doc.Source[identNode.Start:identNode.End], identID) {
										return
									}
								}
							}
						}
					case ast.KindLocalFunction:
						identNode := doc.Tree.Nodes[stmtNode.Left]

						if identNode.Start <= identNode.End && identNode.End <= uint32(len(doc.Source)) {
							if !yield(doc.Source[identNode.Start:identNode.End], stmtNode.Left) {
								return
							}
						}
					}
				}
			case ast.KindFunctionExpr, ast.KindFunctionStmt:
				funcExpr := curr

				if node.Kind == ast.KindFunctionStmt {
					funcExpr = node.Right

					if int(node.Left) < len(doc.Tree.Nodes) && doc.Tree.Nodes[node.Left].Kind == ast.KindMethodName {
						if !yield([]byte("self"), node.Left) {
							return
						}
					}
				}

				if funcExpr != ast.InvalidNode {
					exprNode := doc.Tree.Nodes[funcExpr]

					for i := uint16(0); i < exprNode.Count; i++ {
						paramID := doc.Tree.ExtraList[exprNode.Extra+uint32(i)]
						paramNode := doc.Tree.Nodes[paramID]

						if paramNode.Start <= paramNode.End && paramNode.End <= uint32(len(doc.Source)) {
							if !yield(doc.Source[paramNode.Start:paramNode.End], paramID) {
								return
							}
						}
					}
				}
			case ast.KindForNum:
				var exprsEnd uint32

				if node.Count > 0 {
					lastExprID := doc.Tree.ExtraList[node.Extra+uint32(node.Count-1)]
					exprsEnd = doc.Tree.Nodes[lastExprID].End
				} else {
					exprsEnd = doc.Tree.Nodes[node.Left].End
				}

				if offset > exprsEnd {
					identNode := doc.Tree.Nodes[node.Left]

					if identNode.Start <= identNode.End && identNode.End <= uint32(len(doc.Source)) {
						if !yield(doc.Source[identNode.Start:identNode.End], node.Left) {
							return
						}
					}
				}
			case ast.KindForIn:
				exprListID := ast.NodeID(node.Extra)
				if exprListID != ast.InvalidNode && offset > doc.Tree.Nodes[exprListID].End {
					nameList := doc.Tree.Nodes[node.Left]

					for i := uint16(0); i < nameList.Count; i++ {
						identID := doc.Tree.ExtraList[nameList.Extra+uint32(i)]
						identNode := doc.Tree.Nodes[identID]

						if identNode.Start <= identNode.End && identNode.End <= uint32(len(doc.Source)) {
							if !yield(doc.Source[identNode.Start:identNode.End], identID) {
								return
							}
						}
					}
				}
			}

			curr = node.Parent
		}
	}
}

// ExtractLuaDocFields yields @field names from the cached LuaDoc for a node.
func (doc *Document) ExtractLuaDocFields(id ast.NodeID) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		ld := doc.GetLuaDoc(id)
		if ld == nil {
			return
		}

		for i := range ld.Fields {
			if !yield([]byte(ld.Fields[i].Name)) {
				return
			}
		}
	}
}

// HasDeprecatedTag reports @deprecated state from the cached LuaDoc for a node.
func (doc *Document) HasDeprecatedTag(id ast.NodeID) (bool, string) {
	ld := doc.GetLuaDoc(id)
	if ld == nil {
		return false, ""
	}

	return ld.IsDeprecated, ld.DeprecatedMsg
}

func cleanLuaCommentBytes(dst, raw []byte) []byte {
	isBlock := bytes.HasPrefix(raw, []byte("--["))

	var (
		blockPrefixLen int
		blockSuffixLen int
	)

	if isBlock {
		blockPrefixLen = 3

		for blockPrefixLen < len(raw) && raw[blockPrefixLen] == '=' {
			blockPrefixLen++
		}

		if blockPrefixLen < len(raw) && raw[blockPrefixLen] == '[' {
			blockPrefixLen++
		} else {
			isBlock = false
		}

		if isBlock {
			blockSuffixLen = blockPrefixLen - 2
		}
	}

	minIndent := -1

	if isBlock {
		temp := raw

		var lineNum int

		for len(temp) > 0 {
			var line []byte

			idx := bytes.IndexByte(temp, '\n')
			if idx == -1 {
				line = temp
				temp = nil
			} else {
				line = temp[:idx]
				temp = temp[idx+1:]
			}

			lineNum++
			if lineNum == 1 {
				continue
			}

			line = bytes.TrimRight(line, " \t\r")
			if len(line) == 0 {
				continue
			}

			var indent int

			for indent < len(line) && (line[indent] == ' ' || line[indent] == '\t') {
				indent++
			}

			if minIndent == -1 || indent < minIndent {
				minIndent = indent
			}
		}

		if minIndent == -1 {
			minIndent = 0
		}
	}

	var lineNum int

	for len(raw) > 0 {
		var line []byte

		idx := bytes.IndexByte(raw, '\n')
		if idx == -1 {
			line = raw
			raw = nil
		} else {
			line = raw[:idx]
			raw = raw[idx+1:]
		}

		lineNum++
		line = bytes.TrimRight(line, " \t\r")

		if isBlock {
			if lineNum == 1 {
				if len(line) >= blockPrefixLen {
					line = line[blockPrefixLen:]
				}
			} else {
				var strip int

				for strip < len(line) && strip < minIndent && (line[strip] == ' ' || line[strip] == '\t') {
					strip++
				}

				line = line[strip:]
			}

			if len(raw) == 0 {
				if bytes.HasSuffix(line, []byte("--]]")) && blockSuffixLen == 2 {
					line = line[:len(line)-4]
				} else if len(line) >= blockSuffixLen {
					suffix := line[len(line)-blockSuffixLen:]
					if suffix[0] == ']' && suffix[len(suffix)-1] == ']' {
						valid := true

						for k := 1; k < len(suffix)-1; k++ {
							if suffix[k] != '=' {
								valid = false

								break
							}
						}

						if valid && len(suffix) == blockSuffixLen {
							line = line[:len(line)-blockSuffixLen]
						}
					}
				}
			}
		} else {
			line = bytes.TrimSpace(line)

			if bytes.HasPrefix(line, []byte("---")) {
				line = line[3:]
			} else if bytes.HasPrefix(line, []byte("--")) {
				line = line[2:]
			}

			if len(line) > 0 && line[0] == ' ' {
				line = line[1:]
			}
		}

		dst = append(dst, line...)

		if len(raw) > 0 || len(line) == 0 {
			dst = append(dst, '\n')
		}
	}

	return dst
}
