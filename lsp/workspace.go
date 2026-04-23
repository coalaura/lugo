package lsp

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/coalaura/lugo/ast"
	"github.com/coalaura/lugo/parser"
	"github.com/coalaura/lugo/semantic"
)

type IgnorePattern struct {
	MatchFallback string
	HasSuffix     string
	HasPrefix     string
	ContainsPath  string
	SuffixPath    string
}

func (s *Server) handleDidOpen(req Request) {
	var params DidOpenTextDocumentParams

	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		return
	}

	uri := s.normalizeURI(params.TextDocument.URI)

	if s.isIgnoredURI(uri) {
		return
	}

	s.OpenFiles[uri] = true

	needsRepublish := s.updateDocument(uri, []byte(params.TextDocument.Text))

	if needsRepublish {
		s.publishWorkspaceDiagnostics()
	} else {
		s.publishDiagnostics(uri)
	}

	s.Log.Debugf("Opened document: %s\n", uri)
}

func (s *Server) handleDidChange(req Request) {
	var params DidChangeTextDocumentParams

	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		return
	}

	uri := s.normalizeURI(params.TextDocument.URI)

	if s.isIgnoredURI(uri) {
		if _, ok := s.Documents[uri]; ok {
			s.clearDocument(uri)
		}

		return
	}

	if len(params.ContentChanges) > 0 {
		needsRepublish := s.updateDocument(uri, []byte(params.ContentChanges[0].Text))

		if needsRepublish {
			s.publishWorkspaceDiagnostics()
		} else {
			s.publishDiagnostics(uri)
		}

		s.Log.Debugf("Updated document: %s\n", uri)
	}
}

func (s *Server) handleDidClose(req Request) {
	var params DidCloseTextDocumentParams

	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		return
	}

	uri := s.normalizeURI(params.TextDocument.URI)

	delete(s.OpenFiles, uri)

	path := s.uriToPath(uri)
	if path != "" {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			s.clearDocument(uri)
		}
	} else if strings.HasPrefix(uri, "untitled:") {
		s.clearDocument(uri)
	}

	s.Log.Debugf("Closed document: %s\n", uri)
}

func (s *Server) handleDidChangeConfiguration(req Request) {
	var params DidChangeConfigurationParams

	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		return
	}

	needsReindex, needsRepublish := s.applyInitializationOptions(params.Settings)

	if needsReindex {
		s.refreshWorkspace()
	} else if needsRepublish {
		s.publishWorkspaceDiagnostics()
	}
}

func (s *Server) handleDidChangeWatchedFiles(req Request) {
	var params DidChangeWatchedFilesParams

	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		return
	}

	for _, change := range params.Changes {
		uri := s.normalizeURI(change.URI)

		if s.isIgnoredURI(uri) {
			continue
		}

		switch change.Type {
		case 1, 2: // Created, Changed
			if !s.OpenFiles[uri] {
				path := s.uriToPath(uri)

				stat, statErr := os.Stat(path)

				if b, err := os.ReadFile(path); err == nil {
					needsRepublish := s.updateDocument(uri, b)

					if statErr == nil && s.Documents[uri] != nil {
						s.Documents[uri].ModTime = stat.ModTime()
					}

					if s.isWorkspaceURI(uri) {
						if needsRepublish {
							s.publishWorkspaceDiagnostics()
						} else {
							s.publishDiagnostics(uri)
						}
					}
				}
			}
		case 3: // Deleted
			s.clearDocument(uri)
		}
	}
}

func (s *Server) handleReindex(req Request) {
	s.refreshWorkspace()

	WriteMessage(s.Writer, Response{RPC: "2.0", ID: req.ID, Result: "ok"})
}

func (s *Server) handleReadStd(req Request) {
	var params ReadStdParams

	err := json.Unmarshal(req.Params, &params)
	if err != nil {
		return
	}

	var content string

	filename := params.URI

	if strings.HasPrefix(filename, "std:///") {
		filename = filename[7:]
	} else if strings.HasPrefix(filename, "std:/") {
		filename = filename[5:]
	} else if strings.HasPrefix(filename, "std://") {
		filename = filename[6:]
	}

	b, err := stdlibFS.ReadFile("stdlib/" + filename)
	if err == nil {
		content = ast.String(b)
	}

	WriteMessage(s.Writer, Response{
		RPC:    "2.0",
		ID:     req.ID,
		Result: ReadStdResult{Content: content},
	})
}

