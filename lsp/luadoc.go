package lsp

import "bytes"

var (
	tagParam      = []byte("@param")
	tagReturn     = []byte("@return")
	tagField      = []byte("@field")
	tagDeprecated = []byte("@deprecated")
	tagClass      = []byte("@class")
	tagType       = []byte("@type")
	tagAlias      = []byte("@alias")
	tagGeneric    = []byte("@generic")
	tagOverload   = []byte("@overload")
	tagSee        = []byte("@see")

	kwPublic    = []byte("public ")
	kwPrivate   = []byte("private ")
	kwProtected = []byte("protected ")

	dashPrefix = []byte("- ")
	hashPrefix = []byte("# ")
	qSuffix    = []byte("?")
)

type LuaDocParam struct {
	Name string
	Type string
	Desc string
}

type LuaDocReturn struct {
	Type string
	Desc string
}

type LuaDocField struct {
	Name string
	Type string
	Desc string
}

type LuaDocClass struct {
	Name   string
	Parent string
	Desc   string
}

type LuaDocType struct {
	Type string
	Desc string
}

type LuaDocAlias struct {
	Name string
	Type string
	Desc string
}

type LuaDocGeneric struct {
	Name   string
	Parent string
}

type LuaDoc struct {
	Description   string
	DeprecatedMsg string
	Class         *LuaDocClass
	Type          *LuaDocType
	Alias         *LuaDocAlias
	Generics      []LuaDocGeneric
	Params        []LuaDocParam
	Returns       []LuaDocReturn
	Fields        []LuaDocField
	Overloads     []string
	See           []string
	IsDeprecated  bool
}

// findTypeEnd safely scans past complex types with spaces like 'fun(a: string): number'
func findTypeEnd(s []byte) int {
	var depth int

	for i := 0; i < len(s); i++ {
		c := s[i]

		if c == '(' || c == '<' || c == '{' || c == '[' {
			depth++
		} else if c == ')' || c == '>' || c == '}' || c == ']' {
			depth--
		} else if (c == ' ' || c == '\t') && depth <= 0 {
			var j int

			for j = i - 1; j >= 0 && (s[j] == ' ' || s[j] == '\t'); j-- {
			}

			if j >= 0 && (s[j] == ':' || s[j] == ',' || s[j] == '|') {
				continue // Type is still going (e.g., 'fun(): boolean' or 'string | number')
			}

			var k int

			for k = i + 1; k < len(s) && (s[k] == ' ' || s[k] == '\t'); k++ {
			}

			if k < len(s) && (s[k] == '|' || s[k] == ',') {
				i = k - 1

				continue
			}

			return i
		}
	}

	return -1
}

func extractTypeDesc(s []byte) (typ, desc []byte) {
	s = bytes.TrimSpace(s)

	idx := findTypeEnd(s)
	if idx == -1 {
		return s, nil
	}

	return s[:idx], bytes.TrimSpace(s[idx:])
}

func extractNameParent(s []byte) (name, parent, desc []byte) {
	s = bytes.TrimSpace(s)

	var nameEnd int

	for nameEnd < len(s) && s[nameEnd] != ' ' && s[nameEnd] != '\t' && s[nameEnd] != ':' {
		nameEnd++
	}

	if nameEnd == 0 {
		return nil, nil, s
	}

	name = s[:nameEnd]
	s = bytes.TrimSpace(s[nameEnd:])

	if bytes.HasPrefix(s, []byte(":")) {
		s = bytes.TrimSpace(s[1:])

		var parentEnd int

		for parentEnd < len(s) && s[parentEnd] != ' ' && s[parentEnd] != '\t' {
			parentEnd++
		}

		parent = s[:parentEnd]
		desc = bytes.TrimSpace(s[parentEnd:])
	} else {
		desc = s
	}

	return name, parent, desc
}

