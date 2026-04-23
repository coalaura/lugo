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
)

type DiagPragmas struct {
	FileDisabled map[string]bool
	LineDisabled map[uint32]map[string]bool
}

type Document struct {
	Server             *Server
	Tree               *ast.Tree
	Resolver           *semantic.Resolver
	TypeCache          []TypeSet
	Inferring          []bool
	LuaDocCache        []*LuaDoc
	ActualReads        []uint16
	ExportedGlobalDefs []ExportedSymbol
	Source             []byte
	Errors             []parser.ParseError
	URI                string
	Path               string
	LowerPath          string
	Dir                string
	ModuleName         string
	FiveMRoot          string
	ExportedNode       ast.NodeID
	FiveMEnv           FileEnv
	IsMeta             bool
	IsLibrary          bool
	IsWorkspace        bool
	IsFiveMManifest    bool
	FiveMLuaExports    []FiveMLuaExport
	FiveMResolved      bool
	EnvResolved        bool
	ModTime            time.Time
	DiagPragmas        DiagPragmas
}

func (doc *Document) parseDiagnosticPragmas() {
	doc.DiagPragmas.FileDisabled = make(map[string]bool)
	doc.DiagPragmas.LineDisabled = make(map[uint32]map[string]bool)

	for _, c := range doc.Tree.Comments {
		src := doc.Source[c.Start:c.End]

		idx := bytes.Index(src, []byte("---@diagnostic"))
		if idx == -1 {
			continue
		}

		rest := src[idx+14:]
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
		spaceIdx := bytes.IndexByte(rest, ' ')

		var rulesBytes []byte

		if spaceIdx != -1 {
			rulesBytes = rest[:spaceIdx]
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
			if grandParentID == ast.InvalidNode {
				return ast.InvalidNode
			}

			grandParentNode := doc.Tree.Nodes[grandParentID]
			if grandParentNode.Kind != ast.KindLocalAssign || grandParentNode.Right == ast.InvalidNode {
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
			if grandParentID == ast.InvalidNode {
				return ast.InvalidNode
			}

			grandParentNode := doc.Tree.Nodes[grandParentID]
			if grandParentNode.Kind != ast.KindAssign || grandParentNode.Left != parentID || grandParentNode.Right == ast.InvalidNode {
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

		name := ast.String(doc.Source[pNode.Start:pNode.End])

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
		stmtLine, _ := doc.Tree.Position(stmtStart)
		targetLine := stmtLine - 1

		idx := doc.findCommentIndex(stmtStart)

		for i := idx; i >= 0; i-- {
			c := doc.Tree.Comments[i]

			cStartLine, _ := doc.Tree.Position(c.Start)
			cEndLine, _ := doc.Tree.Position(c.End)

			if cEndLine == targetLine || cEndLine == stmtLine {
				if !yield(c) {
					return
				}

				targetLine = cStartLine - 1
			} else if cEndLine < targetLine {
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

								if !yield(doc.Source[identNode.Start:identNode.End], identID) {
									return
								}
							}
						}
					case ast.KindLocalFunction:
						identNode := doc.Tree.Nodes[stmtNode.Left]

						if !yield(doc.Source[identNode.Start:identNode.End], stmtNode.Left) {
							return
						}
					}
				}
			case ast.KindFunctionExpr, ast.KindFunctionStmt:
				var funcExpr ast.NodeID = curr

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

						if !yield(doc.Source[paramNode.Start:paramNode.End], paramID) {
							return
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

					if !yield(doc.Source[identNode.Start:identNode.End], node.Left) {
						return
					}
				}
			case ast.KindForIn:
				exprListID := ast.NodeID(node.Extra)
				if exprListID != ast.InvalidNode && offset > doc.Tree.Nodes[exprListID].End {
					nameList := doc.Tree.Nodes[node.Left]

					for i := uint16(0); i < nameList.Count; i++ {
						identID := doc.Tree.ExtraList[nameList.Extra+uint32(i)]
						identNode := doc.Tree.Nodes[identID]

						if !yield(doc.Source[identNode.Start:identNode.End], identID) {
							return
						}
					}
				}
			}

			curr = node.Parent
		}
	}
}

// ExtractLuaDocFields performs a highly optimized, zero-allocation byte scan
// for @field annotations in the comments directly above a node.
func (doc *Document) ExtractLuaDocFields(id ast.NodeID) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		fieldToken := []byte("@field")

		for c := range doc.IterateCommentsAbove(id) {
			raw := doc.Source[c.Start:c.End]

			idx := bytes.Index(raw, fieldToken)

			for idx != -1 {
				rest := raw[idx+6:]

				var j int

				for j < len(rest) && (rest[j] == ' ' || rest[j] == '\t') {
					j++
				}

				if bytes.HasPrefix(rest[j:], []byte("public ")) {
					j += 7
				} else if bytes.HasPrefix(rest[j:], []byte("private ")) {
					j += 8
				} else if bytes.HasPrefix(rest[j:], []byte("protected ")) {
					j += 10
				}

				for j < len(rest) && (rest[j] == ' ' || rest[j] == '\t') {
					j++
				}

				startName := j

				for j < len(rest) && rest[j] != ' ' && rest[j] != '\t' && rest[j] != '\n' && rest[j] != '\r' {
					j++
				}

				if j > startName {
					name := rest[startName:j]
					if len(name) > 0 && name[len(name)-1] == '?' {
						name = name[:len(name)-1]
					}

					if !yield(name) {
						return
					}
				}

				next := bytes.Index(rest, fieldToken)
				if next == -1 {
					break
				}

				idx += 6 + next
			}
		}
	}
}

// HasDeprecatedTag performs a fast, zero-allocation byte scan for @deprecated comments directly above a node.
func (doc *Document) HasDeprecatedTag(id ast.NodeID) (bool, string) {
	depToken := []byte("@deprecated")

	var (
		found bool
		msg   string
	)

	for c := range doc.IterateCommentsAbove(id) {
		raw := doc.Source[c.Start:c.End]

		_, after, ok := bytes.Cut(raw, depToken)
		if ok {
			rest := after

			endIdx := bytes.IndexByte(rest, '\n')
			if endIdx == -1 {
				endIdx = len(rest)
			}

			doc.Server.sharedDepBuf = doc.Server.sharedDepBuf[:0]

			msgBytes := cleanLuaCommentBytes(doc.Server.sharedDepBuf, rest[:endIdx])

			msg = string(bytes.TrimSpace(msgBytes))

			found = true

			doc.Server.sharedDepBuf = msgBytes

			break
		}
	}

	return found, msg
}

func cleanLuaCommentBytes(dst, raw []byte) []byte {
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

		line = bytes.TrimSpace(line)

		if bytes.HasPrefix(line, []byte("--[[")) {
			line = line[4:]
		} else if bytes.HasPrefix(line, []byte("---")) {
			line = line[3:]
		} else if bytes.HasPrefix(line, []byte("--")) {
			line = line[2:]
		}

		if bytes.HasSuffix(line, []byte("--]]")) {
			line = line[:len(line)-4]
		} else if bytes.HasSuffix(line, []byte("]]")) {
			line = line[:len(line)-2]
		}

		if len(line) > 0 && line[0] == ' ' {
			line = line[1:]
		}

		dst = append(dst, line...)

		if len(raw) > 0 || len(line) == 0 {
			dst = append(dst, '\n')
		}
	}

	return dst
}
