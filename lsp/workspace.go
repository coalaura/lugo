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

	s.updateDocument(uri, []byte(params.TextDocument.Text))

	s.publishDiagnostics(uri)

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
		s.updateDocument(uri, []byte(params.ContentChanges[0].Text))

		s.publishDiagnostics(uri)

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

				if b, err := os.ReadFile(path); err == nil {
					s.updateDocument(uri, b)

					if s.isWorkspaceURI(uri) {
						s.publishDiagnostics(uri)
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

	if s.RootURI != "" {
		s.Log.Printf("Indexing workspace: %s\n", s.RootURI)

		s.indexWorkspace(s.RootURI, &total, &indexed, &unchanged, &failed)
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
		u, err := url.Parse(rootPathOrURI)
		if err != nil {
			s.Log.Errorf("Invalid workspace URI format: %s\n", rootPathOrURI)

			return
		}

		path = u.Path

		if runtime.GOOS == "windows" && strings.HasPrefix(path, "/") {
			path = path[1:]
		}
	} else {
		path = rootPathOrURI
	}

	path = strings.ReplaceAll(path, "/", string(filepath.Separator))

	if realPath, err := filepath.EvalSymlinks(path); err == nil {
		path = realPath
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		s.Log.Warnf("%q not found\n", path)

		return
	}

	if s.visitedDirs == nil {
		s.visitedDirs = make(map[string]bool, 256)
	} else {
		clear(s.visitedDirs)
	}

	var walk func(dir string)

	walk = func(dir string) {
		if s.visitedDirs[dir] {
			return
		}

		s.visitedDirs[dir] = true

		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		for _, e := range entries {
			fullPath := filepath.Join(dir, e.Name())

			isDir := e.IsDir()
			name := e.Name()

			if s.isIgnored(fullPath, name) {
				continue
			}

			// Check if its a symlink
			if e.Type()&fs.ModeSymlink != 0 {
				realPath, err := filepath.EvalSymlinks(fullPath)
				if err == nil {
					stat, err := os.Stat(realPath)
					if err == nil {
						isDir = stat.IsDir()
						name = stat.Name()

						fullPath = realPath
					} else {
						*failed++

						continue
					}
				} else {
					*failed++

					continue // Broken symlink
				}
			}

			if isDir {
				walk(fullPath)
			} else if strings.HasSuffix(name, ".lua") {
				uri := s.pathToURI(fullPath)

				if s.OpenFiles[uri] {
					if s.activeURIs != nil {
						s.activeURIs[uri] = true
					}

					*unchanged++

					continue
				}

				b, fsErr := os.ReadFile(fullPath)
				if fsErr == nil {
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
				} else {
					*failed++
				}
			}
		}
	}

	walk(path)
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

				s.activeURIs[uri] = true

				*indexed++
			}
		}
	}
}

