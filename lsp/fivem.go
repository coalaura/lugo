package lsp

import (
	"strings"

	"github.com/coalaura/lugo/ast"
)

type FileEnv int

const (
	EnvUnknown FileEnv = iota
	EnvShared
	EnvClient
	EnvServer
)

type FiveMResource struct {
	Name                string
	RootURI             string
	ManifestURI         string
	ClientGlobs         []string
	ServerGlobs         []string
	SharedGlobs         []string
	ClientCrossIncludes []string
	ServerCrossIncludes []string
	SharedCrossIncludes []string
}

// matchFiveMGlob is a highly optimized, zero-allocation backtracking glob matcher.
func matchFiveMGlob(pattern, path string) bool {
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
					if matchFiveMGlob(pattern[p:], path[i:]) {
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

				if matchFiveMGlob(pattern[p:], path[i:]) {
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

			c1 := pattern[p]
			c2 := path[t]

			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 32
			}

			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 32
			}

			if c1 == '\\' {
				c1 = '/'
			}

			if c2 == '\\' {
				c2 = '/'
			}

			if c1 != c2 {
				return false
			}

			p++
			t++
		}
	}

	return t == tLen
}

func unquoteLuaString(s string) string {
	s = strings.TrimSpace(s)

	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') {
		return s[1 : len(s)-1]
	}

	if strings.HasPrefix(s, "[[") && strings.HasSuffix(s, "]]") {
		return s[2 : len(s)-2]
	}

	return s
}

func (s *Server) parseFiveMManifest(doc *Document) *FiveMResource {
	res := &FiveMResource{
		ManifestURI: doc.URI,
	}

	res.RootURI = doc.URI[:strings.LastIndex(doc.URI, "/")]

	parts := strings.Split(res.RootURI, "/")

	res.Name = parts[len(parts)-1]

	for i := 1; i < len(doc.Tree.Nodes); i++ {
		node := doc.Tree.Nodes[i]
		if node.Kind == ast.KindCallExpr || node.Kind == ast.KindMethodCall {
			var funcName string

			if node.Kind == ast.KindCallExpr {
				if int(node.Left) < len(doc.Tree.Nodes) && doc.Tree.Nodes[node.Left].Kind == ast.KindIdent {
					funcNode := doc.Tree.Nodes[node.Left]
					funcName = string(doc.Source[funcNode.Start:funcNode.End])
				}
			}

			if funcName == "" {
				continue
			}

			var (
				targetGlobs *[]string
				targetCross *[]string
			)

			switch funcName {
			case "client_script", "client_scripts":
				targetGlobs = &res.ClientGlobs
				targetCross = &res.ClientCrossIncludes
			case "server_script", "server_scripts":
				targetGlobs = &res.ServerGlobs
				targetCross = &res.ServerCrossIncludes
			case "shared_script", "shared_scripts", "file", "files":
				targetGlobs = &res.SharedGlobs
				targetCross = &res.SharedCrossIncludes
			}

			if targetGlobs != nil && targetCross != nil {
				for j := uint16(0); j < node.Count; j++ {
					if node.Extra+uint32(j) >= uint32(len(doc.Tree.ExtraList)) {
						continue
					}

					argID := doc.Tree.ExtraList[node.Extra+uint32(j)]
					if int(argID) >= len(doc.Tree.Nodes) {
						continue
					}

					argNode := doc.Tree.Nodes[argID]

					if argNode.Kind == ast.KindString {
						strVal := unquoteLuaString(string(doc.Source[argNode.Start:argNode.End]))
						if strings.HasPrefix(strVal, "@") {
							*targetCross = append(*targetCross, strVal)
						} else {
							*targetGlobs = append(*targetGlobs, strVal)
						}
					} else if argNode.Kind == ast.KindTableExpr {
						for k := uint16(0); k < argNode.Count; k++ {
							if argNode.Extra+uint32(k) >= uint32(len(doc.Tree.ExtraList)) {
								continue
							}

							fieldID := doc.Tree.ExtraList[argNode.Extra+uint32(k)]
							if int(fieldID) >= len(doc.Tree.Nodes) {
								continue
							}

							fieldNode := doc.Tree.Nodes[fieldID]
							if fieldNode.Kind == ast.KindString {
								strVal := unquoteLuaString(string(doc.Source[fieldNode.Start:fieldNode.End]))
								if strings.HasPrefix(strVal, "@") {
									*targetCross = append(*targetCross, strVal)
								} else {
									*targetGlobs = append(*targetGlobs, strVal)
								}
							}
						}
					}
				}
			}
		}
	}

	return res
}

func (s *Server) getFileEnv(res *FiveMResource, uri string) FileEnv {
	if env, ok := s.envCache[uri]; ok {
		return env
	}

	var relPath string

	if len(uri) > len(res.RootURI) {
		relPath = uri[len(res.RootURI)+1:]
	} else {
		relPath = uri
	}

	var env FileEnv = EnvUnknown

	for _, glob := range res.SharedGlobs {
		if matchFiveMGlob(glob, relPath) {
			env = EnvShared
			break
		}
	}

	if env == EnvUnknown {
		var isClient bool

		for _, glob := range res.ClientGlobs {
			if matchFiveMGlob(glob, relPath) {
				isClient = true

				break
			}
		}

		var isServer bool

		for _, glob := range res.ServerGlobs {
			if matchFiveMGlob(glob, relPath) {
				isServer = true

				break
			}
		}

		if isClient && isServer {
			env = EnvShared
		} else if isClient {
			env = EnvClient
		} else if isServer {
			env = EnvServer
		}
	}

	s.envCache[uri] = env

	return env
}