func parseLuaDoc(comments []byte) LuaDoc {
	var (
		doc       LuaDoc
		descLines [][]byte
		activeTag string
	)

	for len(comments) > 0 {
		var line []byte

		idx := bytes.IndexByte(comments, '\n')
		if idx == -1 {
			line = comments
			comments = nil
		} else {
			line = comments[:idx]
			comments = comments[idx+1:]
		}

		line = bytes.TrimSpace(line)

		if after, ok := bytes.CutPrefix(line, tagParam); ok {
			activeTag = "param"

			rest := bytes.TrimSpace(after)
			spaceIdx := bytes.IndexByte(rest, ' ')

			if spaceIdx != -1 {
				name := rest[:spaceIdx]
				rest = bytes.TrimSpace(rest[spaceIdx:])

				typ, desc := extractTypeDesc(rest)
				desc = bytes.TrimPrefix(desc, dashPrefix)

				nameStr := string(name)
				typStr := string(typ)

				if bytes.HasSuffix(name, qSuffix) {
					nameStr = nameStr[:len(nameStr)-1]
					typStr += "?"
				}

				doc.Params = append(doc.Params, LuaDocParam{Name: nameStr, Type: typStr, Desc: string(desc)})
			} else {
				nameStr := string(rest)

				if bytes.HasSuffix(rest, qSuffix) {
					nameStr = nameStr[:len(nameStr)-1]
				}

				doc.Params = append(doc.Params, LuaDocParam{Name: nameStr})
			}
		} else if after, ok := bytes.CutPrefix(line, tagReturn); ok {
			activeTag = "return"

			typ, desc := extractTypeDesc(bytes.TrimSpace(after))

			desc = bytes.TrimPrefix(desc, dashPrefix)
			desc = bytes.TrimPrefix(desc, hashPrefix)

			doc.Returns = append(doc.Returns, LuaDocReturn{Type: string(typ), Desc: string(desc)})
		} else if after, ok := bytes.CutPrefix(line, tagField); ok {
			activeTag = "field"

			rest := bytes.TrimSpace(after)

			if bytes.HasPrefix(rest, kwPublic) {
				rest = bytes.TrimSpace(rest[7:])
			} else if bytes.HasPrefix(rest, kwPrivate) {
				rest = bytes.TrimSpace(rest[8:])
			} else if bytes.HasPrefix(rest, kwProtected) {
				rest = bytes.TrimSpace(rest[10:])
			}

			spaceIdx := bytes.IndexByte(rest, ' ')
			if spaceIdx != -1 {
				name := rest[:spaceIdx]
				rest = bytes.TrimSpace(rest[spaceIdx:])

				typ, desc := extractTypeDesc(rest)
				desc = bytes.TrimPrefix(desc, dashPrefix)

				nameStr := string(name)
				typStr := string(typ)

				if bytes.HasSuffix(name, qSuffix) {
					nameStr = nameStr[:len(nameStr)-1]
					typStr = "?" + typStr
				}

				doc.Fields = append(doc.Fields, LuaDocField{Name: nameStr, Type: typStr, Desc: string(desc)})
			} else {
				nameStr := string(rest)

				if bytes.HasSuffix(rest, qSuffix) {
					nameStr = nameStr[:len(nameStr)-1]
				}

				doc.Fields = append(doc.Fields, LuaDocField{Name: nameStr})
			}
		} else if after, ok := bytes.CutPrefix(line, tagDeprecated); ok {
			activeTag = ""

			doc.IsDeprecated = true
			doc.DeprecatedMsg = string(bytes.TrimSpace(after))
		} else if after, ok := bytes.CutPrefix(line, tagClass); ok {
			activeTag = ""

			name, parent, desc := extractNameParent(after)
			if len(name) > 0 {
				doc.Class = &LuaDocClass{Name: string(name), Parent: string(parent), Desc: string(desc)}
			}
		} else if after, ok := bytes.CutPrefix(line, tagType); ok {
			activeTag = ""

			typ, desc := extractTypeDesc(bytes.TrimSpace(after))

			doc.Type = &LuaDocType{Type: string(typ), Desc: string(desc)}
		} else if after, ok := bytes.CutPrefix(line, tagAlias); ok {
			activeTag = ""

			rest := bytes.TrimSpace(after)

			spaceIdx := bytes.IndexByte(rest, ' ')
			if spaceIdx != -1 {
				name := rest[:spaceIdx]
				rest = bytes.TrimSpace(rest[spaceIdx:])

				typ, desc := extractTypeDesc(rest)

				doc.Alias = &LuaDocAlias{Name: string(name), Type: string(typ), Desc: string(desc)}
			} else {
				doc.Alias = &LuaDocAlias{Name: string(rest)}
			}
		} else if after, ok := bytes.CutPrefix(line, tagGeneric); ok {
			activeTag = ""

			rest := bytes.TrimSpace(after)

			for gStr := range bytes.SplitSeq(rest, []byte(",")) {
				name, parent, _ := extractNameParent(gStr)
				if len(name) > 0 {
					doc.Generics = append(doc.Generics, LuaDocGeneric{Name: string(name), Parent: string(parent)})
				}
			}
		} else if after, ok := bytes.CutPrefix(line, tagOverload); ok {
			activeTag = ""

			doc.Overloads = append(doc.Overloads, string(bytes.TrimSpace(after)))
		} else if after, ok := bytes.CutPrefix(line, tagSee); ok {
			activeTag = ""

			doc.See = append(doc.See, string(bytes.TrimSpace(after)))
		} else {
			if len(line) > 0 {
				if line[0] != '@' { // Ignore @meta, @diagnostic, etc.
					switch activeTag {
					case "param":
						if doc.Params[len(doc.Params)-1].Desc != "" {
							doc.Params[len(doc.Params)-1].Desc += "\n" + string(line)
						} else {
							doc.Params[len(doc.Params)-1].Desc = string(line)
						}
					case "return":
						if doc.Returns[len(doc.Returns)-1].Desc != "" {
							doc.Returns[len(doc.Returns)-1].Desc += "\n" + string(line)
						} else {
							doc.Returns[len(doc.Returns)-1].Desc = string(line)
						}
					case "field":
						if doc.Fields[len(doc.Fields)-1].Desc != "" {
							doc.Fields[len(doc.Fields)-1].Desc += "\n" + string(line)
						} else {
							doc.Fields[len(doc.Fields)-1].Desc = string(line)
						}
					default:
						descLines = append(descLines, line)
					}
				} else {
					activeTag = ""

					descLines = append(descLines, line)
				}
			} else {
				activeTag = ""

				descLines = append(descLines, line) // Preserve empty lines for paragraph gaps
			}
		}
	}

	// Clean up leading/trailing empty description lines
	for len(descLines) > 0 && len(descLines[0]) == 0 {
		descLines = descLines[1:]
	}

	for len(descLines) > 0 && len(descLines[len(descLines)-1]) == 0 {
		descLines = descLines[:len(descLines)-1]
	}

	if len(descLines) > 0 {
		doc.Description = string(bytes.Join(descLines, []byte("\n")))
	}

	return doc
}