func (s *Server) refreshWorkspace() {
	/*
		cpuFile, err := os.Create("C:\\Users\\Laura\\lugo\\lugo_cpu.prof")
		if err == nil {
			pprof.StartCPUProfile(cpuFile)
		}

		traceFile, err := os.Create("C:\\Users\\Laura\\lugo\\lugo_trace.out")
		if err == nil {
			trace.Start(traceFile)
		}
	*/

	s.Log.Println("Starting workspace re-index...")

	s.IsIndexing = true

	start := time.Now()

	if s.activeURIs == nil {
		s.activeURIs = make(map[string]bool, len(s.Documents))
	} else {
		clear(s.activeURIs)
	}

	clear(s.FiveMResources)
	clear(s.FiveMResourceByName)

	var (
		total     int
		indexed   int
		unchanged int
		failed    int
	)

	s.indexEmbeddedStdlib(&total, &indexed, &unchanged)

	for _, libPath := range s.LibraryPaths {
		s.Log.Printf("Indexing external library: %s\n", libPath)

		s.indexWorkspace(libPath, &total, &indexed, &unchanged, &failed)
	}

	for _, wf := range s.WorkspaceFolders {
		s.Log.Printf("Indexing workspace folder: %s\n", wf)

		s.indexWorkspace(wf, &total, &indexed, &unchanged, &failed)
	}

	for uri := range s.Documents {
		if !s.activeURIs[uri] && !s.OpenFiles[uri] {
			s.clearDocument(uri)
		}
	}

	for uri, doc := range s.Documents {
		if s.OpenFiles[uri] && !s.activeURIs[uri] {
			total += len(doc.Source)

			s.updateDocument(uri, doc.Source)
		}
	}

	if s.FeatureFiveM {
		for _, doc := range s.Documents {
			if doc.IsFiveMManifest {
				res := s.parseFiveMManifest(doc)

				s.FiveMResources[res.RootURI] = res
				s.FiveMResourceByName[res.Name] = res
			}
		}
	}

	s.activeURIs = nil
	s.IsIndexing = false

	took := time.Since(start)

	s.Log.Printf("Re-indexed workspace in %s (indexed=%d, unchanged=%d, failed=%d)\n", took, indexed, unchanged, failed)

	s.publishWorkspaceDiagnostics()

	took = time.Since(start)

	s.Log.Printf("Total time taken for %d bytes: %s\n", total, took)

	/*
		if traceFile != nil {
			trace.Stop()

			traceFile.Close()
		}

		if cpuFile != nil {
			pprof.StopCPUProfile()

			cpuFile.Close()
		}
	*/
}

func (s *Server) indexWorkspace(rootPathOrURI string, total, indexed, unchanged, failed *int) {
	var path string

	if strings.HasPrefix(rootPathOrURI, "file://") {
		path = s.uriToPath(rootPathOrURI)
	} else {
		path = rootPathOrURI
	}

	path = strings.ReplaceAll(path, "/", string(filepath.Separator))

	if _, err := os.Stat(path); os.IsNotExist(err) {
		s.Log.Warnf("%q not found\n", path)

		return
	}

	if s.visitedDirs == nil {
		s.visitedDirs = make(map[string]bool, 256)
	} else {
		clear(s.visitedDirs)
	}

	var walk func(dir string, isSymlink bool)

	walk = func(dir string, isSymlink bool) {
		realDir := dir
		if isSymlink {
			if r, err := filepath.EvalSymlinks(dir); err == nil {
				realDir = r
			}
		}

		if s.visitedDirs[realDir] {
			return
		}

		s.visitedDirs[realDir] = true

		entries, err := os.ReadDir(realDir)
		if err != nil {
			return
		}

		for _, e := range entries {
			fullPath := filepath.Join(realDir, e.Name())

			isDir := e.IsDir()
			name := e.Name()

			if s.isIgnored(fullPath, name) {
				continue
			}

			isSym := e.Type()&fs.ModeSymlink != 0
			if isSym {
				stat, err := os.Stat(fullPath)
				if err == nil {
					isDir = stat.IsDir()
				} else {
					*failed++

					continue
				}
			}

			if isDir {
				walk(fullPath, isSym)
			} else if len(name) > 4 && name[len(name)-4:] == ".lua" {
				uri := s.normalizeURI(s.pathToURI(fullPath))

				if s.OpenFiles[uri] {
					if s.activeURIs != nil {
						s.activeURIs[uri] = true
					}

					*unchanged++

					continue
				}

				stat, statErr := os.Stat(fullPath)
				if statErr == nil {
					if existing, ok := s.Documents[uri]; ok && existing.ModTime.Equal(stat.ModTime()) {
						if s.activeURIs != nil {
							s.activeURIs[uri] = true
						}

						*unchanged++

						continue
					}
				}

				b, fsErr := os.ReadFile(fullPath)
				if fsErr == nil {
					if existing, ok := s.Documents[uri]; ok && bytes.Equal(existing.Source, b) {
						if statErr == nil {
							existing.ModTime = stat.ModTime()
						}

						if s.activeURIs != nil {
							s.activeURIs[uri] = true
						}

						*unchanged++

						continue
					}

					*total += len(b)

					s.updateDocument(uri, b)

					if statErr == nil && s.Documents[uri] != nil {
						s.Documents[uri].ModTime = stat.ModTime()
					}

					if s.activeURIs != nil {
						s.activeURIs[uri] = true
					}

					*indexed++
				} else {
					*failed++
				}
			}
		}
	}

	walk(path, true)
}