func (s *Server) updateDocument(uri string, source []byte) {
	var (
		tree *ast.Tree
		doc  *Document
	)

	if existing, exists := s.Documents[uri]; exists {
		if bytes.Equal(existing.Source, source) {
			return
		}

		doc = existing
		doc.Source = source

		s.removeDocumentGlobals(uri, doc)

		clear(doc.ExportedGlobals)
		clear(doc.ExportedGlobalDefs)

		tree = existing.Tree
		tree.Reset(source)
	} else {
		tree = ast.NewTree(source)

		doc = &Document{
			Server:             s,
			URI:                uri,
			Source:             source,
			Tree:               tree,
			Resolver:           semantic.New(tree),
			ExportedGlobals:    make(map[GlobalKey]ast.NodeID),
			ExportedGlobalDefs: make(map[ast.NodeID]GlobalKey),
		}

		s.Documents[uri] = doc
	}

	p := s.sharedParser

	p.MaxErrors = s.MaxParseErrors

	p.Reset(source, tree)

	rootID := p.Parse()

	if cap(doc.TypeCache) >= len(tree.Nodes) {
		doc.TypeCache = doc.TypeCache[:len(tree.Nodes)]
		clear(doc.TypeCache)

		doc.Inferring = doc.Inferring[:len(tree.Nodes)]
		clear(doc.Inferring)
	} else {
		doc.TypeCache = make([]TypeSet, len(tree.Nodes))
		doc.Inferring = make([]bool, len(tree.Nodes))
	}

	doc.IsMeta = false

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

	for _, defID := range res.GlobalDefs {
		node := tree.Nodes[defID]
		if node.Start == node.End {
			continue
		}

		identBytes := tree.Source[node.Start:node.End]
		hash := ast.HashBytes(identBytes)

		depth := getASTDepth(tree, defID)

		s.setGlobalSymbol(GlobalKey{ReceiverHash: 0, PropHash: hash}, uri, defID, depth, string(identBytes))

		for name := range doc.ExtractLuaDocFields(defID) {
			fieldHash := ast.HashBytes(name)

			var sb strings.Builder

			sb.Grow(len(identBytes) + 1 + len(name))
			sb.Write(identBytes)
			sb.WriteByte('.')
			sb.Write(name)

			s.setGlobalSymbol(GlobalKey{ReceiverHash: hash, PropHash: fieldHash}, uri, defID, depth, sb.String())
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

						if bytes.Equal(fd.ReceiverName, localName) {
							propBytes := doc.Source[doc.Tree.Nodes[fd.NodeID].Start:doc.Tree.Nodes[fd.NodeID].End]

							var sb strings.Builder

							sb.Grow(len(identBytes) + 1 + len(propBytes))
							sb.Write(identBytes)
							sb.WriteByte('.')
							sb.Write(propBytes)

							s.setGlobalSymbol(GlobalKey{ReceiverHash: hash, PropHash: fd.PropHash}, uri, fd.NodeID, depth, sb.String())
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

							s.setGlobalSymbol(GlobalKey{ReceiverHash: newRecHash, PropHash: fd.PropHash}, uri, fd.NodeID, depth, sb.String())
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

			depth := getASTDepth(tree, fd.NodeID)

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

			s.setGlobalSymbol(GlobalKey{ReceiverHash: globalRecHash, PropHash: fd.PropHash}, uri, fd.NodeID, depth, sb.String())
		}
	}

	if doc.ExportedNode != ast.InvalidNode {
		modName := s.uriToModuleName(uri)
		if modName == "" {
			modName = "module"
		}

		modHash := ast.HashBytesConcat([]byte("module:"), nil, []byte(uri))

		exportNode := doc.Tree.Nodes[doc.ExportedNode]
		if exportNode.Kind == ast.KindIdent {
			exportDef := doc.Resolver.References[doc.ExportedNode]
			if exportDef != ast.InvalidNode {
				for _, fd := range doc.Resolver.FieldDefs {
					if fd.ReceiverDef == exportDef {
						depth := getASTDepth(doc.Tree, fd.NodeID)

						propName := doc.Source[doc.Tree.Nodes[fd.NodeID].Start:doc.Tree.Nodes[fd.NodeID].End]

						s.setGlobalSymbol(GlobalKey{ReceiverHash: modHash, PropHash: fd.PropHash}, uri, fd.NodeID, depth, modName+"."+string(propName))
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

							depth := getASTDepth(doc.Tree, field.Left)

							propName := doc.Source[key.Start:key.End]

							s.setGlobalSymbol(GlobalKey{ReceiverHash: modHash, PropHash: propHash}, uri, field.Left, depth, modName+"."+string(propName))
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
			pID := doc.Tree.Nodes[pf.PropNodeID].Parent
			if pID != ast.InvalidNode && int(pID) < len(doc.Tree.Nodes) {
				pNode := doc.Tree.Nodes[pID]
				if pNode.Kind == ast.KindMemberExpr || pNode.Kind == ast.KindMethodCall {
					reqModName = s.getRequireModName(doc, pNode.Left)
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

	s.Documents[uri] = doc
}

func (s *Server) clearDocument(uri string) {
	if doc, ok := s.Documents[uri]; ok {
		s.removeDocumentGlobals(uri, doc)
	}

	delete(s.Documents, uri)

	WriteMessage(s.Writer, OutgoingNotification{
		RPC:    "2.0",
		Method: "textDocument/publishDiagnostics",
		Params: PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: []Diagnostic{},
		},
	})
}

func (s *Server) compileIgnorePatterns() {
	s.compiledIgnores = make([]IgnorePattern, 0, len(s.IgnoreGlobs))

	for _, g := range s.IgnoreGlobs {
		cleanGlob := strings.TrimPrefix(strings.TrimPrefix(g, "**/"), "*/")
		cleanGlob = strings.TrimSuffix(strings.TrimSuffix(cleanGlob, "/**"), "/*")

		if cleanGlob == "" {
			continue
		}

		if !strings.ContainsAny(cleanGlob, "*?[") {
			cleanPath := filepath.FromSlash(cleanGlob)

			s.compiledIgnores = append(s.compiledIgnores, IgnorePattern{
				ContainsPath: string(filepath.Separator) + cleanPath + string(filepath.Separator),
				SuffixPath:   string(filepath.Separator) + cleanPath,
				HasSuffix:    cleanGlob,
			})
		} else if strings.HasPrefix(cleanGlob, "*") && !strings.ContainsAny(cleanGlob[1:], "*?[") {
			s.compiledIgnores = append(s.compiledIgnores, IgnorePattern{HasSuffix: cleanGlob[1:]})
		} else if strings.HasSuffix(cleanGlob, "*") && !strings.ContainsAny(cleanGlob[:len(cleanGlob)-1], "*?[") {
			s.compiledIgnores = append(s.compiledIgnores, IgnorePattern{HasPrefix: cleanGlob[:len(cleanGlob)-1]})
		} else {
			s.compiledIgnores = append(s.compiledIgnores, IgnorePattern{MatchFallback: g})
		}
	}
}

func (s *Server) isIgnored(fullPath, name string) bool {
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
	path := s.uriToPath(uri)

	if path == "" {
		return false
	}

	return s.isIgnored(path, filepath.Base(path))
}

func (s *Server) isWorkspaceURI(uri string) bool {
	if strings.HasPrefix(uri, "std:///") {
		return false
	}

	path := s.uriToPath(uri)

	if path == "" {
		return false
	}

	lowerPath := strings.ToLower(path)

	for _, libPath := range s.lowerLibraryPaths {
		if strings.HasPrefix(lowerPath, libPath) {
			return false
		}
	}

	if s.RootURI == "" {
		return true
	}

	return strings.HasPrefix(lowerPath, s.lowerRootPath)
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

func (s *Server) uriToModuleName(uri string) string {
	if strings.HasPrefix(uri, "std:///") {
		name := uri[7:]

		name = strings.TrimSuffix(name, ".lua")

		return strings.ReplaceAll(name, "/", ".")
	}

	path := s.uriToPath(uri)
	if path == "" {
		return ""
	}

	var bestRoot string

	if s.RootURI != "" {
		rootPath := s.uriToPath(s.RootURI)
		if strings.HasPrefix(strings.ToLower(path), strings.ToLower(rootPath)) {
			bestRoot = rootPath
		}
	}

	for _, lib := range s.LibraryPaths {
		if strings.HasPrefix(strings.ToLower(path), strings.ToLower(lib)) {
			if len(lib) > len(bestRoot) {
				bestRoot = lib
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

	return s.pathToURI(s.uriToPath(uri))
}

func (s *Server) resolveModule(currentURI string, modName string) *Document {
	if modName == "" {
		return nil
	}

	for _, d := range s.Documents {
		if s.uriToModuleName(d.URI) == modName {
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
		currentDir = filepath.ToSlash(filepath.Dir(s.uriToPath(currentURI)))
	}

	var rootDir string

	if s.RootURI != "" {
		rootDir = filepath.ToSlash(s.uriToPath(s.RootURI))
	}

	for uri, d := range s.Documents {
		if strings.HasPrefix(uri, "std://") {
			continue
		}

		path := filepath.ToSlash(s.uriToPath(uri))

		if strings.HasSuffix(path, suffix1) || strings.HasSuffix(path, suffix2) {
			score := 1

			if currentDir != "" && strings.HasPrefix(path, currentDir) {
				score = 3
			} else if rootDir != "" && strings.HasPrefix(path, rootDir) {
				score = 2
			}

			if score > bestScore {
				bestScore = score
				bestMatch = d
			}
		}
	}

	return bestMatch
}
