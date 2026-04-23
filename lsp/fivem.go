package lsp

import (
	"slices"
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
	ClientExports       []string
	ServerExports       []string
}

func (r *FiveMResource) Equal(other *FiveMResource) bool {
	if r == other {
		return true
	}

	if r == nil || other == nil {
		return false
	}

	if r.Name != other.Name || r.RootURI != other.RootURI || r.ManifestURI != other.ManifestURI {
		return false
	}

	if !slices.Equal(r.ClientGlobs, other.ClientGlobs) {
		return false
	}

	if !slices.Equal(r.ServerGlobs, other.ServerGlobs) {
		return false
	}

	if !slices.Equal(r.SharedGlobs, other.SharedGlobs) {
		return false
	}

	if !slices.Equal(r.ClientCrossIncludes, other.ClientCrossIncludes) {
		return false
	}

	if !slices.Equal(r.ServerCrossIncludes, other.ServerCrossIncludes) {
		return false
	}

	if !slices.Equal(r.SharedCrossIncludes, other.SharedCrossIncludes) {
		return false
	}

	if !slices.Equal(r.ClientExports, other.ClientExports) {
		return false
	}

	if !slices.Equal(r.ServerExports, other.ServerExports) {
		return false
	}

	return true
}

func unquoteLuaString(s string) string {
	s = strings.TrimSpace(s)

	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') {
		if s[len(s)-1] == s[0] {
			return s[1 : len(s)-1]
		}

		return s[1:]
	}

	if strings.HasPrefix(s, "[") {
		idx := strings.IndexByte(s[1:], '[')
		if idx != -1 {
			start := 2 + idx
			if start < len(s) && s[start] == '\n' {
				start++
			}

			end := len(s) - (2 + idx)
			if start <= end {
				return s[start:end]
			}
		}
	}

	return s
}

func (s *Server) parseFiveMManifest(doc *Document) *FiveMResource {
	res := &FiveMResource{
		ManifestURI: doc.URI,
	}

	res.RootURI = doc.URI[:strings.LastIndex(doc.URI, "/")]

	parts := strings.Split(res.RootURI, "/")

	res.Name = strings.ToLower(parts[len(parts)-1])

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

			var targetExports *[]string

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
			case "export", "exports", "client_export", "client_exports":
				targetExports = &res.ClientExports
			case "server_export", "server_exports":
				targetExports = &res.ServerExports
			}

			if targetExports != nil {
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

						*targetExports = append(*targetExports, strVal)
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

								*targetExports = append(*targetExports, strVal)
							}
						}
					}
				}
			} else if targetGlobs != nil && targetCross != nil {
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

func (s *Server) getDocFileEnv(res *FiveMResource, doc *Document) FileEnv {
	if doc.EnvResolved {
		return doc.FiveMEnv
	}

	var relPath string

	if len(doc.URI) > len(res.RootURI) {
		relPath = doc.URI[len(res.RootURI)+1:]
	} else {
		relPath = ""
	}

	var env FileEnv = EnvUnknown

	for _, glob := range res.SharedGlobs {
		if matchGlob(glob, relPath) {
			env = EnvShared
			break
		}
	}

	if env == EnvUnknown {
		var (
			isClient bool
			isServer bool
		)

		for _, glob := range res.ClientGlobs {
			if matchGlob(glob, relPath) {
				isClient = true

				break
			}
		}

		for _, glob := range res.ServerGlobs {
			if matchGlob(glob, relPath) {
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

	doc.FiveMEnv = env
	doc.EnvResolved = true

	return env
}