func (s *Server) indexEmbeddedStdlib(total, indexed, unchanged *int) {
	entries, err := stdlibFS.ReadDir("stdlib")
	if err != nil {
		return
	}

	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".lua") {
			b, err := stdlibFS.ReadFile("stdlib/" + e.Name())
			if err == nil {
				uri := "std:///" + e.Name()

				if existing, ok := s.Documents[uri]; ok && bytes.Equal(existing.Source, b) {
					s.activeURIs[uri] = true

					*unchanged++

					continue
				}

				*total += len(b)

				s.updateDocument(uri, b)

				if s.activeURIs != nil {
					s.activeURIs[uri] = true
				}

				*indexed++
			}
		}
	}
}

func (s *Server) updateDocument(uri string, source []byte) bool {
	var (
		needsWorkspaceRepublish bool
		tree                    *ast.Tree
		doc                     *Document
	)

	if existing, exists := s.Documents[uri]; exists {
		if bytes.Equal(existing.Source, source) {
			return false
		}

		doc = existing
		doc.Source = source

		s.removeDocumentGlobals(uri, doc)

		doc.ExportedGlobalDefs = doc.ExportedGlobalDefs[:0]

		tree = existing.Tree

		tree.Reset(source)
	} else {
		tree = ast.NewTree(source)

		doc = &Document{
			Server:   s,
			URI:      uri,
			Source:   source,
			Tree:     tree,
			Resolver: semantic.New(tree),
		}

		doc.Path = s.uriToPath(uri)
		doc.LowerPath = strings.ToLower(doc.Path)
		doc.Dir = filepath.Dir(doc.Path)
		doc.IsLibrary = s.checkIsLibrary(uri, doc.LowerPath)
		doc.IsWorkspace = s.checkIsWorkspace(uri, doc.LowerPath)
		doc.ModuleName = s.computeModuleName(uri, doc.Path, doc.LowerPath)
		doc.IsFiveMManifest = strings.HasSuffix(uri, "/fxmanifest.lua") || strings.HasSuffix(uri, "/__resource.lua")

		s.Documents[uri] = doc
	}

	p := s.sharedParser

	p.MaxErrors = s.MaxParseErrors

	p.Reset(source, tree)

	rootID := p.Parse()

	doc.IsMeta = false
	doc.FiveMResolved = false
	doc.EnvResolved = false

	for _, c := range tree.Comments {
		if bytes.Contains(tree.Source[c.Start:c.End], []byte("@meta")) {
			doc.IsMeta = true

			break
		}
	}

	if len(p.Errors) > 0 {
		if cap(doc.Errors) >= len(p.Errors) {
			doc.Errors = doc.Errors[:len(p.Errors)]
		} else {
			doc.Errors = make([]parser.ParseError, len(p.Errors))
		}

		copy(doc.Errors, p.Errors)
	} else {
		doc.Errors = doc.Errors[:0]
	}

	res := doc.Resolver

	res.Reset()
	res.Resolve(rootID)
	res.Cleanup()

	doc.parseDiagnosticPragmas()

	if s.FeatureFiveM {
		doc.FiveMLuaExports = doc.FiveMLuaExports[:0]

		for i := 1; i < len(tree.Nodes); i++ {
			node := tree.Nodes[i]
			if node.Kind == ast.KindCallExpr && int(node.Left) < len(tree.Nodes) {
				leftNode := tree.Nodes[node.Left]
				if leftNode.Kind == ast.KindIdent && doc.Resolver.References[node.Left] == ast.InvalidNode {
					if bytes.Equal(doc.Source[leftNode.Start:leftNode.End], []byte("exports")) {
						if node.Count >= 2 && node.Extra+1 < uint32(len(tree.ExtraList)) {
							arg1ID := tree.ExtraList[node.Extra]
							arg2ID := tree.ExtraList[node.Extra+1]

							if int(arg1ID) < len(tree.Nodes) && tree.Nodes[arg1ID].Kind == ast.KindString {
								exportName := unquoteLuaString(string(doc.Source[tree.Nodes[arg1ID].Start:tree.Nodes[arg1ID].End]))

								doc.FiveMLuaExports = append(doc.FiveMLuaExports, FiveMLuaExport{
									Name:   exportName,
									NodeID: arg2ID,
								})
							}
						}
					}
				}
			}
		}
	}

	doc.ExportedNode = ast.InvalidNode

	if rootID != ast.InvalidNode && int(rootID) < len(tree.Nodes) {
		root := tree.Nodes[rootID]
		if root.Kind == ast.KindFile && root.Left != ast.InvalidNode {
			block := tree.Nodes[root.Left]

			for i := uint16(0); i < block.Count; i++ {
				if block.Extra+uint32(i) >= uint32(len(tree.ExtraList)) {
					continue
				}

				stmtID := tree.ExtraList[block.Extra+uint32(i)]
				if int(stmtID) < len(tree.Nodes) {
					stmt := tree.Nodes[stmtID]
					if stmt.Kind == ast.KindReturn && stmt.Left != ast.InvalidNode {
						exprList := tree.Nodes[stmt.Left]
						if exprList.Count > 0 && exprList.Extra < uint32(len(tree.ExtraList)) {
							doc.ExportedNode = tree.ExtraList[exprList.Extra]
						}
					}
				}
			}
		}
	}

	fieldDefsByLocal := make(map[ast.NodeID][]int, len(res.FieldDefs))

	for i, fd := range res.FieldDefs {
		if fd.ReceiverDef != ast.InvalidNode {
			fieldDefsByLocal[fd.ReceiverDef] = append(fieldDefsByLocal[fd.ReceiverDef], i)
		}
	}

	for i := 0; i < len(tree.Comments); {
		startComment := tree.Comments[i]
		lastComment := startComment

		for j := i + 1; j < len(tree.Comments); j++ {
			nextC := tree.Comments[j]

			l1, _ := tree.Position(lastComment.End)
			l2, _ := tree.Position(nextC.Start)

			if l2 <= l1+1 {
				lastComment = nextC
			} else {
				break
			}
		}

		fullBlock := tree.Source[startComment.Start:lastComment.End]

		if !bytes.Contains(fullBlock, []byte("@class")) && !bytes.Contains(fullBlock, []byte("@alias")) && !bytes.Contains(fullBlock, []byte("@export")) {
			for j := i + 1; j < len(tree.Comments); j++ {
				if tree.Comments[j].Start <= lastComment.End {
					i = j
				} else {
					break
				}
			}

			i++

			continue
		}

		s.sharedCommentBuf = s.sharedCommentBuf[:0]
		cleaned := cleanLuaCommentBytes(s.sharedCommentBuf, fullBlock)

		luadoc := parseLuaDoc(cleaned, s.FeatureFormatAlerts)

		var virtualNodeID ast.NodeID = ast.InvalidNode

		if luadoc.Export != "" && s.FeatureFiveM {
			virtualNodeID = ast.NodeID(len(tree.Nodes))

			tree.Nodes = append(tree.Nodes, ast.Node{
				Kind:   ast.KindFunctionExpr,
				Start:  lastComment.End,
				End:    lastComment.End,
				Parent: ast.InvalidNode,
			})

			doc.FiveMLuaExports = append(doc.FiveMLuaExports, FiveMLuaExport{
				Name:   luadoc.Export,
				NodeID: virtualNodeID,
			})
		}

		var typeName string

		if luadoc.Class != nil && luadoc.Class.Name != "" {
			typeName = luadoc.Class.Name
		} else if luadoc.Alias != nil && luadoc.Alias.Name != "" {
			typeName = luadoc.Alias.Name
		}

		if typeName != "" {
			nameBytes := []byte(typeName)

			var (
				recHash  uint64
				propHash uint64
			)

			lastDot := bytes.LastIndexByte(nameBytes, '.')
			if lastDot != -1 {
				recHash = ast.HashBytes(nameBytes[:lastDot])
				propHash = ast.HashBytes(nameBytes[lastDot+1:])
			} else {
				recHash = 0
				propHash = ast.HashBytes(nameBytes)
			}

			var parentName string

			if luadoc.Class != nil {
				parentName = luadoc.Class.Parent
			}

			if virtualNodeID == ast.InvalidNode {
				virtualNodeID = ast.NodeID(len(tree.Nodes))

				tree.Nodes = append(tree.Nodes, ast.Node{
					Kind:   ast.KindTableExpr,
					Start:  lastComment.End,
					End:    lastComment.End,
					Parent: ast.InvalidNode,
				})
			}

			s.setGlobalSymbol(GlobalKey{ReceiverHash: recHash, PropHash: propHash}, uri, virtualNodeID, typeName, parentName, true, luadoc.IsDeprecated, luadoc.DeprecatedMsg)

			if len(luadoc.Fields) > 0 {
				classHash := ast.HashBytes(nameBytes)

				for _, field := range luadoc.Fields {
					fieldHash := ast.HashBytes([]byte(field.Name))

					var sb strings.Builder

					sb.Grow(len(nameBytes) + 1 + len(field.Name))
					sb.Write(nameBytes)
					sb.WriteByte('.')
					sb.WriteString(field.Name)

					fieldVirtualNodeID := ast.NodeID(len(tree.Nodes))

					var kind ast.NodeKind = ast.KindIdent
					if strings.Contains(field.Type, "fun") || strings.Contains(field.Type, "function") {
						kind = ast.KindFunctionExpr
					}

					tree.Nodes = append(tree.Nodes, ast.Node{
						Kind:   kind,
						Start:  lastComment.End,
						End:    lastComment.End,
						Parent: ast.InvalidNode,
					})

					s.setGlobalSymbol(GlobalKey{ReceiverHash: classHash, PropHash: fieldHash}, uri, fieldVirtualNodeID, sb.String(), "", true, luadoc.IsDeprecated, luadoc.DeprecatedMsg)
				}
			}
		}

		for j := i + 1; j < len(tree.Comments); j++ {
			if tree.Comments[j].Start <= lastComment.End {
				i = j
			} else {
				break
			}
		}

		i++
	}

	for _, defID := range res.GlobalDefs {
		node := tree.Nodes[defID]
		if node.Start == node.End {
			continue
		}

		identBytes := tree.Source[node.Start:node.End]
		hash := ast.HashBytes(identBytes)

		isRoot := isRootLevel(tree, defID)
		isDep, depMsg := doc.HasDeprecatedTag(defID)

		s.setGlobalSymbol(GlobalKey{ReceiverHash: 0, PropHash: hash}, uri, defID, string(identBytes), "", isRoot, isDep, depMsg)

		for name := range doc.ExtractLuaDocFields(defID) {
			fieldHash := ast.HashBytes(name)

			var sb strings.Builder

			sb.Grow(len(identBytes) + 1 + len(name))
			sb.Write(identBytes)
			sb.WriteByte('.')
			sb.Write(name)

			s.setGlobalSymbol(GlobalKey{ReceiverHash: hash, PropHash: fieldHash}, uri, defID, sb.String(), "", isRoot, isDep, depMsg)
		}

		// Module Aliasing
		valID := doc.getAssignedValue(defID)

		if valID != ast.InvalidNode {
			valNode := tree.Nodes[valID]

			if valNode.Kind == ast.KindIdent {
				localDefID := doc.Resolver.References[valID]

				if localDefID != ast.InvalidNode {
					localName := doc.Source[doc.Tree.Nodes[localDefID].Start:doc.Tree.Nodes[localDefID].End]
					globalBytes := tree.Source[node.Start:node.End]

					for _, fdIdx := range fieldDefsByLocal[localDefID] {
						fd := res.FieldDefs[fdIdx]

						fdIsRoot := isRootLevel(tree, fd.NodeID)
						fdIsDep, fdDepMsg := doc.HasDeprecatedTag(fd.NodeID)

						if bytes.Equal(fd.ReceiverName, localName) {
							propBytes := doc.Source[doc.Tree.Nodes[fd.NodeID].Start:doc.Tree.Nodes[fd.NodeID].End]

							var sb strings.Builder

							sb.Grow(len(identBytes) + 1 + len(propBytes))
							sb.Write(identBytes)
							sb.WriteByte('.')
							sb.Write(propBytes)

							s.setGlobalSymbol(GlobalKey{ReceiverHash: hash, PropHash: fd.PropHash}, uri, fd.NodeID, sb.String(), "", fdIsRoot, fdIsDep, fdDepMsg)
						} else if len(fd.ReceiverName) > len(localName) && bytes.HasPrefix(fd.ReceiverName, localName) && fd.ReceiverName[len(localName)] == '.' {
							suffix := fd.ReceiverName[len(localName)+1:]

							newRecHash := ast.HashBytesConcat(globalBytes, []byte{'.'}, suffix)

							propBytes := doc.Source[doc.Tree.Nodes[fd.NodeID].Start:doc.Tree.Nodes[fd.NodeID].End]

							var sb strings.Builder

							sb.Grow(len(identBytes) + 2 + len(suffix) + len(propBytes))
							sb.Write(identBytes)
							sb.WriteByte('.')
							sb.Write(suffix)
							sb.WriteByte('.')
							sb.Write(propBytes)

							s.setGlobalSymbol(GlobalKey{ReceiverHash: newRecHash, PropHash: fd.PropHash}, uri, fd.NodeID, sb.String(), "", fdIsRoot, fdIsDep, fdDepMsg)
						}
					}
				}
			}
		}
	}

	// Index global table fields
	for _, fd := range res.FieldDefs {
		var (
			globalRecName []byte
			globalRecHash uint64
		)

		if fd.ReceiverDef == ast.InvalidNode {
			globalRecName = fd.ReceiverName
			globalRecHash = fd.ReceiverHash
		} else {
			valID := doc.getAssignedValue(fd.ReceiverDef)
			if valID != ast.InvalidNode {
				globalRecName = s.getGlobalPath(doc, valID, 0)
				if globalRecName != nil {
					globalRecHash = ast.HashBytes(globalRecName)
				}
			}
		}

		if globalRecName != nil {
			if bytes.Equal(globalRecName, []byte("self")) {
				continue
			}

			propBytes := doc.Source[doc.Tree.Nodes[fd.NodeID].Start:doc.Tree.Nodes[fd.NodeID].End]

			sep := byte('.')

			if doc.Tree.Nodes[doc.Tree.Nodes[fd.NodeID].Parent].Kind == ast.KindMethodName {
				sep = ':'
			}

			var sb strings.Builder

			sb.Grow(len(globalRecName) + 1 + len(propBytes))
			sb.Write(globalRecName)
			sb.WriteByte(sep)
			sb.Write(propBytes)

			isRoot := isRootLevel(tree, fd.NodeID)
			isDep, depMsg := doc.HasDeprecatedTag(fd.NodeID)

			s.setGlobalSymbol(GlobalKey{ReceiverHash: globalRecHash, PropHash: fd.PropHash}, uri, fd.NodeID, sb.String(), "", isRoot, isDep, depMsg)
		}
	}

	if doc.ExportedNode != ast.InvalidNode {
		modName := doc.ModuleName
		if modName == "" {
			modName = "module"
		}

		modHash := ast.HashBytesConcat([]byte("module:"), nil, []byte(uri))

		exportNode := doc.Tree.Nodes[doc.ExportedNode]
		if exportNode.Kind == ast.KindIdent {
			exportDef := doc.Resolver.References[doc.ExportedNode]
			if exportDef != ast.InvalidNode {
				exportDefNode := doc.Tree.Nodes[exportDef]
				exportHash := ast.HashBytes(doc.Source[exportDefNode.Start:exportDefNode.End])

				for _, fd := range doc.Resolver.FieldDefs {
					if fd.ReceiverDef == exportDef && fd.ReceiverHash == exportHash {
						propName := doc.Source[doc.Tree.Nodes[fd.NodeID].Start:doc.Tree.Nodes[fd.NodeID].End]

						isRoot := isRootLevel(doc.Tree, fd.NodeID)
						isDep, depMsg := doc.HasDeprecatedTag(fd.NodeID)

						s.setGlobalSymbol(GlobalKey{ReceiverHash: modHash, PropHash: fd.PropHash}, uri, fd.NodeID, modName+"."+string(propName), "", isRoot, isDep, depMsg)
					}
				}
			}
		} else if exportNode.Kind == ast.KindTableExpr {
			for i := uint16(0); i < exportNode.Count; i++ {
				if exportNode.Extra+uint32(i) >= uint32(len(doc.Tree.ExtraList)) {
					continue
				}

				fieldID := doc.Tree.ExtraList[exportNode.Extra+uint32(i)]
				if int(fieldID) < len(doc.Tree.Nodes) {
					field := doc.Tree.Nodes[fieldID]
					if field.Kind == ast.KindRecordField {
						key := doc.Tree.Nodes[field.Left]
						if key.Kind == ast.KindIdent {
							propHash := ast.HashBytes(doc.Source[key.Start:key.End])
							propName := doc.Source[key.Start:key.End]

							isRoot := isRootLevel(doc.Tree, field.Left)
							isDep, depMsg := doc.HasDeprecatedTag(field.Left)

							s.setGlobalSymbol(GlobalKey{ReceiverHash: modHash, PropHash: propHash}, uri, field.Left, modName+"."+string(propName), "", isRoot, isDep, depMsg)
						}
					}
				}
			}
		}
	}

	for i, pf := range res.PendingFields {
		var reqModName string

		if pf.ReceiverDef != ast.InvalidNode {
			valID := doc.getAssignedValue(pf.ReceiverDef)
			reqModName = s.getRequireModName(doc, valID)
		} else {
			parentID := doc.Tree.Nodes[pf.PropNodeID].Parent
			if parentID != ast.InvalidNode && int(parentID) < len(doc.Tree.Nodes) {
				parentNode := doc.Tree.Nodes[parentID]
				if parentNode.Kind == ast.KindMemberExpr || parentNode.Kind == ast.KindMethodCall {
					reqModName = s.getRequireModName(doc, parentNode.Left)
				}
			}
		}

		if reqModName != "" {
			targetDoc := s.resolveModule(uri, reqModName)
			if targetDoc != nil {
				modHash := ast.HashBytesConcat([]byte("module:"), nil, []byte(targetDoc.URI))
				res.PendingFields[i].ReceiverHash = modHash
			}
		}

		if res.References[pf.PropNodeID] == ast.InvalidNode {
			var recHash uint64

			if pf.ReceiverDef != ast.InvalidNode {
				valID := doc.getAssignedValue(pf.ReceiverDef)
				if valID != ast.InvalidNode {
					path := s.getGlobalPath(doc, valID, 0)
					if path != nil {
						recHash = ast.HashBytes(path)
					}
				}
			} else {
				recHash = pf.ReceiverHash
			}

			if recHash != 0 {
				key := GlobalKey{ReceiverHash: recHash, PropHash: pf.PropHash}

				actualKey := key
				currRec := recHash

				for range 10 {
					if _, exists := s.GlobalIndex[actualKey]; exists {
						break
					}

					nextRec := s.getGlobalAlias(currRec)
					if nextRec == 0 {
						break
					}

					currRec = nextRec
					actualKey = GlobalKey{ReceiverHash: currRec, PropHash: pf.PropHash}
				}
			}
		}
	}

	if cap(doc.TypeCache) >= len(tree.Nodes) {
		doc.TypeCache = doc.TypeCache[:len(tree.Nodes)]

		clear(doc.TypeCache)
	} else {
		doc.TypeCache = make([]TypeSet, len(tree.Nodes))
	}

	if cap(doc.Inferring) >= len(tree.Nodes) {
		doc.Inferring = doc.Inferring[:len(tree.Nodes)]

		clear(doc.Inferring)
	} else {
		doc.Inferring = make([]bool, len(tree.Nodes))
	}

	if cap(doc.LuaDocCache) >= len(tree.Nodes) {
		doc.LuaDocCache = doc.LuaDocCache[:len(tree.Nodes)]

		clear(doc.LuaDocCache)
	} else {
		doc.LuaDocCache = make([]*LuaDoc, len(tree.Nodes))
	}

	if cap(doc.ActualReads) >= len(tree.Nodes) {
		doc.ActualReads = doc.ActualReads[:len(tree.Nodes)]

		clear(doc.ActualReads)
	} else {
		doc.ActualReads = make([]uint16, len(tree.Nodes))
	}

	for refID, defID := range doc.Resolver.References {
		if defID != ast.InvalidNode && ast.NodeID(refID) != defID {
			if s.isActualRead(doc, ast.NodeID(refID), defID) {
				doc.ActualReads[defID]++
			}
		}
	}

	if s.FeatureFiveM && doc.IsFiveMManifest {
		res := s.parseFiveMManifest(doc)

		oldRes := s.FiveMResources[res.RootURI]

		if !res.Equal(oldRes) {
			s.FiveMResources[res.RootURI] = res
			s.FiveMResourceByName[res.Name] = res

			for dUri, d := range s.Documents {
				if strings.HasPrefix(dUri, res.RootURI) {
					d.EnvResolved = false
					d.FiveMResolved = false
				}
			}

			needsWorkspaceRepublish = true
		}
	}

	s.Documents[uri] = doc

	return needsWorkspaceRepublish
}

func (s *Server) clearDocument(uri string) {
	if doc, ok := s.Documents[uri]; ok {
		s.removeDocumentGlobals(uri, doc)
	}

	delete(s.Documents, uri)

	if !s.IsCI {
		WriteMessage(s.Writer, OutgoingNotification{
			RPC:    "2.0",
			Method: "textDocument/publishDiagnostics",
			Params: PublishDiagnosticsParams{
				URI:         uri,
				Diagnostics: []Diagnostic{},
			},
		})
	}
}

func (s *Server) compileIgnorePatterns() {
	s.compiledIgnores = make([]IgnorePattern, 0, len(s.IgnoreGlobs))

	for _, g := range s.IgnoreGlobs {
		cleanGlob := strings.TrimPrefix(strings.TrimPrefix(g, "**/"), "*/")
		cleanGlob = strings.TrimSuffix(strings.TrimSuffix(cleanGlob, "/**"), "/*")

		if cleanGlob == "" {
			continue
		}

		if !strings.ContainsAny(cleanGlob, "*?") {
			cleanPath := filepath.FromSlash(cleanGlob)

			s.compiledIgnores = append(s.compiledIgnores, IgnorePattern{
				ContainsPath: string(filepath.Separator) + cleanPath + string(filepath.Separator),
				SuffixPath:   string(filepath.Separator) + cleanPath,
				HasSuffix:    cleanGlob,
			})
		} else if strings.HasPrefix(cleanGlob, "*") && !strings.ContainsAny(cleanGlob[1:], "*?") {
			s.compiledIgnores = append(s.compiledIgnores, IgnorePattern{HasSuffix: cleanGlob[1:]})
		} else if strings.HasSuffix(cleanGlob, "*") && !strings.ContainsAny(cleanGlob[:len(cleanGlob)-1], "*?") {
			s.compiledIgnores = append(s.compiledIgnores, IgnorePattern{HasPrefix: cleanGlob[:len(cleanGlob)-1]})
		} else {
			s.compiledIgnores = append(s.compiledIgnores, IgnorePattern{MatchFallback: g})
		}
	}
}

func (s *Server) isIgnored(fullPath, name string) bool {
	if s.FeatureFiveM && (name == "fxmanifest.lua" || name == "__resource.lua") {
		return false
	}

	for _, p := range s.compiledIgnores {
		if p.HasSuffix != "" && strings.HasSuffix(name, p.HasSuffix) {
			return true
		}

		if p.HasPrefix != "" && strings.HasPrefix(name, p.HasPrefix) {
			return true
		}

		if p.ContainsPath != "" && strings.Contains(fullPath, p.ContainsPath) {
			return true
		}

		if p.SuffixPath != "" && strings.HasSuffix(fullPath, p.SuffixPath) {
			return true
		}

		if p.MatchFallback != "" {
			if matched, _ := filepath.Match(p.MatchFallback, name); matched {
				return true
			}
		}
	}

	return false
}

func (s *Server) isIgnoredURI(uri string) bool {
	if doc, ok := s.Documents[uri]; ok {
		return s.isIgnored(doc.Path, filepath.Base(doc.Path))
	}

	path := s.uriToPath(uri)

	if path == "" {
		return false
	}

	return s.isIgnored(path, filepath.Base(path))
}

func (s *Server) checkIsLibrary(uri, lowerPath string) bool {
	if strings.HasPrefix(uri, "std://") {
		return true
	}

	if lowerPath == "" {
		return false
	}

	for _, libPath := range s.lowerLibraryPaths {
		if strings.HasPrefix(lowerPath, libPath) {
			return true
		}
	}

	return false
}

func (s *Server) checkIsWorkspace(uri, lowerPath string) bool {
	if strings.HasPrefix(uri, "std://") {
		return false
	}

	if lowerPath == "" {
		return false
	}

	for _, libPath := range s.lowerLibraryPaths {
		if strings.HasPrefix(lowerPath, libPath) {
			return false
		}
	}

	if len(s.lowerWorkspaceFolders) == 0 {
		return true
	}

	for _, wf := range s.lowerWorkspaceFolders {
		if strings.HasPrefix(lowerPath, wf) {
			return true
		}
	}

	return false
}

func (s *Server) isWorkspaceURI(uri string) bool {
	if doc, ok := s.Documents[uri]; ok {
		return doc.IsWorkspace
	}

	return s.checkIsWorkspace(uri, strings.ToLower(s.uriToPath(uri)))
}

func (s *Server) uriToPath(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return ""
	}

	path := uri[7:]

	if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}

	return filepath.Clean(filepath.FromSlash(path))
}

func (s *Server) pathToURI(pathStr string) string {
	cleanPath := filepath.Clean(pathStr)

	if runtime.GOOS == "windows" {
		if len(cleanPath) > 1 && cleanPath[1] == ':' {
			cleanPath = strings.ToLower(cleanPath[:1]) + cleanPath[1:]
		}

		return "file:///" + filepath.ToSlash(cleanPath)
	}

	return "file://" + filepath.ToSlash(cleanPath)
}

func (s *Server) computeModuleName(uri, path, lowerPath string) string {
	if strings.HasPrefix(uri, "std:///") {
		name := uri[7:]

		name = strings.TrimSuffix(name, ".lua")

		return strings.ReplaceAll(name, "/", ".")
	}

	if path == "" {
		return ""
	}

	var (
		bestRoot string
		bestLen  int
	)

	for i, wf := range s.lowerWorkspaceFolders {
		if strings.HasPrefix(lowerPath, wf) {
			if len(wf) > bestLen {
				bestLen = len(wf)
				bestRoot = s.uriToPath(s.WorkspaceFolders[i])
			}
		}
	}

	for i, lib := range s.lowerLibraryPaths {
		if strings.HasPrefix(lowerPath, lib) {
			if len(lib) > bestLen {
				bestLen = len(lib)
				bestRoot = s.LibraryPaths[i]
			}
		}
	}

	if bestRoot != "" {
		rel, err := filepath.Rel(bestRoot, path)
		if err == nil {
			rel = filepath.ToSlash(rel)

			rel = strings.TrimSuffix(rel, ".lua")
			rel = strings.TrimSuffix(rel, "/init")

			return strings.ReplaceAll(rel, "/", ".")
		}
	}

	base := filepath.Base(path)

	base = strings.TrimSuffix(base, ".lua")

	return base
}

func (s *Server) normalizeURI(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return uri
	}

	if s.uriCache == nil {
		s.uriCache = make(map[string]string, 1024)
	} else if res, ok := s.uriCache[uri]; ok {
		return res
	}

	path := s.uriToPath(uri)
	dir := filepath.Dir(path)

	if s.symlinkCache == nil {
		s.symlinkCache = make(map[string]string, 256)
	}

	realDir, ok := s.symlinkCache[dir]
	if !ok {
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			realDir = r
		} else {
			realDir = dir
		}

		s.symlinkCache[dir] = realDir
	}

	path = filepath.Join(realDir, filepath.Base(path))

	res := s.pathToURI(path)

	s.uriCache[uri] = res

	return res
}

func (s *Server) resolveModule(currentURI string, modName string) *Document {
	if modName == "" {
		return nil
	}

	for _, d := range s.Documents {
		if d.ModuleName == modName {
			return d
		}
	}

	modPath := strings.ReplaceAll(modName, ".", "/")

	suffix1 := "/" + modPath + ".lua"
	suffix2 := "/" + modPath + "/init.lua"

	var (
		bestMatch *Document
		bestScore int
	)

	currentDir := ""
	if currentURI != "" {
		if doc, ok := s.Documents[currentURI]; ok {
			currentDir = filepath.ToSlash(doc.Dir)
		} else {
			currentDir = filepath.ToSlash(filepath.Dir(s.uriToPath(currentURI)))
		}
	}

	var rootDirs []string

	for _, wf := range s.WorkspaceFolders {
		rootDirs = append(rootDirs, filepath.ToSlash(s.uriToPath(wf)))
	}

	for _, d := range s.Documents {
		if strings.HasPrefix(d.URI, "std://") {
			continue
		}

		path := filepath.ToSlash(d.Path)

		if strings.HasSuffix(path, suffix1) || strings.HasSuffix(path, suffix2) {
			score := 1

			if currentDir != "" && strings.HasPrefix(path, currentDir) {
				score = 3
			} else {
				for _, rootDir := range rootDirs {
					if strings.HasPrefix(path, rootDir) {
						score = 2
						break
					}
				}
			}

			if score > bestScore {
				bestScore = score
				bestMatch = d
			}
		}
	}

	return bestMatch
}
